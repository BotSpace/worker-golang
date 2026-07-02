// Package releases — celery cleanup_failed_releases: CrashLoop holatidagi
// release deployment'larni k8s'dan o'chiradi, DB'da relase.status=failed qiladi.
package releases

import (
	"context"

	"go.uber.org/zap"

	"github.com/BotSpace/worker-golang/internal/shared/database/sqlcgen"
	"github.com/BotSpace/worker-golang/internal/shared/k8sx"
)

type Usecase struct {
	Q   *sqlcgen.Queries
	K8s *k8sx.Client
	Log *zap.Logger
}

func (u *Usecase) Run(ctx context.Context) {
	pods, err := u.K8s.EnginePods(ctx)
	if err != nil {
		u.Log.Error("cleanup_failed_releases: list pods", zap.Error(err))
		return
	}

	processed := map[string]bool{}
	cleaned := 0
	for _, pod := range pods {
		name := k8sx.ReleaseEngineName(pod)
		if name == "" || processed[name] || !k8sx.IsCrashlooping(pod) {
			continue
		}
		processed[name] = true
		u.Log.Warn("crashloop detected", zap.String("engine", name), zap.String("pod", pod.Name))

		if err := u.K8s.RemoveDeployment(ctx, name); err != nil {
			u.Log.Error("cleanup_failed_releases: remove", zap.String("engine", name), zap.Error(err))
			// Davom etamiz — DB holatini baribir yangilaymiz (backend bilan bir xil).
		}
		rows, err := u.Q.MarkReleaseFailedByService(ctx, &name)
		if err != nil {
			u.Log.Error("cleanup_failed_releases: db", zap.String("engine", name), zap.Error(err))
			continue
		}
		u.Log.Info("release marked failed", zap.String("engine", name), zap.Int64("rows", rows))
		cleaned++
	}
	if cleaned > 0 {
		u.Log.Info("cleanup_failed_releases done", zap.Int("cleaned", cleaned))
	}
}
