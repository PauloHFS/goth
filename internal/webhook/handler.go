package webhook

import (
	"database/sql"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/PauloHFS/goth/internal/db"
	"github.com/PauloHFS/goth/internal/logging"
	"github.com/PauloHFS/goth/internal/validator"
)

type webhookInput struct {
	ExternalID string `json:"id" validate:"required"`
}

type Handler struct {
	queries *db.Queries
}

func NewHandler(q *db.Queries) *Handler {
	return &Handler{queries: q}
}

// ServeHTTP handles incoming webhooks
// @Summary Receber Webhook
// @Description Recebe um payload de webhook, persiste no banco e enfileira um job.
// @Tags webhooks
// @Accept json
// @Produce json
// @Param source path string true "Fonte do webhook (ex: stripe)"
// @Param payload body webhookInput true "Payload do webhook"
// @Success 200 {string} string "OK"
// @Failure 400 {string} string "Bad Request"
// @Router /webhooks/{source} [post]
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	source := r.PathValue("source")
	if source == "" {
		http.Error(w, "source required", http.StatusBadRequest)
		return
	}

	ctx, event := logging.NewEventContext(r.Context())

	event.Add(
		slog.String("source", source),
		slog.String("method", r.Method),
		slog.String("path", r.URL.Path),
	)

	payload, err := io.ReadAll(r.Body)
	if err != nil {
		event.Add(
			slog.String("outcome", "error"),
			slog.String("error", "failed to read body"),
		)
		logging.Get().Log(ctx, slog.LevelError, "webhook processing failed", event.Attrs()...)
		http.Error(w, "failed to read body", http.StatusInternalServerError)
		return
	}

	var input webhookInput
	if err := json.Unmarshal(payload, &input); err != nil {
		event.Add(
			slog.String("outcome", "error"),
			slog.String("error", "invalid json"),
		)
		logging.Get().Log(ctx, slog.LevelError, "webhook processing failed", event.Attrs()...)
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	if err := validator.Validate(input); err != nil {
		event.Add(
			slog.String("outcome", "error"),
			slog.String("error", "validation failed"),
		)
		logging.Get().Log(ctx, slog.LevelError, "webhook processing failed", event.Attrs()...)
		http.Error(w, "validation failed", http.StatusUnprocessableEntity)
		return
	}

	event.Add(slog.String("external_id", input.ExternalID))

	headers, _ := json.Marshal(r.Header)

	webhook, err := h.queries.CreateWebhook(r.Context(), db.CreateWebhookParams{
		Source:     source,
		ExternalID: sql.NullString{String: input.ExternalID, Valid: true},
		Payload:    payload,
		Headers:    headers,
	})
	if err != nil {
		event.Add(
			slog.String("outcome", "error"),
			slog.String("error", "failed to store webhook"),
		)
		logging.Get().Log(ctx, slog.LevelError, "webhook processing failed", event.Attrs()...)
		http.Error(w, "failed to store webhook", http.StatusInternalServerError)
		return
	}

	event.Add(slog.Int64("webhook_id", webhook.ID))

	jobPayload, _ := json.Marshal(map[string]int64{"webhook_id": webhook.ID})
	_, err = h.queries.CreateJob(r.Context(), db.CreateJobParams{
		TenantID: sql.NullString{String: "default", Valid: true},
		Type:     "process_webhook",
		Payload:  jobPayload,
		RunAt:    sql.NullTime{Time: time.Now(), Valid: true},
	})

	if err != nil {
		event.Add(
			slog.String("outcome", "error"),
			slog.String("error", "failed to enqueue job"),
		)
		logging.Get().Log(ctx, slog.LevelError, "webhook processing failed", event.Attrs()...)
		http.Error(w, "failed to enqueue job", http.StatusInternalServerError)
		return
	}

	event.Add(
		slog.String("outcome", "success"),
		slog.Int("status", http.StatusOK),
		slog.Float64("duration_ms", float64(time.Since(start).Nanoseconds())/1e6),
	)

	logging.Get().Log(ctx, slog.LevelInfo, "webhook processed", event.Attrs()...)

	w.WriteHeader(http.StatusOK)
}
