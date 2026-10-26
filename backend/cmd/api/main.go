package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/siteyonet/siteyonet/internal/config"
	"github.com/siteyonet/siteyonet/internal/handler"
	"github.com/siteyonet/siteyonet/internal/middleware"
	"github.com/siteyonet/siteyonet/internal/repository"
	"github.com/siteyonet/siteyonet/internal/service"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	cfg := config.Load()
	middleware.JWTSecret = []byte(cfg.JWTSecret)
	log.Printf("Starting SiteYonet API Server on port %s (env: %s)...", cfg.Port, cfg.Environment)

	// Initialize DB Connection
	db, err := repository.NewDB(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Fatal: Database initialization failed: %v", err)
	}
	defer db.Pool.Close()

	// Run Database Migrations Automatically
	if err := db.Migrate("migrations"); err != nil {
		log.Fatalf("Migration failed: %v", err)
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
