package redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"psa/internal/domain"
)

func (c *Cache) SaveProfessionTrend(ctx context.Context, professionID uuid.UUID, trend *domain.ProfessionTrend) error {
	const op = "internal.repository.redis.trend.SaveProfessionTrend"

	key := fmt.Sprintf(ProfessionTrendKeyPrefix, professionID.String())

	jsonData, err := json.Marshal(trend)
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	return c.client.Set(ctx, key, jsonData, c.ttl/6).Err()
}

func (c *Cache) GetProfessionTrend(ctx context.Context, professionID uuid.UUID) (*domain.ProfessionTrend, error) {
	const op = "internal.repository.redis.trend.GetProfessionTrend"

	key := fmt.Sprintf(ProfessionTrendKeyPrefix, professionID.String())

	data, err := c.client.Get(ctx, key).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, nil
		}

		return nil, fmt.Errorf("%s: %w", op, err)
	}

	var trend domain.ProfessionTrend
	if err = json.Unmarshal(data, &trend); err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	return &trend, nil
}
