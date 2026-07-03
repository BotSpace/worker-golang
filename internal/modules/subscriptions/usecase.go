// Package subscriptions — backend-golang's user_subscription janitor: this
// is net-new Go-side logic, not a Django port (Django never actually
// transitioned UserSubscription out of "active" — status/expires_at existed
// but nothing moved it on expiry). Moved here from backend-golang's
// in-process scheduler (internal/server/scheduler.go) — periodic jobs belong
// in the worker, not the API process.
package subscriptions

import (
	"context"

	"go.uber.org/zap"

	"github.com/BotSpace/worker-golang/internal/shared/database/sqlcgen"
)

type Usecase struct {
	Q   *sqlcgen.Queries
	Log *zap.Logger
}

// Run moves active subscriptions past expires_at into grace_period, and
// grace_period subscriptions past their 3-day grace window into suspended.
func (u *Usecase) Run(ctx context.Context) {
	toGrace, err := u.Q.ExpireActiveSubscriptions(ctx)
	if err != nil {
		u.Log.Error("expire_subscriptions: active->grace_period", zap.Error(err))
		return
	}
	suspended, err := u.Q.SuspendGracePeriodSubscriptions(ctx)
	if err != nil {
		u.Log.Error("expire_subscriptions: grace_period->suspended", zap.Error(err))
		return
	}
	if len(toGrace) > 0 || len(suspended) > 0 {
		u.Log.Info("expire_subscriptions done",
			zap.Int("to_grace_period", len(toGrace)),
			zap.Int("suspended", len(suspended)))
	}
}
