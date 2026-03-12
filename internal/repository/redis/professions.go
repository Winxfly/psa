package redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/redis/go-redis/v9"

	"psa/internal/domain"
)

func (c *Cache) SaveProfessionsList(ctx context.Context, professions []domain.ActiveProfession) error {
	const op = "internal.repository.redis.professions.SaveProfessionsList"

	jsonData, err := json.Marshal(professions)
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	// ttl = 24h -> c.ttl/2 = 12h
	return c.client.Set(ctx, ProfessionListKey, jsonData, c.ttl/2).Err()
}

func (c *Cache) GetProfessionsList(ctx context.Context) ([]domain.ActiveProfession, error) {
	const op = "internal.repository.redis.professions.GetProfessionsList"

	data, err := c.client.Get(ctx, ProfessionListKey).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, nil
		}

		return nil, fmt.Errorf("%s: %w", op, err)
	}

	var professions []domain.ActiveProfession
	if err = json.Unmarshal(data, &professions); err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	return professions, nil
}
