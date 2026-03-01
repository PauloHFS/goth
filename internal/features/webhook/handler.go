package webhook

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/PauloHFS/goth/internal/db"
	"github.com/PauloHFS/goth/internal/features/jobs"
	httpErr "github.com/PauloHFS/goth/internal/platform/http"
)

type CreateWebhookParams struct {
	Source     string
	ExternalID string
	Payload    []byte
	Headers    []byte
}

type WebhookRepository interface {
	Create(ctx context.Context, params CreateWebhookParams) (db.Webhook, error)
	CreateWithIdempotency(ctx context.Context, params CreateWebhookParams) (db.Webhook, error)
}

type WebhookHandler struct {
	repo     WebhookRepository
	jobQueue jobs.JobQueue
}

func NewWebhookHandler(repo WebhookRepository, jobQueue jobs.JobQueue) *WebhookHandler {
	return &WebhookHandler{
		repo:     repo,
		jobQueue: jobQueue,
	}
}

func (h *WebhookHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	source := r.PathValue("source")
	if source == "" {
		httpErr.HandleError(w, r, httpErr.NewValidationError("source required", nil), "webhook_source")
		return
	}

	payload, err := io.ReadAll(r.Body)
	if err != nil {
		httpErr.HandleError(w, r, err, "read_webhook_body")
		return
	}

	var input struct {
		ExternalID string `json:"id"`
	}
	if err := json.Unmarshal(payload, &input); err != nil {
		httpErr.HandleError(w, r, httpErr.NewValidationError("invalid json", nil), "parse_webhook_json")
		return
	}

	headers, _ := json.Marshal(r.Header)

	// Idempotency: Tenta criar webhook com unique constraint
	// Se já existir (duplicata), o banco retorna erro de unique constraint
	webhook, err := h.repo.CreateWithIdempotency(r.Context(), CreateWebhookParams{
		Source:     source,
		ExternalID: input.ExternalID,
		Payload:    payload,
		Headers:    headers,
	})
	if err != nil {
		// Verifica se é erro de unique constraint (webhook duplicado)
		// Se for duplicata, retorna 200 OK (já processado)
		if isUniqueConstraintError(err) {
			w.WriteHeader(http.StatusOK)
			return
		}
		httpErr.HandleError(w, r, err, "store_webhook")
		return
	}

	// Gera idempotency key baseada no source + external_id
	idempotencyKey := fmt.Sprintf("%s:%s", source, input.ExternalID)

	jobPayload, _ := json.Marshal(map[string]interface{}{
		"webhook_id": webhook.ID,
		"source":     source,
	})

	// Enqueue com idempotency key para prevenir jobs duplicados
	_ = h.jobQueue.EnqueueWithIdempotency(r.Context(), "process_webhook", jobPayload, "", idempotencyKey)

	w.WriteHeader(http.StatusOK)
}

// isUniqueConstraintError verifica se o erro é de unique constraint do SQLite
func isUniqueConstraintError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "UNIQUE constraint failed")
}

type webhookRepository struct {
	db *sql.DB
	q  *db.Queries
}

func NewRepository(dbConn *sql.DB) WebhookRepository {
	return &webhookRepository{db: dbConn, q: db.New(dbConn)}
}

func (r *webhookRepository) Create(ctx context.Context, params CreateWebhookParams) (db.Webhook, error) {
	return r.q.CreateWebhook(ctx, db.CreateWebhookParams{
		Source:     params.Source,
		ExternalID: sql.NullString{String: params.ExternalID, Valid: params.ExternalID != ""},
		Payload:    params.Payload,
		Headers:    params.Headers,
	})
}

func (r *webhookRepository) CreateWithIdempotency(ctx context.Context, params CreateWebhookParams) (db.Webhook, error) {
	return r.q.CreateWebhookWithIdempotency(ctx, db.CreateWebhookWithIdempotencyParams{
		Source:     params.Source,
		ExternalID: sql.NullString{String: params.ExternalID, Valid: params.ExternalID != ""},
		Payload:    params.Payload,
		Headers:    params.Headers,
	})
}
