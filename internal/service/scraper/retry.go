package scraper

import (
	"context"
	"fmt"
	"math"
	"math/rand/v2"
	"time"
)

const (
	retryMaxAttempts  = 3
	retryInitialDelay = 500 * time.Millisecond
	retryMaxDelay     = 5 * time.Second
	retryMultiplier   = 2.0
)

func retry(ctx context.Context, op string, fn func() error) error {
	var lastErr error

	for attempt := 0; attempt < retryMaxAttempts; attempt++ {
		if attempt > 0 {
			wait := calculateRetryWait(attempt - 1)

			timer := time.NewTimer(wait)
			select {
			case <-ctx.Done():
				timer.Stop()
				return fmt.Errorf("%s: %w", op, ctx.Err())
			case <-timer.C:
			}
		}

		if err := fn(); err != nil {
			lastErr = err
			continue
		}

		return nil
	}

	return fmt.Errorf("%s: max retry attempts exceeded: %w", op, lastErr)
}

func calculateRetryWait(attempt int) time.Duration {
	baseDelay := float64(retryInitialDelay) * math.Pow(retryMultiplier, float64(attempt))

	if baseDelay > float64(retryMaxDelay) {
		baseDelay = float64(retryMaxDelay)
	}

	delay := baseDelay/2 + (rand.Float64() * baseDelay / 2)

	return time.Duration(delay)
}
