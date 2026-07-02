// Package broadcast — celery send_message: broadcast xabarini loyihaning barcha
// foydalanuvchilariga (legacy Mongo users kolleksiyasi) yuboradi. file_id kesh,
// rate-limit va statistika backend'dagi bilan bir xil.
package broadcast

import (
	"context"
	"encoding/json"
	"time"

	"go.uber.org/zap"

	"github.com/BotSpace/worker-golang/internal/shared/database/sqlcgen"
	"github.com/BotSpace/worker-golang/internal/shared/mongox"
	"github.com/BotSpace/worker-golang/internal/shared/storagex"
	"github.com/BotSpace/worker-golang/internal/shared/telegramx"
)

type Args struct {
	MessageID int64 `json:"message_id"`
}

// Django MessageModel.buttons: 2D massiv [{title, link?, callback_data?,
// request_contact?, request_location?}].
type button struct {
	Title           string `json:"title"`
	Link            string `json:"link"`
	CallbackData    string `json:"callback_data"`
	RequestContact  bool   `json:"request_contact"`
	RequestLocation bool   `json:"request_location"`
}

type Usecase struct {
	Q       *sqlcgen.Queries
	Mongo   *mongox.Client
	Storage *storagex.Storage
	Log     *zap.Logger
}

func (u *Usecase) Handle(ctx context.Context, raw json.RawMessage) error {
	var args Args
	if err := json.Unmarshal(raw, &args); err != nil {
		return err
	}
	msg, err := u.Q.GetMessage(ctx, args.MessageID)
	if err != nil {
		u.Log.Error("broadcast: message topilmadi", zap.Int64("id", args.MessageID), zap.Error(err))
		return nil // qayta urinish befoyda
	}

	fail := func() {
		_ = u.Q.MarkMessageFailed(ctx, msg.ID)
	}

	hasUsers, err := u.Q.HasUsersCollection(ctx, msg.ProjectID)
	if err != nil || !hasUsers {
		u.Log.Error("broadcast: users kolleksiyasi yo'q", zap.Int64("project", msg.ProjectID))
		fail()
		return nil
	}

	token, err := u.Q.GetProjectEnv(ctx, sqlcgen.GetProjectEnvParams{ProjectID: msg.ProjectID, Key: "RELEASE_BOT_TOKEN"})
	if err != nil {
		// backend release_bot_token fallback: BOT_TOKEN (debug)
		token, err = u.Q.GetProjectEnv(ctx, sqlcgen.GetProjectEnvParams{ProjectID: msg.ProjectID, Key: "BOT_TOKEN"})
		if err != nil {
			u.Log.Error("broadcast: bot token yo'q", zap.Int64("project", msg.ProjectID))
			fail()
			return nil
		}
	}
	tg := telegramx.New(token)

	users, err := u.Mongo.ProjectUsers(ctx, msg.ProjectID)
	if err != nil {
		u.Log.Error("broadcast: mongo users", zap.Int64("project", msg.ProjectID), zap.Error(err))
		fail()
		return nil
	}

	total := int64(len(users))
	_ = u.Q.UpdateMessageTotal(ctx, sqlcgen.UpdateMessageTotalParams{ID: msg.ID, Total: total})

	kb := buildKeyboard(msg.ButtonType, msg.Buttons)
	var cachedFileID string
	if msg.FileID != nil {
		cachedFileID = *msg.FileID
	}

	var done, failed int64
	for _, user := range users {
		chatID, ok := chatIDOf(user)
		if !ok {
			failed++
			continue
		}
		sent, newFileID, err := u.sendOne(ctx, tg, msg, chatID, cachedFileID, kb)
		if err != nil {
			failed++
			u.Log.Debug("broadcast: send failed", zap.Int64("chat", chatID), zap.Error(err))
			time.Sleep(20 * time.Millisecond)
			continue
		}
		if !sent {
			failed++
			continue
		}
		if newFileID != "" && cachedFileID == "" {
			cachedFileID = newFileID
			_ = u.Q.UpdateMessageFileID(ctx, sqlcgen.UpdateMessageFileIDParams{ID: msg.ID, FileID: &cachedFileID})
		}
		done++
		time.Sleep(50 * time.Millisecond) // ~20 msg/s — Telegram limitidan pastda
	}

	status := "progress"
	switch {
	case done == total:
		status = "done"
	case failed == total:
		status = "failed"
	}
	_ = u.Q.UpdateMessageResult(ctx, sqlcgen.UpdateMessageResultParams{ID: msg.ID, Done: done, Fail: failed, Status: status})
	u.Log.Info("broadcast done", zap.Int64("message", msg.ID),
		zap.Int64("done", done), zap.Int64("fail", failed), zap.Int64("total", total))
	return nil
}

// sendOne — bitta userga yuborish. Media bo'lsa: file_id kesh → tez; birinchi
// yuborishda S3'dan yuklab upload qilinadi va file_id qaytadi.
func (u *Usecase) sendOne(ctx context.Context, tg *telegramx.Client, msg sqlcgen.Message, chatID int64, cachedFileID string, kb *telegramx.Keyboard) (sent bool, newFileID string, err error) {
	switch msg.MessageType {
	case "photo", "video":
		method, field := "sendPhoto", "photo"
		if msg.MessageType == "video" {
			method, field = "sendVideo", "video"
		}
		if cachedFileID != "" {
			_, err = tg.SendMediaByFileID(ctx, method, field, chatID, cachedFileID, msg.Caption, kb)
			return err == nil, "", err
		}
		if msg.File == nil || *msg.File == "" {
			return false, "", nil
		}
		content, rerr := u.Storage.Read(ctx, *msg.File)
		if rerr != nil {
			return false, "", rerr
		}
		m, serr := tg.SendMediaUpload(ctx, method, field, chatID, *msg.File, content, msg.Caption, kb)
		if serr != nil {
			return false, "", serr
		}
		if msg.MessageType == "photo" && len(m.Photo) > 0 {
			newFileID = m.Photo[len(m.Photo)-1].FileID
		} else if m.Video != nil {
			newFileID = m.Video.FileID
		}
		return true, newFileID, nil
	default:
		_, err = tg.SendMessage(ctx, chatID, msg.Caption, "", kb)
		return err == nil, "", err
	}
}

func buildKeyboard(buttonType string, rawButtons []byte) *telegramx.Keyboard {
	var rows [][]button
	if err := json.Unmarshal(rawButtons, &rows); err != nil || len(rows) == 0 {
		return nil
	}
	kb := &telegramx.Keyboard{}
	if buttonType == "inline" {
		for _, row := range rows {
			var line []telegramx.InlineButton
			for _, b := range row {
				switch {
				case b.Link != "":
					line = append(line, telegramx.InlineButton{Text: b.Title, URL: b.Link})
				case b.CallbackData != "":
					line = append(line, telegramx.InlineButton{Text: b.Title, CallbackData: b.CallbackData})
				}
			}
			if len(line) > 0 {
				kb.InlineKeyboard = append(kb.InlineKeyboard, line)
			}
		}
		if len(kb.InlineKeyboard) == 0 {
			return nil
		}
		return kb
	}
	kb.ResizeKeyboard = true
	for _, row := range rows {
		var line []telegramx.ReplyButton
		for _, b := range row {
			line = append(line, telegramx.ReplyButton{
				Text:            b.Title,
				RequestContact:  b.RequestContact,
				RequestLocation: b.RequestLocation,
			})
		}
		if len(line) > 0 {
			kb.Keyboard = append(kb.Keyboard, line)
		}
	}
	if len(kb.Keyboard) == 0 {
		return nil
	}
	return kb
}

// chatIDOf — mongo hujjatidagi chat_id (int32/int64/float64 bo'lishi mumkin).
func chatIDOf(doc map[string]any) (int64, bool) {
	switch v := doc["chat_id"].(type) {
	case int32:
		return int64(v), true
	case int64:
		return v, true
	case float64:
		return int64(v), true
	default:
		return 0, false
	}
}
