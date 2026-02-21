package middleware

import (
	"net/http"
	"strconv"
	"strings"
)

type CORSConfig struct {
	AllowedOrigins   []string
	AllowedMethods   []string
	AllowedHeaders   []string
	ExposedHeaders   []string
	AllowCredentials bool
	MaxAge           int
	AllowedPatterns  []string
}

func DefaultCORSConfig() CORSConfig {
	return CORSConfig{
		AllowedOrigins:   []string{},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH", "HEAD"},
		AllowedHeaders:   []string{"Content-Type", "Authorization", "X-Request-ID", "X-CSRF-Token", "Accept", "Accept-Language"},
		ExposedHeaders:   []string{"X-Request-ID", "X-RateLimit-Limit", "X-RateLimit-Remaining", "X-RateLimit-Reset"},
		AllowCredentials: true,
		MaxAge:           300,
		AllowedPatterns:  []string{},
	}
}

func CORS(cfg CORSConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") == "" {
				next.ServeHTTP(w, r)
				return
			}

			origin := r.Header.Get("Origin")

			allowed := false
			allowedOrigin := ""

			if origin != "" {
				for _, o := range cfg.AllowedOrigins {
					if o == "*" {
						allowed = true
						allowedOrigin = origin
						break
					}
					if strings.EqualFold(o, origin) {
						allowed = true
						allowedOrigin = origin
						break
					}
					if strings.HasPrefix(o, "*.") {
						pattern := strings.TrimPrefix(o, "*.")
						if strings.HasSuffix(origin, pattern) {
							allowed = true
							allowedOrigin = origin
							break
						}
					}
				}

				for _, pattern := range cfg.AllowedPatterns {
					if matchWildcard(origin, pattern) {
						allowed = true
						allowedOrigin = origin
						break
					}
				}
			}

			if !allowed && len(cfg.AllowedOrigins) > 0 {
				next.ServeHTTP(w, r)
				return
			}

			if allowedOrigin != "" {
				w.Header().Set("Access-Control-Allow-Origin", allowedOrigin)
			} else if len(cfg.AllowedOrigins) == 0 {
				w.Header().Set("Access-Control-Allow-Origin", "*")
			}

			w.Header().Set("Access-Control-Allow-Methods", strings.Join(cfg.AllowedMethods, ", "))
			w.Header().Set("Access-Control-Allow-Headers", strings.Join(cfg.AllowedHeaders, ", "))
			w.Header().Set("Access-Control-Expose-Headers", strings.Join(cfg.ExposedHeaders, ", "))

			if cfg.AllowCredentials {
				w.Header().Set("Access-Control-Allow-Credentials", "true")
			}

			if cfg.MaxAge > 0 {
				w.Header().Set("Access-Control-Max-Age", strconv.Itoa(cfg.MaxAge))
			}

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func matchWildcard(s, pattern string) bool {
	pattern = strings.ReplaceAll(pattern, ".", "\\.")
	pattern = strings.ReplaceAll(pattern, "*", ".*")
	pattern = "^" + pattern + "$"

	matcher := strings.NewReplacer(
		".*", ".*",
	).Replace(pattern)

	return wildcardMatch(s, matcher)
}

func wildcardMatch(s, pattern string) bool {
	if pattern == "*" {
		return true
	}

	parts := strings.Split(pattern, ".*")

	if len(parts) == 1 {
		return s == pattern
	}

	if !strings.HasPrefix(s, parts[0]) {
		return false
	}

	rest := s[len(parts[0]):]

	for i := 1; i < len(parts)-1; i++ {
		idx := strings.Index(rest, parts[i])
		if idx == -1 {
			return false
		}
		rest = rest[idx+len(parts[i]):]
	}

	return strings.HasSuffix(rest, parts[len(parts)-1])
}
