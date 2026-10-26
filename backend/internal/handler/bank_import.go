package handler

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/siteyonet/siteyonet/internal/domain"
	"github.com/siteyonet/siteyonet/internal/middleware"
)

func (h *Handler) ImportBankCSV(w http.ResponseWriter, r *http.Request) {
	u := middleware.GetCurrentUser(r.Context())
	if u.Role != "manager" && u.Role != "accountant" && u.Role != "superadmin" {
		middleware.RespondError(w, 403, "FORBIDDEN", "Yetkisiz işlem")
		return
	}
	var req struct {
		CSV string `json:"csv"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2<<20)).Decode(&req); err != nil {
		middleware.RespondError(w, 400, "BAD_REQUEST", "Geçersiz CSV")
		return
	}
	records, err := csv.NewReader(strings.NewReader(req.CSV)).ReadAll()
	if err != nil || len(records) < 2 || strings.Join(records[0], ",") != "date,reference,description,amount_cents" {
		middleware.RespondError(w, 400, "BAD_REQUEST", "CSV başlığı: date,reference,description,amount_cents")
		return
	}
	rows, err := h.db.Executor(r.Context()).Query(r.Context(), `SELECT u.id,b.name,u.unit_number FROM "fy-units" u JOIN "fy-blocks" b ON b.id=u.block_id WHERE u.site_id=$1 ORDER BY b.name,u.unit_number`, u.SiteID)
	if err != nil {
		middleware.RespondError(w, 500, "DB_ERROR", err.Error())
		return
	}
	var units []domain.Unit
	for rows.Next() {
		var unit domain.Unit
		if err := rows.Scan(&unit.ID, &unit.BlockName, &unit.UnitNumber); err != nil {
			rows.Close()
			middleware.RespondError(w, 500, "DB_ERROR", err.Error())
			return
		}
		units = append(units, unit)
	}
	rows.Close()
	matched := 0
	for i, row := range records[1:] {
		if len(row) != 4 {
			middleware.RespondError(w, 400, "BAD_REQUEST", fmt.Sprintf("CSV satırı %d geçersiz", i+2))
			return
		}
		date, err := time.Parse("2006-01-02", row[0])
		amount, amountErr := strconv.ParseInt(row[3], 10, 64)
		if err != nil || amountErr != nil || amount == 0 {
			middleware.RespondError(w, 400, "BAD_REQUEST", fmt.Sprintf("CSV satırı %d tarih veya tutar hatası", i+2))
			return
		}
		var unitID *uuid.UUID
		score := 0.0
		status := "UNMATCHED"
		if candidate, err := h.paymentService.FuzzyMatchUnit(row[2], units); err == nil && amount > 0 {
			unitID = &candidate.UnitID
			score = candidate.Score
			status = "AUTO_MATCHED"
			matched++
		}
		if _, err := h.db.Executor(r.Context()).Exec(r.Context(), `INSERT INTO "fy-bank-statements" (site_id,statement_date,reference_number,raw_description,amount_cents,matched_unit_id,match_confidence,match_status) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`, u.SiteID, date, row[1], row[2], amount, unitID, score, status); err != nil {
			middleware.RespondError(w, 500, "DB_ERROR", err.Error())
			return
		}
	}
	h.db.LogAudit(r.Context(), "IMPORT_BANK_CSV", "fy-bank-statements", u.SiteID.String(), nil, map[string]interface{}{"rows": len(records) - 1, "matched": matched})
	respondJSON(w, 201, map[string]interface{}{"imported": len(records) - 1, "matched": matched, "unmatched": len(records) - 1 - matched})
}
