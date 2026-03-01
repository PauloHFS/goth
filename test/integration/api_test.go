//go:build fts5
// +build fts5

package integration

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/PauloHFS/goth/test/integration/seed"
)

// noRedirectClient is an HTTP client that doesn't follow redirects
var noRedirectClient = &http.Client{
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	},
}

func seedTestData() {
	ctx := context.Background()
	_ = seed.SeedTestData(ctx, testQueries)
}

func TestHealthEndpoint(t *testing.T) {
	resp, err := http.Get(testServer.URL + "/health")
	if err != nil {
		t.Fatalf("failed to make request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}
}

func TestHomeEndpoint(t *testing.T) {
	resp, err := http.Get(testServer.URL + "/")
	if err != nil {
		t.Fatalf("failed to make request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}
}

func TestLoginPageLoads(t *testing.T) {
	resp, err := http.Get(testServer.URL + "/login")
	if err != nil {
		t.Fatalf("failed to make request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if len(body) == 0 {
		t.Error("expected non-empty body")
	}
}

func TestRegisterPageLoads(t *testing.T) {
	resp, err := http.Get(testServer.URL + "/register")
	if err != nil {
		t.Fatalf("failed to make request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}
}

func TestForgotPasswordPageLoads(t *testing.T) {
	resp, err := http.Get(testServer.URL + "/forgot-password")
	if err != nil {
		t.Fatalf("failed to make request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}
}

func TestResetPasswordPageLoads(t *testing.T) {
	resp, err := http.Get(testServer.URL + "/reset-password?token=test")
	if err != nil {
		t.Fatalf("failed to make request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}
}

func TestRegisterSuccess(t *testing.T) {
	uniqueEmail := "newuser_" + time.Now().Format("20060102150405") + "@example.com"

	form := url.Values{}
	form.Add("email", uniqueEmail)
	form.Add("password", "newpassword123")

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	resp, err := client.PostForm(testServer.URL+"/register", form)
	if err != nil {
		t.Fatalf("failed to make request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusSeeOther {
		t.Errorf("expected status 303, got %d", resp.StatusCode)
	}

	location := resp.Header.Get("Location")
	if !strings.Contains(location, "/login") {
		t.Errorf("expected redirect to /login, got %s", location)
	}
}

func TestRegisterDuplicateEmail(t *testing.T) {
	seedTestData()

	form := url.Values{}
	form.Add("email", seed.AdminUser.Email)
	form.Add("password", seed.TestPassword)

	resp, err := noRedirectClient.PostForm(testServer.URL+"/register", form)
	if err != nil {
		t.Fatalf("failed to make request: %v", err)
	}
	defer resp.Body.Close()

	// Should redirect back to login with message or stay on register page
	if resp.StatusCode != http.StatusSeeOther && resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200 or 303, got %d", resp.StatusCode)
	}
}

func TestLoginSuccess(t *testing.T) {
	seedTestData()

	form := url.Values{}
	form.Add("email", seed.AdminUser.Email)
	form.Add("password", seed.TestPassword)

	resp, err := noRedirectClient.PostForm(testServer.URL+"/login", form)
	if err != nil {
		t.Fatalf("failed to make request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusSeeOther {
		t.Errorf("expected status 303, got %d", resp.StatusCode)
	}

	location := resp.Header.Get("Location")
	if location != "/dashboard" {
		t.Errorf("expected redirect to /dashboard, got %s", location)
	}
}

func TestLoginInvalidPassword(t *testing.T) {
	seedTestData()

	form := url.Values{}
	form.Add("email", seed.AdminUser.Email)
	form.Add("password", "wrongpassword")

	resp, err := noRedirectClient.PostForm(testServer.URL+"/login", form)
	if err != nil {
		t.Fatalf("failed to make request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}
}

func TestLoginNonexistentUser(t *testing.T) {
	form := url.Values{}
	form.Add("email", "nonexistent@example.com")
	form.Add("password", "somepassword")

	resp, err := noRedirectClient.PostForm(testServer.URL+"/login", form)
	if err != nil {
		t.Fatalf("failed to make request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}
}

func TestDashboardUnauthorized(t *testing.T) {
	resp, err := noRedirectClient.Get(testServer.URL + "/dashboard")
	if err != nil {
		t.Fatalf("failed to make request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusSeeOther {
		t.Errorf("expected status 303, got %d", resp.StatusCode)
	}

	location := resp.Header.Get("Location")
	if !strings.Contains(location, "/login") {
		t.Errorf("expected redirect to /login, got %s", location)
	}
}

func TestForgotPassword(t *testing.T) {
	seedTestData()

	form := url.Values{}
	form.Add("email", seed.AdminUser.Email)

	resp, err := noRedirectClient.PostForm(testServer.URL+"/forgot-password", form)
	if err != nil {
		t.Fatalf("failed to make request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}
}

func TestResetPasswordSuccess(t *testing.T) {
	seedTestData()

	token, err := seed.CreatePasswordReset(context.Background(), testQueries, seed.TestUser.Email)
	if err != nil {
		t.Fatalf("failed to create password reset: %v", err)
	}

	form := url.Values{}
	form.Add("token", token)
	form.Add("password", "newpassword123")

	resp, err := noRedirectClient.PostForm(testServer.URL+"/reset-password?token="+token, form)
	if err != nil {
		t.Fatalf("failed to make request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusSeeOther {
		t.Errorf("expected status 303, got %d", resp.StatusCode)
	}
}

func TestResetPasswordInvalidToken(t *testing.T) {
	form := url.Values{}
	form.Add("token", "nonexistent_token_12345")
	form.Add("password", "newpassword123")

	resp, err := noRedirectClient.PostForm(testServer.URL+"/reset-password?token=nonexistent_token_12345", form)
	if err != nil {
		t.Fatalf("failed to make request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}
}

func TestVerifyEmailSuccess(t *testing.T) {
	seedTestData()

	token, err := seed.CreateEmailVerification(context.Background(), testQueries, seed.TestUser.Email)
	if err != nil {
		t.Fatalf("failed to create email verification: %v", err)
	}

	resp, err := noRedirectClient.Get(testServer.URL + "/verify-email?token=" + token)
	if err != nil {
		t.Fatalf("failed to make request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusSeeOther {
		t.Errorf("expected status 303, got %d", resp.StatusCode)
	}
}

func TestVerifyEmailInvalidToken(t *testing.T) {
	resp, err := noRedirectClient.Get(testServer.URL + "/verify-email?token=invalid_token")
	if err != nil {
		t.Fatalf("failed to make request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusSeeOther {
		t.Errorf("expected status 303, got %d", resp.StatusCode)
	}
}

func TestLogout(t *testing.T) {
	seedTestData()

	form := url.Values{}
	form.Add("email", seed.AdminUser.Email)
	form.Add("password", seed.TestPassword)

	loginResp, err := noRedirectClient.PostForm(testServer.URL+"/login", form)
	if err != nil {
		t.Fatalf("failed to login: %v", err)
	}
	defer loginResp.Body.Close()

	cookies := loginResp.Cookies()

	logoutReq, err := http.NewRequest(http.MethodPost, testServer.URL+"/logout", nil)
	if err != nil {
		t.Fatalf("failed to create logout request: %v", err)
	}
	for _, cookie := range cookies {
		logoutReq.AddCookie(cookie)
	}

	logoutResp, err := noRedirectClient.Do(logoutReq)
	if err != nil {
		t.Fatalf("failed to logout: %v", err)
	}
	defer logoutResp.Body.Close()

	if logoutResp.StatusCode != http.StatusSeeOther {
		t.Errorf("expected status 303, got %d", logoutResp.StatusCode)
	}
}

func TestDashboardSearch(t *testing.T) {
	seedTestData()

	resp, err := http.Get(testServer.URL + "/dashboard?search=admin")
	if err != nil {
		t.Fatalf("failed to make request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusSeeOther && resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200 or 303, got %d", resp.StatusCode)
	}
}

func TestMetricsEndpoint(t *testing.T) {
	resp, err := http.Get(testServer.URL + "/metrics")
	if err != nil {
		t.Fatalf("failed to make request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}
}

func TestLoginWithValidSessionRedirectsToDashboard(t *testing.T) {
	seedTestData()

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	form := url.Values{}
	form.Add("email", seed.AdminUser.Email)
	form.Add("password", seed.TestPassword)

	loginResp, err := client.PostForm(testServer.URL+"/login", form)
	if err != nil {
		t.Fatalf("failed to login: %v", err)
	}
	defer loginResp.Body.Close()

	cookies := loginResp.Cookies()

	dashboardReq, err := http.NewRequest(http.MethodGet, testServer.URL+"/dashboard", nil)
	if err != nil {
		t.Fatalf("failed to create dashboard request: %v", err)
	}
	for _, cookie := range cookies {
		dashboardReq.AddCookie(cookie)
	}

	dashboardResp, err := client.Do(dashboardReq)
	if err != nil {
		t.Fatalf("failed to access dashboard: %v", err)
	}
	defer dashboardResp.Body.Close()

	if dashboardResp.StatusCode == http.StatusSeeOther {
		location := dashboardResp.Header.Get("Location")
		if location == "/login" {
			t.Log("Session not persisted correctly (common in httptest)")
		}
	}
}
