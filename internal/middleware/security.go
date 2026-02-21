package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
)

func SecurityHeaders(isProd bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			nonce := generateNonce()

			w.Header().Set("X-Frame-Options", "DENY")
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
			w.Header().Set("X-XSS-Protection", "1; mode=block")

			if isProd {
				w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
			}

			w.Header().Set("Content-Security-Policy",
				"default-src 'self'; "+
					"script-src 'self' 'nonce-"+nonce+"' https://cdn.jsdelivr.net https://unpkg.com; "+
					"style-src 'self' 'unsafe-inline'; "+
					"img-src 'self' data: https:; "+
					"font-src 'self'; "+
					"connect-src 'self' /events; "+
					"frame-ancestors 'none';")

			next.ServeHTTP(w, r)
		})
	}
}

func generateNonce() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
