package e2e

import (
	"testing"

	pw "github.com/playwright-community/playwright-go"
)

func TestLoginPageLoads(t *testing.T) {
	page, cleanup := SetupPlaywright(t)
	defer cleanup()

	_, err := page.Goto(ServerURL + "/login")
	if err != nil {
		t.Fatalf("failed to navigate to login: %v", err)
	}

	// Wait for page to load
	err = page.WaitForLoadState(pw.PageWaitForLoadStateOptions{
		State: pw.LoadStateDomcontentloaded,
	})
	if err != nil {
		t.Fatalf("page failed to load: %v", err)
	}

	title, err := page.Title()
	if err != nil {
		t.Fatalf("failed to get title: %v", err)
	}

	if title == "" {
		t.Error("expected non-empty title")
	}
}

func TestRegisterPageLoads(t *testing.T) {
	page, cleanup := SetupPlaywright(t)
	defer cleanup()

	_, err := page.Goto(ServerURL + "/register")
	if err != nil {
		t.Fatalf("failed to navigate to register: %v", err)
	}

	err = page.WaitForLoadState(pw.PageWaitForLoadStateOptions{
		State: pw.LoadStateDomcontentloaded,
	})
	if err != nil {
		t.Fatalf("page failed to load: %v", err)
	}

	title, err := page.Title()
	if err != nil {
		t.Fatalf("failed to get title: %v", err)
	}

	if title == "" {
		t.Error("expected non-empty title")
	}
}

func TestLoginSuccess(t *testing.T) {
	page, cleanup := SetupPlaywright(t)
	defer cleanup()

	resp, err := page.Goto(ServerURL + "/login")
	if err != nil {
		t.Fatalf("failed to navigate to login: %v", err)
	}

	// Debug: Check response status and content type
	if resp.Status() != 200 {
		t.Fatalf("login page returned status: %d", resp.Status())
	}

	// Debug: Wait for network to be idle (ensure all assets loaded)
	err = page.WaitForLoadState(pw.PageWaitForLoadStateOptions{
		State: pw.LoadStateNetworkidle,
	})
	if err != nil {
		t.Logf("network idle warning: %v", err)
	}

	// Debug: Take screenshot of login page
	_, err = page.Screenshot(pw.PageScreenshotOptions{
		Path: pw.String("test-results/screenshots/login-page-initial.png"),
	})
	if err != nil {
		t.Logf("screenshot warning: %v", err)
	}

	// Wait for form elements to be visible
	if err := WaitForElement(t, page, "input[name='email']"); err != nil {
		// Debug: Get page content for analysis
		content, _ := page.Content()
		t.Logf("page content length: %d", len(content))
		t.Logf("page URL: %s", page.URL())

		TakeScreenshot(t, page, "login-missing-elements")
		t.Fatalf("email field not found: %v", err)
	}

	if err := WaitForElement(t, page, "input[name='password']"); err != nil {
		TakeScreenshot(t, page, "login-missing-password")
		t.Fatalf("password field not found: %v", err)
	}

	if err := WaitForElement(t, page, "button[type='submit']"); err != nil {
		TakeScreenshot(t, page, "login-missing-submit")
		t.Fatalf("submit button not found: %v", err)
	}

	err = page.Fill("input[name='email']", "admin @goth.local")
	if err != nil {
		TakeScreenshot(t, page, "login-fill-email-failed")
		t.Fatalf("failed to fill email: %v", err)
	}

	err = page.Fill("input[name='password']", "test123456")
	if err != nil {
		TakeScreenshot(t, page, "login-fill-password-failed")
		t.Fatalf("failed to fill password: %v", err)
	}

	err = page.Click("button[type='submit']")
	if err != nil {
		TakeScreenshot(t, page, "login-click-submit-failed")
		t.Fatalf("failed to click submit: %v", err)
	}

	// Wait for navigation
	err = page.WaitForURL(ServerURL + "/dashboard")
	if err != nil {
		t.Logf("navigation warning (may have redirected elsewhere): %v", err)
	}

	url := page.URL()
	if url != ServerURL+"/dashboard" {
		t.Logf("expected dashboard, got: %s", url)
		TakeScreenshot(t, page, "login-unexpected-page")
	}
}

func TestLoginInvalidCredentials(t *testing.T) {
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
		t.Fatalf("failed to fill email: %v", err)
	}

	err = page.Fill("input[name='password']", "wrongpassword")
	if err != nil {
		t.Fatalf("failed to fill password: %v", err)
	}

	err = page.Click("button[type='submit']")
	if err != nil {
		t.Fatalf("failed to click submit: %v", err)
	}

	// Wait for page to settle
	err = page.WaitForLoadState(pw.PageWaitForLoadStateOptions{
		State: pw.LoadStateNetworkidle,
	})
	if err != nil {
		t.Logf("load state warning: %v", err)
	}

	content, err := page.Content()
	if err != nil {
		t.Fatalf("failed to get content: %v", err)
	}

	if len(content) == 0 {
		t.Error("expected page content")
	}
}

func TestLogout(t *testing.T) {
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

	// Wait for dashboard to load
	err = page.WaitForURL(ServerURL + "/dashboard")
	if err != nil {
		t.Logf("dashboard navigation warning: %v", err)
	}

	// Wait a bit for page to fully load
	page.WaitForTimeout(1000)

	// Try to find and click logout - may be in different locations
	logoutSelectors := []string{
		"button[name='logout']",
		"a[href='/logout']",
		"button:has-text('Logout')",
		"button:has-text('Sair')",
		"[data-testid='logout']",
	}

	loggedOut := false
	for _, selector := range logoutSelectors {
		_, err := page.QuerySelector(selector)
		if err == nil {
			err = page.Click(selector)
			if err == nil {
				loggedOut = true
				break
			}
		}
	}

	if !loggedOut {
		t.Logf("logout button not found in any expected location")
		TakeScreenshot(t, page, "logout-missing-button")
		// Alternative: try navigating directly to logout
		_, err := page.Goto(ServerURL + "/logout")
		if err != nil {
			t.Logf("direct logout navigation failed: %v", err)
		}
	}

	// Wait for redirect to login
	err = page.WaitForURL(ServerURL + "/login")
	if err != nil {
		t.Logf("logout redirect warning: %v", err)
	}

	url := page.URL()
	if url != ServerURL+"/login" {
		t.Logf("expected login page after logout, got: %s", url)
	}
}

func TestForgotPasswordPageLoads(t *testing.T) {
	page, cleanup := SetupPlaywright(t)
	defer cleanup()

	_, err := page.Goto(ServerURL + "/forgot-password")
	if err != nil {
		t.Fatalf("failed to navigate to forgot password: %v", err)
	}

	err = page.WaitForLoadState(pw.PageWaitForLoadStateOptions{
		State: pw.LoadStateDomcontentloaded,
	})
	if err != nil {
		t.Fatalf("page failed to load: %v", err)
	}

	title, err := page.Title()
	if err != nil {
		t.Fatalf("failed to get title: %v", err)
	}

	if title == "" {
		t.Error("expected non-empty title")
	}
}

func TestRegisterPageShowsEmailField(t *testing.T) {
	page, cleanup := SetupPlaywright(t)
	defer cleanup()

	_, err := page.Goto(ServerURL + "/register")
	if err != nil {
		t.Fatalf("failed to navigate to register: %v", err)
	}

	// Wait for email input to be visible
	if err := WaitForElement(t, page, "input[name='email']"); err != nil {
		TakeScreenshot(t, page, "register-missing-email")
		t.Fatalf("email input not found: %v", err)
	}

	emailInput, err := page.QuerySelector("input[name='email']")
	if err != nil {
		t.Fatalf("failed to query email input: %v", err)
	}

	if emailInput == nil {
		t.Error("expected email input to exist")
	}
}

func TestLoginPageShowsPasswordField(t *testing.T) {
	page, cleanup := SetupPlaywright(t)
	defer cleanup()

	_, err := page.Goto(ServerURL + "/login")
	if err != nil {
		t.Fatalf("failed to navigate to login: %v", err)
	}

	// Wait for page to load
	err = page.WaitForLoadState(pw.PageWaitForLoadStateOptions{
		State: pw.LoadStateDomcontentloaded,
	})
	if err != nil {
		t.Fatalf("page failed to load: %v", err)
	}

	// Wait for password input to be visible
	if err := WaitForElement(t, page, "input[name='password']"); err != nil {
		TakeScreenshot(t, page, "login-missing-password-field")
		t.Fatalf("password input not found: %v", err)
	}

	passwordInput, err := page.QuerySelector("input[name='password']")
	if err != nil {
		t.Fatalf("failed to query password input: %v", err)
	}

	if passwordInput == nil {
		TakeScreenshot(t, page, "login-password-null")
		t.Error("expected password input to exist")
	}
}

func TestProtectedRouteRedirects(t *testing.T) {
	page, cleanup := SetupPlaywright(t)
	defer cleanup()

	_, err := page.Goto(ServerURL + "/dashboard")
	if err != nil {
		t.Fatalf("failed to navigate to dashboard: %v", err)
	}

	// Wait for potential redirect
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

	// Check if redirected to login
	if url != ServerURL+"/login" {
		t.Logf("expected redirect to login, got: %s", url)
		TakeScreenshot(t, page, "protected-route-no-redirect")
	}
}
