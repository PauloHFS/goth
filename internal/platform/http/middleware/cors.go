package middleware

import (
	"net/http"
	"strconv"
	"strings"
	"sync"
)

type CORSConfig struct {
	AllowedOrigins   []string
	AllowedMethods   []string
	AllowedHeaders   []string
	ExposedHeaders   []string
	AllowCredentials bool
	MaxAge           int
}

var defaultCORSConfig = CORSConfig{
	AllowedOrigins:   []string{"http://localhost:8080", "http://localhost:3000"},
	AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH"},
	AllowedHeaders:   []string{"Content-Type", "Authorization", "X-Requested-With", "X-CSRF-Token"},
	ExposedHeaders:   []string{"Content-Length", "X-CSRF-Token"},
	AllowCredentials: true,
	MaxAge:           3600,
}

func CORS(cfg *CORSConfig) func(http.Handler) http.Handler {
	if cfg == nil {
		cfg = &defaultCORSConfig
	}

	allowedOrigins := make(map[string]bool)
	for _, origin := range cfg.AllowedOrigins {
		allowedOrigins[strings.ToLower(origin)] = true
	}

	isOriginAllowed := func(origin string) bool {
		originLower := strings.ToLower(origin)
		if allowedOrigins["*"] {
			return true
		}
		return allowedOrigins[originLower]
	}

	methods := strings.Join(cfg.AllowedMethods, ", ")
	headers := strings.Join(cfg.AllowedHeaders, ", ")
	exposedHeaders := strings.Join(cfg.ExposedHeaders, ", ")

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			if r.Method == "OPTIONS" {
				if !isOriginAllowed(origin) {
					w.WriteHeader(http.StatusForbidden)
					return
				}

				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Methods", methods)
				w.Header().Set("Access-Control-Allow-Headers", headers)
				w.Header().Set("Access-Control-Allow-Credentials", boolToString(cfg.AllowCredentials))
				w.Header().Set("Access-Control-Max-Age", stringToString(cfg.MaxAge))

				if exposedHeaders != "" {
					w.Header().Set("Access-Control-Expose-Headers", exposedHeaders)
				}

				w.WriteHeader(http.StatusNoContent)
				return
			}

			if isOriginAllowed(origin) {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				if exposedHeaders != "" {
					w.Header().Set("Access-Control-Expose-Headers", exposedHeaders)
				}
				if cfg.AllowCredentials {
					w.Header().Set("Access-Control-Allow-Credentials", "true")
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}

func boolToString(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

func stringToString(i int) string {
	return strconv.Itoa(i)
}

type corsBuilder struct {
	mu             sync.Mutex
	allowedOrigins map[string]bool
	config         *CORSConfig
}

func NewCORSBuilder() *corsBuilder {
	return &corsBuilder{
		allowedOrigins: make(map[string]bool),
		config:         &defaultCORSConfig,
	}
}

func (b *corsBuilder) AddOrigin(origin string) *corsBuilder {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.allowedOrigins[strings.ToLower(origin)] = true
	return b
}

func (b *corsBuilder) Build() CORSConfig {
	b.mu.Lock()
	defer b.mu.Unlock()

	origins := make([]string, 0, len(b.allowedOrigins))
	for origin := range b.allowedOrigins {
		origins = append(origins, origin)
	}

	if len(origins) == 0 {
		origins = defaultCORSConfig.AllowedOrigins
	}

	return CORSConfig{
		AllowedOrigins:   origins,
		AllowedMethods:   b.config.AllowedMethods,
		AllowedHeaders:   b.config.AllowedHeaders,
		ExposedHeaders:   b.config.ExposedHeaders,
		AllowCredentials: b.config.AllowCredentials,
		MaxAge:           b.config.MaxAge,
	}
}

func NewCORSConfigFromStrings(
	origins, methods, headers, exposedHeaders string,
	allowCredentials bool,
	maxAge int,
) CORSConfig {
	return CORSConfig{
		AllowedOrigins:   parseCommaList(origins),
		AllowedMethods:   parseCommaList(methods),
		AllowedHeaders:   parseCommaList(headers),
		ExposedHeaders:   parseCommaList(exposedHeaders),
		AllowCredentials: allowCredentials,
		MaxAge:           maxAge,
	}
}

func parseCommaList(s string) []string {
	if s == "" {
		return nil
	}
	var result []string
	for _, item := range strings.Split(s, ",") {
		trimmed := strings.TrimSpace(item)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
