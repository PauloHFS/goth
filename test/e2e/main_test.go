package e2e

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	pw "github.com/playwright-community/playwright-go"
)

const (
	defaultServerURL = "http://localhost:8080"
	testTimeout      = 30 * time.Second
	maxRetries       = 2 // Retry flaky tests once
)

// ServerURL is the base URL for the test server.
// It can be overridden via the SERVER_URL environment variable.
var ServerURL string

var (
	pwInstance *pw.Playwright
	browser    pw.Browser
	pwOnce     bool
)

func init() {
	ServerURL = os.Getenv("SERVER_URL")
	if ServerURL == "" {
		ServerURL = defaultServerURL
	}

	// Ensure test-results directories exist
	os.MkdirAll("test-results/screenshots", 0755)
	os.MkdirAll("test-results/videos", 0755)
}

// SetupPlaywright initializes a new Playwright browser instance and page.
// It returns the page and a cleanup function that should be deferred.
func SetupPlaywright(t *testing.T) (pw.Page, func()) {
	var err error
	if !pwOnce {
		// Install playwright browsers on first run
		if err = pw.Install(); err != nil {
			t.Fatalf("failed to install playwright: %v", err)
		}
		pwOnce = true
	}

	if pwInstance == nil {
		pwInstance, err = pw.Run()
		if err != nil {
			t.Fatalf("failed to create playwright: %v", err)
		}
	}

	if browser == nil {
		// Configure browser for testing
		browser, err = pwInstance.Chromium.Launch(pw.BrowserTypeLaunchOptions{
			Headless: pw.Bool(true),
			Args:     []string{"--no-sandbox", "--disable-setuid-sandbox"},
		})
		if err != nil {
			t.Fatalf("failed to launch browser: %v", err)
		}
	}

	// Create isolated browser context for each test
	// This ensures cookies/sessions don't leak between tests
	context, err := browser.NewContext(pw.BrowserNewContextOptions{
		// Ignore HTTPS errors in test environment
		IgnoreHttpsErrors: pw.Bool(true),
	})
	if err != nil {
		t.Fatalf("failed to create browser context: %v", err)
	}

	page, err := context.NewPage()
	if err != nil {
		t.Fatalf("failed to create page: %v", err)
	}

	// Set default timeout
	page.SetDefaultTimeout(float64(testTimeout.Milliseconds()))

	cleanup := func() {
		if page != nil {
			page.Close()
		}
		if context != nil {
			context.Close()
		}
	}

	return page, cleanup
}

// WaitForElement waits for an element to be visible before interacting
func WaitForElement(t *testing.T, page pw.Page, selector string) error {
	timeout := float64(testTimeout.Milliseconds())
	_, err := page.WaitForSelector(selector, pw.PageWaitForSelectorOptions{
		State:   pw.WaitForSelectorStateVisible,
		Timeout: &timeout,
	})
	if err != nil {
		return fmt.Errorf("element %q not found: %w", selector, err)
	}
	return nil
}

// TakeScreenshot captures a screenshot for debugging
func TakeScreenshot(t *testing.T, page pw.Page, name string) {
	screenshot, err := page.Screenshot()
	if err != nil {
		t.Logf("failed to take screenshot: %v", err)
		return
	}

	// Create screenshots directory if it doesn't exist
	screenshotsDir := "test-results/screenshots"
	if err := os.MkdirAll(screenshotsDir, 0755); err != nil {
		t.Logf("failed to create screenshots directory: %v", err)
		return
	}

	filename := filepath.Join(screenshotsDir, fmt.Sprintf("%s-%d.png", name, time.Now().UnixNano()))
	if err := os.WriteFile(filename, screenshot, 0644); err != nil {
		t.Logf("failed to save screenshot: %v", err)
		return
	}

	t.Logf("screenshot saved: %s", filename)
}

// SaveVideo saves the current video recording
func SaveVideo(t *testing.T, page pw.Page, testName string) {
	video := page.Video()
	if video == nil {
		return
	}

	// Create videos directory
	videosDir := "test-results/videos"
	if err := os.MkdirAll(videosDir, 0755); err != nil {
		t.Logf("failed to create videos directory: %v", err)
		return
	}

	// Save video with test name
	videoPath := filepath.Join(videosDir, fmt.Sprintf("%s-%d.webm", testName, time.Now().UnixNano()))
	err := video.SaveAs(videoPath)
	if err != nil {
		t.Logf("failed to save video: %v", err)
		return
	}

	t.Logf("video saved: %s", videoPath)
}

// RunWithRetry executes a test function with retries for flaky tests
func RunWithRetry(t *testing.T, maxRetries int, testFunc func(t *testing.T) error) {
	t.Helper()

	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		// Create fresh page for each attempt
		page, cleanup := SetupPlaywright(t)

		// Wrap test function to capture errors
		err := testFunc(t)

		if err == nil {
			// Test passed, cleanup and return
			cleanup()
			return
		}

		lastErr = err

		// Save video and screenshot on failure
		if attempt < maxRetries {
			t.Logf("test failed (attempt %d/%d), retrying... Error: %v", attempt+1, maxRetries+1, err)
			SaveVideo(t, page, t.Name())
			TakeScreenshot(t, page, fmt.Sprintf("retry-attempt-%d", attempt+1))
			cleanup()
			// Small delay before retry
			time.Sleep(2 * time.Second)
		} else {
			// Final attempt failed
			t.Logf("test failed after %d attempts. Saving artifacts...", maxRetries+1)
			SaveVideo(t, page, t.Name())
			TakeScreenshot(t, page, "final-failure")
			cleanup()
		}
	}

	// All retries exhausted, fail the test
	if lastErr != nil {
		t.Fatalf("test failed after %d retries: %v", maxRetries, lastErr)
	}
}

// TeardownPlaywright cleans up all Playwright resources.
// Call this in TestMain after all tests complete.
func TeardownPlaywright() {
	if browser != nil {
		browser.Close()
		browser = nil
	}
	if pwInstance != nil {
		pwInstance.Stop()
		pwInstance = nil
	}
	pwOnce = false
}
