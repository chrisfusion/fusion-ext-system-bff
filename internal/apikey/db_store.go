package apikey

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DBStore resolves API keys from a PostgreSQL table.
type DBStore struct {
	pool *pgxpool.Pool
}

func NewDBStore(pool *pgxpool.Pool) *DBStore {
	return &DBStore{pool: pool}
}

func (s *DBStore) Lookup(ctx context.Context, keyHash string) (string, string, error) {
	var systemID, systemName string
	err := s.pool.QueryRow(ctx,
		`SELECT system_id, system_name FROM system_api_keys WHERE hashed_key = $1 AND active = TRUE`,
		keyHash,
	).Scan(&systemID, &systemName)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", "", ErrNotFound
		}
		return "", "", err
	}
	return systemID, systemName, nil
}
