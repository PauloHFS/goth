package e2e

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	pw "github.com/playwright-community/playwright-go"
)

const (
	defaultServerURL     = "http://localhost:8080"
	testTimeout          = 30 * time.Second
	maxRetries           = 2 // Retry flaky tests once
	serverStartupTimeout = 10 * time.Second
)

// ServerURL is the base URL for the test server.
// It can be overridden via the SERVER_URL environment variable.
var ServerURL string

var (
	pwInstance   *pw.Playwright
	browser      pw.Browser
	pwOnce       bool
	serverCmd    *exec.Cmd
	serverCtx    context.Context
	serverCancel context.CancelFunc
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

// startServerIfNeeded starts the Go server if it's not already running
func startServerIfNeeded(t *testing.T) {
	// Check if server is already running
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(ServerURL + "/health")
	if err == nil && resp.StatusCode == 200 {
		resp.Body.Close()
		t.Logf("Server already running at %s", ServerURL)
		return
	}

	// Server not running, start it
	t.Logf("Starting test server at %s...", ServerURL)

	// Create context for server lifecycle
	serverCtx, serverCancel = context.WithCancel(context.Background())

	// Build server binary first
	buildCmd := exec.CommandContext(serverCtx, "go", "build", "-o", "bin/goth-test", "./internal/cmd/api")
	buildCmd.Env = append(os.Environ(), "GOENV=test")
	if err := buildCmd.Run(); err != nil {
		t.Fatalf("failed to build test server: %v", err)
	}

	// Start server
	serverCmd = exec.CommandContext(serverCtx, "./bin/goth-test", "server")
	serverCmd.Env = append(os.Environ(),
		"GOENV=test",
		"HTTP_ADDR=:8080",
		"DB_PATH=./storage/test.db",
	)
	serverCmd.Stdout = nil // Suppress output during tests
	serverCmd.Stderr = nil

	if err := serverCmd.Start(); err != nil {
		t.Fatalf("failed to start server: %v", err)
	}

	// Wait for server to be ready
	if err := waitForServer(t, serverStartupTimeout); err != nil {
		serverCancel()
		t.Fatalf("server failed to start: %v", err)
	}

	t.Logf("Test server started successfully")
}

// waitForServer waits for the server to be responsive
func waitForServer(t *testing.T, timeout time.Duration) error {
	client := &http.Client{Timeout: 2 * time.Second}
	start := time.Now()

	for time.Since(start) < timeout {
		resp, err := client.Get(ServerURL + "/health")
		if err == nil && resp.StatusCode == 200 {
			resp.Body.Close()
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}

	return fmt.Errorf("server did not start within %v", timeout)
}

// stopServer stops the test server if it was started by tests
func stopServer(t *testing.T) {
	if serverCancel != nil {
		t.Logf("Stopping test server...")
		serverCancel()
		if serverCmd != nil && serverCmd.Process != nil {
			// Force kill after graceful shutdown
			serverCmd.Process.Kill()
		}
		// Clean up test database
		os.Remove("./storage/test.db")
		serverCmd = nil
		serverCtx = nil
		serverCancel = nil
	}
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

// TestMain is the entry point for all E2E tests.
// It handles server lifecycle (start/stop) and Playwright teardown.
func TestMain(m *testing.M) {
	// Start server before running tests
	// Use a dummy testing.T for logging
	dummyT := &testing.T{}
	startServerIfNeeded(dummyT)

	// Run all tests
	exitCode := m.Run()

	// Cleanup
	stopServer(dummyT)
	TeardownPlaywright()

	// Exit with appropriate code
	os.Exit(exitCode)
}
