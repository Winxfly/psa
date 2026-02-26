package cors_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"psa/internal/config"
	"psa/internal/handler/http/middleware/cors"
)

func TestCORSMiddleware_OriginValidation(t *testing.T) {
	tests := []struct {
		name                string
		config              config.CORS
		requestOrigin       string
		expectedAllowOrigin string
		expectedCORSHeaders bool
	}{
		{
			name: "allowed_origin",
			config: config.CORS{
				AllowedOrigins:     []string{"http://localhost:3000"},
				AllowedCredentials: true,
			},
			requestOrigin:       "http://localhost:3000",
			expectedAllowOrigin: "http://localhost:3000",
			expectedCORSHeaders: true,
		},
		{
			name: "disallowed_origin",
			config: config.CORS{
				AllowedOrigins:     []string{"http://localhost:3000"},
				AllowedCredentials: true,
			},
			requestOrigin:       "https://evil.com",
			expectedAllowOrigin: "",
			expectedCORSHeaders: false,
		},
		{
			name: "wildcard_origin",
			config: config.CORS{
				AllowedOrigins:     []string{"*"},
				AllowedCredentials: false,
			},
			requestOrigin:       "https://example.com",
			expectedAllowOrigin: "https://example.com",
			expectedCORSHeaders: true,
		},
		{
			name: "no_origin_header",
			config: config.CORS{
				AllowedOrigins:     []string{"*"},
				AllowedCredentials: false,
			},
			requestOrigin:       "",
			expectedAllowOrigin: "",
			expectedCORSHeaders: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := mustNewMiddleware(t, tt.config).Handler()(http.HandlerFunc(
				func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
				},
			))

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			if tt.requestOrigin != "" {
				req.Header.Set("Origin", tt.requestOrigin)
			}
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			allowOrigin := rr.Header().Get("Access-Control-Allow-Origin")
			varyHeader := rr.Header().Get("Vary")

			if tt.expectedCORSHeaders {
				if allowOrigin != tt.expectedAllowOrigin {
					t.Errorf("Expected Allow-Origin: %s, got: %s", tt.expectedAllowOrigin, allowOrigin)
				}
				if varyHeader != "Origin" {
					t.Errorf("Expected Vary: Origin, got: %s", varyHeader)
				}
			} else {
				if allowOrigin != "" {
					t.Errorf("Expected no Allow-Origin header, got: %s", allowOrigin)
				}
			}
		})
	}
}

func TestCORSMiddleware_Preflight(t *testing.T) {
	cfg := config.CORS{
		AllowedOrigins:     []string{"http://localhost:3000"},
		AllowedMethods:     []string{"GET", "POST", "PUT", "DELETE"},
		AllowedHeaders:     []string{"Content-Type", "Authorization"},
		AllowedCredentials: true,
		MaxAge:             86400,
	}

	handler := mustNewMiddleware(t, cfg).Handler()(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		},
	))

	req := httptest.NewRequest(http.MethodOptions, "/test", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	req.Header.Set("Access-Control-Request-Method", "POST")
	req.Header.Set("Access-Control-Request-Headers", "Content-Type, Authorization")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Errorf("Expected 204, got %d", rr.Code)
	}

	headers := rr.Header()
	if headers.Get("Access-Control-Allow-Origin") != "http://localhost:3000" {
		t.Error("Missing or incorrect Allow-Origin")
	}
	if headers.Get("Access-Control-Allow-Methods") != "GET, POST, PUT, DELETE" {
		t.Error("Missing or incorrect Allow-Methods")
	}
	if headers.Get("Access-Control-Allow-Headers") != "Content-Type, Authorization" {
		t.Error("Missing or incorrect Allow-Headers")
	}
	if headers.Get("Access-Control-Allow-Credentials") != "true" {
		t.Error("Missing or incorrect Allow-Credentials")
	}
	if headers.Get("Access-Control-Max-Age") != "86400" {
		t.Error("Missing or incorrect Max-Age")
	}
	if headers.Get("Vary") != "Origin" {
		t.Error("Missing Vary: Origin")
	}
}

func TestCORSMiddleware_Preflight_MethodNotAllowed(t *testing.T) {
	cfg := config.CORS{
		AllowedOrigins: []string{"http://localhost:3000"},
		AllowedMethods: []string{"GET", "POST"},
	}

	handler := mustNewMiddleware(t, cfg).Handler()(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		},
	))

	req := httptest.NewRequest(http.MethodOptions, "/test", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	req.Header.Set("Access-Control-Request-Method", "DELETE")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("Expected 403, got %d", rr.Code)
	}
}

func TestCORSMiddleware_Preflight_HeadersNotAllowed(t *testing.T) {
	cfg := config.CORS{
		AllowedOrigins: []string{"http://localhost:3000"},
		AllowedHeaders: []string{"Content-Type"},
	}

	handler := mustNewMiddleware(t, cfg).Handler()(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		},
	))

	req := httptest.NewRequest(http.MethodOptions, "/test", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	req.Header.Set("Access-Control-Request-Method", "POST")
	req.Header.Set("Access-Control-Request-Headers", "X-Custom-Header")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("Expected 403, got %d", rr.Code)
	}
}

func TestCORSMiddleware_Preflight_MissingMethod(t *testing.T) {
	cfg := config.CORS{
		AllowedOrigins: []string{"http://localhost:3000"},
	}

	handler := mustNewMiddleware(t, cfg).Handler()(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		},
	))

	req := httptest.NewRequest(http.MethodOptions, "/test", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	// Access-Control-Request-Method отсутствует
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	// Без Access-Control-Request-Method — это не preflight, просто обычный OPTIONS
	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", rr.Code)
	}
}

func TestCORSMiddleware_Preflight_MethodCaseInsensitive(t *testing.T) {
	cfg := config.CORS{
		AllowedOrigins: []string{"http://localhost:3000"},
		AllowedMethods: []string{"GET", "POST"},
	}

	handler := mustNewMiddleware(t, cfg).Handler()(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		},
	))

	req := httptest.NewRequest(http.MethodOptions, "/test", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	req.Header.Set("Access-Control-Request-Method", "post") // lowercase
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Errorf("Expected 204, got %d", rr.Code)
	}
}

func TestCORSMiddleware_NoCredentialsHeader_WhenFalse(t *testing.T) {
	cfg := config.CORS{
		AllowedOrigins:     []string{"*"},
		AllowedCredentials: false,
	}

	handler := mustNewMiddleware(t, cfg).Handler()(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		},
	))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Origin", "https://example.com")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Header().Get("Access-Control-Allow-Credentials") != "" {
		t.Error("Should not set Allow-Credentials when false")
	}
}

func TestCORSMiddleware_VaryHeader(t *testing.T) {
	cfg := config.CORS{
		AllowedOrigins: []string{"http://localhost:3000"},
	}

	handler := mustNewMiddleware(t, cfg).Handler()(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		},
	))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Header().Get("Vary") != "Origin" {
		t.Errorf("Expected Vary: Origin, got: %s", rr.Header().Get("Vary"))
	}
}

func TestCORSMiddleware_VaryHeaderMerge(t *testing.T) {
	cfg := config.CORS{
		AllowedOrigins: []string{"http://localhost:3000"},
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Vary", "Accept-Encoding")
		w.WriteHeader(http.StatusOK)
	})

	corsHandler := mustNewMiddleware(t, cfg).Handler()(handler)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	rr := httptest.NewRecorder()

	corsHandler.ServeHTTP(rr, req)

	vary := rr.Header().Values("Vary")
	if len(vary) < 2 {
		t.Errorf("Expected multiple Vary headers, got: %v", vary)
	}
}

func TestCORSMiddleware_WildcardHeaders(t *testing.T) {
	cfg := config.CORS{
		AllowedOrigins: []string{"http://localhost:3000"},
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{"*"},
	}

	handler := mustNewMiddleware(t, cfg).Handler()(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		},
	))

	req := httptest.NewRequest(http.MethodOptions, "/test", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	req.Header.Set("Access-Control-Request-Method", "POST")
	req.Header.Set("Access-Control-Request-Headers", "X-Custom-Header, X-Another")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Errorf("Expected 204, got %d", rr.Code)
	}
}

func TestCORSMiddleware_Preflight_NoOrigin(t *testing.T) {
	cfg := config.CORS{
		AllowedOrigins: []string{"*"},
	}

	handler := mustNewMiddleware(t, cfg).Handler()(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		},
	))

	req := httptest.NewRequest(http.MethodOptions, "/test", nil)
	// Origin отсутствует
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	// Без Origin — это не CORS preflight, просто пропускаем
	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", rr.Code)
	}
}

func TestCORSMiddleware_Preflight_VaryHeaders(t *testing.T) {
	cfg := config.CORS{
		AllowedOrigins:     []string{"http://localhost:3000"},
		AllowedMethods:     []string{"GET", "POST"},
		AllowedHeaders:     []string{"Content-Type"},
		AllowedCredentials: true,
		MaxAge:             86400,
	}

	handler := mustNewMiddleware(t, cfg).Handler()(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		},
	))

	req := httptest.NewRequest(http.MethodOptions, "/test", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	req.Header.Set("Access-Control-Request-Method", "POST")
	req.Header.Set("Access-Control-Request-Headers", "Content-Type")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	// Проверяем все Vary заголовки
	varyHeaders := rr.Header().Values("Vary")
	if len(varyHeaders) < 3 {
		t.Errorf("Expected at least 3 Vary headers, got: %v", varyHeaders)
	}

	// Проверяем наличие всех required Vary
	hasOrigin := false
	hasRequestMethod := false
	hasRequestHeaders := false

	for _, v := range varyHeaders {
		switch v {
		case "Origin":
			hasOrigin = true
		case "Access-Control-Request-Method":
			hasRequestMethod = true
		case "Access-Control-Request-Headers":
			hasRequestHeaders = true
		}
	}

	if !hasOrigin {
		t.Error("Missing Vary: Origin")
	}
	if !hasRequestMethod {
		t.Error("Missing Vary: Access-Control-Request-Method")
	}
	if !hasRequestHeaders {
		t.Error("Missing Vary: Access-Control-Request-Headers")
	}
}

func TestCORSMiddleware_ActualRequest_NoPreflightHeaders(t *testing.T) {
	cfg := config.CORS{
		AllowedOrigins:     []string{"http://localhost:3000"},
		AllowedMethods:     []string{"GET", "POST"},
		AllowedHeaders:     []string{"Content-Type"},
		AllowedCredentials: true,
	}

	handler := mustNewMiddleware(t, cfg).Handler()(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		},
	))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	// У обычного запроса не должно быть preflight заголовков
	if rr.Header().Get("Access-Control-Allow-Methods") != "" {
		t.Error("Actual request should not have Access-Control-Allow-Methods")
	}
	if rr.Header().Get("Access-Control-Allow-Headers") != "" {
		t.Error("Actual request should not have Access-Control-Allow-Headers")
	}
	if rr.Header().Get("Access-Control-Max-Age") != "" {
		t.Error("Actual request should not have Access-Control-Max-Age")
	}

	// Но Origin должен быть
	if rr.Header().Get("Access-Control-Allow-Origin") != "http://localhost:3000" {
		t.Error("Actual request should have Access-Control-Allow-Origin")
	}
}

func TestCORSMiddleware_HeadersCaseInsensitive(t *testing.T) {
	cfg := config.CORS{
		AllowedOrigins: []string{"http://localhost:3000"},
		AllowedMethods: []string{"POST"},
		AllowedHeaders: []string{"Content-Type", "Authorization"},
	}

	handler := mustNewMiddleware(t, cfg).Handler()(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		},
	))

	req := httptest.NewRequest(http.MethodOptions, "/test", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	req.Header.Set("Access-Control-Request-Method", "POST")
	// Заголовки в lowercase
	req.Header.Set("Access-Control-Request-Headers", "content-type, authorization")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Errorf("Expected 204, got %d", rr.Code)
	}
}

func TestCORSMiddleware_MultipleOrigins(t *testing.T) {
	cfg := config.CORS{
		AllowedOrigins:     []string{"http://localhost:3000", "http://localhost:4000", "https://example.com"},
		AllowedMethods:     []string{"GET", "POST"},
		AllowedHeaders:     []string{"Content-Type"},
		AllowedCredentials: true,
	}

	handler := mustNewMiddleware(t, cfg).Handler()(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		},
	))

	tests := []struct {
		name           string
		origin         string
		shouldAllow    bool
		expectedOrigin string
	}{
		{"first_origin", "http://localhost:3000", true, "http://localhost:3000"},
		{"second_origin", "http://localhost:4000", true, "http://localhost:4000"},
		{"third_origin", "https://example.com", true, "https://example.com"},
		{"disallowed_origin", "https://evil.com", false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			req.Header.Set("Origin", tt.origin)
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			allowOrigin := rr.Header().Get("Access-Control-Allow-Origin")
			if tt.shouldAllow {
				if allowOrigin != tt.expectedOrigin {
					t.Errorf("Expected Allow-Origin: %s, got: %s", tt.expectedOrigin, allowOrigin)
				}
			} else {
				if allowOrigin != "" {
					t.Errorf("Expected no Allow-Origin, got: %s", allowOrigin)
				}
			}
		})
	}
}

func TestCORSMiddleware_EmptyAllowedMethods(t *testing.T) {
	cfg := config.CORS{
		AllowedOrigins: []string{"http://localhost:3000"},
		AllowedMethods: []string{}, // Пустой список
		AllowedHeaders: []string{"Content-Type"},
	}

	handler := mustNewMiddleware(t, cfg).Handler()(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		},
	))

	req := httptest.NewRequest(http.MethodOptions, "/test", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	req.Header.Set("Access-Control-Request-Method", "POST")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	// Метод не разрешён — должен быть 403
	if rr.Code != http.StatusForbidden {
		t.Errorf("Expected 403, got %d", rr.Code)
	}
}

func TestCORSMiddleware_EmptyAllowedHeaders(t *testing.T) {
	cfg := config.CORS{
		AllowedOrigins: []string{"http://localhost:3000"},
		AllowedMethods: []string{"POST"},
		AllowedHeaders: []string{}, // Пустой список
	}

	handler := mustNewMiddleware(t, cfg).Handler()(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		},
	))

	req := httptest.NewRequest(http.MethodOptions, "/test", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	req.Header.Set("Access-Control-Request-Method", "POST")
	req.Header.Set("Access-Control-Request-Headers", "Content-Type")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	// Заголовки не разрешены — должен быть 403
	if rr.Code != http.StatusForbidden {
		t.Errorf("Expected 403, got %d", rr.Code)
	}
}

func TestCORSMiddleware_MaxAgeZero(t *testing.T) {
	cfg := config.CORS{
		AllowedOrigins:     []string{"http://localhost:3000"},
		AllowedMethods:     []string{"GET", "POST"},
		AllowedHeaders:     []string{"Content-Type"},
		AllowedCredentials: true,
		MaxAge:             0, // MaxAge = 0
	}

	handler := mustNewMiddleware(t, cfg).Handler()(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		},
	))

	req := httptest.NewRequest(http.MethodOptions, "/test", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	req.Header.Set("Access-Control-Request-Method", "POST")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	// Max-Age не должен устанавливаться при значении 0
	if rr.Header().Get("Access-Control-Max-Age") != "" {
		t.Errorf("Expected no Max-Age header, got: %s", rr.Header().Get("Access-Control-Max-Age"))
	}
}

func TestCORSMiddleware_MixedHeadersAllowed(t *testing.T) {
	cfg := config.CORS{
		AllowedOrigins: []string{"http://localhost:3000"},
		AllowedMethods: []string{"POST"},
		AllowedHeaders: []string{"Content-Type", "Authorization"},
	}

	handler := mustNewMiddleware(t, cfg).Handler()(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		},
	))

	req := httptest.NewRequest(http.MethodOptions, "/test", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	req.Header.Set("Access-Control-Request-Method", "POST")
	// Смесь разрешённых и неразрешённых заголовков
	req.Header.Set("Access-Control-Request-Headers", "Content-Type, X-Custom-Header")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	// Неразрешённый заголовок — должен быть 403
	if rr.Code != http.StatusForbidden {
		t.Errorf("Expected 403, got %d", rr.Code)
	}
}

func TestCORSMiddleware_OriginValidation_StatusCode(t *testing.T) {
	cfg := config.CORS{
		AllowedOrigins:     []string{"http://localhost:3000"},
		AllowedMethods:     []string{"GET"},
		AllowedHeaders:     []string{"Content-Type"},
		AllowedCredentials: true,
	}

	handler := mustNewMiddleware(t, cfg).Handler()(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		},
	))

	tests := []struct {
		name           string
		origin         string
		expectedStatus int
	}{
		{"allowed_origin", "http://localhost:3000", http.StatusOK},
		{"disallowed_origin", "https://evil.com", http.StatusOK},
		{"no_origin", "", http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			}
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			if rr.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, rr.Code)
			}
		})
	}
}

func TestNewMiddleware_WildcardWithCredentials(t *testing.T) {
	cfg := config.CORS{
		AllowedOrigins:     []string{"*"},
		AllowedCredentials: true,
	}

	_, err := cors.NewMiddleware(cfg)
	if err == nil {
		t.Error("Expected error for wildcard + credentials, got nil")
	}
}

// Helper
func mustNewMiddleware(t *testing.T, cfg config.CORS) *cors.Middleware {
	t.Helper()
	mw, err := cors.NewMiddleware(cfg)
	if err != nil {
		t.Fatalf("Failed to create middleware: %v", err)
	}
	return mw
}
