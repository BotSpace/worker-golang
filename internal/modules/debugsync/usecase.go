// Package debugsync — celery sync_debug: 30 daqiqadan oshgan RUNNING debug
// botlarni k8s'dan o'chirib, DB'da "stopping" qiladi (template loyihalar bundan
// mustasno — ular debug-demo sifatida doim ishlaydi).
package debugsync

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"go.uber.org/zap"

	"github.com/BotSpace/worker-golang/internal/shared/database/sqlcgen"
	"github.com/BotSpace/worker-golang/internal/shared/k8sx"
)

const debugLifetime = 30 * time.Minute

type Usecase struct {
	Q   *sqlcgen.Queries
	K8s *k8sx.Client
	Log *zap.Logger
}

func (u *Usecase) Run(ctx context.Context) {
	cutoff := pgtype.Timestamptz{Time: time.Now().Add(-debugLifetime), Valid: true}
	rows, err := u.Q.ListExpiredRunningDebugs(ctx, cutoff)
	if err != nil {
		u.Log.Error("sync_debug: list", zap.Error(err))
		return
	}
	for _, row := range rows {
		if row.ServiceID == nil || *row.ServiceID == "" {
			continue
		}
		if err := u.K8s.RemoveDeployment(ctx, *row.ServiceID); err != nil {
			u.Log.Error("sync_debug: remove", zap.String("service", *row.ServiceID), zap.Error(err))
			continue
		}
		if err := u.Q.MarkDebugStopping(ctx, row.ID); err != nil {
			u.Log.Error("sync_debug: mark stopping", zap.Int64("id", row.ID), zap.Error(err))
		}
	}
	if len(rows) > 0 {
		u.Log.Info("sync_debug done", zap.Int("stopped", len(rows)))
	}
}
