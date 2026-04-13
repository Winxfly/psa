package closer

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func newLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
}

// newCloser creates a fresh closer instance for isolated testing.
func newCloser() *closer {
	return &closer{}
}

// --- Add before CloseAll ---

func TestAddBeforeCloseAll(t *testing.T) {
	t.Parallel()
	log := newLogger()
	c := newCloser()

	var mu sync.Mutex
	var closed []string

	c.add("res1", func(ctx context.Context) error {
		mu.Lock()
		closed = append(closed, "res1")
		mu.Unlock()
		return nil
	})
	c.add("res2", func(ctx context.Context) error {
		mu.Lock()
		closed = append(closed, "res2")
		mu.Unlock()
		return nil
	})

	err := c.closeAll(context.Background(), log)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// LIFO order: res2 then res1
	if len(closed) != 2 || closed[0] != "res2" || closed[1] != "res1" {
		t.Fatalf("expected LIFO order [res2, res1], got %v", closed)
	}
}

// --- LIFO order preserved even with errors ---

func TestLIFOWithErrors(t *testing.T) {
	t.Parallel()
	log := newLogger()
	c := newCloser()

	var mu sync.Mutex
	var order []string

	c.add("a", func(ctx context.Context) error {
		mu.Lock()
		order = append(order, "a")
		mu.Unlock()
		return errors.New("err a")
	})
	c.add("b", func(ctx context.Context) error {
		mu.Lock()
		order = append(order, "b")
		mu.Unlock()
		return nil
	})

	_ = c.closeAll(context.Background(), log)

	if len(order) != 2 || order[0] != "b" || order[1] != "a" {
		t.Fatalf("expected LIFO [b a], got %v", order)
	}
}

// --- CloseAll idempotent (sync.Once) ---

func TestCloseAllIdempotent(t *testing.T) {
	t.Parallel()
	log := newLogger()
	c := newCloser()

	callCount := 0
	c.add("once_res", func(ctx context.Context) error {
		callCount++
		return nil
	})

	err := c.closeAll(context.Background(), log)
	if err != nil {
		t.Fatalf("first CloseAll failed: %v", err)
	}

	err = c.closeAll(context.Background(), log)
	if err != nil {
		t.Fatalf("second CloseAll failed: %v", err)
	}

	if callCount != 1 {
		t.Fatalf("expected 1 call, got %d", callCount)
	}
}

// --- Add after CloseAll is ignored ---

func TestAddAfterCloseAll(t *testing.T) {
	t.Parallel()
	log := newLogger()
	c := newCloser()

	closed := false

	c.add("early", func(ctx context.Context) error {
		return nil
	})

	_ = c.closeAll(context.Background(), log)

	c.add("late", func(ctx context.Context) error {
		closed = true
		return nil
	})

	// Second CloseAll is a no-op due to sync.Once.
	// The "late" resource was added after the first CloseAll set closed=true,
	// so it was silently dropped.
	_ = c.closeAll(context.Background(), log)

	if closed {
		t.Fatal("late resource should not have been closed")
	}
}

// --- Resource error propagation ---

func TestResourceErrorPropagation(t *testing.T) {
	t.Parallel()
	log := newLogger()
	c := newCloser()

	expectedErr := errors.New("close failed")
	c.add("good", func(ctx context.Context) error { return nil })
	c.add("bad", func(ctx context.Context) error { return expectedErr })
	c.add("good2", func(ctx context.Context) error { return nil })

	err := c.closeAll(context.Background(), log)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected error to contain %q, got %v", expectedErr, err)
	}
}

// --- Multiple errors are joined ---

func TestMultipleErrorsJoined(t *testing.T) {
	t.Parallel()
	log := newLogger()
	c := newCloser()

	err1 := errors.New("err1")
	err2 := errors.New("err2")
	c.add("a", func(ctx context.Context) error { return err1 })
	c.add("b", func(ctx context.Context) error { return err2 })

	err := c.closeAll(context.Background(), log)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !errors.Is(err, err1) || !errors.Is(err, err2) {
		t.Fatalf("expected both errors, got %v", err)
	}
}

// --- Timeout is enforced ---

func TestTimeoutEnforced(t *testing.T) {
	t.Parallel()
	log := newLogger()
	c := newCloser()

	parentCtx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	c.add("slow", func(ctx context.Context) error {
		select {
		case <-time.After(10 * time.Second):
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	})

	err := c.closeAll(parentCtx, log)
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected DeadlineExceeded, got %v", err)
	}
}

// --- Resource that ignores ctx is still killed by goroutine wrapper ---

func TestResourceIgnoresCtx(t *testing.T) {
	t.Parallel()
	log := newLogger()
	c := newCloser()

	parentCtx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	c.add("stubborn", func(ctx context.Context) error {
		time.Sleep(10 * time.Second)
		return nil
	})

	start := time.Now()
	err := c.closeAll(parentCtx, log)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected error from ignored-ctx resource, got nil")
	}

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected DeadlineExceeded, got %v", err)
	}

	// Should return within ~200ms (parent ctx deadline), not 10s.
	if elapsed > 2*time.Second {
		t.Fatalf("took too long (%v), goroutine wrapper not working", elapsed)
	}
}

// --- Concurrent Add calls are thread-safe ---

func TestConcurrentAdd(t *testing.T) {
	t.Parallel()
	log := newLogger()
	c := newCloser()

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			name := fmt.Sprintf("res_%d", n)
			c.add(name, func(ctx context.Context) error { return nil })
		}(i)
	}
	wg.Wait()

	err := c.closeAll(context.Background(), log)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- No resources registered ---

func TestNoResources(t *testing.T) {
	t.Parallel()
	log := newLogger()
	c := newCloser()

	err := c.closeAll(context.Background(), log)
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

// --- Canceled context ---

func TestCanceledContext(t *testing.T) {
	t.Parallel()
	log := newLogger()
	c := newCloser()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	c.add("cancel_test", func(ctx context.Context) error {
		<-ctx.Done()
		return ctx.Err()
	})

	err := c.closeAll(ctx, log)
	if err == nil {
		t.Fatal("expected error from canceled context, got nil")
	}

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected Canceled error, got %v", err)
	}
}

// --- Elapsed time is recorded ---

func TestElapsedTimeRecorded(t *testing.T) {
	t.Parallel()
	log := newLogger()
	c := newCloser()

	c.add("quick", func(ctx context.Context) error {
		time.Sleep(50 * time.Millisecond)
		return nil
	})

	err := c.closeAll(context.Background(), log)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- Goroutine leak check (best-effort diagnostic) ---

func TestNoGoroutineLeak(t *testing.T) {
	t.Parallel()
	log := newLogger()
	c := newCloser()

	before := runtime.NumGoroutine()

	for i := 0; i < 5; i++ {
		c.add(fmt.Sprintf("leak_%d", i), func(ctx context.Context) error {
			return nil
		})
		_ = c.closeAll(context.Background(), log)
	}

	time.Sleep(100 * time.Millisecond)

	after := runtime.NumGoroutine()
	if after > before+5 {
		t.Logf("goroutines before=%d, after=%d — possible leak", before, after)
	}
}

// --- Public API: Add and CloseAll are exported correctly ---

func TestGlobalAPI(t *testing.T) {
	// No t.Parallel() — uses global closer, must run isolated.

	var called atomic.Bool
	Add("api_test", func(ctx context.Context) error {
		called.Store(true)
		return nil
	})

	log := newLogger()
	_ = CloseAll(context.Background(), log)

	if !called.Load() {
		t.Fatal("global Add/CloseAll did not execute the resource")
	}
}
