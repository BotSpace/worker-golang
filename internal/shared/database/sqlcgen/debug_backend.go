package sqlcgen

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
)

type ExpiredDebug struct {
	ID        pgtype.UUID
	ServiceID *string
}

func (q *Queries) ListExpiredRunningDebugsBackend(ctx context.Context, cutoff pgtype.Timestamptz) ([]ExpiredDebug, error) {
	const query = `
SELECT d.id, d.service_id
FROM debug d
JOIN projects p ON p.id = d.project_id
WHERE d.status = 'running'
  AND d.updated_at <= $1
  AND p.is_template = false
  AND p.deleted_at IS NULL
`
	rows, err := q.db.Query(ctx, query, cutoff)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []ExpiredDebug
	for rows.Next() {
		var item ExpiredDebug
		if err := rows.Scan(&item.ID, &item.ServiceID); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (q *Queries) MarkDebugStoppingBackend(ctx context.Context, id pgtype.UUID) error {
	const query = `UPDATE debug SET status = 'stopping', updated_at = now() WHERE id = $1`
	_, err := q.db.Exec(ctx, query, id)
	return err
}
