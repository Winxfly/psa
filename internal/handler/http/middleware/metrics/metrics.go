package metrics

import (
	"net/http"
	"time"

	appmetrics "psa/internal/metrics"
)

type Middleware struct {
	scope       string
	httpMetrics *appmetrics.HTTPMetrics
}

type responseWriter struct {
	http.ResponseWriter
	status       int
	bytesWritten int
	wroteHeader  bool
}

func New(scope string, httpMetrics *appmetrics.HTTPMetrics) *Middleware {
	return &Middleware{
		scope:       scope,
		httpMetrics: httpMetrics,
	}
}

func (m *Middleware) Handler() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ww := &responseWriter{
				ResponseWriter: w,
				status:         http.StatusOK,
			}

			start := time.Now()
			next.ServeHTTP(ww, r)

			m.httpMetrics.ObserveRequest(
				m.scope,
				r.Method,
				routeLabel(r),
				ww.status,
				time.Since(start),
			)
		})
	}
}

func (w *responseWriter) WriteHeader(code int) {
	if !w.wroteHeader {
		w.status = code
		w.wroteHeader = true
	}

	w.ResponseWriter.WriteHeader(code)
}

func (w *responseWriter) Write(b []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}

	n, err := w.ResponseWriter.Write(b)
	w.bytesWritten += n

	return n, err
}

func routeLabel(r *http.Request) string {
	if r.Pattern != "" {
		return r.Pattern
	}
	if r.URL.Path != "" {
		return r.URL.Path
	}

	return "unknown"
}
