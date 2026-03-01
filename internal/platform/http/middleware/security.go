package middleware

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"

	"github.com/PauloHFS/goth/internal/contextkeys"
)

// CSPConfig configura a Content Security Policy
type CSPConfig struct {
	// ScriptSrc define fontes permitidas para scripts
	ScriptSrc []string
	// StyleSrc define fontes permitidas para estilos
	StyleSrc []string
	// ImgSrc define fontes permitidas para imagens
	ImgSrc []string
	// ConnectSrc define fontes permitidas para XHR/fetch/WebSocket
	ConnectSrc []string
	// FontSrc define fontes permitidas para fonts
	FontSrc []string
	// ObjectSrc define fontes permitidas para object/embed
	ObjectSrc []string
	// MediaSrc define fontes permitidas para media
	MediaSrc []string
	// FrameSrc define fontes permitidas para iframe
	FrameSrc []string
	// DefaultSrc define fallback para todas as diretivas
	DefaultSrc []string
	// BaseUri define URLs permitidas para <base>
	BaseUri []string
	// FormAction define URLs permitidas para form action
	FormAction []string
	// FrameAncestors define quais origins podem embedar esta página
	FrameAncestors []string
	// UpgradeInsecureRequests converte HTTP para HTTPS
	UpgradeInsecureRequests bool
	// BlockAllMixedContent bloqueia conteúdo misto
	BlockAllMixedContent bool
	// ReportUri URI para reportar violações (CSP Level 2)
	ReportUri string
	// ReportTo nome do endpoint Report-To (CSP Level 3)
	ReportTo string
}

// DefaultCSPConfig retorna uma CSP segura e compatível com HTMX
func DefaultCSPConfig() CSPConfig {
	return CSPConfig{
		// HTMX requer 'unsafe-inline' para scripts inline
		// 'wasm-unsafe-eval' permite WebAssembly de forma segura
		ScriptSrc: []string{"'self'", "'unsafe-inline'", "https://cdn.jsdelivr.net"},
		// Estilos inline são comuns em SSR
		StyleSrc: []string{"'self'", "'unsafe-inline'", "https://fonts.googleapis.com"},
		// Imagens de múltiplas fontes
		ImgSrc: []string{"'self'", "data:", "https:", "blob:"},
		// Conexões para API externa e analytics
		ConnectSrc: []string{"'self'", "https://*.googleapis.com"},
		// Fonts do Google
		FontSrc: []string{"'self'", "https://fonts.gstatic.com"},
		// Bloqueia object/embed por segurança
		ObjectSrc: []string{"'none'"},
		// Media de fontes próprias
		MediaSrc: []string{"'self'"},
		// Iframes apenas de fontes próprias
		FrameSrc: []string{"'self'"},
		// Fallback geral
		DefaultSrc: []string{"'self'"},
		// Base URI restrito
		BaseUri: []string{"'self'"},
		// Form action restrito
		FormAction: []string{"'self'"},
		// Previne clickjacking via frame ancestors
		FrameAncestors: []string{"'none'"},
		// Upgrade HTTP para HTTPS em browsers compatíveis
		UpgradeInsecureRequests: true,
		// Bloqueia conteúdo misto
		BlockAllMixedContent: true,
	}
}

// generateNonce gera um nonce criptograficamente seguro para CSP Level 3
func generateNonce() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(bytes), nil
}

// BuildCSPHeaderWithNonce constrói o header CSP com nonce para scripts
func BuildCSPHeaderWithNonce(cfg CSPConfig, nonce string) string {
	var parts []string

	if len(cfg.DefaultSrc) > 0 {
		parts = append(parts, fmt.Sprintf("default-src %s", strings.Join(cfg.DefaultSrc, " ")))
	}
	if len(cfg.ScriptSrc) > 0 {
		// Substituir 'unsafe-inline' por nonce se disponível
		scriptSrc := make([]string, len(cfg.ScriptSrc))
		for i, src := range cfg.ScriptSrc {
			if src == "'unsafe-inline'" && nonce != "" {
				scriptSrc[i] = fmt.Sprintf("'nonce-%s'", nonce)
			} else {
				scriptSrc[i] = src
			}
		}
		parts = append(parts, fmt.Sprintf("script-src %s", strings.Join(scriptSrc, " ")))
	}
	if len(cfg.StyleSrc) > 0 {
		parts = append(parts, fmt.Sprintf("style-src %s", strings.Join(cfg.StyleSrc, " ")))
	}
	if len(cfg.ImgSrc) > 0 {
		parts = append(parts, fmt.Sprintf("img-src %s", strings.Join(cfg.ImgSrc, " ")))
	}
	if len(cfg.ConnectSrc) > 0 {
		parts = append(parts, fmt.Sprintf("connect-src %s", strings.Join(cfg.ConnectSrc, " ")))
	}
	if len(cfg.FontSrc) > 0 {
		parts = append(parts, fmt.Sprintf("font-src %s", strings.Join(cfg.FontSrc, " ")))
	}
	if len(cfg.ObjectSrc) > 0 {
		parts = append(parts, fmt.Sprintf("object-src %s", strings.Join(cfg.ObjectSrc, " ")))
	}
	if len(cfg.MediaSrc) > 0 {
		parts = append(parts, fmt.Sprintf("media-src %s", strings.Join(cfg.MediaSrc, " ")))
	}
	if len(cfg.FrameSrc) > 0 {
		parts = append(parts, fmt.Sprintf("frame-src %s", strings.Join(cfg.FrameSrc, " ")))
	}
	if len(cfg.BaseUri) > 0 {
		parts = append(parts, fmt.Sprintf("base-uri %s", strings.Join(cfg.BaseUri, " ")))
	}
	if len(cfg.FormAction) > 0 {
		parts = append(parts, fmt.Sprintf("form-action %s", strings.Join(cfg.FormAction, " ")))
	}
	if len(cfg.FrameAncestors) > 0 {
		parts = append(parts, fmt.Sprintf("frame-ancestors %s", strings.Join(cfg.FrameAncestors, " ")))
	}
	if cfg.UpgradeInsecureRequests {
		parts = append(parts, "upgrade-insecure-requests")
	}
	if cfg.BlockAllMixedContent {
		parts = append(parts, "block-all-mixed-content")
	}
	if cfg.ReportUri != "" {
		parts = append(parts, fmt.Sprintf("report-uri %s", cfg.ReportUri))
	}
	if cfg.ReportTo != "" {
		parts = append(parts, fmt.Sprintf("report-to %s", cfg.ReportTo))
	}

	return strings.Join(parts, "; ")
}

// BuildCSPHeader constrói o header CSP a partir da config
func BuildCSPHeader(cfg CSPConfig) string {
	var parts []string

	if len(cfg.DefaultSrc) > 0 {
		parts = append(parts, fmt.Sprintf("default-src %s", strings.Join(cfg.DefaultSrc, " ")))
	}
	if len(cfg.ScriptSrc) > 0 {
		parts = append(parts, fmt.Sprintf("script-src %s", strings.Join(cfg.ScriptSrc, " ")))
	}
	if len(cfg.StyleSrc) > 0 {
		parts = append(parts, fmt.Sprintf("style-src %s", strings.Join(cfg.StyleSrc, " ")))
	}
	if len(cfg.ImgSrc) > 0 {
		parts = append(parts, fmt.Sprintf("img-src %s", strings.Join(cfg.ImgSrc, " ")))
	}
	if len(cfg.ConnectSrc) > 0 {
		parts = append(parts, fmt.Sprintf("connect-src %s", strings.Join(cfg.ConnectSrc, " ")))
	}
	if len(cfg.FontSrc) > 0 {
		parts = append(parts, fmt.Sprintf("font-src %s", strings.Join(cfg.FontSrc, " ")))
	}
	if len(cfg.ObjectSrc) > 0 {
		parts = append(parts, fmt.Sprintf("object-src %s", strings.Join(cfg.ObjectSrc, " ")))
	}
	if len(cfg.MediaSrc) > 0 {
		parts = append(parts, fmt.Sprintf("media-src %s", strings.Join(cfg.MediaSrc, " ")))
	}
	if len(cfg.FrameSrc) > 0 {
		parts = append(parts, fmt.Sprintf("frame-src %s", strings.Join(cfg.FrameSrc, " ")))
	}
	if len(cfg.BaseUri) > 0 {
		parts = append(parts, fmt.Sprintf("base-uri %s", strings.Join(cfg.BaseUri, " ")))
	}
	if len(cfg.FormAction) > 0 {
		parts = append(parts, fmt.Sprintf("form-action %s", strings.Join(cfg.FormAction, " ")))
	}
	if len(cfg.FrameAncestors) > 0 {
		parts = append(parts, fmt.Sprintf("frame-ancestors %s", strings.Join(cfg.FrameAncestors, " ")))
	}
	if cfg.UpgradeInsecureRequests {
		parts = append(parts, "upgrade-insecure-requests")
	}
	if cfg.BlockAllMixedContent {
		parts = append(parts, "block-all-mixed-content")
	}
	if cfg.ReportUri != "" {
		parts = append(parts, fmt.Sprintf("report-uri %s", cfg.ReportUri))
	}
	if cfg.ReportTo != "" {
		parts = append(parts, fmt.Sprintf("report-to %s", cfg.ReportTo))
	}

	return strings.Join(parts, "; ")
}

// SecurityHeaders middleware adiciona headers de segurança
func SecurityHeaders(isProd bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Headers básicos de segurança
			w.Header().Set("X-Frame-Options", "DENY")
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
			w.Header().Set("X-XSS-Protection", "1; mode=block")

			// Gerar nonce para CSP Level 3 (opcional, melhora segurança)
			nonce, err := generateNonce()
			if err != nil {
				nonce = "" // Fallback para unsafe-inline se geração falhar
			}

			// Injetar nonce no contexto para uso em templates
			if nonce != "" {
				ctx := context.WithValue(r.Context(), contextkeys.CSPNonceKey, nonce)
				r = r.WithContext(ctx)
			}

			// Content Security Policy (CSP) - compatível com HTMX
			cspConfig := DefaultCSPConfig()
			if !isProd {
				// Em desenvolvimento, permitir mais flexibilidade
				cspConfig.ScriptSrc = append(cspConfig.ScriptSrc, "http://localhost:*")
				cspConfig.StyleSrc = append(cspConfig.StyleSrc, "http://localhost:*")
				cspConfig.ConnectSrc = append(cspConfig.ConnectSrc, "http://localhost:*", "ws://localhost:*")
			}

			// Usar CSP com nonce se disponível
			if nonce != "" {
				w.Header().Set("Content-Security-Policy", BuildCSPHeaderWithNonce(cspConfig, nonce))
			} else {
				w.Header().Set("Content-Security-Policy", BuildCSPHeader(cspConfig))
			}

			// Permissions Policy (antigo Feature Policy)
			// Restringe funcionalidades do browser
			w.Header().Set("Permissions-Policy",
				"geolocation=(), microphone=(), camera=(), payment=(), usb=(), magnetometer=(), gyroscope=(), accelerometer=()")

			// HSTS apenas em produção
			if isProd {
				w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains; preload")
			}

			next.ServeHTTP(w, r)
		})
	}
}

// GetCSPNonce extrai o nonce do contexto de forma type-safe
func GetCSPNonce(ctx context.Context) string {
	if nonce, ok := ctx.Value(contextkeys.CSPNonceKey).(string); ok {
		return nonce
	}
	return ""
}
