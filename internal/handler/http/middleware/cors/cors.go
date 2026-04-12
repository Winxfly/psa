package cors

import (
	"errors"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"psa/internal/config"
)

var ErrInvalidCORSConfig = errors.New(
	"cors: wildcard origin with credentials is not allowed by specification",
)

type Middleware struct {
	config config.CORS
}

func NewMiddleware(cfg config.CORS) (*Middleware, error) {
	if len(cfg.AllowedOrigins) == 1 &&
		cfg.AllowedOrigins[0] == "*" &&
		cfg.AllowedCredentials {

		return nil, ErrInvalidCORSConfig
	}

	return &Middleware{
		config: cfg,
	}, nil
}

func (m *Middleware) Handler() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin == "" {
				next.ServeHTTP(w, r)
				return
			}

			allowedOrigin, ok := m.getAllowedOrigin(origin)
			if !ok {
				w.WriteHeader(http.StatusForbidden)
				return
			}

			if r.Method == http.MethodOptions &&
				r.Header.Get("Access-Control-Request-Method") != "" {

				m.handlePreflight(w, r, allowedOrigin)
				return
			}

			m.setCORSHeaders(w, allowedOrigin, false)
			next.ServeHTTP(w, r)
		})
	}
}

func (m *Middleware) getAllowedOrigin(origin string) (string, bool) {
	if len(m.config.AllowedOrigins) == 1 &&
		m.config.AllowedOrigins[0] == "*" {

		return origin, true
	}

	for _, allowed := range m.config.AllowedOrigins {
		if allowed == origin {
			return origin, true
		}
	}

	return "", false
}

func (m *Middleware) handlePreflight(
	w http.ResponseWriter,
	r *http.Request,
	origin string,
) {
	requestMethod := r.Header.Get("Access-Control-Request-Method")
	requestHeaders := r.Header.Get("Access-Control-Request-Headers")

	if !m.isMethodAllowed(requestMethod) {
		w.WriteHeader(http.StatusForbidden)
		return
	}

	if requestHeaders != "" && !m.areHeadersAllowed(requestHeaders) {
		w.WriteHeader(http.StatusForbidden)
		return
	}

	m.setCORSHeaders(w, origin, true)
	w.WriteHeader(http.StatusNoContent)
}

func (m *Middleware) setCORSHeaders(
	w http.ResponseWriter,
	origin string,
	isPreflight bool,
) {
	w.Header().Set("Access-Control-Allow-Origin", origin)
	w.Header().Add("Vary", "Origin")

	if m.config.AllowedCredentials {
		w.Header().Set("Access-Control-Allow-Credentials", "true")
	}

	if isPreflight {
		if len(m.config.AllowedMethods) > 0 {
			w.Header().Set(
				"Access-Control-Allow-Methods",
				strings.Join(m.config.AllowedMethods, ", "),
			)
		}

		if len(m.config.AllowedHeaders) > 0 {
			w.Header().Set(
				"Access-Control-Allow-Headers",
				strings.Join(m.config.AllowedHeaders, ", "),
			)
		}

		if m.config.MaxAge > 0 {
			w.Header().Set(
				"Access-Control-Max-Age",
				strconv.Itoa(m.config.MaxAge),
			)
		}

		w.Header().Add("Vary", "Access-Control-Request-Method")
		w.Header().Add("Vary", "Access-Control-Request-Headers")
	}
}

func (m *Middleware) isMethodAllowed(method string) bool {
	method = strings.ToUpper(method)

	for _, allowed := range m.config.AllowedMethods {
		if strings.ToUpper(allowed) == method {
			return true
		}
	}

	return false
}

func (m *Middleware) areHeadersAllowed(requestedHeaders string) bool {
	if requestedHeaders == "" {
		return true
	}

	if slices.Contains(m.config.AllowedHeaders, "*") {
		return true
	}

	headers := strings.Split(requestedHeaders, ",")

	for _, h := range headers {
		h = strings.TrimSpace(strings.ToLower(h))
		if h == "" {
			continue
		}

		allowed := false

		for _, allowedHeader := range m.config.AllowedHeaders {
			if strings.EqualFold(h, strings.TrimSpace(allowedHeader)) {
				allowed = true
				break
			}
		}

		if !allowed {
			return false
		}
	}

	return true
}
