package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/siteyonet/siteyonet/internal/config"
	"github.com/siteyonet/siteyonet/internal/handler"
	"github.com/siteyonet/siteyonet/internal/repository"
	"github.com/siteyonet/siteyonet/internal/service"
)

func main() {
	cfg := config.Load()
	log.Printf("Starting SiteYonet API Server on port %s (env: %s)...", cfg.Port, cfg.Environment)

	// Initialize DB Connection
	db, err := repository.NewDB(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Fatal: Database initialization failed: %v", err)
	}
	defer db.Pool.Close()

	// Run Database Migrations Automatically
	if err := runMigrations(db, "migrations"); err != nil {
		log.Printf("Warning: Automatic migration check: %v", err)
	}

	// Initialize Domain Services
	accrualEngine := service.NewAccrualEngine()
	paymentService := service.NewPaymentService(accrualEngine)
	accountingService := service.NewAccountingService()
	certificateService := service.NewCertificateService()

	// Initialize HTTP Handlers & Routes
	h := handler.NewHandler(db, accrualEngine, paymentService, accountingService, certificateService)
	router := h.InitRoutes()

	server := &http.Server{
		Addr:         fmt.Sprintf(":%s", cfg.Port),
		Handler:      router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Start server in background goroutine
	go func() {
		log.Printf("API Server listening on http://0.0.0.0:%s", cfg.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server ListenAndServe failed: %v", err)
		}
	}()

	// Graceful Shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down API server gracefully...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited cleanly.")
}

// runMigrations executes SQL files in order from the migrations directory
func runMigrations(db *repository.DB, dir string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Check if directory exists
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		// Try fallback relative path
		dir = filepath.Join("backend", dir)
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			return fmt.Errorf("migrations directory not found")
		}
	}

	files, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	for _, file := range files {
		if filepath.Ext(file.Name()) == ".sql" {
			filePath := filepath.Join(dir, file.Name())
			sqlBytes, err := os.ReadFile(filePath)
			if err != nil {
				return fmt.Errorf("failed to read migration %s: %w", file.Name(), err)
			}

			log.Printf("Applying database migration: %s", file.Name())
			if _, err := db.Pool.Exec(ctx, string(sqlBytes)); err != nil {
				log.Printf("Notice on %s: %v (might already exist)", file.Name(), err)
			}
		}
	}

	return nil
}
