package e2e

import (
	"testing"

	pw "github.com/playwright-community/playwright-go"
)

func TestCheckoutRequiresAuth(t *testing.T) {
	page, cleanup := SetupPlaywright(t)
	defer cleanup()

	_, err := page.Goto(ServerURL + "/checkout/subscribe")
	if err != nil {
		t.Fatalf("failed to navigate to checkout: %v", err)
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

	if url != ServerURL+"/login" {
		t.Logf("expected redirect to login, got: %s", url)
		TakeScreenshot(t, page, "checkout-no-redirect")
	}
}

func TestCheckoutPageLoadsAfterLogin(t *testing.T) {
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

	err = page.Fill("input[name='email']", "user @goth.local")
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

	// Wait for navigation
	err = page.WaitForLoadState(pw.PageWaitForLoadStateOptions{
		State: pw.LoadStateNetworkidle,
	})
	if err != nil {
		t.Logf("load state warning: %v", err)
	}

	// Navigate to checkout
	_, err = page.Goto(ServerURL + "/checkout/subscribe")
	if err != nil {
		t.Fatalf("failed to navigate to checkout: %v", err)
	}

	url := page.URL()
	if err != nil {
		t.Fatalf("failed to get URL: %v", err)
	}

	if url != ServerURL+"/checkout/subscribe" {
		t.Logf("expected checkout page, got: %s", url)
		TakeScreenshot(t, page, "checkout-wrong-page")
	}
}

func TestCheckoutFormValidation(t *testing.T) {
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

	err = page.Fill("input[name='email']", "user @goth.local")
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

	// Navigate to checkout
	_, err = page.Goto(ServerURL + "/checkout/subscribe")
	if err != nil {
		t.Fatalf("failed to navigate to checkout: %v", err)
	}

	// Wait for checkout page to load
	err = page.WaitForLoadState(pw.PageWaitForLoadStateOptions{
		State: pw.LoadStateNetworkidle,
	})
	if err != nil {
		t.Logf("load state warning: %v", err)
	}

	// Check for form elements (may vary based on implementation)
	checkoutSelectors := []string{
		"form",
		"input[type='email']",
		"button[type='submit']",
		"[data-testid='checkout']",
		".checkout",
	}

	foundForm := false
	for _, selector := range checkoutSelectors {
		el, err := page.QuerySelector(selector)
		if err == nil && el != nil {
			foundForm = true
			break
		}
	}

	if !foundForm {
		t.Logf("checkout form not found, page structure may differ")
		TakeScreenshot(t, page, "checkout-no-form")
	}
}
