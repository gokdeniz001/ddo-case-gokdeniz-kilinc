package handler

import (
	"encoding/json"
	"net/http"

	"github.com/siteyonet/siteyonet/internal/middleware"
)

func (h *Handler) SaveBudget(w http.ResponseWriter, r *http.Request) {
	u := middleware.GetCurrentUser(r.Context())
	if u.Role != "manager" && u.Role != "accountant" && u.Role != "superadmin" {
		middleware.RespondError(w, 403, "FORBIDDEN", "Yetkisiz işlem")
		return
	}
	var req struct {
		Category string `json:"category"`
		Amount   int64  `json:"amount_cents"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Category == "" || req.Amount < 0 {
		middleware.RespondError(w, 400, "BAD_REQUEST", "Geçersiz bütçe")
		return
	}
	if _, err := h.db.Executor(r.Context()).Exec(r.Context(), `INSERT INTO "fy-budgets" (site_id,category,amount_cents) VALUES ($1,$2,$3) ON CONFLICT (site_id,category) DO UPDATE SET amount_cents=EXCLUDED.amount_cents`, u.SiteID, req.Category, req.Amount); err != nil {
		middleware.RespondError(w, 500, "DB_ERROR", err.Error())
		return
	}
	// The budget key has no UUID; keep the changed values in the append-only audit record.
	h.db.LogAudit(r.Context(), "SAVE_BUDGET", "fy-budgets", u.SiteID.String(), nil, req)
	respondJSON(w, 200, req)
}

func (h *Handler) Reconciliation(w http.ResponseWriter, r *http.Request) {
	u := middleware.GetCurrentUser(r.Context())
	if u.Role == "owner" || u.Role == "tenant" {
		middleware.RespondError(w, 403, "FORBIDDEN", "Yetkisiz işlem")
		return
	}
	var bank, cash, statement int64
	if err := h.db.Executor(r.Context()).QueryRow(r.Context(), `SELECT COALESCE(sum(l.debit_cents-l.credit_cents) FILTER (WHERE a.code='102'),0),COALESCE(sum(l.debit_cents-l.credit_cents) FILTER (WHERE a.code='100'),0) FROM "fy-journal-lines" l JOIN "fy-ledger-accounts" a ON a.id=l.account_id WHERE a.site_id=$1`, u.SiteID).Scan(&bank, &cash); err != nil {
		middleware.RespondError(w, 500, "DB_ERROR", err.Error())
		return
	}
	if err := h.db.Executor(r.Context()).QueryRow(r.Context(), `SELECT COALESCE(sum(amount_cents),0) FROM "fy-bank-statements" WHERE site_id=$1`, u.SiteID).Scan(&statement); err != nil {
		middleware.RespondError(w, 500, "DB_ERROR", err.Error())
		return
	}
	respondJSON(w, 200, map[string]interface{}{"book_bank_cents": bank, "book_cash_cents": cash, "imported_statement_net_cents": statement, "difference_cents": statement - bank, "reconciled": statement == bank})
}
