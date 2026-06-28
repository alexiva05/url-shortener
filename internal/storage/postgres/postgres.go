package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/alexiva05/url-shortener/internal/storage"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Storage struct {
	db *pgxpool.Pool
}

func NewStorage(ctx context.Context, db *pgxpool.Pool) (*Storage, error) {
	_, err := db.Exec(ctx, `
CREATE TABLE IF NOT EXISTS urls (
    id        BIGSERIAL PRIMARY KEY,
    alias     TEXT NOT NULL UNIQUE,
    url       TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS urls_alias ON urls(alias);
`)
	if err != nil {
		return nil, fmt.Errorf("Storage - NewStorage: %w", err)
	}

	return &Storage{
		db: db,
	}, nil
}

func (s *Storage) SaveURL(ctx context.Context, urlToSave string, alias string) (int64, error) {
	q := `
INSERT INTO urls (alias, url) VALUES ($1, $2)
RETURNING id;
`
	var id int64
	var pgErr *pgconn.PgError

	err := s.db.QueryRow(ctx, q, urlToSave, alias).Scan(&id)
	if err != nil {
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return 0, fmt.Errorf("URL %s already exists: %w", urlToSave, storage.ErrURLExists)
		}
		return 0, fmt.Errorf("Storage - SaveURL: %w", err)
	}

	return id, nil
}
