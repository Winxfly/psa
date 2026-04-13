package health

import (
	"context"
	"fmt"
)

type DBPinger interface {
	Ping(ctx context.Context) error
}

type dbCheck struct {
	db DBPinger
}

func NewDBCheck(db DBPinger) *dbCheck {
	return &dbCheck{db: db}
}

func (d *dbCheck) Name() string {
	return "db"
}

func (d *dbCheck) Check(ctx context.Context) error {
	if err := d.db.Ping(ctx); err != nil {
		return fmt.Errorf("db ping failed: %w", err)
	}
	return nil
}
