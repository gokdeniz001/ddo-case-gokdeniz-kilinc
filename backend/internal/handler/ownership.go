package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/siteyonet/siteyonet/internal/middleware"
	"github.com/siteyonet/siteyonet/internal/service"
)

func (h *Handler) TransferOwnership(w http.ResponseWriter, r *http.Request) {
	u := middleware.GetCurrentUser(r.Context())
	if u.Role != "manager" && u.Role != "superadmin" {
		middleware.RespondError(w, 403, "FORBIDDEN", "Yetkisiz işlem")
		return
	}
	unit, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		middleware.RespondError(w, 400, "BAD_REQUEST", "Geçersiz daire")
		return
	}
	var req struct {
		Owner uuid.UUID `json:"new_owner_id"`
		Date  string    `json:"transfer_date"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		middleware.RespondError(w, 400, "BAD_REQUEST", "Geçersiz devir")
		return
	}
	date, err := time.Parse("2006-01-02", req.Date)
	if err != nil || req.Owner == uuid.Nil {
		middleware.RespondError(w, 400, "BAD_REQUEST", "Geçersiz devir tarihi veya malik")
		return
	}
	exec := h.db.Executor(r.Context())
	var old uuid.UUID
	if err := exec.QueryRow(r.Context(), `SELECT owner_id FROM "fy-units" WHERE id=$1 AND site_id=$2 FOR UPDATE`, unit, u.SiteID).Scan(&old); err != nil {
		middleware.RespondError(w, 404, "NOT_FOUND", "Daire veya eski malik bulunamadı")
		return
	}
	if old == req.Owner {
		middleware.RespondError(w, 400, "BAD_REQUEST", "Yeni malik farklı olmalı")
		return
	}
	var next uuid.UUID
	if err := exec.QueryRow(r.Context(), `SELECT id FROM "fy-ox-users" WHERE id=$1 AND site_id=$2 AND is_active`, req.Owner, u.SiteID).Scan(&next); err != nil {
		middleware.RespondError(w, 400, "BAD_REQUEST", "Yeni malik bu sitede bulunamadı")
		return
	}
	var bad bool
	if err := exec.QueryRow(r.Context(), `SELECT EXISTS(SELECT FROM "fy-unit-occupancies" WHERE unit_id=$1 AND occupancy_type='owner' AND start_date >= $2::date)`, unit, date).Scan(&bad); err != nil || bad {
		middleware.RespondError(w, 400, "BAD_REQUEST", "Devir tarihi mevcut geçmişten sonra olmalı")
		return
	}
	tag, err := exec.Exec(r.Context(), `UPDATE "fy-unit-occupancies" SET end_date=$1::date-1 WHERE unit_id=$2 AND occupancy_type='owner' AND end_date IS NULL`, date, unit)
	if err == nil && tag.RowsAffected() == 0 {
		_, err = exec.Exec(r.Context(), `INSERT INTO "fy-unit-occupancies" (unit_id,user_id,occupancy_type,start_date,end_date) VALUES ($1,$2,'owner',date_trunc('month',$3::date)::date,$3::date-1)`, unit, old, date)
	}
	if err == nil {
		_, err = exec.Exec(r.Context(), `INSERT INTO "fy-unit-occupancies" (unit_id,user_id,occupancy_type,start_date) VALUES ($1,$2,'owner',$3)`, unit, next, date)
	}
	if err == nil {
		_, err = exec.Exec(r.Context(), `UPDATE "fy-units" SET owner_id=$1,updated_at=now() WHERE id=$2`, next, unit)
	}
	if err != nil {
		middleware.RespondError(w, 500, "DB_ERROR", err.Error())
		return
	}
	h.db.LogAudit(r.Context(), "TRANSFER_OWNERSHIP", "fy-units", unit.String(), map[string]interface{}{"owner_id": old}, req)
	respondJSON(w, 200, req)
}

func (h *Handler) loadTransfers(r *http.Request, month time.Time, units []service.UnitShareInput) error {
	end := month.AddDate(0, 1, 0)
	for i := range units {
		rows, err := h.db.Executor(r.Context()).Query(r.Context(), `SELECT user_id,start_date,end_date FROM "fy-unit-occupancies" WHERE unit_id=$1 AND occupancy_type='owner' AND start_date<$3::date AND (end_date IS NULL OR end_date >= $2::date) ORDER BY start_date`, units[i].UnitID, month, end)
		if err != nil {
			return err
		}
		type ownership struct {
			id    uuid.UUID
			start time.Time
			end   *time.Time
		}
		var owners []ownership
		for rows.Next() {
			var o ownership
			if err := rows.Scan(&o.id, &o.start, &o.end); err != nil {
				rows.Close()
				return err
			}
			owners = append(owners, o)
		}
		rows.Close()
		if len(owners) > 2 {
			return fmt.Errorf("a month with multiple ownership transfers is not supported")
		}
		if len(owners) == 1 {
			units[i].OwnerID = owners[0].id
		}
		if len(owners) == 2 {
			days := int(end.Sub(month).Hours() / 24)
			oldDays := int(owners[1].start.Sub(month).Hours() / 24)
			units[i].Transfers = []service.OwnershipTransfer{{OldUserID: owners[0].id, NewUserID: owners[1].id, ActiveDaysOld: oldDays, ActiveDaysNew: days - oldDays, TotalDaysInMonth: days}}
		}
	}
	return nil
}
