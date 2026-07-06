// Package disksnap — celery snapshot_project_disk: reliz qilingan har loyihaning
// disk hajmi (fayllar + CMS) snapshot'ini yozadi (monitoring tarix charti).
package disksnap

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
	"go.uber.org/zap"

	"github.com/BotSpace/worker-golang/internal/shared/database"
	"github.com/BotSpace/worker-golang/internal/shared/database/sqlcgen"
	"github.com/BotSpace/worker-golang/internal/shared/mongox"
)

type Usecase struct {
	Q     *sqlcgen.Queries
	Mongo *mongox.Client
	Log   *zap.Logger
}

func (u *Usecase) Run(ctx context.Context) {
	projects, err := u.Q.ListReleasedProjectsForDisk(ctx)
	if err != nil {
		u.Log.Error("snapshot_project_disk: list", zap.Error(err))
		return
	}
	count := 0
	for _, project := range projects {
		projectID := uuidFromPG(project.ID)
		filesBytes, err := u.Q.SumProjectFileBytes(ctx, project.ID)
		if err != nil {
			u.Log.Warn("snapshot: files bytes", zap.String("project", projectID), zap.Error(err))
			continue
		}

		cmsBytes, err := u.Mongo.TenantDataSize(ctx, "botspace_cms", mongoTenant(project))
		if err != nil {
			cmsBytes = 0 // db bo'lmasligi normal (CMS ishlatmagan loyiha)
		}
		err = u.Q.InsertDiskSnapshot(ctx, sqlcgen.InsertDiskSnapshotParams{
			ProjectID:      project.ID,
			DiskBytes:      filesBytes + cmsBytes,
			DiskFilesBytes: filesBytes,
			DiskCmsBytes:   cmsBytes,
		})
		if err != nil {
			u.Log.Warn("snapshot: insert", zap.String("project", projectID), zap.Error(err))
			continue
		}
		count++
	}
	u.Log.Info("snapshot_project_disk done", zap.Int("wrote", count))
}

func uuidFromPG(id pgtype.UUID) string {
	return database.UUIDString(id)
}

func mongoTenant(project sqlcgen.ListReleasedProjectsForDiskRow) any {
	if project.LegacyID != nil {
		return int64(*project.LegacyID)
	}
	return uuidFromPG(project.ID)
}
