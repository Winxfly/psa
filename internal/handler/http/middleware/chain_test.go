package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"psa/internal/handler/http/middleware"
)

func TestChain_Add(t *testing.T) {
	chain := middleware.NewChain()

	var executed []string

	mw1 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			executed = append(executed, "mw1-before")
			next.ServeHTTP(w, r)
			executed = append(executed, "mw1-after")
		})
	}

	mw2 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			executed = append(executed, "mw2-before")
			next.ServeHTTP(w, r)
			executed = append(executed, "mw2-after")
		})
	}

	chain.Add(mw1, mw2)

	finalHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		executed = append(executed, "handler")
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rr := httptest.NewRecorder()

	chain.Then(finalHandler).ServeHTTP(rr, req)

	// Ожидаемый порядок: mw1-before → mw2-before → handler → mw2-after → mw1-after
	expected := []string{"mw1-before", "mw2-before", "handler", "mw2-after", "mw1-after"}

	if len(executed) != len(expected) {
		t.Fatalf("Expected %d executions, got %d", len(expected), len(executed))
	}

	for i, e := range expected {
		if executed[i] != e {
			t.Errorf("Step %d: expected %s, got %s", i, e, executed[i])
		}
	}
}

func TestChain_ThenFunc(t *testing.T) {
	chain := middleware.NewChain()

	var executed bool

	mw := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			executed = true
			next.ServeHTTP(w, r)
		})
	}

	chain.Add(mw)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rr := httptest.NewRecorder()

	chain.ThenFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}).ServeHTTP(rr, req)

	if !executed {
		t.Error("Middleware was not executed")
	}

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}
}

func TestChain_Empty(t *testing.T) {
	chain := middleware.NewChain()

	finalHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot) // 418
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rr := httptest.NewRecorder()

	chain.Then(finalHandler).ServeHTTP(rr, req)

	if rr.Code != http.StatusTeapot {
		t.Errorf("Expected status 418 (teapot), got %d", rr.Code)
	}
}
