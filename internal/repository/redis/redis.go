package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/redis/go-redis/v9/maintnotifications"

	"psa/internal/config"
)

const (
	ProfessionSkillsKeyPrefix = "profession:%s:skills"
	ProfessionTrendKeyPrefix  = "profession:%s:trend"
	ProfessionListKey         = "profession:list"
)

type Cache struct {
	client *redis.Client
	ttl    time.Duration
}

func New(cfg config.Redis) (*Cache, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
		MaintNotificationsConfig: &maintnotifications.Config{
			Mode: maintnotifications.ModeDisabled,
		},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to redis: %w", err)
	}

	return &Cache{
		client: client,
		ttl:    cfg.DefaultTTL,
	}, nil
}

func (c *Cache) Close() error {
	return c.client.Close()
}

func (c *Cache) Ping(ctx context.Context) error {
	return c.client.Ping(ctx).Err()
}
