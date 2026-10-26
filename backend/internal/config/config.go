package config

import (
	"os"
)

type Config struct {
	Port        string
	DatabaseURL string
	Environment string
	JWTSecret   string
}

func Load() *Config {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://postgres:postgres@localhost:5432/siteyonet?sslmode=disable"
	}

	env := os.Getenv("ENV")
	if env == "" {
		env = "development"
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "super-secret-jwt-key-siteyonet-production-0c7cc1"
	}

	return &Config{
		Port:        port,
		DatabaseURL: dbURL,
		Environment: env,
		JWTSecret:   jwtSecret,
	}
}
