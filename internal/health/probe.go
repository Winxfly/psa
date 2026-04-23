package health

import (
	"context"
	"log/slog"
	"time"
)

type DependencyMetrics interface {
	SetDependencyUp(name string, up bool)
}

type Probe struct {
	log       *slog.Logger
	interval  time.Duration
	metrics   DependencyMetrics
	checks    []Check
	lastState map[string]bool
}

func NewProbe(log *slog.Logger, interval time.Duration, metrics DependencyMetrics, checks ...Check) *Probe {
	return &Probe{
		log:       log.With("component", "dependency_probe"),
		interval:  interval,
		metrics:   metrics,
		checks:    checks,
		lastState: make(map[string]bool, len(checks)),
	}
}

func (p *Probe) Start(ctx context.Context) {
	p.runOnce(ctx)

	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.runOnce(ctx)
		}
	}
}

func (p *Probe) runOnce(parent context.Context) {
	ctx, cancel := context.WithTimeout(parent, 5*time.Second)
	defer cancel()

	for _, check := range p.checks {
		err := check.Check(ctx)
		up := err == nil

		p.metrics.SetDependencyUp(check.Name(), up)
		p.logState(check.Name(), up, err)
	}
}

func (p *Probe) logState(name string, up bool, err error) {
	prev, known := p.lastState[name]
	p.lastState[name] = up

	if up {
		if !known || !prev {
			p.log.Info("dependency_up", "dependency", name)
		}
		return
	}

	if !known || prev {
		p.log.Warn("dependency_down", "dependency", name, "error", err)
	}
}
