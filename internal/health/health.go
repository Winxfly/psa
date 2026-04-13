package health

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

type Check interface {
	Name() string
	Check(ctx context.Context) error
}

type Checker struct {
	checks []Check
	mu     sync.RWMutex
}

func New(checks ...Check) *Checker {
	return &Checker{
		checks: checks,
	}
}

func (c *Checker) AddCheck(check Check) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.checks = append(c.checks, check)
}

func (c *Checker) LivenessHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"status": "ok",
		})
	}
}

func (c *Checker) ReadinessHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c.handleReadiness(w, r)
	}
}

type readinessResponse struct {
	Status string            `json:"status"`
	Checks map[string]string `json:"checks"`
}

func (c *Checker) handleReadiness(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	c.mu.RLock()
	checks := make([]Check, len(c.checks))
	copy(checks, c.checks)
	c.mu.RUnlock()

	resp := readinessResponse{
		Status: "ok",
		Checks: make(map[string]string, len(checks)),
	}

	overallStatus := http.StatusOK

	for _, check := range checks {
		if err := check.Check(ctx); err != nil {
			resp.Checks[check.Name()] = "fail"
			resp.Status = "fail"
			overallStatus = http.StatusServiceUnavailable
		} else {
			resp.Checks[check.Name()] = "ok"
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(overallStatus)
	_ = json.NewEncoder(w).Encode(resp)
}
