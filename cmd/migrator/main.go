package main

import (
	"errors"
	"flag"
	"fmt"
	"psa/internal/config"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

func main() {
	var (
		migrationsPath string
		up             bool
		down           bool
		forceVersion   int
	)

	flag.StringVar(&migrationsPath, "migrations-path", "", "path to migrations")
	flag.BoolVar(&up, "up", false, "apply up migrations")
	flag.BoolVar(&down, "down", false, "apply down migrations")
	flag.IntVar(&forceVersion, "force", -1, "force database schema version")
	flag.Parse()

	if migrationsPath == "" {
		panic("migrations-path is required")
	}

	cfg := config.MustLoad()

	storagePath := fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=%s",
		cfg.StoragePath.Username,
		cfg.StoragePath.Password,
		cfg.StoragePath.Host,
		cfg.StoragePath.Port,
		cfg.StoragePath.Database,
		cfg.StoragePath.SSLMode,
	)

	m, err := migrate.New("file://"+migrationsPath, storagePath)
	if err != nil {
		panic(fmt.Sprintf("failed to create migration instance: %v", err))
	}

	// --force
	if forceVersion >= 0 {
		fmt.Printf("Force setting migration version to %d...\n", forceVersion)
		if err := m.Force(forceVersion); err != nil {
			panic(fmt.Sprintf("failed to force version: %v", err))
		}
		fmt.Println("force completed successfully")
		return
	}

	// --up / --down
	switch {
	case up:
		fmt.Println("Applying (up) migrations...")
		if err := m.Up(); err != nil {
			if errors.Is(err, migrate.ErrNoChange) {
				fmt.Println("no migrations to apply")
				return
			}
			panic(fmt.Sprintf("migration error: %v", err))
		}
		fmt.Println("migrations applied successfully")
	case down:
		fmt.Println("Rolling back (down) all migrations...")
		if err := m.Down(); err != nil {
			if errors.Is(err, migrate.ErrNoChange) {
				fmt.Println("no migrations to rollback")
				return
			}
			panic(fmt.Sprintf("rollback error: %v", err))
		}
		fmt.Println("rollback completed successfully")
	default:
		panic("please provide --up, --down or --force")
	}
}
