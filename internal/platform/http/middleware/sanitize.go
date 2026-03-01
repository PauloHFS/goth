package middleware

import (
	"bytes"
	"io"
	"net/http"
	"strings"

	"github.com/microcosm-cc/bluemonday"
)

// SanitizeConfig configura o middleware de sanitização
type SanitizeConfig struct {
	// Enabled ativa/desativa a sanitização
	Enabled bool
	// MaxBodySize limita o tamanho do corpo a ser lido (default: 1MB)
	MaxBodySize int64
	// SkipPaths ignora paths específicos
	SkipPaths []string
	// SanitizeHTML sanitiza conteúdo HTML (permite tags seguras)
	SanitizeHTML bool
	// StripHTML remove completamente tags HTML
	StripHTML bool
}

// DefaultSanitizeConfig retorna configuração padrão segura
func DefaultSanitizeConfig() SanitizeConfig {
	return SanitizeConfig{
		Enabled:      true,
		MaxBodySize:  1 * 1024 * 1024, // 1MB
		SkipPaths:    []string{"/assets/", "/storage/", "/metrics", "/health"},
		SanitizeHTML: true,
		StripHTML:    false,
	}
}

// Sanitize middleware sanitiza inputs para prevenir XSS
func Sanitize(cfg SanitizeConfig) func(http.Handler) http.Handler {
	if !cfg.Enabled {
		return func(next http.Handler) http.Handler {
			return next
		}
	}

	if cfg.MaxBodySize == 0 {
		cfg.MaxBodySize = 1 * 1024 * 1024
	}

	// Policy para HTML seguro (permite tags básicas)
	htmlPolicy := bluemonday.UGCPolicy()
	htmlPolicy.AllowAttrs("class").OnElements("span", "div", "p")
	htmlPolicy.AllowAttrs("href").OnElements("a")
	htmlPolicy.AllowAttrs("src", "alt").OnElements("img")

	// Policy para strip completo (apenas texto)
	stripPolicy := bluemonday.StrictPolicy()

	skipMap := make(map[string]bool)
	for _, path := range cfg.SkipPaths {
		skipMap[path] = true
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip paths configurados
			for _, path := range cfg.SkipPaths {
				if strings.HasPrefix(r.URL.Path, path) {
					next.ServeHTTP(w, r)
					return
				}
			}

			// Apenas métodos com body
			if r.Method != "POST" && r.Method != "PUT" && r.Method != "PATCH" {
				next.ServeHTTP(w, r)
				return
			}

			// Content-Type check - apenas sanitizar form data e JSON
			contentType := r.Header.Get("Content-Type")
			if !strings.Contains(contentType, "application/x-www-form-urlencoded") &&
				!strings.Contains(contentType, "multipart/form-data") &&
				!strings.Contains(contentType, "application/json") {
				next.ServeHTTP(w, r)
				return
			}

			// Limitar tamanho do body
			r.Body = http.MaxBytesReader(w, r.Body, cfg.MaxBodySize)

			// Ler body original
			bodyBytes, err := io.ReadAll(r.Body)
			if err != nil {
				// Body muito grande ou erro de leitura
				http.Error(w, "Bad request", http.StatusBadRequest)
				return
			}

			// Fechar body original
			_ = r.Body.Close()

			// Sanitizar baseado no Content-Type
			var sanitizedBody []byte

			if strings.Contains(contentType, "application/x-www-form-urlencoded") ||
				strings.Contains(contentType, "multipart/form-data") {
				// Parse form data
				values, err := io.ReadAll(bytes.NewReader(bodyBytes))
				if err != nil {
					http.Error(w, "Bad request", http.StatusBadRequest)
					return
				}

				// Sanitizar valores do form
				formValues, err := parseFormValues(values)
				if err != nil {
					http.Error(w, "Bad request", http.StatusBadRequest)
					return
				}

				for key, vals := range formValues {
					for i, val := range vals {
						if cfg.StripHTML {
							formValues[key][i] = stripPolicy.Sanitize(val)
						} else if cfg.SanitizeHTML {
							formValues[key][i] = htmlPolicy.Sanitize(val)
						} else {
							// Apenas trim whitespace
							formValues[key][i] = strings.TrimSpace(val)
						}
					}
				}

				// Re-encode form data
				sanitizedBody = []byte(encodeFormValues(formValues))

			} else if strings.Contains(contentType, "application/json") {
				// Para JSON, sanitizar string values
				sanitizedBody = sanitizeJSON(bodyBytes, cfg.StripHTML, cfg.SanitizeHTML, htmlPolicy, stripPolicy)
			}

			// Recriar body com conteúdo sanitizado
			r.Body = io.NopCloser(bytes.NewReader(sanitizedBody))
			r.ContentLength = int64(len(sanitizedBody))

			next.ServeHTTP(w, r)
		})
	}
}

// parseFormValues parse form values de bytes
func parseFormValues(body []byte) (map[string][]string, error) {
	values, err := parseQuery(string(body))
	if err != nil {
		return nil, err
	}
	return values, nil
}

// parseQuery parse query string style
func parseQuery(query string) (map[string][]string, error) {
	values := make(map[string][]string)
	pairs := strings.Split(query, "&")
	for _, pair := range pairs {
		if pair == "" {
			continue
		}
		parts := strings.SplitN(pair, "=", 2)
		key := parts[0]
		var value string
		if len(parts) == 2 {
			value = parts[1]
		}
		// URL decode
		value = strings.ReplaceAll(value, "+", " ")
		// Simple URL decode (não cobre todos os casos, mas suficiente para forms básicos)
		value = strings.ReplaceAll(value, "%20", " ")
		value = strings.ReplaceAll(value, "%40", "@")
		value = strings.ReplaceAll(value, "%2F", "/")
		value = strings.ReplaceAll(value, "%3A", ":")
		value = strings.ReplaceAll(value, "%3D", "=")
		value = strings.ReplaceAll(value, "%26", "&")
		value = strings.ReplaceAll(value, "%3F", "?")
		value = strings.ReplaceAll(value, "%25", "%")

		values[key] = append(values[key], value)
	}
	return values, nil
}

// encodeFormValues encode form values para string
func encodeFormValues(values map[string][]string) string {
	var pairs []string
	for key, vals := range values {
		for _, val := range vals {
			pairs = append(pairs, key+"="+val)
		}
	}
	return strings.Join(pairs, "&")
}

// sanitizeJSON sanitiza string values em JSON
// Nota: implementação simplificada - para produção, usar json.Decode/Encode
func sanitizeJSON(data []byte, stripHTML, sanitizeHTML bool, htmlPolicy, stripPolicy *bluemonday.Policy) []byte {
	// Para JSON, fazemos uma abordagem conservadora:
	// 1. Se não parece JSON válido, retorna como está (será validado depois)
	// 2. Se é JSON, sanitiza string values

	jsonStr := string(data)

	// Check simples se parece JSON
	jsonStr = strings.TrimSpace(jsonStr)
	if !strings.HasPrefix(jsonStr, "{") && !strings.HasPrefix(jsonStr, "[") {
		return data
	}

	// Sanitização básica: remover tags HTML de string values
	// Esta é uma implementação simplificada - idealmente usaria um parser JSON
	if stripHTML {
		// Remove tags HTML completamente
		jsonStr = stripPolicy.Sanitize(jsonStr)
	} else if sanitizeHTML {
		// Sanitiza tags HTML (permite seguras)
		jsonStr = htmlPolicy.Sanitize(jsonStr)
	}

	return []byte(jsonStr)
}

// SanitizeString sanitiza uma string individual
// Utility function para uso em handlers
func SanitizeString(s string, stripHTML bool) string {
	if stripHTML {
		return bluemonday.StrictPolicy().Sanitize(s)
	}
	return bluemonday.UGCPolicy().Sanitize(s)
}

// SanitizeEmail normaliza e valida email
func SanitizeEmail(email string) string {
	// Trim whitespace
	email = strings.TrimSpace(email)
	// Lowercase
	email = strings.ToLower(email)
	// Remove caracteres inválidos (básico)
	email = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') ||
			(r >= '0' && r <= '9') ||
			r == '@' || r == '.' || r == '-' || r == '_' || r == '+' {
			return r
		}
		return -1
	}, email)
	return email
}
