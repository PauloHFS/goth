package billing

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/PauloHFS/goth/internal/db"
	httpErr "github.com/PauloHFS/goth/internal/platform/http"
)

type WebhookHandler struct {
	queries      *db.Queries
	webhookToken string
	hmacSecret   string
	logger       *slog.Logger
}

type AsaasWebhookPayload struct {
	Event        string                     `json:"event"`
	Payment      *AsaasPaymentResponse      `json:"payment,omitempty"`
	Subscription *AsaasSubscriptionResponse `json:"subscription,omitempty"`
}

type AsaasPaymentResponse struct {
	ID          string `json:"id"`
	Status      string `json:"status"`
	PaymentDate string `json:"paymentDate"`
}

type AsaasSubscriptionResponse struct {
	ID          string `json:"id"`
	Status      string `json:"status"`
	NextDueDate string `json:"nextDueDate"`
}

// ProcessWebhookEvent processa um evento de webhook de forma idempotente
// Este método é chamado pelo worker assíncrono
func (h *WebhookHandler) ProcessWebhookEvent(ctx context.Context, webhookID int64, source string, eventID string) error {
	// Verifica se o evento já foi processado (idempotência)
	processed, err := h.queries.IsWebhookEventProcessed(ctx, db.IsWebhookEventProcessedParams{
		EventSource: source,
		EventID:     eventID,
	})
	if err == nil && processed == 1 {
		h.logger.DebugContext(ctx, "webhook event already processed, skipping",
			slog.Int64("webhook_id", webhookID),
			slog.String("source", source),
			slog.String("event_id", eventID),
		)
		return nil
	}

	// Busca o webhook do banco
	webhook, err := h.queries.GetWebhookByID(ctx, webhookID)
	if err != nil {
		return err
	}

	var payload AsaasWebhookPayload
	if err := json.Unmarshal(webhook.Payload, &payload); err != nil {
		return err
	}

	h.logger.InfoContext(ctx, "processing asaas webhook",
		slog.String("event", payload.Event),
		slog.String("event_id", eventID),
	)

	// Processa o evento baseado no tipo
	switch payload.Event {
	case "PAYMENT_RECEIVED":
		h.handlePaymentReceived(ctx, payload)
	case "PAYMENT_CONFIRMED":
		h.handlePaymentConfirmed(ctx, payload)
	case "PAYMENT_OVERDUE":
		h.handlePaymentOverdue(ctx, payload)
	case "PAYMENT_REFUNDED":
		h.handlePaymentRefunded(ctx, payload)
	case "PAYMENT_CANCELED":
		h.handlePaymentCanceled(ctx, payload)
	case "SUBSCRIPTION_UPDATED":
		h.handleSubscriptionUpdated(ctx, payload)
	case "SUBSCRIPTION_CANCELED":
		h.handleSubscriptionCanceled(ctx, payload)
	case "SUBSCRIPTION_DELETED":
		h.handleSubscriptionDeleted(ctx, payload)
	default:
		h.logger.InfoContext(ctx, "unhandled webhook event", slog.String("event", payload.Event))
	}

	// Marca o evento como processado (idempotência)
	return h.queries.CreateProcessedWebhookEvent(ctx, db.CreateProcessedWebhookEventParams{
		EventSource: source,
		EventID:     eventID,
		WebhookID:   sql.NullInt64{Int64: webhookID, Valid: true},
	})
}

func NewWebhookHandler(queries *db.Queries, webhookToken, hmacSecret string, logger *slog.Logger) *WebhookHandler {
	return &WebhookHandler{
		queries:      queries,
		webhookToken: webhookToken,
		hmacSecret:   hmacSecret,
		logger:       logger,
	}
}

func (h *WebhookHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httpErr.HandleError(w, r, httpErr.NewValidationError("method not allowed", nil), "webhook_method")
		return
	}

	receivedToken := r.Header.Get("X-Asaas-Token")
	if receivedToken != h.webhookToken {
		h.logger.Warn("unauthorized webhook request: invalid token")
		httpErr.HandleError(w, r, httpErr.NewUnauthorizedError("invalid token"), "webhook_auth")
		return
	}

	if h.hmacSecret != "" {
		if err := h.validateHMACSignature(r); err != nil {
			h.logger.Warn("unauthorized webhook request: invalid signature", "error", err)
			httpErr.HandleError(w, r, httpErr.NewUnauthorizedError("invalid signature"), "webhook_signature")
			return
		}
	}

	const maxBodySize = 1 << 20
	r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)

	body, err := io.ReadAll(r.Body)
	if err != nil {
		h.logger.Error("failed to read webhook body", "error", err)
		httpErr.HandleError(w, r, httpErr.NewValidationError("failed to read body", nil), "read_webhook_body")
		return
	}

	var payload AsaasWebhookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		h.logger.Error("failed to parse webhook payload", "error", err)
		httpErr.HandleError(w, r, httpErr.NewValidationError("invalid payload", nil), "parse_webhook_payload")
		return
	}

	h.logger.Info("webhook received", "event", payload.Event)

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	switch payload.Event {
	case "PAYMENT_RECEIVED":
		h.handlePaymentReceived(ctx, payload)
	case "PAYMENT_CONFIRMED":
		h.handlePaymentConfirmed(ctx, payload)
	case "PAYMENT_OVERDUE":
		h.handlePaymentOverdue(ctx, payload)
	case "PAYMENT_REFUNDED":
		h.handlePaymentRefunded(ctx, payload)
	case "PAYMENT_CANCELED":
		h.handlePaymentCanceled(ctx, payload)
	case "SUBSCRIPTION_UPDATED":
		h.handleSubscriptionUpdated(ctx, payload)
	case "SUBSCRIPTION_CANCELED":
		h.handleSubscriptionCanceled(ctx, payload)
	case "SUBSCRIPTION_DELETED":
		h.handleSubscriptionDeleted(ctx, payload)
	default:
		h.logger.Info("unhandled webhook event", "event", payload.Event)
	}

	w.WriteHeader(http.StatusOK)
}

func (h *WebhookHandler) validateHMACSignature(r *http.Request) error {
	receivedSignature := r.Header.Get("X-Signature")
	if receivedSignature == "" {
		receivedSignature = r.Header.Get("X-Hub-Signature-256")
	}

	if receivedSignature == "" {
		return nil
	}

	if len(receivedSignature) > 7 && receivedSignature[:7] == "sha256=" {
		receivedSignature = receivedSignature[7:]
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}

	r.Body = io.NopCloser(bytes.NewReader(body))

	mac := hmac.New(sha256.New, []byte(h.hmacSecret))
	mac.Write(body)
	expectedSignature := hex.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(receivedSignature), []byte(expectedSignature)) {
		return &signatureError{"signature mismatch"}
	}

	return nil
}

func (h *WebhookHandler) handlePaymentReceived(ctx context.Context, payload AsaasWebhookPayload) {
	if payload.Payment == nil {
		return
	}

	if h.queries == nil {
		h.logger.Debug("skipping payment update - no queries")
		return
	}

	paymentDate := sql.NullTime{Time: time.Now(), Valid: true}
	_, err := h.queries.UpdateAsaasPaymentStatus(ctx, db.UpdateAsaasPaymentStatusParams{
		Status:      "RECEIVED",
		PaymentDate: paymentDate,
		ID:          payload.Payment.ID,
	})
	if err != nil {
		h.logger.Error("failed to update payment status", "payment_id", payload.Payment.ID, "error", err)
		return
	}

	h.logger.Info("payment received", "payment_id", payload.Payment.ID)
}

func (h *WebhookHandler) handlePaymentConfirmed(ctx context.Context, payload AsaasWebhookPayload) {
	if payload.Payment == nil {
		return
	}

	if h.queries == nil {
		h.logger.Debug("skipping payment update - no queries")
		return
	}

	paymentDate := sql.NullTime{Time: time.Now(), Valid: true}
	_, err := h.queries.UpdateAsaasPaymentStatus(ctx, db.UpdateAsaasPaymentStatusParams{
		Status:      "CONFIRMED",
		PaymentDate: paymentDate,
		ID:          payload.Payment.ID,
	})
	if err != nil {
		h.logger.Error("failed to update payment status", "payment_id", payload.Payment.ID, "error", err)
		return
	}

	h.logger.Info("payment confirmed", "payment_id", payload.Payment.ID)
}

func (h *WebhookHandler) handlePaymentOverdue(ctx context.Context, payload AsaasWebhookPayload) {
	if payload.Payment == nil {
		return
	}

	if h.queries == nil {
		h.logger.Debug("skipping payment update - no queries")
		return
	}

	_, err := h.queries.UpdateAsaasPaymentStatus(ctx, db.UpdateAsaasPaymentStatusParams{
		Status:      "OVERDUE",
		PaymentDate: sql.NullTime{},
		ID:          payload.Payment.ID,
	})
	if err != nil {
		h.logger.Error("failed to update payment status", "payment_id", payload.Payment.ID, "error", err)
		return
	}

	h.logger.Info("payment overdue", "payment_id", payload.Payment.ID)
}

func (h *WebhookHandler) handlePaymentRefunded(ctx context.Context, payload AsaasWebhookPayload) {
	if payload.Payment == nil {
		return
	}

	if h.queries == nil {
		h.logger.Debug("skipping payment update - no queries")
		return
	}

	_, err := h.queries.UpdateAsaasPaymentStatus(ctx, db.UpdateAsaasPaymentStatusParams{
		Status:      "REFUNDED",
		PaymentDate: sql.NullTime{},
		ID:          payload.Payment.ID,
	})
	if err != nil {
		h.logger.Error("failed to update payment status", "payment_id", payload.Payment.ID, "error", err)
		return
	}

	h.logger.Info("payment refunded", "payment_id", payload.Payment.ID)
}

func (h *WebhookHandler) handlePaymentCanceled(ctx context.Context, payload AsaasWebhookPayload) {
	if payload.Payment == nil {
		return
	}

	if h.queries == nil {
		h.logger.Debug("skipping payment update - no queries")
		return
	}

	_, err := h.queries.UpdateAsaasPaymentStatus(ctx, db.UpdateAsaasPaymentStatusParams{
		Status:      "CANCELED",
		PaymentDate: sql.NullTime{},
		ID:          payload.Payment.ID,
	})
	if err != nil {
		h.logger.Error("failed to update payment status", "payment_id", payload.Payment.ID, "error", err)
		return
	}

	h.logger.Info("payment canceled", "payment_id", payload.Payment.ID)
}

func (h *WebhookHandler) handleSubscriptionUpdated(ctx context.Context, payload AsaasWebhookPayload) {
	if payload.Subscription == nil {
		return
	}

	if h.queries == nil {
		h.logger.Debug("skipping subscription update - no queries")
		return
	}

	nextBillingDate, _ := time.Parse("2006-01-02", payload.Subscription.NextDueDate)
	_, err := h.queries.UpdateAsaasSubscription(ctx, db.UpdateAsaasSubscriptionParams{
		Status:          payload.Subscription.Status,
		NextBillingDate: sql.NullTime{Time: nextBillingDate, Valid: true},
		AsaasResponse:   nil,
		ID:              payload.Subscription.ID,
	})
	if err != nil {
		h.logger.Error("failed to update subscription", "subscription_id", payload.Subscription.ID, "error", err)
		return
	}

	h.logger.Info("subscription updated", "subscription_id", payload.Subscription.ID)
}

func (h *WebhookHandler) handleSubscriptionCanceled(ctx context.Context, payload AsaasWebhookPayload) {
	if payload.Subscription == nil {
		return
	}

	if h.queries == nil {
		h.logger.Debug("skipping subscription update - no queries")
		return
	}

	_, err := h.queries.UpdateAsaasSubscription(ctx, db.UpdateAsaasSubscriptionParams{
		Status:          "CANCELED",
		NextBillingDate: sql.NullTime{},
		AsaasResponse:   nil,
		ID:              payload.Subscription.ID,
	})
	if err != nil {
		h.logger.Error("failed to cancel subscription", "subscription_id", payload.Subscription.ID, "error", err)
		return
	}

	h.logger.Info("subscription canceled", "subscription_id", payload.Subscription.ID)
}

func (h *WebhookHandler) handleSubscriptionDeleted(ctx context.Context, payload AsaasWebhookPayload) {
	if payload.Subscription == nil {
		return
	}

	if h.queries == nil {
		h.logger.Debug("skipping subscription update - no queries")
		return
	}

	_, err := h.queries.UpdateAsaasSubscription(ctx, db.UpdateAsaasSubscriptionParams{
		Status:          "DELETED",
		NextBillingDate: sql.NullTime{},
		AsaasResponse:   nil,
		ID:              payload.Subscription.ID,
	})
	if err != nil {
		h.logger.Error("failed to delete subscription", "subscription_id", payload.Subscription.ID, "error", err)
		return
	}

	h.logger.Info("subscription deleted", "subscription_id", payload.Subscription.ID)
}
