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

func (c *Cache) SaveProfessionData(ctx context.Context, data *domain.ProfessionDetail) error {
	const op = "internal.repository.redis.skills.SaveProfessionData"

	key := fmt.Sprintf(ProfessionSkillsKeyPrefix, data.ProfessionID.String())

	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	return c.client.Set(ctx, key, jsonData, c.ttl).Err()
}

func (c *Cache) GetProfessionData(ctx context.Context, professionID uuid.UUID) (*domain.ProfessionDetail, error) {
	const op = "internal.repository.redis.skills.GetProfessionData"

	key := fmt.Sprintf(ProfessionSkillsKeyPrefix, professionID.String())

	data, err := c.client.Get(ctx, key).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, nil
		}

		return nil, fmt.Errorf("%s: %w", op, err)
	}

	var skills domain.ProfessionDetail
	if err := json.Unmarshal(data, &skills); err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	return &skills, nil
}
