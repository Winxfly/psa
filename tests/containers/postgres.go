package containers

import (
	"context"
	"fmt"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	postgresImage = "postgres:16.2"
)

type PostgresContainer struct {
	Container *postgres.PostgresContainer
	DSN       string
	Host      string
	Port      string
}

func StartPostgres(ctx context.Context) (*PostgresContainer, error) {
	container, err := postgres.Run(
		ctx,
		postgresImage,
		postgres.WithDatabase("test"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("start postgres container: %w", err)
	}

	host, err := container.Host(ctx)
	if err != nil {
		return nil, fmt.Errorf("get host: %w", err)
	}

	port, err := container.MappedPort(ctx, "5432/tcp")
	if err != nil {
		_ = container.Terminate(ctx)
		return nil, fmt.Errorf("get port: %w", err)
	}

	dsn := fmt.Sprintf(
		"postgres://test:test@%s:%s/test?sslmode=disable",
		host,
		port.Port(),
	)

	return &PostgresContainer{
		Container: container,
		DSN:       dsn,
		Host:      host,
		Port:      port.Port(),
	}, nil
}
