package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"

	"psa/internal/config"
)

func main() {
	email := os.Getenv("ADMIN_EMAIL")
	password := os.Getenv("ADMIN_PASSWORD")

	if email == "" || password == "" {
		log.Fatal("ADMIN_EMAIL and ADMIN_PASSWORD must be set")
	}

	cfg := config.MustLoad()

	dsn := fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=%s",
		cfg.StoragePath.Username,
		cfg.StoragePath.Password,
		cfg.StoragePath.Host,
		cfg.StoragePath.Port,
		cfg.StoragePath.Database,
		cfg.StoragePath.SSLMode,
	)

	ctx := context.Background()

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		log.Fatalf("cannot connect to database: %v", err)
	}
	defer pool.Close()

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		log.Fatalf("cannot hash password: %v", err)
	}

	hashedPasswordB64 := base64.StdEncoding.EncodeToString(hashedPassword)

	log.Printf("Attempting to create admin: %s\n", email)

	_, err = pool.Exec(ctx, `
		INSERT INTO users (email, hashed_password, is_admin)
		VALUES ($1, $2, true)
		ON CONFLICT (email) DO NOTHING;
	`, email, hashedPasswordB64)
	if err != nil {
		log.Fatalf("failed to insert admin: %v", err)
	}

	log.Println("Admin creation process completed (may already exist)")
}
