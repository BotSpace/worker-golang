package sqlcgen

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
)

type DiskProject struct {
	ID       pgtype.UUID
	LegacyID pgtype.Int4
}

func (q *Queries) ListReleasedProjectsForDisk(ctx context.Context) ([]DiskProject, error) {
	const query = `
SELECT DISTINCT p.id, p.legacy_id
FROM projects p
LEFT JOIN relase r ON r.project_id = p.id
LEFT JOIN debug d ON d.project_id = p.id
WHERE p.deleted_at IS NULL
  AND (r.id IS NOT NULL OR d.id IS NOT NULL)
`
	rows, err := q.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []DiskProject
	for rows.Next() {
		var item DiskProject
		if err := rows.Scan(&item.ID, &item.LegacyID); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (q *Queries) SumProjectFileBytesForDisk(ctx context.Context, projectID pgtype.UUID) (int64, error) {
	const query = `SELECT COALESCE(SUM(filesize), 0)::bigint FROM file WHERE project_id = $1`
	row := q.db.QueryRow(ctx, query, projectID)
	var total int64
	err := row.Scan(&total)
	return total, err
}

type InsertProjectResourceSnapshotParams struct {
	ProjectID      pgtype.UUID
	DiskBytes      int64
	DiskFilesBytes int64
	DiskCmsBytes   int64
}

func (q *Queries) InsertProjectResourceSnapshot(ctx context.Context, arg InsertProjectResourceSnapshotParams) error {
	const query = `
INSERT INTO project_resource_snapshot (project_id, disk_bytes, disk_files_bytes, disk_cms_bytes, created_at)
VALUES ($1, $2, $3, $4, now())
`
	_, err := q.db.Exec(ctx, query,
		arg.ProjectID,
		arg.DiskBytes,
		arg.DiskFilesBytes,
		arg.DiskCmsBytes,
	)
	return err
}
