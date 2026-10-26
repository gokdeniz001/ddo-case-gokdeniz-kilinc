package middleware

import (
	"net/http"
)

// DDOHeaderMiddleware adds the required "X-DDO-Build: 0c7cc1" header to every response
func DDOHeaderMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-DDO-Build", "0c7cc1")
		next.ServeHTTP(w, r)
	})
}
