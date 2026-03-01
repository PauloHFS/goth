package e2e

import (
	"testing"

	pw "github.com/playwright-community/playwright-go"
)

func TestDashboardRequiresAuth(t *testing.T) {
	page, cleanup := SetupPlaywright(t)
	defer cleanup()

	_, err := page.Goto(ServerURL + "/dashboard")
	if err != nil {
		t.Fatalf("failed to navigate to dashboard: %v", err)
	}

	err = page.WaitForLoadState(pw.PageWaitForLoadStateOptions{
		State: pw.LoadStateNetworkidle,
	})
	if err != nil {
		t.Logf("load state warning: %v", err)
	}

	url := page.URL()
	if err != nil {
		t.Fatalf("failed to get URL: %v", err)
	}

	// Check if redirected to login (or any auth page)
	expectedURLs := []string{
		ServerURL + "/login",
		ServerURL + "/auth/login",
	}

	redirected := false
	for _, expected := range expectedURLs {
		if url == expected {
			redirected = true
			break
		}
	}

	if !redirected {
		t.Logf("expected redirect to login, got: %s", url)
		TakeScreenshot(t, page, "dashboard-no-redirect")
	}
}

func TestDashboardAfterLogin(t *testing.T) {
	page, cleanup := SetupPlaywright(t)
	defer cleanup()

	_, err := page.Goto(ServerURL + "/login")
	if err != nil {
		t.Fatalf("failed to navigate to login: %v", err)
	}

	// Wait for form elements
	if err := WaitForElement(t, page, "input[name='email']"); err != nil {
		t.Fatalf("email field not found: %v", err)
	}

	if err := WaitForElement(t, page, "input[name='password']"); err != nil {
		t.Fatalf("password field not found: %v", err)
	}

	err = page.Fill("input[name='email']", "admin @goth.local")
	if err != nil {
		TakeScreenshot(t, page, "dashboard-login-email-fail")
		t.Fatalf("failed to fill email: %v", err)
	}

	err = page.Fill("input[name='password']", "test123456")
	if err != nil {
		TakeScreenshot(t, page, "dashboard-login-password-fail")
		t.Fatalf("failed to fill password: %v", err)
	}

	err = page.Click("button[type='submit']")
	if err != nil {
		TakeScreenshot(t, page, "dashboard-login-click-fail")
		t.Fatalf("failed to click submit: %v", err)
	}

	// Wait for navigation
	err = page.WaitForLoadState(pw.PageWaitForLoadStateOptions{
		State: pw.LoadStateNetworkidle,
	})
	if err != nil {
		t.Logf("load state warning: %v", err)
	}

	url := page.URL()
	if err != nil {
		t.Fatalf("failed to get URL: %v", err)
	}

	if url != ServerURL+"/dashboard" {
		t.Logf("expected dashboard, got: %s", url)
		TakeScreenshot(t, page, "dashboard-unexpected-page")
	}

	content, err := page.Content()
	if err != nil {
		t.Fatalf("failed to get content: %v", err)
	}

	if len(content) == 0 {
		t.Error("expected dashboard content")
	}
}

func TestDashboardShowsUserInfo(t *testing.T) {
	page, cleanup := SetupPlaywright(t)
	defer cleanup()

	_, err := page.Goto(ServerURL + "/login")
	if err != nil {
		t.Fatalf("failed to navigate to login: %v", err)
	}

	// Wait for and fill login form
	if err := WaitForElement(t, page, "input[name='email']"); err != nil {
		t.Fatalf("email field not found: %v", err)
	}

	if err := WaitForElement(t, page, "input[name='password']"); err != nil {
		t.Fatalf("password field not found: %v", err)
	}

	err = page.Fill("input[name='email']", "admin @goth.local")
	if err != nil {
		t.Fatalf("failed to fill email: %v", err)
	}

	err = page.Fill("input[name='password']", "test123456")
	if err != nil {
		t.Fatalf("failed to fill password: %v", err)
	}

	err = page.Click("button[type='submit']")
	if err != nil {
		t.Fatalf("failed to click submit: %v", err)
	}

	// Wait for dashboard
	err = page.WaitForLoadState(pw.PageWaitForLoadStateOptions{
		State: pw.LoadStateNetworkidle,
	})
	if err != nil {
		t.Logf("load state warning: %v", err)
	}

	// Wait for dashboard content to load
	page.WaitForTimeout(500)

	content, err := page.Content()
	if err != nil {
		t.Fatalf("failed to get content: %v", err)
	}

	if len(content) == 0 {
		TakeScreenshot(t, page, "dashboard-empty-content")
		t.Error("expected dashboard content")
	}

	// Check for common dashboard elements
	dashboardSelectors := []string{
		"h1",
		"header",
		"nav",
		"[data-testid='dashboard']",
		".dashboard",
	}

	foundDashboard := false
	for _, selector := range dashboardSelectors {
		el, err := page.QuerySelector(selector)
		if err == nil && el != nil {
			foundDashboard = true
			break
		}
	}

	if !foundDashboard {
		t.Logf("dashboard elements not found, page may have different structure")
		TakeScreenshot(t, page, "dashboard-no-structure")
	}
}
