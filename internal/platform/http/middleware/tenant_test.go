package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/PauloHFS/goth/internal/contextkeys"
)

func TestTenantExtractor(t *testing.T) {
	tests := []struct {
		name           string
		header         string
		host           string
		path           string
		expectedTenant string
	}{
		{
			name:           "X-Tenant-ID header takes priority",
			header:         "custom-tenant",
			host:           "other.example.com",
			path:           "/tenant/other2/dashboard",
			expectedTenant: "custom-tenant",
		},
		{
			name:           "subdomain extraction",
			header:         "",
			host:           "acme.example.com",
			path:           "/",
			expectedTenant: "acme",
		},
		{
			name:           "localhost ignored (returns default)",
			header:         "",
			host:           "localhost:8080",
			path:           "/",
			expectedTenant: "default",
		},
		{
			name:           "path-based extraction when no header or subdomain",
			header:         "",
			host:           "example.com",
			path:           "/tenant/path-based/dashboard",
			expectedTenant: "path-based",
		},
		{
			name:           "www prefix ignored (returns default)",
			header:         "",
			host:           "www.example.com",
			path:           "/",
			expectedTenant: "default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var capturedTenant string

			handler := TenantExtractor("default")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				capturedTenant = GetTenantID(r.Context())
			}))

			req := httptest.NewRequest("GET", tt.path, nil)
			if tt.header != "" {
				req.Header.Set("X-Tenant-ID", tt.header)
			}
			req.Host = tt.host

			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			if capturedTenant != tt.expectedTenant {
				t.Errorf("expected tenant %q, got %q", tt.expectedTenant, capturedTenant)
			}
		})
	}
}

func TestGetTenantID(t *testing.T) {
	t.Run("returns tenant from context", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), contextkeys.TenantContextKey, "test-tenant")
		tenant := GetTenantID(ctx)
		if tenant != "test-tenant" {
			t.Errorf("expected test-tenant, got %s", tenant)
		}
	})

	t.Run("returns default when not in context", func(t *testing.T) {
		tenant := GetTenantID(context.Background())
		if tenant != DefaultTenantID {
			t.Errorf("expected default tenant, got %s", tenant)
		}
	})
}
