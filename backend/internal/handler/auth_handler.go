package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/siteyonet/siteyonet/internal/domain"
	appMiddleware "github.com/siteyonet/siteyonet/internal/middleware"
	"golang.org/x/crypto/bcrypt"
)

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginResponse struct {
	Token     string      `json:"token"`
	ExpiresAt time.Time   `json:"expires_at"`
	User      domain.User `json:"user"`
	Site      *domain.Site `json:"site,omitempty"`
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		appMiddleware.RespondError(w, http.StatusBadRequest, "BAD_REQUEST", "Invalid login payload")
		return
	}

	ctx := r.Context()
	var user domain.User
	var siteID *uuid.UUID

	// Query from "fy-ox-users" table (Exact requirement)
	query := `
		SELECT id, site_id, email, password_hash, full_name, COALESCE(phone, ''), role, is_active, created_at, updated_at
		FROM "fy-ox-users"
		WHERE email = $1 AND is_active = true
	`
	err := h.db.Pool.QueryRow(ctx, query, req.Email).Scan(
		&user.ID,
		&siteID,
		&user.Email,
		&user.PasswordHash,
		&user.FullName,
		&user.Phone,
		&user.Role,
		&user.IsActive,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			appMiddleware.RespondError(w, http.StatusUnauthorized, "AUTH_FAILED", "E-posta veya parola hatalı")
			return
		}
		appMiddleware.RespondError(w, http.StatusInternalServerError, "DB_ERROR", "Veritabanı hatası: "+err.Error())
		return
	}

	user.SiteID = siteID

	// Verify password hash
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		appMiddleware.RespondError(w, http.StatusUnauthorized, "AUTH_FAILED", "E-posta veya parola hatalı")
		return
	}

	// Fetch site details if user belongs to a site
	var site domain.Site
	effectiveSiteID := uuid.Nil
	if siteID != nil {
		effectiveSiteID = *siteID
		_ = h.db.Pool.QueryRow(ctx, `
			SELECT id, name, code, COALESCE(address, ''), city, COALESCE(district, ''), COALESCE(bank_name, ''), COALESCE(bank_iban, ''), default_late_fee_rate, created_at, updated_at
			FROM "fy-sites"
			WHERE id = $1
		`, *siteID).Scan(
			&site.ID, &site.Name, &site.Code, &site.Address, &site.City, &site.District,
			&site.BankName, &site.BankIBAN, &site.DefaultLateFeeRate, &site.CreatedAt, &site.UpdatedAt,
		)
	}

	// Generate JWT (24 hours validity)
	duration := 24 * time.Hour
	token, err := appMiddleware.GenerateToken(user.ID, effectiveSiteID, user.Role, user.Email, user.FullName, duration)
	if err != nil {
		appMiddleware.RespondError(w, http.StatusInternalServerError, "TOKEN_ERROR", "Token oluşturulamadı")
		return
	}

	resp := LoginResponse{
		Token:     token,
		ExpiresAt: time.Now().Add(duration),
		User:      user,
	}
	if siteID != nil {
		resp.Site = &site
	}

	respondJSON(w, http.StatusOK, resp)
}

func (h *Handler) GetMe(w http.ResponseWriter, r *http.Request) {
	claims := appMiddleware.GetCurrentUser(r.Context())
	if claims == nil {
		appMiddleware.RespondError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Giriş yapmanız gerekmektedir")
		return
	}

	var user domain.User
	err := h.db.Pool.QueryRow(r.Context(), `
		SELECT id, site_id, email, full_name, COALESCE(phone, ''), role, is_active, created_at, updated_at
		FROM "fy-ox-users"
		WHERE id = $1
	`, claims.UserID).Scan(
		&user.ID, &user.SiteID, &user.Email, &user.FullName, &user.Phone, &user.Role,
		&user.IsActive, &user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		appMiddleware.RespondError(w, http.StatusNotFound, "NOT_FOUND", "Kullanıcı bulunamadı")
		return
	}

	respondJSON(w, http.StatusOK, user)
}
