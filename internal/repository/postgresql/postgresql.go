package postgresql

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	"psa/internal/config"
	postgresql "psa/internal/repository/postgresql/generated"
)

type Storage struct {
	Pool    *pgxpool.Pool
	Queries *postgresql.Queries
}

func New(cfg config.StoragePath) (*Storage, error) {
	const op = "repository.postgresql.New"

	connStr := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s",
		cfg.Username, cfg.Password, cfg.Host,
		cfg.Port, cfg.Database, cfg.SSLMode)

	pool, err := pgxpool.New(context.Background(), connStr)
	if err != nil {
		return nil, fmt.Errorf("%s: %w:", op, err)
	}

	err = pool.Ping(context.Background())
	if err != nil {
		return nil, fmt.Errorf("%s: %w:", op, err)
	}

	queries := postgresql.New(pool)

	return &Storage{Pool: pool,
		Queries: queries,
	}, nil
}

func (s *Storage) Close() {
	if s.Pool != nil {
		s.Pool.Close()
	}
}
