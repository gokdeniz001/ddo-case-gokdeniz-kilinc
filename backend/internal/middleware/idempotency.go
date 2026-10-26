package middleware

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type responseRecorder struct {
	http.ResponseWriter
	statusCode int
	body       *bytes.Buffer
}

func (r *responseRecorder) WriteHeader(code int) {
	r.statusCode = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	r.body.Write(b)
	return r.ResponseWriter.Write(b)
}

type MemoryIdempotencyStore struct {
	mu    sync.RWMutex
	cache map[string]CachedResponse
}

type CachedResponse struct {
	StatusCode int
	Body       []byte
	CreatedAt  time.Time
}

var IdempotencyStore = &MemoryIdempotencyStore{
	cache: make(map[string]CachedResponse),
}

// IdempotencyMiddleware ensures idempotent state-changing POST requests
func IdempotencyMiddleware(dbPool *pgxpool.Pool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				next.ServeHTTP(w, r)
				return
			}

			idempotencyKey := r.Header.Get("Idempotency-Key")
			if idempotencyKey == "" {
				next.ServeHTTP(w, r)
				return
			}

			// Read body to compute hash
			bodyBytes, err := io.ReadAll(r.Body)
			if err != nil {
				RespondError(w, http.StatusBadRequest, "BAD_REQUEST", "Could not read request body")
				return
			}
			r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

			hasher := sha256.New()
			hasher.Write(bodyBytes)
			reqHash := hex.EncodeToString(hasher.Sum(nil))

			user := GetCurrentUser(r.Context())
			siteID := ""
			if user != nil {
				siteID = user.SiteID.String()
			}
			compositeKey := siteID + ":" + idempotencyKey

			// Check DB or memory cache
			IdempotencyStore.mu.RLock()
			cached, exists := IdempotencyStore.cache[compositeKey]
			IdempotencyStore.mu.RUnlock()

			if exists {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("X-Cache-Lookup", "HIT-IDEMPOTENT")
				w.WriteHeader(cached.StatusCode)
				_, _ = w.Write(cached.Body)
				return
			}

			// Record response
			rec := &responseRecorder{
				ResponseWriter: w,
				statusCode:     http.StatusOK,
				body:           bytes.NewBuffer(nil),
			}

			next.ServeHTTP(rec, r)

			// If success response, store in memory and db
			if rec.statusCode >= 200 && rec.statusCode < 300 {
				IdempotencyStore.mu.Lock()
				IdempotencyStore.cache[compositeKey] = CachedResponse{
					StatusCode: rec.statusCode,
					Body:       rec.body.Bytes(),
					CreatedAt:  time.Now(),
				}
				IdempotencyStore.mu.Unlock()

				// Optionally persist in DB if pool is available
				if dbPool != nil && user != nil {
					go func(key, sID, uID, rHash string, code int, respBody []byte) {
						ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
						defer cancel()
						var jsonVal interface{}
						_ = json.Unmarshal(respBody, &jsonVal)
						_, _ = dbPool.Exec(ctx, `
							INSERT INTO "fy-idempotency-keys" (key, site_id, user_id, request_hash, response_code, response_body)
							VALUES ($1, $2::uuid, $3::uuid, $4, $5, $6)
							ON CONFLICT (key, site_id) DO NOTHING
						`, key, sID, uID, rHash, code, respBody)
					}(idempotencyKey, siteID, user.UserID.String(), reqHash, rec.statusCode, rec.body.Bytes())
				}
			}
		})
	}
}
