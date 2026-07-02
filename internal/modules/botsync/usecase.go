// Package botsync — celery sync_bot: har loyiha botining username va profil
// rasmini Telegram'dan sinxronlaydi (soatiga bir marta).
package botsync

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"

	"github.com/BotSpace/worker-golang/internal/shared/database/sqlcgen"
	"github.com/BotSpace/worker-golang/internal/shared/storagex"
	"github.com/BotSpace/worker-golang/internal/shared/telegramx"
)

type Usecase struct {
	Q       *sqlcgen.Queries
	Storage *storagex.Storage
	Log     *zap.Logger
}

func (u *Usecase) Run(ctx context.Context) {
	rows, err := u.Q.ListProjectsWithReleaseToken(ctx)
	if err != nil {
		u.Log.Error("sync_bot: list", zap.Error(err))
		return
	}
	for _, row := range rows {
		// Telegram rate-limit'ga hurmat (backend'dagi 0.5s bilan bir xil).
		time.Sleep(500 * time.Millisecond)
		if err := u.syncOne(ctx, row); err != nil {
			u.Log.Warn("sync_bot: project failed", zap.Int64("project", row.ID), zap.Error(err))
		}
	}
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
