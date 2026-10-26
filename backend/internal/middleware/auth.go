package middleware

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/siteyonet/siteyonet/internal/domain"
)

type contextKey string

const (
	UserContextKey contextKey = "currentUser"
)

type UserClaims struct {
	UserID   uuid.UUID `json:"user_id"`
	SiteID   uuid.UUID `json:"site_id"`
	Role     string    `json:"role"`
	Email    string    `json:"email"`
	FullName string    `json:"full_name"`
	jwt.RegisteredClaims
}

var JWTSecret = []byte("super-secret-jwt-key-siteyonet-production-0c7cc1")

func GenerateToken(userID, siteID uuid.UUID, role, email, fullName string, duration time.Duration) (string, error) {
	claims := UserClaims{
		UserID:   userID,
		SiteID:   siteID,
		Role:     role,
		Email:    email,
		FullName: fullName,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(duration)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "siteyonet-api",
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(JWTSecret)
}

func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			// If not provided, continue without claims (public routes will pass, protected will fail)
			next.ServeHTTP(w, r)
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			RespondError(w, http.StatusUnauthorized, "AUTH_INVALID_HEADER", "Authorization header format must be Bearer {token}")
			return
		}

		tokenString := parts[1]
		claims := &UserClaims{}
		token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return JWTSecret, nil
		})

		if err != nil || !token.Valid {
			RespondError(w, http.StatusUnauthorized, "AUTH_INVALID_TOKEN", "Token is invalid or expired")
			return
		}

		ctx := context.WithValue(r.Context(), UserContextKey, claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequireRoles enforces that request contains one of the allowed roles
func RequireRoles(allowedRoles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, ok := r.Context().Value(UserContextKey).(*UserClaims)
			if !ok || claims == nil {
				RespondError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
				return
			}

			if claims.Role == domain.RoleSuperAdmin {
				next.ServeHTTP(w, r)
				return
			}

			matched := false
			for _, role := range allowedRoles {
				if claims.Role == role {
					matched = true
					break
				}
			}

			if !matched {
				RespondError(w, http.StatusForbidden, "FORBIDDEN", "You do not have permission to access this resource")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// Helper to respond with standard error JSON
func RespondError(w http.ResponseWriter, statusCode int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	resp := domain.APIError{}
	resp.Error.Code = code
	resp.Error.Message = message
	_ = json.NewEncoder(w).Encode(resp)
}

func GetCurrentUser(ctx context.Context) *UserClaims {
	claims, ok := ctx.Value(UserContextKey).(*UserClaims)
	if !ok {
		return nil
	}
	return claims
}
