package main

import (
	"github.com/siteyonet/siteyonet/internal/config"
	"github.com/siteyonet/siteyonet/internal/repository"
	"log"
	"log/slog"
	"os"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	db, err := repository.NewDB(config.Load().DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Pool.Close()
	if err := db.Migrate("migrations"); err != nil {
		log.Fatal(err)
	}
	log.Println("All migrations applied successfully")
}
