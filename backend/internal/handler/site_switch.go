package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/siteyonet/siteyonet/internal/domain"
	"github.com/siteyonet/siteyonet/internal/middleware"
)

func (h *Handler) SwitchSite(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetCurrentUser(r.Context())
	var req struct {
		SiteID uuid.UUID `json:"site_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		middleware.RespondError(w, 400, "BAD_REQUEST", "Geçersiz site")
		return
	}
	role := user.Role
	if role != domain.RoleSuperAdmin {
		if err := h.db.Executor(r.Context()).QueryRow(r.Context(), `SELECT role FROM "fy-user-sites" WHERE user_id=$1 AND site_id=$2`, user.UserID, req.SiteID).Scan(&role); err != nil {
			middleware.RespondError(w, 403, "FORBIDDEN", "Bu sitede yetkiniz yok")
			return
		}
	}
	var site domain.Site
	if err := h.db.Executor(r.Context()).QueryRow(r.Context(), `SELECT id,name,code,city FROM "fy-sites" WHERE id=$1`, req.SiteID).Scan(&site.ID, &site.Name, &site.Code, &site.City); err != nil {
		middleware.RespondError(w, 404, "NOT_FOUND", "Site bulunamadı")
		return
	}
	token, err := middleware.GenerateToken(user.UserID, site.ID, role, user.Email, user.FullName, time.Hour)
	if err != nil {
		middleware.RespondError(w, 500, "TOKEN_ERROR", "Token üretilemedi")
		return
	}
	refresh, hash, err := refreshPair()
	if err != nil {
		middleware.RespondError(w, 500, "TOKEN_ERROR", "Token üretilemedi")
		return
	}
	if _, err = h.db.Pool.Exec(r.Context(), `INSERT INTO "fy-refresh-sessions" (token_hash,user_id,site_id,expires_at) VALUES ($1,$2,$3,$4)`, hash, user.UserID, site.ID, time.Now().Add(7*24*time.Hour)); err != nil {
		middleware.RespondError(w, 500, "DB_ERROR", "Oturum kaydedilemedi")
		return
	}
	respondJSON(w, 200, map[string]interface{}{"token": token, "refresh_token": refresh, "site": site, "role": role})
}
