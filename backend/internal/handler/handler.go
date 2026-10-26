package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	appMiddleware "github.com/siteyonet/siteyonet/internal/middleware"
	"github.com/siteyonet/siteyonet/internal/repository"
	"github.com/siteyonet/siteyonet/internal/service"
)

type Handler struct {
	db                 *repository.DB
	accrualEngine      *service.AccrualEngine
	paymentService     *service.PaymentService
	accountingService  *service.AccountingService
	certificateService *service.CertificateService
}

func NewHandler(
	db *repository.DB,
	accrualEngine *service.AccrualEngine,
	paymentService *service.PaymentService,
	accountingService *service.AccountingService,
	certificateService *service.CertificateService,
) *Handler {
	return &Handler{
		db:                 db,
		accrualEngine:      accrualEngine,
		paymentService:     paymentService,
		accountingService:  accountingService,
		certificateService: certificateService,
	}
}

func (h *Handler) InitRoutes() http.Handler {
	r := chi.NewRouter()

	// Global Middlewares
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))
	r.Use(appMiddleware.DDOHeaderMiddleware) // Requirement: X-DDO-Build: 0c7cc1 on every response

	// CORS
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token", "Idempotency-Key", "X-DDO-Build"},
		ExposedHeaders:   []string{"Link", "X-DDO-Build", "Idempotency-Key"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Special Health Endpoints
	r.Get("/health", h.HealthCheck)
	r.Get("/_hz/0c7c", h.DDOHealthCheck) // Requirement: GET /_hz/0c7c -> 200 OK

	// Public verification endpoint for clearance certificates
	r.Get("/api/v1/certificates/verify/{code}", h.VerifyCertificate)

	// Auth Public Routes
	r.Route("/api/v1/auth", func(r chi.Router) {
		r.Post("/login", h.Login)
	})

	// Protected API Routes
	r.Route("/api/v1", func(r chi.Router) {
		r.Use(appMiddleware.AuthMiddleware)
		r.Use(appMiddleware.IdempotencyMiddleware(h.db.Pool))

		// Current User
		r.Get("/auth/me", h.GetMe)

		// F-1: Sites, Blocks, Units
		r.Get("/sites", h.ListSites)
		r.Get("/sites/{id}", h.GetSite)
		r.Get("/blocks", h.ListBlocks)
		r.Post("/blocks", h.CreateBlock)
		r.Get("/units", h.ListUnits)
		r.Post("/units", h.CreateUnit)

		// F-2: Dues Definitions & Accruals (T-1 Accrual Engine)
		r.Get("/dues-definitions", h.ListDuesDefinitions)
		r.Post("/dues-definitions", h.CreateDuesDefinition)
		r.Get("/accruals", h.ListAccruals)
		r.Post("/accruals", h.CreateAccrual) // T-1: Largest Remainder cent allocation
		r.Post("/accruals/{id}/reverse", h.ReverseAccrual)

		// F-3: Debts, Payments, Restructuring (T-2 Payment Allocation & Fuzzy matching)
		r.Get("/debts", h.ListDebts)
		r.Post("/payments", h.ProcessPayment) // T-2: Waterfall allocation
		r.Post("/debts/restructure", h.RestructureDebts)
		r.Post("/bank-statements/match", h.FuzzyMatchBankStatement) // T-2: Fuzzy match
		r.Get("/bank-statements", h.ListBankStatements)

		// F-4: Expenses
		r.Get("/expenses", h.ListExpenses)
		r.Post("/expenses", h.CreateExpense)

		// F-5: Announcements, Decisions, Maintenance Requests
		r.Get("/announcements", h.ListAnnouncements)
		r.Post("/announcements", h.CreateAnnouncement)
		r.Get("/decisions", h.ListDecisions)
		r.Post("/decisions", h.CreateDecision)
		r.Get("/maintenance", h.ListMaintenanceRequests)
		r.Post("/maintenance", h.CreateMaintenanceRequest)
		r.Put("/maintenance/{id}/status", h.UpdateMaintenanceStatus)

		// F-6: Accounting Periods & Double-entry Ledger (T-4)
		r.Get("/periods", h.ListFiscalPeriods)
		r.Post("/periods", h.CreateFiscalPeriod)
		r.Post("/periods/{id}/close", h.CloseFiscalPeriod)
		r.Get("/ledger-accounts", h.ListLedgerAccounts)
		r.Get("/journal-entries", h.ListJournalEntries)
		r.Post("/journal-entries", h.CreateJournalEntry) // T-4: Double-entry validation
		r.Post("/journal-entries/{id}/reverse", h.ReverseJournalEntry)

		// F-7: Reports & Clearance Certificates (T-4)
		r.Get("/reports/summary", h.GetFinancialSummary)
		r.Get("/reports/budget-comparison", h.GetBudgetComparison)
		r.Get("/certificates", h.ListCertificates)
		r.Post("/certificates", h.IssueClearanceCertificate)

		// Audit Logs (Auditor / Manager)
		r.Get("/audit-logs", h.ListAuditLogs)
	})

	return r
}

// ----------------------------------------------------
// Health Checks
// ----------------------------------------------------
func (h *Handler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	err := h.db.CheckHealth(ctx)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"status":   "unhealthy",
			"database": "disconnected",
			"error":    err.Error(),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status":    "healthy",
		"database":  "connected",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"version":   "1.0.0",
	})
}

func (h *Handler) DDOHealthCheck(w http.ResponseWriter, r *http.Request) {
	// Requirement: GET /_hz/0c7c returns 200 OK
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("OK 0c7c"))
}

// Helper to respond with JSON
func respondJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(data)
}
