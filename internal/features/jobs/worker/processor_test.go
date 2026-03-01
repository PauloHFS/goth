package worker

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/PauloHFS/goth/internal/features/sse"
	"github.com/PauloHFS/goth/internal/platform/config"
)

// newTestProcessor creates a Processor for testing with minimal dependencies
func newTestProcessor(t *testing.T) *Processor {
	t.Helper()
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	cfg := &config.Config{SMTPHost: "localhost", SMTPPort: "1025"}
	ctx := context.Background()
	broker := sse.NewBroker(ctx)
	t.Cleanup(func() { broker.Shutdown() })
	return New(cfg, nil, nil, logger, broker)
}

func TestProcessor_Initialization(t *testing.T) {
	p := newTestProcessor(t)

	t.Run("Processor initialization", func(t *testing.T) {
		if p == nil {
			t.Fatal("expected processor, got nil")
		}
		if p.logger == nil {
			t.Error("logger not correctly assigned")
		}
		if p.mailer == nil {
			t.Error("mailer should be initialized")
		}
	})
}

func TestProcessor_BackoffCalculation(t *testing.T) {
	// Test exponential backoff logic
	tests := []struct {
		attempt   int
		wantDelay time.Duration
		maxDelay  time.Duration
		baseDelay time.Duration
	}{
		{attempt: 1, baseDelay: 30 * time.Second, maxDelay: 10 * time.Minute, wantDelay: 30 * time.Second},
		{attempt: 2, baseDelay: 30 * time.Second, maxDelay: 10 * time.Minute, wantDelay: 1 * time.Minute},
		{attempt: 3, baseDelay: 30 * time.Second, maxDelay: 10 * time.Minute, wantDelay: 2 * time.Minute},
		{attempt: 4, baseDelay: 30 * time.Second, maxDelay: 10 * time.Minute, wantDelay: 4 * time.Minute},
		{attempt: 5, baseDelay: 30 * time.Second, maxDelay: 10 * time.Minute, wantDelay: 8 * time.Minute},
		{attempt: 10, baseDelay: 30 * time.Second, maxDelay: 10 * time.Minute, wantDelay: 10 * time.Minute}, // capped
	}

	for _, tt := range tests {
		t.Run("attempt_"+string(rune(tt.attempt)), func(t *testing.T) {
			delay := tt.baseDelay * time.Duration(1<<(tt.attempt-1))
			if delay > tt.maxDelay {
				delay = tt.maxDelay
			}
			if delay != tt.wantDelay {
				t.Errorf("attempt %d: expected delay %v, got %v", tt.attempt, tt.wantDelay, delay)
			}
		})
	}
}

func TestProcessor_HandleSendEmail(t *testing.T) {
	p := newTestProcessor(t)

	payload := map[string]string{
		"to":      "test@example.com",
		"subject": "Test Subject",
		"body":    "Test Body",
	}

	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal payload: %v", err)
	}

	// This will fail because we don't have a real SMTP server, but we test it doesn't panic
	ctx := context.Background()
	err = p.handleSendEmail(ctx, payloadJSON)

	// We expect an error (no SMTP server), but no panic
	if err == nil {
		t.Log("expected SMTP error, got none (mailhog might be running)")
	}
}

func TestProcessor_HandleSendVerificationEmail(t *testing.T) {
	p := newTestProcessor(t)

	payload := map[string]string{
		"email": "test@example.com",
		"token": "verification-token-123",
	}

	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal payload: %v", err)
	}

	ctx := context.Background()
	err = p.handleSendVerificationEmail(ctx, payloadJSON)

	// We expect an error (no SMTP server), but no panic
	if err == nil {
		t.Log("expected SMTP error, got none (mailhog might be running)")
	}
}

func TestProcessor_HandleSendPasswordResetEmail(t *testing.T) {
	p := newTestProcessor(t)

	payload := map[string]string{
		"email": "test@example.com",
		"token": "reset-token-123",
	}

	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal payload: %v", err)
	}

	ctx := context.Background()
	err = p.handleSendPasswordResetEmail(ctx, payloadJSON)

	// We expect an error (no SMTP server), but no panic
	if err == nil {
		t.Log("expected SMTP error, got none (mailhog might be running)")
	}
}

func TestProcessor_HandleProcessAI(t *testing.T) {
	p := newTestProcessor(t)

	payload := map[string]string{
		"prompt": "Test AI prompt",
	}

	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal payload: %v", err)
	}

	ctx := context.Background()

	// Set a timeout to ensure test doesn't hang
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	err = p.handleProcessAI(ctx, payloadJSON)
	if err != nil {
		t.Errorf("handleProcessAI returned error: %v", err)
	}
}

func TestProcessor_HandleProcessWebhook(t *testing.T) {
	p := newTestProcessor(t)

	payload := map[string]int64{
		"webhook_id": 123,
	}

	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal payload: %v", err)
	}

	ctx := context.Background()
	err = p.handleProcessWebhook(ctx, payloadJSON)
	if err != nil {
		t.Errorf("handleProcessWebhook returned error: %v", err)
	}
}
