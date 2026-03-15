package containers

import (
	"context"
	"fmt"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/redis"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	redisImage = "redis:8.2-alpine"
)

type RedisContainer struct {
	Container *redis.RedisContainer
	Addr      string
}

func StartRedis(ctx context.Context) (*RedisContainer, error) {
	container, err := redis.Run(
		ctx,
		redisImage,
		testcontainers.WithWaitStrategy(
			wait.ForLog("Ready to accept connections").
				WithOccurrence(1).
				WithStartupTimeout(30*time.Second),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("start redis container: %w", err)
	}

	host, err := container.Host(ctx)
	if err != nil {
		_ = container.Terminate(ctx)
		return nil, fmt.Errorf("get host: %w", err)
	}

	port, err := container.MappedPort(ctx, "6379/tcp")
	if err != nil {
		_ = container.Terminate(ctx)
		return nil, fmt.Errorf("get port: %w", err)
	}

	return &RedisContainer{
		Container: container,
		Addr:      fmt.Sprintf("%s:%s", host, port.Port()),
	}, nil
}
