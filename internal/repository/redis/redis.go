package redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"psa/internal/config"
	"psa/internal/entity"
	"time"
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

func (c *Cache) SaveProfessionData(ctx context.Context, data *entity.ProfessionDetail) error {
	const op = "internal.repository.redis.SaveProfessionData"

	key := fmt.Sprintf("profession:%s:skills", data.ProfessionID.String())

	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	return c.client.Set(ctx, key, jsonData, c.ttl).Err()
}

func (c *Cache) GetProfessionData(ctx context.Context, professionID uuid.UUID) (*entity.ProfessionDetail, error) {
	const op = "internal.repository.redis.GetProfessionData"

	key := fmt.Sprintf("profession:%s:skills", professionID.String())

	data, err := c.client.Get(ctx, key).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, nil
		}

		return nil, fmt.Errorf("%s: %w", op, err)
	}

	var skills entity.ProfessionDetail
	if err := json.Unmarshal(data, &skills); err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	return &skills, nil
}
