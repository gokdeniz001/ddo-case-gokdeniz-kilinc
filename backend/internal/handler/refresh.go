package handler

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"time"

	"github.com/siteyonet/siteyonet/internal/domain"
	"github.com/siteyonet/siteyonet/internal/middleware"
)

func refreshPair() (string, string, error) {
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", "", err
	}
	token := hex.EncodeToString(raw[:])
	sum := sha256.Sum256([]byte(token))
	return token, hex.EncodeToString(sum[:]), nil
}

// Refresh tokens are opaque, hashed at rest, single-use, and rotated atomically.
func (h *Handler) Refresh(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Token string `json:"refresh_token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Token == "" {
		middleware.RespondError(w, 400, "BAD_REQUEST", "Refresh token gerekli")
		return
	}
	sum := sha256.Sum256([]byte(req.Token))
	tx, err := h.db.Pool.Begin(r.Context())
	if err != nil {
		middleware.RespondError(w, 500, "DB_ERROR", "İşlem başlatılamadı")
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }()
	var user domain.User
	err = tx.QueryRow(r.Context(), `SELECT u.id,s.site_id,u.email,u.full_name,COALESCE(m.role,u.role) FROM "fy-refresh-sessions" s JOIN "fy-ox-users" u ON u.id=s.user_id LEFT JOIN "fy-user-sites" m ON m.user_id=u.id AND m.site_id=s.site_id WHERE s.token_hash=$1 AND s.expires_at>now() AND u.is_active AND (m.user_id IS NOT NULL OR u.site_id=s.site_id OR u.role='superadmin') FOR UPDATE OF s`, hex.EncodeToString(sum[:])).Scan(&user.ID, &user.SiteID, &user.Email, &user.FullName, &user.Role)
	if err != nil || user.SiteID == nil {
		middleware.RespondError(w, 401, "AUTH_FAILED", "Oturum geçersiz veya süresi dolmuş")
		return
	}
	refresh, hash, err := refreshPair()
	if err != nil {
		middleware.RespondError(w, 500, "TOKEN_ERROR", "Token üretilemedi")
		return
	}
	access, err := middleware.GenerateToken(user.ID, *user.SiteID, user.Role, user.Email, user.FullName, time.Hour)
	if err != nil {
		middleware.RespondError(w, 500, "TOKEN_ERROR", "Token üretilemedi")
		return
	}
	if _, err = tx.Exec(r.Context(), `DELETE FROM "fy-refresh-sessions" WHERE token_hash=$1`, hex.EncodeToString(sum[:])); err != nil {
		middleware.RespondError(w, 500, "DB_ERROR", "Oturum yenilenemedi")
		return
	}
	if _, err = tx.Exec(r.Context(), `INSERT INTO "fy-refresh-sessions" (token_hash,user_id,site_id,expires_at) VALUES ($1,$2,$3,$4)`, hash, user.ID, user.SiteID, time.Now().Add(7*24*time.Hour)); err != nil {
		middleware.RespondError(w, 500, "DB_ERROR", "Oturum yenilenemedi")
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		middleware.RespondError(w, 500, "DB_ERROR", "Oturum kaydedilemedi")
		return
	}
	respondJSON(w, 200, map[string]interface{}{"token": access, "refresh_token": refresh, "expires_at": time.Now().Add(time.Hour)})
}
