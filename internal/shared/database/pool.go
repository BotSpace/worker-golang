package database

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/BotSpace/worker-golang/internal/shared/database/sqlcgen"
)

func New(ctx context.Context, url string) (*pgxpool.Pool, *sqlcgen.Queries, error) {
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, nil, err
	}
	return pool, sqlcgen.New(pool), nil
}
