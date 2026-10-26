package middleware

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const TransactionKey contextKey = "tenantTransaction"

// The request transaction owns both the business changes and cached response.
// A database advisory lock serializes identical keys, including across API replicas.
func IdempotencyMiddleware(_ *pgxpool.Pool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := r.Header.Get("Idempotency-Key")
			if r.Method != http.MethodPost || key == "" {
				next.ServeHTTP(w, r)
				return
			}
			user := GetCurrentUser(r.Context())
			tx, ok := r.Context().Value(TransactionKey).(pgx.Tx)
			if user == nil || !ok {
				RespondError(w, 500, "TX_REQUIRED", "Transaction required")
				return
			}
			body, err := io.ReadAll(io.LimitReader(r.Body, 1048577))
			if err != nil || len(body) > 1048576 {
				RespondError(w, 400, "BAD_REQUEST", "Request body too large or unreadable")
				return
			}
			r.Body = io.NopCloser(bytes.NewReader(body))
			sum := sha256.Sum256(append([]byte(r.Method+" "+r.URL.RequestURI()+" "+user.UserID.String()+"\n"), body...))
			hash := hex.EncodeToString(sum[:])
			if _, err := tx.Exec(r.Context(), `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, user.SiteID.String()+":"+key); err != nil {
				RespondError(w, 500, "DB_ERROR", "Idempotency lock failed")
				return
			}
			var storedHash string
			var code int
			var cached []byte
			err = tx.QueryRow(r.Context(), `SELECT request_hash,response_code,response_body FROM "fy-idempotency-keys" WHERE site_id=$1 AND key=$2`, user.SiteID, key).Scan(&storedHash, &code, &cached)
			if err == nil {
				if storedHash != hash {
					RespondError(w, 409, "IDEMPOTENCY_CONFLICT", "Key already used for another request")
					return
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(code)
				_, _ = w.Write(cached)
				return
			}
			if err != pgx.ErrNoRows {
				RespondError(w, 500, "DB_ERROR", "Idempotency lookup failed")
				return
			}
			rec := httptest.NewRecorder()
			next.ServeHTTP(rec, r)
			if rec.Code >= 200 && rec.Code < 300 {
				_, err = tx.Exec(r.Context(), `INSERT INTO "fy-idempotency-keys" (key,site_id,user_id,request_hash,response_code,response_body) VALUES ($1,$2,$3,$4,$5,$6::jsonb)`, key, user.SiteID, user.UserID, hash, rec.Code, rec.Body.String())
				if err != nil {
					RespondError(w, 500, "DB_ERROR", "Idempotency persistence failed")
					return
				}
			}
			for k, v := range rec.Header() {
				w.Header()[k] = v
			}
			w.WriteHeader(rec.Code)
			_, _ = w.Write(rec.Body.Bytes())
		})
	}
}
