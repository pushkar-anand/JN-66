// Package store provides typed data access methods over the sqlc-generated layer.
package store

//go:generate go tool mockgen -source=../sqlc/querier.go -destination=mock_querier_test.go -package=store

import "github.com/jackc/pgx/v5/pgxpool"

// DB is a shared handle all stores embed.
type DB struct {
	pool *pgxpool.Pool
}

// newDB wraps a pool in a DB.
func newDB(pool *pgxpool.Pool) DB { return DB{pool: pool} }
