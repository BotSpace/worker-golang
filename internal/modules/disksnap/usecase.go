// Package disksnap — celery snapshot_project_disk: reliz qilingan har loyihaning
// disk hajmi (fayllar + CMS) snapshot'ini yozadi (monitoring tarix charti).
package disksnap

import (
	"context"

	"go.uber.org/zap"

	"github.com/BotSpace/worker-golang/internal/shared/database/sqlcgen"
	"github.com/BotSpace/worker-golang/internal/shared/mongox"
)

type Usecase struct {
	Q     *sqlcgen.Queries
	Mongo *mongox.Client
	Log   *zap.Logger
}

func (u *Usecase) Run(ctx context.Context) {
	ids, err := u.Q.ListReleasedProjectIDs(ctx)
	if err != nil {
		u.Log.Error("snapshot_project_disk: list", zap.Error(err))
		return
	}
	count := 0
	for _, pid := range ids {
		filesBytes, err := u.Q.SumProjectFileBytes(ctx, pid)
		if err != nil {
			u.Log.Warn("snapshot: files bytes", zap.Int64("project", pid), zap.Error(err))
			continue
		}
		// ponytail: backend JSON-taxmin o'rniga Mongo dbStats.dataSize — aniqroq va bir chaqiruv.
		cmsBytes, err := u.Mongo.ProjectDataSize(ctx, pid)
		if err != nil {
			cmsBytes = 0 // db bo'lmasligi normal (CMS ishlatmagan loyiha)
		}
		err = u.Q.InsertDiskSnapshot(ctx, sqlcgen.InsertDiskSnapshotParams{
			ProjectID:      pid,
			DiskBytes:      filesBytes + cmsBytes,
			DiskFilesBytes: filesBytes,
			DiskCmsBytes:   cmsBytes,
		})
		if err != nil {
			u.Log.Warn("snapshot: insert", zap.Int64("project", pid), zap.Error(err))
			continue
		}
		count++
	}
	u.Log.Info("snapshot_project_disk done", zap.Int("wrote", count))
}
