// Package botsync — celery sync_bot: har loyiha botining username va profil
// rasmini Telegram'dan sinxronlaydi (soatiga bir marta).
//
// Token blacklist: bir marta Unauthorized/Not Found bergan token Redis'da
// belgilanadi va keyingi sikllarda O'TKAZIB YUBORILADI (Telegram'ga behuda
// so'rov ketmasin). Kalit token hash'idan yasaladi — foydalanuvchi tokenni
// YANGILASA hash boshqacha bo'ladi va yangi token avtomatik qayta tekshiriladi.
package botsync

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/BotSpace/worker-golang/internal/shared/database/sqlcgen"
	"github.com/BotSpace/worker-golang/internal/shared/storagex"
	"github.com/BotSpace/worker-golang/internal/shared/telegramx"
)

// badTokenTTL — blacklist muddati. Token o'zgarmasa ham vaqti-vaqti bilan
// qayta tekshirib ko'ramiz (masalan bot Telegram'da qayta faollashtirilgan bo'lsa).
const badTokenTTL = 7 * 24 * time.Hour

type Usecase struct {
	Q       *sqlcgen.Queries
	Storage *storagex.Storage
	Redis   *redis.Client
	Log     *zap.Logger
}

func badTokenKey(token string) string {
	h := sha256.Sum256([]byte(token))
	return "goworker:badtoken:" + hex.EncodeToString(h[:8])
}

// isAuthError — token bilan bog'liq DOIMIY xato (retry befoyda). Tarmoq/5xx
// xatolari blacklist'ga TUSHMAYDI.
func isAuthError(err error) bool {
	s := err.Error()
	return strings.Contains(s, "Unauthorized") || strings.Contains(s, "Not Found")
}

func (u *Usecase) Run(ctx context.Context) {
	rows, err := u.Q.ListProjectsWithReleaseToken(ctx)
	if err != nil {
		u.Log.Error("sync_bot: list", zap.Error(err))
		return
	}
	var synced, skipped, newBad int
	for _, row := range rows {
		if row.BotToken == "" {
			continue
		}
		key := badTokenKey(row.BotToken)
		if n, err := u.Redis.Exists(ctx, key).Result(); err == nil && n > 0 {
			skipped++
			continue
		}

		// Telegram rate-limit'ga hurmat (backend'dagi 0.5s bilan bir xil) —
		// faqat REAL so'rov qilinadigan loyihalar orasida.
		time.Sleep(500 * time.Millisecond)

		if err := u.syncOne(ctx, row); err != nil {
			if isAuthError(err) {
				// Doimiy xato — belgilab qo'yamiz, keyingi sikllarda urinmaymiz.
				if serr := u.Redis.Set(ctx, key, time.Now().Format(time.RFC3339), badTokenTTL).Err(); serr != nil {
					u.Log.Warn("sync_bot: blacklist yozilmadi", zap.Error(serr))
				}
				newBad++
				u.Log.Warn("sync_bot: token yaroqsiz, blacklist'ga qo'shildi",
					zap.Int64("project", row.ID), zap.Error(err))
			} else {
				u.Log.Warn("sync_bot: project failed", zap.Int64("project", row.ID), zap.Error(err))
			}
			continue
		}
		synced++
	}
	u.Log.Info("sync_bot done",
		zap.Int("synced", synced),
		zap.Int("skipped_bad_token", skipped),
		zap.Int("new_bad_token", newBad),
		zap.Int("total", len(rows)))
}

func (u *Usecase) syncOne(ctx context.Context, row sqlcgen.ListProjectsWithReleaseTokenRow) error {
	tg := telegramx.New(row.BotToken)
	me, err := tg.GetMe(ctx)
	if err != nil {
		return fmt.Errorf("getMe: %w", err)
	}

	username := row.Username
	avatar := row.Avatar
	avatarFileID := row.AvatarFileID
	changed := false

	if username == nil || *username != me.Username {
		username = &me.Username
		changed = true
	}

	fileID, err := tg.LatestProfilePhotoFileID(ctx, me.ID)
	if err != nil {
		return fmt.Errorf("profile photos: %w", err)
	}
	if fileID != "" && (avatarFileID == nil || *avatarFileID != fileID) {
		content, err := tg.DownloadFile(ctx, fileID)
		if err != nil {
			return fmt.Errorf("download avatar: %w", err)
		}
		name := fmt.Sprintf("avatar/%d.jpg", me.ID)
		if err := u.Storage.Write(ctx, name, content, "image/jpeg"); err != nil {
			return fmt.Errorf("save avatar: %w", err)
		}
		avatar = &name
		avatarFileID = &fileID
		changed = true
		u.Log.Info("bot avatar updated", zap.Int64("project", row.ID), zap.String("avatar", name))
	}

	if !changed {
		return nil
	}
	return u.Q.UpdateProjectBotInfo(ctx, sqlcgen.UpdateProjectBotInfoParams{
		ID:           row.ID,
		Username:     username,
		Avatar:       avatar,
		AvatarFileID: avatarFileID,
	})
}
