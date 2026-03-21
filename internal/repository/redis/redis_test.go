//go:build integration

package redis

import "github.com/redis/go-redis/v9"

// ClientTest returns the underlying redis client for testing purposes.
// This method is only available in integration tests.
func (c *Cache) ClientTest() *redis.Client {
	return c.client
}
