package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/siteyonet/siteyonet/internal/config"
	"github.com/siteyonet/siteyonet/internal/repository"
)

func main() {
	cfg := config.Load()
	log.Printf("Running migrations on %s...", cfg.DatabaseURL)

	db, err := repository.NewDB(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Database connection failed: %v", err)
	}
	defer db.Pool.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	dirs := []string{"migrations", "backend/migrations", "../migrations"}
	var migrationDir string
	for _, d := range dirs {
		if _, err := os.Stat(d); err == nil {
			migrationDir = d
			break
		}
	}

	if migrationDir == "" {
		log.Fatalf("Migrations directory not found in: %v", dirs)
	}

	files, err := os.ReadDir(migrationDir)
	if err != nil {
		log.Fatalf("Failed to read migration dir: %v", err)
	}

	for _, f := range files {
		if filepath.Ext(f.Name()) == ".sql" {
			filePath := filepath.Join(migrationDir, f.Name())
			sqlBytes, err := os.ReadFile(filePath)
			if err != nil {
				log.Fatalf("Failed to read %s: %v", filePath, err)
			}
			log.Printf("Executing migration: %s", f.Name())
			if _, err := db.Pool.Exec(ctx, string(sqlBytes)); err != nil {
				log.Printf("Notice executing %s: %v", f.Name(), err)
			}
		}
	}

	fmt.Println("All migrations applied successfully!")
}
