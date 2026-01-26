package logger

import (
	"context"
	"log/slog"
	"net/http"
	"psa/pkg/logger/loggerctx"
	"time"

	"github.com/google/uuid"
)

type key string

const (
	requestIDKey key = "request_id"
)

type responseWriter struct {
	http.ResponseWriter
	status       int
	bytesWritten int
	wroteHeader  bool
}

func (w *responseWriter) WriteHeader(code int) {
	if !w.wroteHeader {
		w.status = code
		w.wroteHeader = true
		w.ResponseWriter.WriteHeader(code)
	}
}

func (w *responseWriter) Write(b []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	n, err := w.ResponseWriter.Write(b)
	w.bytesWritten += n
	return n, err
}

func NewLoggerMiddleware(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestID := uuid.New().String()
			logWithRequestID := log.With(slog.String("request_id", requestID))

			ctx := context.WithValue(r.Context(), requestIDKey, requestID)
			ctx = loggerctx.WithLogger(ctx, logWithRequestID)
			r = r.WithContext(ctx)

			ww := &responseWriter{ResponseWriter: w}
			start := time.Now()

			log.InfoContext(ctx, "request started",
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.String("remote_addr", r.RemoteAddr),
				slog.String("user_agent", r.UserAgent()),
			)

			defer func() {
				log.InfoContext(ctx, "request completed",
					slog.Int("status", ww.status),
					slog.Int("bytes", ww.bytesWritten),
					slog.String("duration", time.Since(start).String()),
				)
			}()

			next.ServeHTTP(ww, r)
		})
	}
}
