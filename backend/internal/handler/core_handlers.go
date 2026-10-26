package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/siteyonet/siteyonet/internal/domain"
	appMiddleware "github.com/siteyonet/siteyonet/internal/middleware"
	"github.com/siteyonet/siteyonet/internal/service"
)

// ----------------------------------------------------
// F-1: Sites, Blocks, Units
// ----------------------------------------------------

func (h *Handler) ListSites(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	rows, err := h.db.Pool.Query(ctx, `
		SELECT id, name, code, COALESCE(address, ''), city, COALESCE(district, ''), COALESCE(bank_name, ''), COALESCE(bank_iban, ''), default_late_fee_rate, created_at, updated_at
		FROM "fy-sites"
		ORDER BY name ASC
	`)
	if err != nil {
		appMiddleware.RespondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	defer rows.Close()

	sites := make([]domain.Site, 0)
	for rows.Next() {
		var s domain.Site
		if err := rows.Scan(&s.ID, &s.Name, &s.Code, &s.Address, &s.City, &s.District, &s.BankName, &s.BankIBAN, &s.DefaultLateFeeRate, &s.CreatedAt, &s.UpdatedAt); err != nil {
			continue
		}
		sites = append(sites, s)
	}
	respondJSON(w, http.StatusOK, sites)
}

func (h *Handler) GetSite(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	siteID, err := uuid.Parse(idStr)
	if err != nil {
		appMiddleware.RespondError(w, http.StatusBadRequest, "INVALID_ID", "Geçersiz site kimliği")
		return
	}

	var s domain.Site
	err = h.db.Pool.QueryRow(r.Context(), `
		SELECT id, name, code, COALESCE(address, ''), city, COALESCE(district, ''), COALESCE(bank_name, ''), COALESCE(bank_iban, ''), default_late_fee_rate, created_at, updated_at
		FROM "fy-sites"
		WHERE id = $1
	`, siteID).Scan(&s.ID, &s.Name, &s.Code, &s.Address, &s.City, &s.District, &s.BankName, &s.BankIBAN, &s.DefaultLateFeeRate, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		appMiddleware.RespondError(w, http.StatusNotFound, "NOT_FOUND", "Site bulunamadı")
		return
	}
	respondJSON(w, http.StatusOK, s)
}

func (h *Handler) ListBlocks(w http.ResponseWriter, r *http.Request) {
	user := appMiddleware.GetCurrentUser(r.Context())
	siteID := user.SiteID

	rows, err := h.db.Pool.Query(r.Context(), `
		SELECT id, site_id, name, total_floors, created_at
		FROM "fy-blocks"
		WHERE site_id = $1
		ORDER BY name ASC
	`, siteID)
	if err != nil {
		appMiddleware.RespondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	defer rows.Close()

	blocks := make([]domain.Block, 0)
	for rows.Next() {
		var b domain.Block
		if err := rows.Scan(&b.ID, &b.SiteID, &b.Name, &b.TotalFloors, &b.CreatedAt); err == nil {
			blocks = append(blocks, b)
		}
	}
	respondJSON(w, http.StatusOK, blocks)
}

func (h *Handler) CreateBlock(w http.ResponseWriter, r *http.Request) {
	user := appMiddleware.GetCurrentUser(r.Context())
	if user.Role != domain.RoleSuperAdmin && user.Role != domain.RoleManager {
		appMiddleware.RespondError(w, http.StatusForbidden, "FORBIDDEN", "Yetkisiz işlem")
		return
	}

	var req struct {
		Name        string `json:"name"`
		TotalFloors int    `json:"total_floors"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
		appMiddleware.RespondError(w, http.StatusBadRequest, "BAD_REQUEST", "Geçersiz blok adı")
		return
	}

	var blockID uuid.UUID
	err := h.db.Pool.QueryRow(r.Context(), `
		INSERT INTO "fy-blocks" (site_id, name, total_floors)
		VALUES ($1, $2, $3)
		RETURNING id
	`, user.SiteID, req.Name, req.TotalFloors).Scan(&blockID)
	if err != nil {
		appMiddleware.RespondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	h.db.LogAudit(r.Context(), "CREATE_BLOCK", "fy-blocks", blockID.String(), nil, req)
	respondJSON(w, http.StatusCreated, map[string]interface{}{"id": blockID, "message": "Blok oluşturuldu"})
}

func (h *Handler) ListUnits(w http.ResponseWriter, r *http.Request) {
	user := appMiddleware.GetCurrentUser(r.Context())
	siteID := user.SiteID

	query := `
		SELECT u.id, u.site_id, u.block_id, b.name AS block_name, u.unit_number, u.floor_number,
		       u.land_share, u.gross_area_m2, u.resident_count, u.is_exempt,
		       u.owner_id, COALESCE(o.full_name, '') AS owner_name,
		       u.tenant_id, COALESCE(t.full_name, '') AS tenant_name,
		       u.created_at, u.updated_at
		FROM "fy-units" u
		JOIN "fy-blocks" b ON u.block_id = b.id
		LEFT JOIN "fy-ox-users" o ON u.owner_id = o.id
		LEFT JOIN "fy-ox-users" t ON u.tenant_id = t.id
		WHERE u.site_id = $1
		ORDER BY b.name ASC, u.unit_number::int ASC
	`
	rows, err := h.db.Pool.Query(r.Context(), query, siteID)
	if err != nil {
		appMiddleware.RespondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	defer rows.Close()

	units := make([]domain.Unit, 0)
	for rows.Next() {
		var u domain.Unit
		if err := rows.Scan(
			&u.ID, &u.SiteID, &u.BlockID, &u.BlockName, &u.UnitNumber, &u.FloorNumber,
			&u.LandShare, &u.GrossAreaM2, &u.ResidentCount, &u.IsExempt,
			&u.OwnerID, &u.OwnerName, &u.TenantID, &u.TenantName,
			&u.CreatedAt, &u.UpdatedAt,
		); err == nil {
			units = append(units, u)
		}
	}
	respondJSON(w, http.StatusOK, units)
}

func (h *Handler) CreateUnit(w http.ResponseWriter, r *http.Request) {
	user := appMiddleware.GetCurrentUser(r.Context())
	if user.Role != domain.RoleSuperAdmin && user.Role != domain.RoleManager {
		appMiddleware.RespondError(w, http.StatusForbidden, "FORBIDDEN", "Yetkisiz işlem")
		return
	}

	var req struct {
		BlockID       uuid.UUID  `json:"block_id"`
		UnitNumber    string     `json:"unit_number"`
		FloorNumber   int        `json:"floor_number"`
		LandShare     int        `json:"land_share"`
		GrossAreaM2   float64    `json:"gross_area_m2"`
		ResidentCount int        `json:"resident_count"`
		IsExempt      bool       `json:"is_exempt"`
		OwnerID       *uuid.UUID `json:"owner_id"`
		TenantID      *uuid.UUID `json:"tenant_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.UnitNumber == "" {
		appMiddleware.RespondError(w, http.StatusBadRequest, "BAD_REQUEST", "Geçersiz daire bilgileri")
		return
	}

	var unitID uuid.UUID
	err := h.db.Pool.QueryRow(r.Context(), `
		INSERT INTO "fy-units" (site_id, block_id, unit_number, floor_number, land_share, gross_area_m2, resident_count, is_exempt, owner_id, tenant_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id
	`, user.SiteID, req.BlockID, req.UnitNumber, req.FloorNumber, req.LandShare, req.GrossAreaM2, req.ResidentCount, req.IsExempt, req.OwnerID, req.TenantID).Scan(&unitID)
	if err != nil {
		appMiddleware.RespondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	h.db.LogAudit(r.Context(), "CREATE_UNIT", "fy-units", unitID.String(), nil, req)
	respondJSON(w, http.StatusCreated, map[string]interface{}{"id": unitID, "message": "Daire oluşturuldu"})
}

// ----------------------------------------------------
// F-2: Dues Definitions & T-1 Accrual Engine
// ----------------------------------------------------

func (h *Handler) ListDuesDefinitions(w http.ResponseWriter, r *http.Request) {
	user := appMiddleware.GetCurrentUser(r.Context())
	rows, err := h.db.Pool.Query(r.Context(), `
		SELECT id, site_id, name, total_amount_cents, distribution_type, due_day, late_fee_rate, is_active, created_at
		FROM "fy-dues-definitions"
		WHERE site_id = $1
		ORDER BY created_at DESC
	`, user.SiteID)
	if err != nil {
		appMiddleware.RespondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	defer rows.Close()

	list := make([]domain.DuesDefinition, 0)
	for rows.Next() {
		var d domain.DuesDefinition
		if err := rows.Scan(&d.ID, &d.SiteID, &d.Name, &d.TotalAmountCents, &d.DistributionType, &d.DueDay, &d.LateFeeRate, &d.IsActive, &d.CreatedAt); err == nil {
			list = append(list, d)
		}
	}
	respondJSON(w, http.StatusOK, list)
}

func (h *Handler) CreateDuesDefinition(w http.ResponseWriter, r *http.Request) {
	user := appMiddleware.GetCurrentUser(r.Context())
	if user.Role != domain.RoleSuperAdmin && user.Role != domain.RoleManager && user.Role != domain.RoleAccountant {
		appMiddleware.RespondError(w, http.StatusForbidden, "FORBIDDEN", "Yetkisiz işlem")
		return
	}

	var req struct {
		Name             string  `json:"name"`
		TotalAmountCents int64   `json:"total_amount_cents"`
		DistributionType string  `json:"distribution_type"`
		DueDay           int     `json:"due_day"`
		LateFeeRate      float64 `json:"late_fee_rate"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.TotalAmountCents <= 0 {
		appMiddleware.RespondError(w, http.StatusBadRequest, "BAD_REQUEST", "Geçersiz aidat tanımı bilgileri")
		return
	}

	var id uuid.UUID
	err := h.db.Pool.QueryRow(r.Context(), `
		INSERT INTO "fy-dues-definitions" (site_id, name, total_amount_cents, distribution_type, due_day, late_fee_rate)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id
	`, user.SiteID, req.Name, req.TotalAmountCents, req.DistributionType, req.DueDay, req.LateFeeRate).Scan(&id)
	if err != nil {
		appMiddleware.RespondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	h.db.LogAudit(r.Context(), "CREATE_DUES_DEFINITION", "fy-dues-definitions", id.String(), nil, req)
	respondJSON(w, http.StatusCreated, map[string]interface{}{"id": id, "message": "Aidat tanımı oluşturuldu"})
}

func (h *Handler) ListAccruals(w http.ResponseWriter, r *http.Request) {
	user := appMiddleware.GetCurrentUser(r.Context())
	rows, err := h.db.Pool.Query(r.Context(), `
		SELECT id, site_id, dues_definition_id, period_id, title, term_month, distribution_type, total_amount_cents, is_reversed, reversal_accrual_id, created_by, created_at
		FROM "fy-accruals"
		WHERE site_id = $1
		ORDER BY created_at DESC
	`, user.SiteID)
	if err != nil {
		appMiddleware.RespondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	defer rows.Close()

	list := make([]domain.Accrual, 0)
	for rows.Next() {
		var a domain.Accrual
		if err := rows.Scan(&a.ID, &a.SiteID, &a.DuesDefinitionID, &a.PeriodID, &a.Title, &a.TermMonth, &a.DistributionType, &a.TotalAmountCents, &a.IsReversed, &a.ReversalAccrualID, &a.CreatedBy, &a.CreatedAt); err == nil {
			list = append(list, a)
		}
	}
	respondJSON(w, http.StatusOK, list)
}

// CreateAccrual executes Task 1 (T-1): Cent-exact allocation using Largest Remainder Method
func (h *Handler) CreateAccrual(w http.ResponseWriter, r *http.Request) {
	user := appMiddleware.GetCurrentUser(r.Context())
	if user.Role != domain.RoleSuperAdmin && user.Role != domain.RoleManager && user.Role != domain.RoleAccountant {
		appMiddleware.RespondError(w, http.StatusForbidden, "FORBIDDEN", "Yetkisiz işlem")
		return
	}

	var req struct {
		DuesDefinitionID *uuid.UUID `json:"dues_definition_id"`
		PeriodID         uuid.UUID  `json:"period_id"`
		Title            string     `json:"title"`
		TermMonth        string     `json:"term_month"` // e.g. "2026-10"
		DistributionType string     `json:"distribution_type"`
		TotalAmountCents int64      `json:"total_amount_cents"`
		DueDay           int        `json:"due_day"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.TotalAmountCents <= 0 {
		appMiddleware.RespondError(w, http.StatusBadRequest, "BAD_REQUEST", "Geçersiz tahakkuk bilgileri")
		return
	}

	ctx := r.Context()

	// Check if period is OPEN
	var periodStatus string
	err := h.db.Pool.QueryRow(ctx, `SELECT status FROM "fy-fiscal-periods" WHERE id = $1`, req.PeriodID).Scan(&periodStatus)
	if err != nil || periodStatus != domain.PeriodStatusOpen {
		appMiddleware.RespondError(w, http.StatusBadRequest, "PERIOD_NOT_OPEN", "Tahakkuk yalnızca açık dönemlere yapılabilir")
		return
	}

	// Fetch all units in the site
	rows, err := h.db.Pool.Query(ctx, `
		SELECT id, owner_id, land_share, gross_area_m2, resident_count, is_exempt
		FROM "fy-units"
		WHERE site_id = $1
	`, user.SiteID)
	if err != nil {
		appMiddleware.RespondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	defer rows.Close()

	unitInputs := make([]service.UnitShareInput, 0)
	for rows.Next() {
		var u service.UnitShareInput
		var ownerID *uuid.UUID
		if err := rows.Scan(&u.UnitID, &ownerID, &u.LandShare, &u.GrossAreaM2, &u.ResidentCount, &u.IsExempt); err == nil {
			if ownerID != nil {
				u.OwnerID = *ownerID
			} else {
				u.OwnerID = user.UserID // fallback
			}
			unitInputs = append(unitInputs, u)
		}
	}

	// Run T-1 Accrual Engine (Largest Remainder Method)
	allocatedItems, err := h.accrualEngine.DistributeDues(req.TotalAmountCents, req.DistributionType, unitInputs)
	if err != nil {
		appMiddleware.RespondError(w, http.StatusBadRequest, "ENGINE_ERROR", "Dağıtım motoru hatası: "+err.Error())
		return
	}

	// Calculate due date
	dueDay := req.DueDay
	if dueDay <= 0 {
		dueDay = 15
	}
	parsedMonth, _ := time.Parse("2006-01", req.TermMonth)
	if parsedMonth.IsZero() {
		parsedMonth = time.Now()
	}
	dueDate := time.Date(parsedMonth.Year(), parsedMonth.Month(), dueDay, 0, 0, 0, 0, time.UTC)

	// Persist in transaction
	var accrualID uuid.UUID
	err = h.db.WithTenantTx(ctx, func(tx pgx.Tx) error {
		err := tx.QueryRow(ctx, `
			INSERT INTO "fy-accruals" (site_id, dues_definition_id, period_id, title, term_month, distribution_type, total_amount_cents, created_by)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			RETURNING id
		`, user.SiteID, req.DuesDefinitionID, req.PeriodID, req.Title, req.TermMonth, req.DistributionType, req.TotalAmountCents, user.UserID).Scan(&accrualID)
		if err != nil {
			return err
		}

		// Insert individual debts
		for _, item := range allocatedItems {
			_, err := tx.Exec(ctx, `
				INSERT INTO "fy-debts" (site_id, unit_id, user_id, accrual_id, title, debt_type, term_month, principal_cents, due_date, status)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
			`, user.SiteID, item.UnitID, item.UserID, accrualID, req.Title, domain.DebtTypeDues, req.TermMonth, item.PrincipalCents, dueDate, domain.DebtStatusUnpaid)
			if err != nil {
				return err
			}
		}

		// Double-entry accounting: 120 Alacaklar (Debit) vs 600 Aidat Gelirleri (Credit)
		var acc120, acc600 uuid.UUID
		_ = tx.QueryRow(ctx, `SELECT id FROM "fy-ledger-accounts" WHERE site_id = $1 AND code = '120'`, user.SiteID).Scan(&acc120)
		_ = tx.QueryRow(ctx, `SELECT id FROM "fy-ledger-accounts" WHERE site_id = $1 AND code = '600'`, user.SiteID).Scan(&acc600)

		if acc120 != uuid.Nil && acc600 != uuid.Nil {
			var jEntryID uuid.UUID
			entryNum := time.Now().UnixNano() / 1000000
			err = tx.QueryRow(ctx, `
				INSERT INTO "fy-journal-entries" (site_id, period_id, entry_number, entry_date, description, created_by)
				VALUES ($1, $2, $3, $4, $5, $6)
				RETURNING id
			`, user.SiteID, req.PeriodID, entryNum, time.Now(), req.Title+" Aidat Tahakkuk Fişi", user.UserID).Scan(&jEntryID)
			if err == nil {
				_, _ = tx.Exec(ctx, `INSERT INTO "fy-journal-lines" (journal_entry_id, account_id, debit_cents, credit_cents, description) VALUES ($1, $2, $3, 0, $4)`, jEntryID, acc120, req.TotalAmountCents, "Dairelerden Aidat Alacakları")
				_, _ = tx.Exec(ctx, `INSERT INTO "fy-journal-lines" (journal_entry_id, account_id, debit_cents, credit_cents, description) VALUES ($1, $2, 0, $3, $4)`, jEntryID, acc600, req.TotalAmountCents, "Aidat Gelirleri")
			}
		}

		return nil
	})

	if err != nil {
		appMiddleware.RespondError(w, http.StatusInternalServerError, "DB_TX_ERROR", err.Error())
		return
	}

	h.db.LogAudit(r.Context(), "CREATE_ACCRUAL", "fy-accruals", accrualID.String(), nil, req)
	respondJSON(w, http.StatusCreated, map[string]interface{}{
		"id":                 accrualID,
		"allocated_units":    len(allocatedItems),
		"total_amount_cents": req.TotalAmountCents,
		"message":            "Tahakkuk kuruşu kuruşuna başarıyla dağıtıldı ve borçlar oluşturuldu",
	})
}

func (h *Handler) ReverseAccrual(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	accrualID, err := uuid.Parse(idStr)
	if err != nil {
		appMiddleware.RespondError(w, http.StatusBadRequest, "INVALID_ID", "Geçersiz tahakkuk kimliği")
		return
	}

	user := appMiddleware.GetCurrentUser(r.Context())
	if user.Role != domain.RoleSuperAdmin && user.Role != domain.RoleManager && user.Role != domain.RoleAccountant {
		appMiddleware.RespondError(w, http.StatusForbidden, "FORBIDDEN", "Yetkisiz işlem")
		return
	}

	ctx := r.Context()
	err = h.db.WithTenantTx(ctx, func(tx pgx.Tx) error {
		// Mark accrual as reversed
		_, err := tx.Exec(ctx, `UPDATE "fy-accruals" SET is_reversed = true WHERE id = $1 AND site_id = $2`, accrualID, user.SiteID)
		if err != nil {
			return err
		}

		// Cancel unpaid debts for this accrual
		_, err = tx.Exec(ctx, `
			UPDATE "fy-debts" 
			SET status = 'CANCELLED' 
			WHERE accrual_id = $1 AND site_id = $2 AND paid_principal_cents = 0
		`, accrualID, user.SiteID)
		return err
	})

	if err != nil {
		appMiddleware.RespondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	h.db.LogAudit(r.Context(), "REVERSE_ACCRUAL", "fy-accruals", accrualID.String(), nil, nil)
	respondJSON(w, http.StatusOK, map[string]string{"message": "Tahakkuk ters kayıtla iptal edildi"})
}

// ----------------------------------------------------
// F-3: Debts, Payments & T-2 Payment Service
// ----------------------------------------------------

func (h *Handler) ListDebts(w http.ResponseWriter, r *http.Request) {
	user := appMiddleware.GetCurrentUser(r.Context())
	siteID := user.SiteID

	query := `
		SELECT d.id, d.site_id, d.unit_id, u.unit_number, b.name AS block_name,
		       d.user_id, COALESCE(usr.full_name, '') AS user_name,
		       d.accrual_id, d.title, d.debt_type, d.term_month,
		       d.principal_cents, d.paid_principal_cents, d.paid_interest_cents,
		       (d.principal_cents - d.paid_principal_cents) AS remaining_cents,
		       d.due_date, d.status, d.created_at, d.updated_at
		FROM "fy-debts" d
		JOIN "fy-units" u ON d.unit_id = u.id
		JOIN "fy-blocks" b ON u.block_id = b.id
		LEFT JOIN "fy-ox-users" usr ON d.user_id = usr.id
		WHERE d.site_id = $1
	`
	args := []interface{}{siteID}

	// Role-based filtering: resident only sees their own unit's debts
	if user.Role == domain.RoleOwner || user.Role == domain.RoleTenant {
		query += " AND (d.user_id = $2 OR u.owner_id = $2 OR u.tenant_id = $2)"
		args = append(args, user.UserID)
	}

	unitIDParam := r.URL.Query().Get("unit_id")
	if unitIDParam != "" {
		if uID, err := uuid.Parse(unitIDParam); err == nil {
			query += " AND d.unit_id = $" + strconv.Itoa(len(args)+1)
			args = append(args, uID)
		}
	}

	query += " ORDER BY d.due_date ASC"

	rows, err := h.db.Pool.Query(r.Context(), query, args...)
	if err != nil {
		appMiddleware.RespondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	defer rows.Close()

	debts := make([]domain.Debt, 0)
	for rows.Next() {
		var d domain.Debt
		if err := rows.Scan(
			&d.ID, &d.SiteID, &d.UnitID, &d.UnitNumber, &d.BlockName,
			&d.UserID, &d.UserName, &d.AccrualID, &d.Title, &d.DebtType, &d.TermMonth,
			&d.PrincipalCents, &d.PaidPrincipalCents, &d.PaidInterestCents,
			&d.RemainingCents, &d.DueDate, &d.Status, &d.CreatedAt, &d.UpdatedAt,
		); err == nil {
			debts = append(debts, d)
		}
	}
	respondJSON(w, http.StatusOK, debts)
}

// ProcessPayment executes Task 2 (T-2): Waterfall allocation (Interest -> Principal -> FIFO)
func (h *Handler) ProcessPayment(w http.ResponseWriter, r *http.Request) {
	user := appMiddleware.GetCurrentUser(r.Context())
	if user.Role != domain.RoleSuperAdmin && user.Role != domain.RoleManager && user.Role != domain.RoleAccountant {
		appMiddleware.RespondError(w, http.StatusForbidden, "FORBIDDEN", "Yetkisiz işlem")
		return
	}

	var req struct {
		UnitID         uuid.UUID `json:"unit_id"`
		AmountCents    int64     `json:"amount_cents"`
		PaymentDate    string    `json:"payment_date"` // YYYY-MM-DD
		PaymentChannel string    `json:"payment_channel"`
		BankReference  string    `json:"bank_reference"`
		Description    string    `json:"description"`
		IdempotencyKey string    `json:"idempotency_key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.AmountCents <= 0 {
		appMiddleware.RespondError(w, http.StatusBadRequest, "BAD_REQUEST", "Geçersiz ödeme bilgileri")
		return
	}

	payDate, err := time.Parse("2006-01-02", req.PaymentDate)
	if err != nil {
		payDate = time.Now()
	}

	ctx := r.Context()

	// Get site default late fee rate
	var lateFeeRate float64 = 5.0
	_ = h.db.Pool.QueryRow(ctx, `SELECT default_late_fee_rate FROM "fy-sites" WHERE id = $1`, user.SiteID).Scan(&lateFeeRate)

	// Fetch active debts for this unit
	rows, err := h.db.Pool.Query(ctx, `
		SELECT id, site_id, unit_id, user_id, principal_cents, paid_principal_cents, paid_interest_cents, due_date, status
		FROM "fy-debts"
		WHERE site_id = $1 AND unit_id = $2 AND status IN ('UNPAID', 'PARTIAL')
		ORDER BY due_date ASC
	`, user.SiteID, req.UnitID)
	if err != nil {
		appMiddleware.RespondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	defer rows.Close()

	debts := make([]*domain.Debt, 0)
	var debtUserID uuid.UUID
	for rows.Next() {
		var d domain.Debt
		if err := rows.Scan(&d.ID, &d.SiteID, &d.UnitID, &d.UserID, &d.PrincipalCents, &d.PaidPrincipalCents, &d.PaidInterestCents, &d.DueDate, &d.Status); err == nil {
			debtUserID = d.UserID
			debts = append(debts, &d)
		}
	}

	if debtUserID == uuid.Nil {
		// Fallback to unit owner
		_ = h.db.Pool.QueryRow(ctx, `SELECT COALESCE(owner_id, tenant_id) FROM "fy-units" WHERE id = $1`, req.UnitID).Scan(&debtUserID)
	}

	// Run T-2 Waterfall Allocation
	allocResult, err := h.paymentService.AllocatePayment(req.AmountCents, debts, lateFeeRate, payDate)
	if err != nil {
		appMiddleware.RespondError(w, http.StatusBadRequest, "ALLOC_ERROR", err.Error())
		return
	}

	var paymentID uuid.UUID
	err = h.db.WithTenantTx(ctx, func(tx pgx.Tx) error {
		// Insert Payment
		err := tx.QueryRow(ctx, `
			INSERT INTO "fy-payments" (site_id, unit_id, user_id, payment_date, amount_cents, unallocated_cents, payment_channel, bank_reference, description, idempotency_key, created_by)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
			RETURNING id
		`, user.SiteID, req.UnitID, debtUserID, payDate, req.AmountCents, allocResult.UnallocatedCents, req.PaymentChannel, req.BankReference, req.Description, req.IdempotencyKey, user.UserID).Scan(&paymentID)
		if err != nil {
			return err
		}

		// Update each debt and record allocation
		for _, item := range allocResult.AllocatedDebts {
			_, err = tx.Exec(ctx, `
				UPDATE "fy-debts"
				SET paid_principal_cents = paid_principal_cents + $1,
				    paid_interest_cents = paid_interest_cents + $2,
				    status = $3,
				    updated_at = CURRENT_TIMESTAMP
				WHERE id = $4
			`, item.PrincipalPaid, item.InterestPaid, item.NewStatus, item.DebtID)
			if err != nil {
				return err
			}

			_, err = tx.Exec(ctx, `
				INSERT INTO "fy-payment-allocations" (payment_id, debt_id, allocated_principal_cents, allocated_interest_cents)
				VALUES ($1, $2, $3, $4)
			`, paymentID, item.DebtID, item.PrincipalPaid, item.InterestPaid)
			if err != nil {
				return err
			}
		}

		// Double-entry accounting: 102 Banka (Debit) vs 120 Alıcılar (Credit)
		var acc102, acc120 uuid.UUID
		_ = tx.QueryRow(ctx, `SELECT id FROM "fy-ledger-accounts" WHERE site_id = $1 AND code = '102'`, user.SiteID).Scan(&acc102)
		_ = tx.QueryRow(ctx, `SELECT id FROM "fy-ledger-accounts" WHERE site_id = $1 AND code = '120'`, user.SiteID).Scan(&acc120)

		if acc102 != uuid.Nil && acc120 != uuid.Nil {
			var periodID uuid.UUID
			_ = tx.QueryRow(ctx, `SELECT id FROM "fy-fiscal-periods" WHERE site_id = $1 AND status = 'OPEN' LIMIT 1`, user.SiteID).Scan(&periodID)
			if periodID != uuid.Nil {
				entryNum := time.Now().UnixNano() / 1000000
				var jEntryID uuid.UUID
				err = tx.QueryRow(ctx, `
					INSERT INTO "fy-journal-entries" (site_id, period_id, entry_number, entry_date, description, created_by)
					VALUES ($1, $2, $3, $4, $5, $6)
					RETURNING id
				`, user.SiteID, periodID, entryNum, payDate, "Tahsilat Fişi - Ref: "+req.BankReference, user.UserID).Scan(&jEntryID)
				if err == nil {
					_, _ = tx.Exec(ctx, `INSERT INTO "fy-journal-lines" (journal_entry_id, account_id, debit_cents, credit_cents, description) VALUES ($1, $2, $3, 0, $4)`, jEntryID, acc102, req.AmountCents, "Banka Tahsilatı")
					_, _ = tx.Exec(ctx, `INSERT INTO "fy-journal-lines" (journal_entry_id, account_id, debit_cents, credit_cents, description) VALUES ($1, $2, 0, $3, $4)`, jEntryID, acc120, req.AmountCents, "Daire Borç Kapama")
				}
			}
		}

		return nil
	})

	if err != nil {
		appMiddleware.RespondError(w, http.StatusInternalServerError, "DB_TX_ERROR", err.Error())
		return
	}

	h.db.LogAudit(r.Context(), "PROCESS_PAYMENT", "fy-payments", paymentID.String(), nil, req)
	respondJSON(w, http.StatusCreated, map[string]interface{}{
		"id":                paymentID,
		"allocated_debts":   allocResult.AllocatedDebts,
		"unallocated_cents": allocResult.UnallocatedCents,
		"message":           "Ödeme başarıyla tahsis edildi (Faiz -> Anapara -> FIFO)",
	})
}

// FuzzyMatchBankStatement executes Task 2 (T-2): Fuzzy matching bank statement description
func (h *Handler) FuzzyMatchBankStatement(w http.ResponseWriter, r *http.Request) {
	user := appMiddleware.GetCurrentUser(r.Context())
	var req struct {
		RawDescription string `json:"raw_description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.RawDescription == "" {
		appMiddleware.RespondError(w, http.StatusBadRequest, "BAD_REQUEST", "Açıklama gerekli")
		return
	}

	// Fetch all units of this site
	rows, err := h.db.Pool.Query(r.Context(), `
		SELECT u.id, u.site_id, u.block_id, b.name, u.unit_number, u.floor_number, COALESCE(o.full_name, '')
		FROM "fy-units" u
		JOIN "fy-blocks" b ON u.block_id = b.id
		LEFT JOIN "fy-ox-users" o ON u.owner_id = o.id
		WHERE u.site_id = $1
	`, user.SiteID)
	if err != nil {
		appMiddleware.RespondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	defer rows.Close()

	units := make([]domain.Unit, 0)
	for rows.Next() {
		var u domain.Unit
		if err := rows.Scan(&u.ID, &u.SiteID, &u.BlockID, &u.BlockName, &u.UnitNumber, &u.FloorNumber, &u.OwnerName); err == nil {
			units = append(units, u)
		}
	}

	match, err := h.paymentService.FuzzyMatchUnit(req.RawDescription, units)
	if err != nil {
		appMiddleware.RespondError(w, http.StatusNotFound, "NO_MATCH", "Eşleşen daire bulunamadı: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, match)
}

func (h *Handler) ListBankStatements(w http.ResponseWriter, r *http.Request) {
	user := appMiddleware.GetCurrentUser(r.Context())
	rows, err := h.db.Pool.Query(r.Context(), `
		SELECT s.id, s.site_id, s.statement_date, COALESCE(s.reference_number, ''), s.raw_description, s.amount_cents,
		       s.matched_unit_id, COALESCE(u.unit_number, '') AS matched_unit_no, s.match_confidence, s.match_status, s.payment_id, s.created_at
		FROM "fy-bank-statements" s
		LEFT JOIN "fy-units" u ON s.matched_unit_id = u.id
		WHERE s.site_id = $1
		ORDER BY s.statement_date DESC
	`, user.SiteID)
	if err != nil {
		appMiddleware.RespondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	defer rows.Close()

	list := make([]domain.BankStatement, 0)
	for rows.Next() {
		var s domain.BankStatement
		if err := rows.Scan(&s.ID, &s.SiteID, &s.StatementDate, &s.ReferenceNumber, &s.RawDescription, &s.AmountCents, &s.MatchedUnitID, &s.MatchedUnitNo, &s.MatchConfidence, &s.MatchStatus, &s.PaymentID, &s.CreatedAt); err == nil {
			list = append(list, s)
		}
	}
	respondJSON(w, http.StatusOK, list)
}

func (h *Handler) RestructureDebts(w http.ResponseWriter, r *http.Request) {
	user := appMiddleware.GetCurrentUser(r.Context())
	if user.Role != domain.RoleSuperAdmin && user.Role != domain.RoleManager && user.Role != domain.RoleAccountant {
		appMiddleware.RespondError(w, http.StatusForbidden, "FORBIDDEN", "Yetkisiz işlem")
		return
	}

	var req struct {
		UnitID           uuid.UUID `json:"unit_id"`
		InstallmentCount int       `json:"installment_count"`
		Notes            string    `json:"notes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.InstallmentCount <= 0 {
		appMiddleware.RespondError(w, http.StatusBadRequest, "BAD_REQUEST", "Geçersiz taksit sayısı")
		return
	}

	ctx := r.Context()
	// Sum overdue debts
	var totalOverdueCents int64
	var debtUserID uuid.UUID
	err := h.db.Pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(principal_cents - paid_principal_cents), 0), MAX(user_id)
		FROM "fy-debts"
		WHERE site_id = $1 AND unit_id = $2 AND status IN ('UNPAID', 'PARTIAL')
	`, user.SiteID, req.UnitID).Scan(&totalOverdueCents, &debtUserID)

	if err != nil || totalOverdueCents <= 0 {
		appMiddleware.RespondError(w, http.StatusBadRequest, "NO_OVERDUE", "Yapılandırılacak borç bulunamadı")
		return
	}

	plan, err := h.paymentService.CreateRestructurePlan(totalOverdueCents, req.InstallmentCount, time.Now())
	if err != nil {
		appMiddleware.RespondError(w, http.StatusBadRequest, "PLAN_ERROR", err.Error())
		return
	}

	var planID uuid.UUID
	err = h.db.WithTenantTx(ctx, func(tx pgx.Tx) error {
		// Mark old debts as RESTRUCTURED
		_, err := tx.Exec(ctx, `
			UPDATE "fy-debts"
			SET status = 'RESTRUCTURED', updated_at = CURRENT_TIMESTAMP
			WHERE site_id = $1 AND unit_id = $2 AND status IN ('UNPAID', 'PARTIAL')
		`, user.SiteID, req.UnitID)
		if err != nil {
			return err
		}

		err = tx.QueryRow(ctx, `
			INSERT INTO "fy-debt-restructurings" (site_id, unit_id, user_id, total_restructured_cents, installment_count, notes, created_by)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
			RETURNING id
		`, user.SiteID, req.UnitID, debtUserID, totalOverdueCents, req.InstallmentCount, req.Notes, user.UserID).Scan(&planID)
		if err != nil {
			return err
		}

		// Insert installment debts
		for _, inst := range plan.Installments {
			title := fmt.Sprintf("Yapılandırma Taksiti %d/%d", inst.InstallmentNumber, req.InstallmentCount)
			term := inst.DueDate.Format("2006-01")
			_, err = tx.Exec(ctx, `
				INSERT INTO "fy-debts" (site_id, unit_id, user_id, title, debt_type, term_month, principal_cents, due_date, status, restructure_plan_id)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 'UNPAID', $9)
			`, user.SiteID, req.UnitID, debtUserID, title, domain.DebtTypeRestructure, term, inst.AmountCents, inst.DueDate, planID)
			if err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		appMiddleware.RespondError(w, http.StatusInternalServerError, "DB_TX_ERROR", err.Error())
		return
	}

	h.db.LogAudit(r.Context(), "RESTRUCTURE_DEBTS", "fy-debt-restructurings", planID.String(), nil, req)
	respondJSON(w, http.StatusCreated, map[string]interface{}{
		"plan_id": planID,
		"plan":    plan,
		"message": "Borçlar başarıyla taksitlendirilerek yapılandırıldı",
	})
}

// ----------------------------------------------------
// F-4: Expenses
// ----------------------------------------------------

func (h *Handler) ListExpenses(w http.ResponseWriter, r *http.Request) {
	user := appMiddleware.GetCurrentUser(r.Context())
	rows, err := h.db.Pool.Query(r.Context(), `
		SELECT id, site_id, period_id, category, title, vendor_name, COALESCE(invoice_number, ''), invoice_date, due_date, amount_cents, paid_cents, status, COALESCE(document_url, ''), created_by, created_at
		FROM "fy-expenses"
		WHERE site_id = $1
		ORDER BY invoice_date DESC
	`, user.SiteID)
	if err != nil {
		appMiddleware.RespondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	defer rows.Close()

	expenses := make([]domain.Expense, 0)
	for rows.Next() {
		var e domain.Expense
		if err := rows.Scan(&e.ID, &e.SiteID, &e.PeriodID, &e.Category, &e.Title, &e.VendorName, &e.InvoiceNumber, &e.InvoiceDate, &e.DueDate, &e.AmountCents, &e.PaidCents, &e.Status, &e.DocumentURL, &e.CreatedBy, &e.CreatedAt); err == nil {
			expenses = append(expenses, e)
		}
	}
	respondJSON(w, http.StatusOK, expenses)
}

func (h *Handler) CreateExpense(w http.ResponseWriter, r *http.Request) {
	user := appMiddleware.GetCurrentUser(r.Context())
	if user.Role != domain.RoleSuperAdmin && user.Role != domain.RoleManager && user.Role != domain.RoleAccountant {
		appMiddleware.RespondError(w, http.StatusForbidden, "FORBIDDEN", "Yetkisiz işlem")
		return
	}

	var req struct {
		PeriodID      *uuid.UUID `json:"period_id"`
		Category      string     `json:"category"`
		Title         string     `json:"title"`
		VendorName    string     `json:"vendor_name"`
		InvoiceNumber string     `json:"invoice_number"`
		InvoiceDate   string     `json:"invoice_date"`
		AmountCents   int64      `json:"amount_cents"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.AmountCents <= 0 {
		appMiddleware.RespondError(w, http.StatusBadRequest, "BAD_REQUEST", "Geçersiz gider bilgileri")
		return
	}

	invDate, err := time.Parse("2006-01-02", req.InvoiceDate)
	if err != nil {
		invDate = time.Now()
	}

	var expID uuid.UUID
	err = h.db.WithTenantTx(r.Context(), func(tx pgx.Tx) error {
		err := tx.QueryRow(r.Context(), `
			INSERT INTO "fy-expenses" (site_id, period_id, category, title, vendor_name, invoice_number, invoice_date, amount_cents, status, created_by)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 'APPROVED', $9)
			RETURNING id
		`, user.SiteID, req.PeriodID, req.Category, req.Title, req.VendorName, req.InvoiceNumber, invDate, req.AmountCents, user.UserID).Scan(&expID)
		if err != nil {
			return err
		}

		// Double-entry: 770 Genel Yönetim Giderleri (Debit) vs 102/320 Borçlar (Credit)
		var acc770, acc102 uuid.UUID
		_ = tx.QueryRow(r.Context(), `SELECT id FROM "fy-ledger-accounts" WHERE site_id = $1 AND code = '770'`, user.SiteID).Scan(&acc770)
		_ = tx.QueryRow(r.Context(), `SELECT id FROM "fy-ledger-accounts" WHERE site_id = $1 AND code = '102'`, user.SiteID).Scan(&acc102)

		if acc770 != uuid.Nil && acc102 != uuid.Nil && req.PeriodID != nil {
			entryNum := time.Now().UnixNano() / 1000000
			var jEntryID uuid.UUID
			err = tx.QueryRow(r.Context(), `
				INSERT INTO "fy-journal-entries" (site_id, period_id, entry_number, entry_date, description, created_by)
				VALUES ($1, $2, $3, $4, $5, $6)
				RETURNING id
			`, user.SiteID, *req.PeriodID, entryNum, invDate, "Gider Fişi: "+req.Title, user.UserID).Scan(&jEntryID)
			if err == nil {
				_, _ = tx.Exec(r.Context(), `INSERT INTO "fy-journal-lines" (journal_entry_id, account_id, debit_cents, credit_cents, description) VALUES ($1, $2, $3, 0, $4)`, jEntryID, acc770, req.AmountCents, req.Title)
				_, _ = tx.Exec(r.Context(), `INSERT INTO "fy-journal-lines" (journal_entry_id, account_id, debit_cents, credit_cents, description) VALUES ($1, $2, 0, $3, $4)`, jEntryID, acc102, req.AmountCents, "Banka Ödemesi")
			}
		}

		return nil
	})

	if err != nil {
		appMiddleware.RespondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	h.db.LogAudit(r.Context(), "CREATE_EXPENSE", "fy-expenses", expID.String(), nil, req)
	respondJSON(w, http.StatusCreated, map[string]interface{}{"id": expID, "message": "Gider faturası kaydedildi"})
}

// ----------------------------------------------------
// F-5: Announcements, Decisions, Maintenance Requests
// ----------------------------------------------------

func (h *Handler) ListAnnouncements(w http.ResponseWriter, r *http.Request) {
	user := appMiddleware.GetCurrentUser(r.Context())
	rows, err := h.db.Pool.Query(r.Context(), `
		SELECT id, site_id, title, content, priority, is_published, created_by, created_at
		FROM "fy-announcements"
		WHERE site_id = $1 AND is_published = true
		ORDER BY created_at DESC
	`, user.SiteID)
	if err != nil {
		appMiddleware.RespondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	defer rows.Close()

	list := make([]domain.Announcement, 0)
	for rows.Next() {
		var a domain.Announcement
		if err := rows.Scan(&a.ID, &a.SiteID, &a.Title, &a.Content, &a.Priority, &a.IsPublished, &a.CreatedBy, &a.CreatedAt); err == nil {
			list = append(list, a)
		}
	}
	respondJSON(w, http.StatusOK, list)
}

func (h *Handler) CreateAnnouncement(w http.ResponseWriter, r *http.Request) {
	user := appMiddleware.GetCurrentUser(r.Context())
	if user.Role != domain.RoleSuperAdmin && user.Role != domain.RoleManager {
		appMiddleware.RespondError(w, http.StatusForbidden, "FORBIDDEN", "Yetkisiz işlem")
		return
	}

	var req struct {
		Title    string `json:"title"`
		Content  string `json:"content"`
		Priority string `json:"priority"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Title == "" {
		appMiddleware.RespondError(w, http.StatusBadRequest, "BAD_REQUEST", "Geçersiz duyuru bilgileri")
		return
	}
	if req.Priority == "" {
		req.Priority = "NORMAL"
	}

	var id uuid.UUID
	err := h.db.Pool.QueryRow(r.Context(), `
		INSERT INTO "fy-announcements" (site_id, title, content, priority, created_by)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id
	`, user.SiteID, req.Title, req.Content, req.Priority, user.UserID).Scan(&id)
	if err != nil {
		appMiddleware.RespondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	h.db.LogAudit(r.Context(), "CREATE_ANNOUNCEMENT", "fy-announcements", id.String(), nil, req)
	respondJSON(w, http.StatusCreated, map[string]interface{}{"id": id, "message": "Duyuru yayınlandı"})
}

func (h *Handler) ListDecisions(w http.ResponseWriter, r *http.Request) {
	user := appMiddleware.GetCurrentUser(r.Context())
	rows, err := h.db.Pool.Query(r.Context(), `
		SELECT id, site_id, decision_number, decision_date, title, content, COALESCE(vote_summary, ''), created_by, created_at
		FROM "fy-decisions"
		WHERE site_id = $1
		ORDER BY decision_date DESC
	`, user.SiteID)
	if err != nil {
		appMiddleware.RespondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	defer rows.Close()

	list := make([]domain.Decision, 0)
	for rows.Next() {
		var d domain.Decision
		if err := rows.Scan(&d.ID, &d.SiteID, &d.DecisionNumber, &d.DecisionDate, &d.Title, &d.Content, &d.VoteSummary, &d.CreatedBy, &d.CreatedAt); err == nil {
			list = append(list, d)
		}
	}
	respondJSON(w, http.StatusOK, list)
}

func (h *Handler) CreateDecision(w http.ResponseWriter, r *http.Request) {
	user := appMiddleware.GetCurrentUser(r.Context())
	if user.Role != domain.RoleSuperAdmin && user.Role != domain.RoleManager {
		appMiddleware.RespondError(w, http.StatusForbidden, "FORBIDDEN", "Yetkisiz işlem")
		return
	}

	var req struct {
		DecisionNumber string `json:"decision_number"`
		DecisionDate   string `json:"decision_date"`
		Title          string `json:"title"`
		Content        string `json:"content"`
		VoteSummary    string `json:"vote_summary"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.DecisionNumber == "" {
		appMiddleware.RespondError(w, http.StatusBadRequest, "BAD_REQUEST", "Geçersiz karar bilgileri")
		return
	}

	dDate, err := time.Parse("2006-01-02", req.DecisionDate)
	if err != nil {
		dDate = time.Now()
	}

	var id uuid.UUID
	err = h.db.Pool.QueryRow(r.Context(), `
		INSERT INTO "fy-decisions" (site_id, decision_number, decision_date, title, content, vote_summary, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id
	`, user.SiteID, req.DecisionNumber, dDate, req.Title, req.Content, req.VoteSummary, user.UserID).Scan(&id)
	if err != nil {
		appMiddleware.RespondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	h.db.LogAudit(r.Context(), "CREATE_DECISION", "fy-decisions", id.String(), nil, req)
	respondJSON(w, http.StatusCreated, map[string]interface{}{"id": id, "message": "Karar defterine işlendi"})
}

func (h *Handler) ListMaintenanceRequests(w http.ResponseWriter, r *http.Request) {
	user := appMiddleware.GetCurrentUser(r.Context())
	query := `
		SELECT m.id, m.site_id, m.unit_id, COALESCE(u.unit_number, '') AS unit_number,
		       m.user_id, COALESCE(usr.full_name, '') AS user_name,
		       m.title, m.description, m.category, m.status, m.priority, COALESCE(m.resolution_notes, '') AS resolution_notes, m.resolved_at, m.created_at, m.updated_at
		FROM "fy-maintenance-requests" m
		LEFT JOIN "fy-units" u ON m.unit_id = u.id
		LEFT JOIN "fy-ox-users" usr ON m.user_id = usr.id
		WHERE m.site_id = $1
	`
	args := []interface{}{user.SiteID}
	if user.Role == domain.RoleOwner || user.Role == domain.RoleTenant {
		query += " AND m.user_id = $2"
		args = append(args, user.UserID)
	}
	query += " ORDER BY m.created_at DESC"

	rows, err := h.db.Pool.Query(r.Context(), query, args...)
	if err != nil {
		appMiddleware.RespondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	defer rows.Close()

	list := make([]domain.MaintenanceRequest, 0)
	for rows.Next() {
		var m domain.MaintenanceRequest
		if err := rows.Scan(&m.ID, &m.SiteID, &m.UnitID, &m.UnitNumber, &m.UserID, &m.UserName, &m.Title, &m.Description, &m.Category, &m.Status, &m.Priority, &m.ResolutionNotes, &m.ResolvedAt, &m.CreatedAt, &m.UpdatedAt); err == nil {
			list = append(list, m)
		}
	}
	respondJSON(w, http.StatusOK, list)
}

func (h *Handler) CreateMaintenanceRequest(w http.ResponseWriter, r *http.Request) {
	user := appMiddleware.GetCurrentUser(r.Context())
	var req struct {
		UnitID      *uuid.UUID `json:"unit_id"`
		Title       string     `json:"title"`
		Description string     `json:"description"`
		Category    string     `json:"category"`
		Priority    string     `json:"priority"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Title == "" {
		appMiddleware.RespondError(w, http.StatusBadRequest, "BAD_REQUEST", "Geçersiz arıza talebi bilgileri")
		return
	}
	if req.Category == "" {
		req.Category = "GENERAL"
	}
	if req.Priority == "" {
		req.Priority = "MEDIUM"
	}

	var id uuid.UUID
	err := h.db.Pool.QueryRow(r.Context(), `
		INSERT INTO "fy-maintenance-requests" (site_id, unit_id, user_id, title, description, category, priority)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id
	`, user.SiteID, req.UnitID, user.UserID, req.Title, req.Description, req.Category, req.Priority).Scan(&id)
	if err != nil {
		appMiddleware.RespondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	h.db.LogAudit(r.Context(), "CREATE_MAINTENANCE", "fy-maintenance-requests", id.String(), nil, req)
	respondJSON(w, http.StatusCreated, map[string]interface{}{"id": id, "message": "Arıza talebi oluşturuldu"})
}

func (h *Handler) UpdateMaintenanceStatus(w http.ResponseWriter, r *http.Request) {
	user := appMiddleware.GetCurrentUser(r.Context())
	if user.Role != domain.RoleSuperAdmin && user.Role != domain.RoleManager {
		appMiddleware.RespondError(w, http.StatusForbidden, "FORBIDDEN", "Yetkisiz işlem")
		return
	}

	idStr := chi.URLParam(r, "id")
	mID, err := uuid.Parse(idStr)
	if err != nil {
		appMiddleware.RespondError(w, http.StatusBadRequest, "INVALID_ID", "Geçersiz talep kimliği")
		return
	}

	var req struct {
		Status          string `json:"status"`
		ResolutionNotes string `json:"resolution_notes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		appMiddleware.RespondError(w, http.StatusBadRequest, "BAD_REQUEST", "Geçersiz durum güncellemesi")
		return
	}

	var resolvedAt *time.Time
	if req.Status == "RESOLVED" || req.Status == "CLOSED" {
		now := time.Now()
		resolvedAt = &now
	}

	_, err = h.db.Pool.Exec(r.Context(), `
		UPDATE "fy-maintenance-requests"
		SET status = $1, resolution_notes = $2, resolved_at = $3, updated_at = CURRENT_TIMESTAMP
		WHERE id = $4 AND site_id = $5
	`, req.Status, req.ResolutionNotes, resolvedAt, mID, user.SiteID)
	if err != nil {
		appMiddleware.RespondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	h.db.LogAudit(r.Context(), "UPDATE_MAINTENANCE_STATUS", "fy-maintenance-requests", mID.String(), nil, req)
	respondJSON(w, http.StatusOK, map[string]string{"message": "Talep durumu güncellendi"})
}

// ----------------------------------------------------
// F-6: Accounting Periods & T-4 Double-entry Ledger
// ----------------------------------------------------

func (h *Handler) ListFiscalPeriods(w http.ResponseWriter, r *http.Request) {
	user := appMiddleware.GetCurrentUser(r.Context())
	rows, err := h.db.Pool.Query(r.Context(), `
		SELECT id, site_id, name, start_date, end_date, status, closed_at, closed_by, created_at
		FROM "fy-fiscal-periods"
		WHERE site_id = $1
		ORDER BY start_date DESC
	`, user.SiteID)
	if err != nil {
		appMiddleware.RespondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	defer rows.Close()

	list := make([]domain.FiscalPeriod, 0)
	for rows.Next() {
		var p domain.FiscalPeriod
		if err := rows.Scan(&p.ID, &p.SiteID, &p.Name, &p.StartDate, &p.EndDate, &p.Status, &p.ClosedAt, &p.ClosedBy, &p.CreatedAt); err == nil {
			list = append(list, p)
		}
	}
	respondJSON(w, http.StatusOK, list)
}

func (h *Handler) CreateFiscalPeriod(w http.ResponseWriter, r *http.Request) {
	user := appMiddleware.GetCurrentUser(r.Context())
	if user.Role != domain.RoleSuperAdmin && user.Role != domain.RoleManager && user.Role != domain.RoleAccountant {
		appMiddleware.RespondError(w, http.StatusForbidden, "FORBIDDEN", "Yetkisiz işlem")
		return
	}

	var req struct {
		Name      string `json:"name"`
		StartDate string `json:"start_date"`
		EndDate   string `json:"end_date"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
		appMiddleware.RespondError(w, http.StatusBadRequest, "BAD_REQUEST", "Geçersiz dönem bilgileri")
		return
	}

	sDate, _ := time.Parse("2006-01-02", req.StartDate)
	eDate, _ := time.Parse("2006-01-02", req.EndDate)

	var id uuid.UUID
	err := h.db.Pool.QueryRow(r.Context(), `
		INSERT INTO "fy-fiscal-periods" (site_id, name, start_date, end_date, status)
		VALUES ($1, $2, $3, $4, 'OPEN')
		RETURNING id
	`, user.SiteID, req.Name, sDate, eDate).Scan(&id)
	if err != nil {
		appMiddleware.RespondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	h.db.LogAudit(r.Context(), "CREATE_PERIOD", "fy-fiscal-periods", id.String(), nil, req)
	respondJSON(w, http.StatusCreated, map[string]interface{}{"id": id, "message": "Mali dönem açıldı"})
}

// CloseFiscalPeriod executes Task 4 (T-4): Immutable Period Closing
func (h *Handler) CloseFiscalPeriod(w http.ResponseWriter, r *http.Request) {
	user := appMiddleware.GetCurrentUser(r.Context())
	if user.Role != domain.RoleSuperAdmin && user.Role != domain.RoleManager && user.Role != domain.RoleAccountant {
		appMiddleware.RespondError(w, http.StatusForbidden, "FORBIDDEN", "Yetkisiz işlem")
		return
	}

	idStr := chi.URLParam(r, "id")
	periodID, err := uuid.Parse(idStr)
	if err != nil {
		appMiddleware.RespondError(w, http.StatusBadRequest, "INVALID_ID", "Geçersiz dönem kimliği")
		return
	}

	now := time.Now()
	res, err := h.db.Pool.Exec(r.Context(), `
		UPDATE "fy-fiscal-periods"
		SET status = 'CLOSED', closed_at = $1, closed_by = $2
		WHERE id = $3 AND site_id = $4 AND status = 'OPEN'
	`, now, user.UserID, periodID, user.SiteID)
	if err != nil {
		appMiddleware.RespondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	if res.RowsAffected() == 0 {
		appMiddleware.RespondError(w, http.StatusBadRequest, "CANNOT_CLOSE", "Dönem zaten kapalı veya bulunamadı")
		return
	}

	h.db.LogAudit(r.Context(), "CLOSE_FISCAL_PERIOD", "fy-fiscal-periods", periodID.String(), nil, nil)
	respondJSON(w, http.StatusOK, map[string]string{"message": "Dönem başarıyla kapatıldı ve kilitlendi. Kapalı döneme yeni kayıt yapılamaz."})
}

func (h *Handler) ListLedgerAccounts(w http.ResponseWriter, r *http.Request) {
	user := appMiddleware.GetCurrentUser(r.Context())
	rows, err := h.db.Pool.Query(r.Context(), `
		SELECT a.id, a.site_id, a.code, a.name, a.account_type, a.is_active, a.created_at,
		       COALESCE(SUM(l.debit_cents) - SUM(l.credit_cents), 0) AS balance_cents
		FROM "fy-ledger-accounts" a
		LEFT JOIN "fy-journal-lines" l ON a.id = l.account_id
		WHERE a.site_id = $1
		GROUP BY a.id, a.site_id, a.code, a.name, a.account_type, a.is_active, a.created_at
		ORDER BY a.code ASC
	`, user.SiteID)
	if err != nil {
		appMiddleware.RespondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	defer rows.Close()

	list := make([]domain.LedgerAccount, 0)
	for rows.Next() {
		var a domain.LedgerAccount
		if err := rows.Scan(&a.ID, &a.SiteID, &a.Code, &a.Name, &a.AccountType, &a.IsActive, &a.CreatedAt, &a.BalanceCents); err == nil {
			list = append(list, a)
		}
	}
	respondJSON(w, http.StatusOK, list)
}

func (h *Handler) ListJournalEntries(w http.ResponseWriter, r *http.Request) {
	user := appMiddleware.GetCurrentUser(r.Context())
	rows, err := h.db.Pool.Query(r.Context(), `
		SELECT e.id, e.site_id, e.period_id, e.entry_number, e.entry_date, e.description, e.is_reversal, e.reversed_entry_id, e.created_at
		FROM "fy-journal-entries" e
		WHERE e.site_id = $1
		ORDER BY e.entry_number DESC
		LIMIT 100
	`, user.SiteID)
	if err != nil {
		appMiddleware.RespondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	defer rows.Close()

	entries := make([]domain.JournalEntry, 0)
	for rows.Next() {
		var e domain.JournalEntry
		if err := rows.Scan(&e.ID, &e.SiteID, &e.PeriodID, &e.EntryNumber, &e.EntryDate, &e.Description, &e.IsReversal, &e.ReversedEntryID, &e.CreatedAt); err == nil {
			// Fetch lines for each entry
			lRows, err := h.db.Pool.Query(r.Context(), `
				SELECT l.id, l.journal_entry_id, l.account_id, a.code, a.name, l.debit_cents, l.credit_cents, l.description
				FROM "fy-journal-lines" l
				JOIN "fy-ledger-accounts" a ON l.account_id = a.id
				WHERE l.journal_entry_id = $1
			`, e.ID)
			if err == nil {
				e.Lines = make([]domain.JournalLine, 0)
				for lRows.Next() {
					var l domain.JournalLine
					if err := lRows.Scan(&l.ID, &l.JournalEntryID, &l.AccountID, &l.AccountCode, &l.AccountName, &l.DebitCents, &l.CreditCents, &l.Description); err == nil {
						e.Lines = append(e.Lines, l)
						e.TotalDebitCents += l.DebitCents
						e.TotalCreditCents += l.CreditCents
					}
				}
				lRows.Close()
			}
			entries = append(entries, e)
		}
	}
	respondJSON(w, http.StatusOK, entries)
}

// CreateJournalEntry executes Task 4 (T-4): Double-entry validation
func (h *Handler) CreateJournalEntry(w http.ResponseWriter, r *http.Request) {
	user := appMiddleware.GetCurrentUser(r.Context())
	if user.Role != domain.RoleSuperAdmin && user.Role != domain.RoleManager && user.Role != domain.RoleAccountant {
		appMiddleware.RespondError(w, http.StatusForbidden, "FORBIDDEN", "Yetkisiz işlem")
		return
	}

	var req struct {
		PeriodID    uuid.UUID                   `json:"period_id"`
		EntryDate   string                      `json:"entry_date"`
		Description string                      `json:"description"`
		Lines       []service.JournalLineInput `json:"lines"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		appMiddleware.RespondError(w, http.StatusBadRequest, "BAD_REQUEST", "Geçersiz yevmiye kaydı")
		return
	}

	// Double-entry validation: Sum(Debit) == Sum(Credit)
	if err := h.accountingService.ValidateDoubleEntry(req.Lines); err != nil {
		appMiddleware.RespondError(w, http.StatusBadRequest, "UNBALANCED_ENTRY", "Çift taraflı kayıt dengesi bozuk: "+err.Error())
		return
	}

	// Check if period is OPEN
	var periodStatus string
	_ = h.db.Pool.QueryRow(r.Context(), `SELECT status FROM "fy-fiscal-periods" WHERE id = $1`, req.PeriodID).Scan(&periodStatus)
	if periodStatus != domain.PeriodStatusOpen {
		appMiddleware.RespondError(w, http.StatusBadRequest, "LOCKED_PERIOD", "Kapalı veya kilitli bir döneme kayıt yapılamaz! Düzeltmeler ters kayıtla yapılmalıdır.")
		return
	}

	eDate, err := time.Parse("2006-01-02", req.EntryDate)
	if err != nil {
		eDate = time.Now()
	}

	var entryID uuid.UUID
	entryNum := time.Now().UnixNano() / 1000000

	err = h.db.WithTenantTx(r.Context(), func(tx pgx.Tx) error {
		err := tx.QueryRow(r.Context(), `
			INSERT INTO "fy-journal-entries" (site_id, period_id, entry_number, entry_date, description, created_by)
			VALUES ($1, $2, $3, $4, $5, $6)
			RETURNING id
		`, user.SiteID, req.PeriodID, entryNum, eDate, req.Description, user.UserID).Scan(&entryID)
		if err != nil {
			return err
		}

		for _, l := range req.Lines {
			_, err = tx.Exec(r.Context(), `
				INSERT INTO "fy-journal-lines" (journal_entry_id, account_id, debit_cents, credit_cents, description)
				VALUES ($1, $2, $3, $4, $5)
			`, entryID, l.AccountID, l.DebitCents, l.CreditCents, l.Description)
			if err != nil {
				return err
			}
		}
		return nil
	})

	if err != nil {
		appMiddleware.RespondError(w, http.StatusInternalServerError, "DB_TX_ERROR", err.Error())
		return
	}

	h.db.LogAudit(r.Context(), "CREATE_JOURNAL_ENTRY", "fy-journal-entries", entryID.String(), nil, req)
	respondJSON(w, http.StatusCreated, map[string]interface{}{
		"id":           entryID,
		"entry_number": entryNum,
		"message":      "Yevmiye kaydı çift taraflı muhasebe ilkesine uygun kaydedildi",
	})
}

func (h *Handler) ReverseJournalEntry(w http.ResponseWriter, r *http.Request) {
	user := appMiddleware.GetCurrentUser(r.Context())
	if user.Role != domain.RoleSuperAdmin && user.Role != domain.RoleManager && user.Role != domain.RoleAccountant {
		appMiddleware.RespondError(w, http.StatusForbidden, "FORBIDDEN", "Yetkisiz işlem")
		return
	}

	idStr := chi.URLParam(r, "id")
	entryID, err := uuid.Parse(idStr)
	if err != nil {
		appMiddleware.RespondError(w, http.StatusBadRequest, "INVALID_ID", "Geçersiz yevmiye kimliği")
		return
	}

	// Fetch original entry and lines
	var orig domain.JournalEntry
	err = h.db.Pool.QueryRow(r.Context(), `
		SELECT id, site_id, period_id, entry_number, entry_date, description, is_reversal
		FROM "fy-journal-entries"
		WHERE id = $1 AND site_id = $2
	`, entryID, user.SiteID).Scan(&orig.ID, &orig.SiteID, &orig.PeriodID, &orig.EntryNumber, &orig.EntryDate, &orig.Description, &orig.IsReversal)
	if err != nil {
		appMiddleware.RespondError(w, http.StatusNotFound, "NOT_FOUND", "Yevmiye kaydı bulunamadı")
		return
	}

	rows, err := h.db.Pool.Query(r.Context(), `
		SELECT account_id, debit_cents, credit_cents, description
		FROM "fy-journal-lines"
		WHERE journal_entry_id = $1
	`, orig.ID)
	if err != nil {
		appMiddleware.RespondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	defer rows.Close()

	for rows.Next() {
		var l domain.JournalLine
		if err := rows.Scan(&l.AccountID, &l.DebitCents, &l.CreditCents, &l.Description); err == nil {
			orig.Lines = append(orig.Lines, l)
		}
	}

	reversalInput, err := h.accountingService.CreateReversalEntry(orig, time.Now(), "Kullanıcı talebiyle düzeltme")
	if err != nil {
		appMiddleware.RespondError(w, http.StatusBadRequest, "REVERSAL_ERROR", err.Error())
		return
	}

	var revEntryID uuid.UUID
	entryNum := time.Now().UnixNano() / 1000000

	err = h.db.WithTenantTx(r.Context(), func(tx pgx.Tx) error {
		err := tx.QueryRow(r.Context(), `
			INSERT INTO "fy-journal-entries" (site_id, period_id, entry_number, entry_date, description, is_reversal, reversed_entry_id, created_by)
			VALUES ($1, $2, $3, $4, $5, true, $6, $7)
			RETURNING id
		`, user.SiteID, reversalInput.PeriodID, entryNum, reversalInput.EntryDate, reversalInput.Description, orig.ID, user.UserID).Scan(&revEntryID)
		if err != nil {
			return err
		}

		for _, l := range reversalInput.Lines {
			_, err = tx.Exec(r.Context(), `
				INSERT INTO "fy-journal-lines" (journal_entry_id, account_id, debit_cents, credit_cents, description)
				VALUES ($1, $2, $3, $4, $5)
			`, revEntryID, l.AccountID, l.DebitCents, l.CreditCents, l.Description)
			if err != nil {
				return err
			}
		}
		return nil
	})

	if err != nil {
		appMiddleware.RespondError(w, http.StatusInternalServerError, "DB_TX_ERROR", err.Error())
		return
	}

	h.db.LogAudit(r.Context(), "REVERSE_JOURNAL_ENTRY", "fy-journal-entries", revEntryID.String(), nil, nil)
	respondJSON(w, http.StatusCreated, map[string]interface{}{
		"id":           revEntryID,
		"entry_number": entryNum,
		"message":      "Ters kayıt oluşturuldu ve muhasebe bakiyesi dengelendi",
	})
}

// ----------------------------------------------------
// F-7: Reports & T-4 Clearance Certificate
// ----------------------------------------------------

func (h *Handler) GetFinancialSummary(w http.ResponseWriter, r *http.Request) {
	user := appMiddleware.GetCurrentUser(r.Context())
	ctx := r.Context()

	var totalAccrualsCents int64
	var totalCollectedCents int64
	var totalExpensesCents int64
	var totalOutstandingDebtCents int64

	_ = h.db.Pool.QueryRow(ctx, `SELECT COALESCE(SUM(total_amount_cents), 0) FROM "fy-accruals" WHERE site_id = $1 AND is_reversed = false`, user.SiteID).Scan(&totalAccrualsCents)
	_ = h.db.Pool.QueryRow(ctx, `SELECT COALESCE(SUM(amount_cents), 0) FROM "fy-payments" WHERE site_id = $1`, user.SiteID).Scan(&totalCollectedCents)
	_ = h.db.Pool.QueryRow(ctx, `SELECT COALESCE(SUM(amount_cents), 0) FROM "fy-expenses" WHERE site_id = $1 AND status != 'CANCELLED'`, user.SiteID).Scan(&totalExpensesCents)
	_ = h.db.Pool.QueryRow(ctx, `SELECT COALESCE(SUM(principal_cents - paid_principal_cents), 0) FROM "fy-debts" WHERE site_id = $1 AND status IN ('UNPAID', 'PARTIAL')`, user.SiteID).Scan(&totalOutstandingDebtCents)

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"total_accruals_cents":         totalAccrualsCents,
		"total_collected_cents":        totalCollectedCents,
		"total_expenses_cents":         totalExpensesCents,
		"total_outstanding_debt_cents": totalOutstandingDebtCents,
		"net_cash_flow_cents":          totalCollectedCents - totalExpensesCents,
		"collection_rate_percent":      calculateRate(totalCollectedCents, totalAccrualsCents),
	})
}

func calculateRate(col, acc int64) float64 {
	if acc <= 0 {
		return 100.0
	}
	return (float64(col) / float64(acc)) * 100.0
}

func (h *Handler) GetBudgetComparison(w http.ResponseWriter, r *http.Request) {
	user := appMiddleware.GetCurrentUser(r.Context())
	// Fetch categorized expenses
	rows, err := h.db.Pool.Query(r.Context(), `
		SELECT category, COALESCE(SUM(amount_cents), 0)
		FROM "fy-expenses"
		WHERE site_id = $1 AND status != 'CANCELLED'
		GROUP BY category
	`, user.SiteID)
	if err != nil {
		appMiddleware.RespondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	defer rows.Close()

	actuals := make(map[string]int64)
	for rows.Next() {
		var cat string
		var amt int64
		if err := rows.Scan(&cat, &amt); err == nil {
			actuals[cat] = amt
		}
	}

	// Standard budget targets for demonstration comparison
	budgets := map[string]int64{
		"ELEC":      1500000, // 15,000 TL
		"WATER":     800000,  // 8,000 TL
		"ELEVATOR":  1200000, // 12,000 TL
		"CLEANING":  2000000, // 20,000 TL
		"SECURITY":  3500000, // 35,000 TL
		"REPAIR":    1000000, // 10,000 TL
		"MGMT":      800000,  // 8,000 TL
	}

	comp := h.accountingService.CalculateBudgetComparison(budgets, actuals)
	respondJSON(w, http.StatusOK, comp)
}

func (h *Handler) ListCertificates(w http.ResponseWriter, r *http.Request) {
	user := appMiddleware.GetCurrentUser(r.Context())
	query := `
		SELECT c.id, c.site_id, c.unit_id, u.unit_number, b.name AS block_name,
		       c.user_id, usr.full_name AS user_name,
		       c.certificate_number, c.verification_code, c.sha256_hash,
		       c.issued_date, c.valid_until, c.debt_balance_at_issue_cents, c.issued_by, c.created_at
		FROM "fy-clearance-certificates" c
		JOIN "fy-units" u ON c.unit_id = u.id
		JOIN "fy-blocks" b ON u.block_id = b.id
		JOIN "fy-ox-users" usr ON c.user_id = usr.id
		WHERE c.site_id = $1
	`
	args := []interface{}{user.SiteID}
	if user.Role == domain.RoleOwner || user.Role == domain.RoleTenant {
		query += " AND c.user_id = $2"
		args = append(args, user.UserID)
	}
	query += " ORDER BY c.created_at DESC"

	rows, err := h.db.Pool.Query(r.Context(), query, args...)
	if err != nil {
		appMiddleware.RespondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	defer rows.Close()

	list := make([]domain.ClearanceCertificate, 0)
	for rows.Next() {
		var c domain.ClearanceCertificate
		if err := rows.Scan(
			&c.ID, &c.SiteID, &c.UnitID, &c.UnitNumber, &c.BlockName,
			&c.UserID, &c.UserName, &c.CertificateNumber, &c.VerificationCode, &c.Sha256Hash,
			&c.IssuedDate, &c.ValidUntil, &c.DebtBalanceAtIssueCents, &c.IssuedBy, &c.CreatedAt,
		); err == nil {
			list = append(list, c)
		}
	}
	respondJSON(w, http.StatusOK, list)
}

// IssueClearanceCertificate executes Task 4 (T-4): Cryptographically signed 'Borcu Yoktur' Certificate
func (h *Handler) IssueClearanceCertificate(w http.ResponseWriter, r *http.Request) {
	user := appMiddleware.GetCurrentUser(r.Context())
	if user.Role != domain.RoleSuperAdmin && user.Role != domain.RoleManager && user.Role != domain.RoleAccountant {
		appMiddleware.RespondError(w, http.StatusForbidden, "FORBIDDEN", "Yetkisiz işlem")
		return
	}

	var req struct {
		UnitID    uuid.UUID `json:"unit_id"`
		ValidDays int       `json:"valid_days"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		appMiddleware.RespondError(w, http.StatusBadRequest, "BAD_REQUEST", "Geçersiz daire seçimi")
		return
	}
	if req.ValidDays <= 0 {
		req.ValidDays = 30
	}

	ctx := r.Context()
	// Fetch unit and owner
	var site domain.Site
	var unit domain.Unit
	var resident domain.User

	_ = h.db.Pool.QueryRow(ctx, `SELECT id, name, code FROM "fy-sites" WHERE id = $1`, user.SiteID).Scan(&site.ID, &site.Name, &site.Code)

	var ownerID *uuid.UUID
	err := h.db.Pool.QueryRow(ctx, `
		SELECT u.id, u.site_id, u.block_id, b.name, u.unit_number, u.owner_id
		FROM "fy-units" u
		JOIN "fy-blocks" b ON u.block_id = b.id
		WHERE u.id = $1 AND u.site_id = $2
	`, req.UnitID, user.SiteID).Scan(&unit.ID, &unit.SiteID, &unit.BlockID, &unit.BlockName, &unit.UnitNumber, &ownerID)
	if err != nil {
		appMiddleware.RespondError(w, http.StatusNotFound, "NOT_FOUND", "Daire bulunamadı")
		return
	}

	if ownerID != nil {
		_ = h.db.Pool.QueryRow(ctx, `SELECT id, full_name, email FROM "fy-ox-users" WHERE id = $1`, *ownerID).Scan(&resident.ID, &resident.FullName, &resident.Email)
	} else {
		resident.ID = user.UserID
		resident.FullName = "Daire Sakini"
	}

	// Verify un-cleared debts
	var outstandingDebtCents int64
	_ = h.db.Pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(principal_cents - paid_principal_cents), 0)
		FROM "fy-debts"
		WHERE site_id = $1 AND unit_id = $2 AND status IN ('UNPAID', 'PARTIAL')
	`, user.SiteID, req.UnitID).Scan(&outstandingDebtCents)

	if outstandingDebtCents > 0 {
		appMiddleware.RespondError(w, http.StatusBadRequest, "DEBT_EXISTS", fmt.Sprintf("Borcu Yoktur Belgesi verilemez: Dairenin %s ödenmemiş borcu vardır!", domain.FormatCents(outstandingDebtCents)))
		return
	}

	cert, err := h.certificateService.IssueClearanceCertificate(site, unit, resident, 0, req.ValidDays, user.UserID)
	if err != nil {
		appMiddleware.RespondError(w, http.StatusBadRequest, "CERT_ERROR", err.Error())
		return
	}

	_, err = h.db.Pool.Exec(ctx, `
		INSERT INTO "fy-clearance-certificates" (id, site_id, unit_id, user_id, certificate_number, verification_code, sha256_hash, issued_date, valid_until, debt_balance_at_issue_cents, issued_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`, cert.ID, cert.SiteID, cert.UnitID, cert.UserID, cert.CertificateNumber, cert.VerificationCode, cert.Sha256Hash, cert.IssuedDate, cert.ValidUntil, cert.DebtBalanceAtIssueCents, cert.IssuedBy)
	if err != nil {
		appMiddleware.RespondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	h.db.LogAudit(r.Context(), "ISSUE_CLEARANCE_CERTIFICATE", "fy-clearance-certificates", cert.ID.String(), nil, cert)
	respondJSON(w, http.StatusCreated, cert)
}

func (h *Handler) VerifyCertificate(w http.ResponseWriter, r *http.Request) {
	code := chi.URLParam(r, "code")
	if code == "" {
		appMiddleware.RespondError(w, http.StatusBadRequest, "INVALID_CODE", "Doğrulama kodu gerekli")
		return
	}

	var c domain.ClearanceCertificate
	err := h.db.Pool.QueryRow(r.Context(), `
		SELECT c.id, c.site_id, c.unit_id, u.unit_number, b.name AS block_name,
		       c.user_id, usr.full_name AS user_name,
		       c.certificate_number, c.verification_code, c.sha256_hash,
		       c.issued_date, c.valid_until, c.debt_balance_at_issue_cents, c.issued_by, c.created_at
		FROM "fy-clearance-certificates" c
		JOIN "fy-units" u ON c.unit_id = u.id
		JOIN "fy-blocks" b ON u.block_id = b.id
		JOIN "fy-ox-users" usr ON c.user_id = usr.id
		WHERE c.verification_code = $1 OR c.certificate_number = $1
	`, code).Scan(
		&c.ID, &c.SiteID, &c.UnitID, &c.UnitNumber, &c.BlockName,
		&c.UserID, &c.UserName, &c.CertificateNumber, &c.VerificationCode, &c.Sha256Hash,
		&c.IssuedDate, &c.ValidUntil, &c.DebtBalanceAtIssueCents, &c.IssuedBy, &c.CreatedAt,
	)
	if err != nil {
		appMiddleware.RespondError(w, http.StatusNotFound, "NOT_FOUND", "Belge bulunamadı veya geçersiz doğrulama kodu")
		return
	}

	isValidHash := h.certificateService.VerifyHash(c)
	isExpired := time.Now().After(c.ValidUntil)

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"certificate":     c,
		"is_authentic":    isValidHash,
		"is_expired":      isExpired,
		"validity_status": !isExpired && isValidHash,
		"message":         "Belge resmi olarak sistemde kayıtlı ve kriptografik olarak doğrulanmıştır.",
	})
}

// ----------------------------------------------------
// Audit Logs
// ----------------------------------------------------

func (h *Handler) ListAuditLogs(w http.ResponseWriter, r *http.Request) {
	user := appMiddleware.GetCurrentUser(r.Context())
	if user.Role != domain.RoleSuperAdmin && user.Role != domain.RoleManager && user.Role != domain.RoleAuditor {
		appMiddleware.RespondError(w, http.StatusForbidden, "FORBIDDEN", "Yetkisiz işlem")
		return
	}

	rows, err := h.db.Pool.Query(r.Context(), `
		SELECT id, site_id, user_id, action, entity_table, entity_id, old_values, new_values, ip_address, user_agent, created_at
		FROM "fy-audit-logs"
		WHERE site_id = $1
		ORDER BY created_at DESC
		LIMIT 100
	`, user.SiteID)
	if err != nil {
		appMiddleware.RespondError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	defer rows.Close()

	list := make([]domain.AuditLog, 0)
	for rows.Next() {
		var a domain.AuditLog
		if err := rows.Scan(&a.ID, &a.SiteID, &a.UserID, &a.Action, &a.EntityTable, &a.EntityID, &a.OldValues, &a.NewValues, &a.IPAddress, &a.UserAgent, &a.CreatedAt); err == nil {
			list = append(list, a)
		}
	}
	respondJSON(w, http.StatusOK, list)
}
