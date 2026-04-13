// Package closer provides a package-level LIFO resource closer for graceful
// shutdown. Use Add to register closable functions during application startup,
// and CloseAll to close them all in reverse order with per-resource timeouts.
package closer

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"
)

// defaultResourceTimeout is the maximum time allowed for a single resource to
// close. If the parent context has a shorter deadline, it takes precedence.
const defaultResourceTimeout = 5 * time.Second

type closeFunc struct {
	name string
	fn   func(context.Context) error
}

type closer struct {
	mu     sync.Mutex
	once   sync.Once
	closed bool
	funcs  []closeFunc
}

var globalCloser = &closer{}

// Add registers a closable resource. Thread-safe.
func Add(name string, fn func(context.Context) error) {
	globalCloser.add(name, fn)
}

// CloseAll closes all registered resources in LIFO order.
//
// Shutdown model: best-effort — the loop always continues, even if the context
// is cancelled. Each resource gets its own per-resource timeout so that one stuck
// resource cannot block the rest.
//
// Returns errors.Join of all close failures (raw errors, no wrapping).
func CloseAll(ctx context.Context, log *slog.Logger) error {
	return globalCloser.closeAll(ctx, log)
}

func (c *closer) add(name string, fn func(context.Context) error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return
	}

	c.funcs = append(c.funcs, closeFunc{name: name, fn: fn})
}

type closeResult struct {
	Name     string
	Err      error
	Elapsed  time.Duration
	TimedOut bool
	Canceled bool
}

func (c *closer) closeAll(ctx context.Context, log *slog.Logger) error {
	var results []closeResult

	c.once.Do(func() {
		c.mu.Lock()
		funcs := make([]closeFunc, len(c.funcs))
		copy(funcs, c.funcs)
		c.closed = true
		c.mu.Unlock()

		// LIFO order — always attempt every resource.
		for i := len(funcs) - 1; i >= 0; i-- {
			results = append(results, closeResource(ctx, funcs[i]))
		}

		// Log each resource individually (searchable by name).
		for _, r := range results {
			logCloseResult(log, r)
		}

		// Summary counters.
		var okCount, failCount, timeoutCount, cancelCount int
		for _, r := range results {
			switch {
			case r.TimedOut:
				timeoutCount++
			case r.Canceled:
				cancelCount++
			case r.Err != nil:
				failCount++
			default:
				okCount++
			}
		}
		log.Info("closer_shutdown_summary",
			"total", len(results),
			"ok", okCount,
			"failed", failCount,
			"timed_out", timeoutCount,
			"canceled", cancelCount,
		)
	})

	// Aggregate raw errors — no wrapping, names are in the logs.
	var errs []error
	for _, r := range results {
		if r.Err != nil {
			errs = append(errs, r.Err)
		}
	}
	return errors.Join(errs...)
}

// closeResource closes a single resource with a per-resource timeout.
//
// A goroutine wrapper is used to ensure the timeout is enforced even if the
// resource function ignores ctx.Done() and blocks indefinitely.
func closeResource(parentCtx context.Context, fn closeFunc) closeResult {
	start := time.Now()

	ctx, cancel := context.WithTimeout(parentCtx, defaultResourceTimeout)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- fn.fn(ctx)
	}()

	var err error
	select {
	case err = <-done:
	case <-ctx.Done():
		err = ctx.Err()
	}

	elapsed := time.Since(start)

	ctxErr := ctx.Err()
	return closeResult{
		Name:     fn.name,
		Err:      err,
		Elapsed:  elapsed,
		TimedOut: errors.Is(ctxErr, context.DeadlineExceeded),
		Canceled: errors.Is(ctxErr, context.Canceled),
	}
}

func logCloseResult(log *slog.Logger, r closeResult) {
	if r.TimedOut {
		log.Warn("closer_resource_timed_out",
			"name", r.Name,
			"elapsed_ms", r.Elapsed.Milliseconds(),
		)
	} else if r.Canceled {
		log.Info("closer_resource_canceled",
			"name", r.Name,
			"elapsed_ms", r.Elapsed.Milliseconds(),
		)
	} else if r.Err != nil {
		log.Error("closer_resource_close_failed",
			"name", r.Name,
			"elapsed_ms", r.Elapsed.Milliseconds(),
			"error", r.Err.Error(),
		)
	} else {
		log.Debug("closer_resource_closed",
			"name", r.Name,
			"elapsed_ms", r.Elapsed.Milliseconds(),
		)
	}
}
