package handler

import (
	"net/http"
	"net/http/httptest"

	"github.com/siteyonet/siteyonet/internal/middleware"
)

// Buffer the response until commit succeeds: callers never see success for a
// rolled back transaction. Every protected query uses a restricted PostgreSQL role.
func (h *Handler) TenantTransaction(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, tx, err := h.db.BeginTenant(r.Context())
		if err != nil {
			middleware.RespondError(w, 500, "DB_TX_ERROR", "Ä°ÅŸlem baÅŸlatÄ±lamadÄ±")
			return
		}
		defer func() { _ = tx.Rollback(r.Context()) }()
		// Serialize mutations within a site before reading balances or period status.
		// This deliberately favors correctness over write throughput for the demo.
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,1))`, middleware.GetCurrentUser(ctx).SiteID.String()); err != nil {
				middleware.RespondError(w, 500, "DB_ERROR", "İşlem kilidi alınamadı")
				return
			}
		}
		rec := httptest.NewRecorder()
		next.ServeHTTP(rec, r.WithContext(ctx))
		if rec.Code < 400 {
			if err := tx.Commit(ctx); err != nil {
				middleware.RespondError(w, 500, "DB_TX_ERROR", "Ä°ÅŸlem kaydedilemedi")
				return
			}
		}
		for k, values := range rec.Header() {
			w.Header()[k] = values
		}
		w.WriteHeader(rec.Code)
		_, _ = w.Write(rec.Body.Bytes())
	})
}
