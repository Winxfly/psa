package cron

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"psa/internal/service/cron/mocks"
	"psa/pkg/logger/loggerctx"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// testDeps содержит зависимости для тестирования Cron
type testDeps struct {
	scraper *mocks.MockScrapingProvider
}

func newDeps(t *testing.T) testDeps {
	t.Helper()
	return testDeps{
		scraper: mocks.NewMockScrapingProvider(t),
	}
}

func newTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func (d testDeps) cron() *Cron {
	log := newTestLogger()
	cron, err := New(log, d.scraper)
	if err != nil {
		panic(err)
	}
	return cron
}

// ==================== New ====================

func TestCron_New(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := newDeps(t)

	// Act
	cron := deps.cron()

	// Assert
	require.NotNil(t, cron)
	assert.NotNil(t, cron.scheduler)
	assert.NotNil(t, cron.log)
	assert.NotNil(t, cron.scraper)
}

// ==================== Start & Stop ====================

func TestCron_Start_JobsRegistered(t *testing.T) {
	t.Parallel()

	// Arrange
	ctx := context.Background()
	deps := newDeps(t)
	cron := deps.cron()

	// Act
	err := cron.Start(ctx)
	require.NoError(t, err)

	// Assert - проверяем что jobs действительно зарегистрированы
	jobs := cron.scheduler.Jobs()
	require.Len(t, jobs, 2)

	// Проверяем имена jobs
	jobNames := make(map[string]bool)
	for _, job := range jobs {
		jobNames[job.Name()] = true
	}
	require.True(t, jobNames["monthly_scraping"], "monthly_scraping job should be registered")
	require.True(t, jobNames["daily_scraping"], "daily_scraping job should be registered")

	// Clean up
	err = cron.Stop(ctx)
	require.NoError(t, err)
}

func TestCron_Stop_NotStarted(t *testing.T) {
	t.Parallel()

	// Arrange
	ctx := context.Background()
	deps := newDeps(t)
	cron := deps.cron()

	// Act
	err := cron.Stop(ctx)

	// Assert
	require.NoError(t, err)
}

func TestCron_Stop_Timeout(t *testing.T) {
	t.Parallel()

	// Arrange
	ctx := context.Background()
	deps := newDeps(t)
	cron := deps.cron()

	// Запускаем scheduler
	err := cron.Start(ctx)
	require.NoError(t, err)

	// Act - пытаемся остановить с очень маленьким timeout
	// Для этого создаём context с timeout меньше чем требуется на shutdown
	shutdownCtx, cancel := context.WithTimeout(ctx, 1*time.Nanosecond)
	defer cancel()

	err = cron.Stop(shutdownCtx)

	// Assert
	// Ожидаем timeout ошибку
	require.Error(t, err)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}

// ==================== runScrapingJobArchive ====================

func TestCron_RunScrapingJobArchive_ContextAndLogger(t *testing.T) {
	t.Parallel()

	// Arrange
	ctx := context.Background()
	deps := newDeps(t)

	var capturedCtx context.Context
	deps.scraper.EXPECT().ProcessActiveProfessionsArchive(mock.MatchedBy(func(ctx context.Context) bool {
		capturedCtx = ctx
		deadline, ok := ctx.Deadline()
		return ok && time.Until(deadline) <= jobTimeout && time.Until(deadline) > 0
	})).Return(nil).Once()

	cron := deps.cron()

	// Act
	cron.runScrapingJobArchive(ctx, "archive")

	// Assert - проверяем context deadline
	require.NotNil(t, capturedCtx, "context should be passed to scraper")
	deadline, ok := capturedCtx.Deadline()
	require.True(t, ok, "context should have deadline")
	assert.InDelta(t, jobTimeout, time.Until(deadline), float64(100*time.Millisecond))

	// Assert - проверяем что логгер был прокинут в контекст
	logger := loggerctx.FromContext(capturedCtx)
	require.NotNil(t, logger, "logger should be propagated to context")
}

func TestCron_RunScrapingJobArchive_Error(t *testing.T) {
	t.Parallel()

	// Arrange
	ctx := context.Background()
	deps := newDeps(t)

	deps.scraper.EXPECT().ProcessActiveProfessionsArchive(mock.MatchedBy(func(ctx context.Context) bool {
		deadline, ok := ctx.Deadline()
		return ok && time.Until(deadline) <= jobTimeout
	})).Return(assert.AnError).Once()

	cron := deps.cron()

	// Act & Assert
	assert.NotPanics(t, func() {
		cron.runScrapingJobArchive(ctx, "test")
	})
}

// ==================== runScrapingJobDaily ====================

func TestCron_RunScrapingJobDaily_ContextAndLogger(t *testing.T) {
	t.Parallel()

	// Arrange
	ctx := context.Background()
	deps := newDeps(t)

	var capturedCtx context.Context
	deps.scraper.EXPECT().ProcessActiveProfessionsDaily(mock.MatchedBy(func(ctx context.Context) bool {
		capturedCtx = ctx
		deadline, ok := ctx.Deadline()
		return ok && time.Until(deadline) <= jobTimeout && time.Until(deadline) > 0
	})).Return(nil).Once()

	cron := deps.cron()

	// Act
	cron.runScrapingJobDaily(ctx, "daily")

	// Assert - проверяем context deadline
	require.NotNil(t, capturedCtx, "context should be passed to scraper")
	deadline, ok := capturedCtx.Deadline()
	require.True(t, ok, "context should have deadline")
	assert.InDelta(t, jobTimeout, time.Until(deadline), float64(100*time.Millisecond))

	// Assert - проверяем что логгер был прокинут в контекст
	logger := loggerctx.FromContext(capturedCtx)
	require.NotNil(t, logger, "logger should be propagated to context")
}

func TestCron_RunScrapingJobDaily_Error(t *testing.T) {
	t.Parallel()

	// Arrange
	ctx := context.Background()
	deps := newDeps(t)

	deps.scraper.EXPECT().ProcessActiveProfessionsDaily(mock.MatchedBy(func(ctx context.Context) bool {
		deadline, ok := ctx.Deadline()
		return ok && time.Until(deadline) <= jobTimeout
	})).Return(assert.AnError).Once()

	cron := deps.cron()

	// Act & Assert
	assert.NotPanics(t, func() {
		cron.runScrapingJobDaily(ctx, "test")
	})
}
