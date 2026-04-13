package health

import (
	"context"
	"fmt"
)

type CachePinger interface {
	Ping(ctx context.Context) error
}

type cacheCheck struct {
	cache CachePinger
}

func NewCacheCheck(cache CachePinger) *cacheCheck {
	return &cacheCheck{cache: cache}
}

func (c *cacheCheck) Name() string {
	return "cache"
}

func (c *cacheCheck) Check(ctx context.Context) error {
	if err := c.cache.Ping(ctx); err != nil {
		return fmt.Errorf("cache ping failed: %w", err)
	}
	return nil
}
