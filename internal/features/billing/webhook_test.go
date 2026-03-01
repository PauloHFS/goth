package billing

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/PauloHFS/goth/internal/db"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
}

func TestWebhookHandler_ServeHTTP_MethodNotAllowed(t *testing.T) {
	handler := NewWebhookHandler(&db.Queries{}, "validtoken", "", testLogger())

	req := httptest.NewRequest(http.MethodGet, "/webhook", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Returns 400 Bad Request for method not allowed (validation error)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestWebhookHandler_ServeHTTP_Unauthorized(t *testing.T) {
	handler := NewWebhookHandler(&db.Queries{}, "validtoken", "", testLogger())

	req := httptest.NewRequest(http.MethodPost, "/webhook", nil)
	req.Header.Set("X-Asaas-Token", "invalidtoken")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

func TestWebhookHandler_ServeHTTP_Authorized(t *testing.T) {
	handler := NewWebhookHandler(&db.Queries{}, "validtoken", "", testLogger())

	payload := AsaasWebhookPayload{
		Event: "UNKNOWN_EVENT",
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewBuffer(body))
	req.Header.Set("X-Asaas-Token", "validtoken")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}
}

func TestWebhookHandler_HandlePaymentReceived(t *testing.T) {
	handler := &WebhookHandler{
		queries:      nil,
		webhookToken: "token",
		logger:       testLogger(),
	}

	payload := AsaasWebhookPayload{
		Event: "PAYMENT_RECEIVED",
		Payment: &AsaasPaymentResponse{
			ID:     "pay_123",
			Status: "RECEIVED",
		},
	}

	handler.handlePaymentReceived(context.Background(), payload)
}

func TestWebhookHandler_HandlePaymentConfirmed(t *testing.T) {
	handler := &WebhookHandler{
		queries:      nil,
		webhookToken: "token",
		logger:       testLogger(),
	}

	payload := AsaasWebhookPayload{
		Event: "PAYMENT_CONFIRMED",
		Payment: &AsaasPaymentResponse{
			ID:     "pay_123",
			Status: "CONFIRMED",
		},
	}

	handler.handlePaymentConfirmed(context.Background(), payload)
}

func TestWebhookHandler_HandlePaymentOverdue(t *testing.T) {
	handler := &WebhookHandler{
		queries:      nil,
		webhookToken: "token",
		logger:       testLogger(),
	}

	payload := AsaasWebhookPayload{
		Event: "PAYMENT_OVERDUE",
		Payment: &AsaasPaymentResponse{
			ID:     "pay_123",
			Status: "OVERDUE",
		},
	}

	handler.handlePaymentOverdue(context.Background(), payload)
}

func TestWebhookHandler_HandlePaymentRefunded(t *testing.T) {
	handler := &WebhookHandler{
		queries:      nil,
		webhookToken: "token",
		logger:       testLogger(),
	}

	payload := AsaasWebhookPayload{
		Event: "PAYMENT_REFUNDED",
		Payment: &AsaasPaymentResponse{
			ID:     "pay_123",
			Status: "REFUNDED",
		},
	}

	handler.handlePaymentRefunded(context.Background(), payload)
}

func TestWebhookHandler_HandlePaymentCanceled(t *testing.T) {
	handler := &WebhookHandler{
		queries:      nil,
		webhookToken: "token",
		logger:       testLogger(),
	}

	payload := AsaasWebhookPayload{
		Event: "PAYMENT_CANCELED",
		Payment: &AsaasPaymentResponse{
			ID:     "pay_123",
			Status: "CANCELED",
		},
	}

	handler.handlePaymentCanceled(context.Background(), payload)
}

func TestWebhookHandler_HandleSubscriptionUpdated(t *testing.T) {
	handler := &WebhookHandler{
		queries:      nil,
		webhookToken: "token",
		logger:       testLogger(),
	}

	payload := AsaasWebhookPayload{
		Event: "SUBSCRIPTION_UPDATED",
		Subscription: &AsaasSubscriptionResponse{
			ID:          "sub_123",
			Status:      "ACTIVE",
			NextDueDate: "2024-12-31",
		},
	}

	handler.handleSubscriptionUpdated(context.Background(), payload)
}

func TestWebhookHandler_HandleSubscriptionCanceled(t *testing.T) {
	handler := &WebhookHandler{
		queries:      nil,
		webhookToken: "token",
		logger:       testLogger(),
	}

	payload := AsaasWebhookPayload{
		Event: "SUBSCRIPTION_CANCELED",
		Subscription: &AsaasSubscriptionResponse{
			ID:     "sub_123",
			Status: "CANCELED",
		},
	}

	handler.handleSubscriptionCanceled(context.Background(), payload)
}

func TestWebhookHandler_HandleSubscriptionDeleted(t *testing.T) {
	handler := &WebhookHandler{
		queries:      nil,
		webhookToken: "token",
		logger:       testLogger(),
	}

	payload := AsaasWebhookPayload{
		Event: "SUBSCRIPTION_DELETED",
		Subscription: &AsaasSubscriptionResponse{
			ID:     "sub_123",
			Status: "DELETED",
		},
	}

	handler.handleSubscriptionDeleted(context.Background(), payload)
}

func TestWebhookHandler_HandleNilPayment(t *testing.T) {
	handler := &WebhookHandler{
		queries:      &db.Queries{},
		webhookToken: "token",
		logger:       testLogger(),
	}

	handler.handlePaymentReceived(context.Background(), AsaasWebhookPayload{
		Event:   "PAYMENT_RECEIVED",
		Payment: nil,
	})

	handler.handlePaymentConfirmed(context.Background(), AsaasWebhookPayload{
		Event:   "PAYMENT_CONFIRMED",
		Payment: nil,
	})

	handler.handlePaymentOverdue(context.Background(), AsaasWebhookPayload{
		Event:   "PAYMENT_OVERDUE",
		Payment: nil,
	})
}

func TestWebhookHandler_HandleNilSubscription(t *testing.T) {
	handler := &WebhookHandler{
		queries:      &db.Queries{},
		webhookToken: "token",
		logger:       testLogger(),
	}

	handler.handleSubscriptionUpdated(context.Background(), AsaasWebhookPayload{
		Event:        "SUBSCRIPTION_UPDATED",
		Subscription: nil,
	})

	handler.handleSubscriptionCanceled(context.Background(), AsaasWebhookPayload{
		Event:        "SUBSCRIPTION_CANCELED",
		Subscription: nil,
	})

	handler.handleSubscriptionDeleted(context.Background(), AsaasWebhookPayload{
		Event:        "SUBSCRIPTION_DELETED",
		Subscription: nil,
	})
}

func TestAsaasWebhookPayload_JSON(t *testing.T) {
	payload := AsaasWebhookPayload{
		Event: "PAYMENT_RECEIVED",
		Payment: &AsaasPaymentResponse{
			ID:          "pay_123",
			Status:      "RECEIVED",
			PaymentDate: "2024-01-01",
		},
	}

	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal payload: %v", err)
	}

	var parsed AsaasWebhookPayload
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal payload: %v", err)
	}

	if parsed.Event != payload.Event {
		t.Errorf("expected event %s, got %s", payload.Event, parsed.Event)
	}
	if parsed.Payment == nil || parsed.Payment.ID != payload.Payment.ID {
		t.Errorf("expected payment ID %s, got %s", payload.Payment.ID, parsed.Payment.ID)
	}
}

func TestSignatureError_Error(t *testing.T) {
	err := &signatureError{msg: "test error"}
	if err.Error() != "test error" {
		t.Errorf("expected 'test error', got %s", err.Error())
	}
}

func TestAsaasPaymentResponse_JSON(t *testing.T) {
	resp := AsaasPaymentResponse{
		ID:          "pay_123",
		Status:      "RECEIVED",
		PaymentDate: "2024-01-01",
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var parsed AsaasPaymentResponse
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if parsed.ID != resp.ID {
		t.Errorf("expected ID %s, got %s", resp.ID, parsed.ID)
	}
}

func TestAsaasSubscriptionResponse_JSON(t *testing.T) {
	resp := AsaasSubscriptionResponse{
		ID:          "sub_123",
		Status:      "ACTIVE",
		NextDueDate: "2024-12-31",
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var parsed AsaasSubscriptionResponse
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if parsed.ID != resp.ID {
		t.Errorf("expected ID %s, got %s", resp.ID, parsed.ID)
	}
}
