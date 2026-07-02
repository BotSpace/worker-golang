// Package chatsend — celery send_telegram_message: CRM chat'dan foydalanuvchiga
// javob yuborish (matn yoki media). Celery'dagi 3x retry backoff bilan.
package chatsend

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"go.uber.org/zap"

	"github.com/BotSpace/worker-golang/internal/shared/database/sqlcgen"
	"github.com/BotSpace/worker-golang/internal/shared/storagex"
	"github.com/BotSpace/worker-golang/internal/shared/telegramx"
)

type Args struct {
	ChatID      int64  `json:"chat_id"`
	MessageType string `json:"message_type"`
	ProjectID   int64  `json:"project_id"`
	Text        string `json:"text"`
	FileID      *int64 `json:"file_id"` // FileModel pk (Telegram file_id EMAS)
}

type Usecase struct {
	Q       *sqlcgen.Queries
	Storage *storagex.Storage
	Log     *zap.Logger
}

var mediaMethods = map[string][2]string{
	"photo":    {"sendPhoto", "photo"},
	"video":    {"sendVideo", "video"},
	"audio":    {"sendAudio", "audio"},
	"document": {"sendDocument", "document"},
	"file":     {"sendDocument", "document"},
}

func (u *Usecase) Handle(ctx context.Context, raw json.RawMessage) error {
	var args Args
	if err := json.Unmarshal(raw, &args); err != nil {
		return err
	}

	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(1<<attempt) * time.Second) // 2s, 4s — celery countdown=2**retries
		}
		if lastErr = u.send(ctx, args); lastErr == nil {
			return nil
		}
		u.Log.Warn("chatsend: attempt failed", zap.Int("attempt", attempt+1), zap.Error(lastErr))
	}
	u.Log.Error("chatsend: gave up", zap.Int64("chat", args.ChatID), zap.Error(lastErr))
	return nil // retry tugadi — stream'da pending qoldirmaymiz
}

func (u *Usecase) send(ctx context.Context, args Args) error {
	token, err := u.Q.GetProjectEnv(ctx, sqlcgen.GetProjectEnvParams{ProjectID: args.ProjectID, Key: "RELEASE_BOT_TOKEN"})
	if err != nil {
		token, err = u.Q.GetProjectEnv(ctx, sqlcgen.GetProjectEnvParams{ProjectID: args.ProjectID, Key: "BOT_TOKEN"})
		if err != nil {
			return fmt.Errorf("bot token yo'q (project=%d)", args.ProjectID)
		}
	}
	tg := telegramx.New(token)

	m, ok := mediaMethods[args.MessageType]
	if !ok {
		_, err := tg.SendMessage(ctx, args.ChatID, orDefault(args.Text, "No message"), "HTML", nil)
		return err
	}

	if args.FileID == nil {
		return fmt.Errorf("media uchun file_id yo'q")
	}
	path, err := u.Q.GetFilePath(ctx, *args.FileID)
	if err != nil {
		return fmt.Errorf("file topilmadi pk=%d: %w", *args.FileID, err)
	}
	content, err := u.Storage.Read(ctx, path)
	if err != nil {
		return fmt.Errorf("storage read: %w", err)
	}
	_, err = tg.SendMediaUpload(ctx, m[0], m[1], args.ChatID, path, content, args.Text, nil)
	return err
}

func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}
