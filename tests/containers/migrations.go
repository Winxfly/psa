package containers

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

func RunMigrations(dsn string, migrationsPath string) error {
	dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return fmt.Errorf("go.mod not found")
		}
		dir = parent
	}
	projectRoot := dir

	absPath := filepath.Join(projectRoot, migrationsPath)

	m, err := migrate.New(
		fmt.Sprintf("file://%s", absPath),
		dsn,
	)
	if err != nil {
		return fmt.Errorf("create migrate: %w", err)
	}

	defer func() {
		_, _ = m.Close()
	}()

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("run migrations: %w", err)
	}

	return nil
}
