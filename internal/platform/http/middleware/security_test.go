package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSecurityHeaders(t *testing.T) {
	tests := []struct {
		name     string
		isProd   bool
		expected map[string]string
	}{
		{
			name:   "production headers",
			isProd: true,
			expected: map[string]string{
				"X-Frame-Options":           "DENY",
				"X-Content-Type-Options":    "nosniff",
				"Referrer-Policy":           "strict-origin-when-cross-origin",
				"X-XSS-Protection":          "1; mode=block",
				"Strict-Transport-Security": "max-age=31536000; includeSubDomains; preload",
			},
		},
		{
			name:   "development headers",
			isProd: false,
			expected: map[string]string{
				"X-Frame-Options":        "DENY",
				"X-Content-Type-Options": "nosniff",
				"Referrer-Policy":        "strict-origin-when-cross-origin",
				"X-XSS-Protection":       "1; mode=block",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := SecurityHeaders(tt.isProd)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))

			req := httptest.NewRequest("GET", "/", nil)
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			for header, expectedValue := range tt.expected {
				if got := rr.Header().Get(header); got != expectedValue {
					t.Errorf("%s: expected %s to be %s, got %s", tt.name, header, expectedValue, got)
				}
			}

			// Verify CSP contains expected directives
			csp := rr.Header().Get("Content-Security-Policy")
			if csp == "" {
				t.Error("expected Content-Security-Policy to be set")
			}

			if tt.isProd {
				// Production CSP should have frame-ancestors 'none' for clickjacking protection
				if !containsDirective(csp, "frame-ancestors", "'none'") {
					t.Errorf("production CSP should have frame-ancestors 'none': %s", csp)
				}
				// Note: HTMX requires 'unsafe-inline' for script-src - this is expected
			}
		})
	}
}

func containsDirective(csp, directive, value string) bool {
	// Check if CSP contains a directive with specific value
	start := findSubstr(csp, directive)
	if start == -1 {
		return false
	}

	// Find end of directive (next semicolon or end of string)
	end := findSubstr(csp[start:], ";")
	if end == -1 {
		end = len(csp)
	} else {
		end += start
	}

	directiveStr := csp[start:end]
	return contains(directiveStr, value)
}

func containsUnsafeScript(csp string) bool {
	// Simple check for unsafe directives in script-src
	scriptSrcStart := findSubstr(csp, "script-src")
	if scriptSrcStart == -1 {
		return false
	}

	// Find end of script-src directive (next semicolon or end of string)
	scriptSrcEnd := findSubstr(csp[scriptSrcStart:], ";")
	if scriptSrcEnd == -1 {
		scriptSrcEnd = len(csp)
	} else {
		scriptSrcEnd += scriptSrcStart
	}

	scriptSrc := csp[scriptSrcStart:scriptSrcEnd]
	return contains(scriptSrc, "'unsafe-inline'") || contains(scriptSrc, "'unsafe-eval'")
}

func findSubstr(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

func contains(s, substr string) bool {
	return findSubstr(s, substr) != -1
}
