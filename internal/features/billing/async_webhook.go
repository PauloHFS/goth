package billing

import (
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/PauloHFS/goth/internal/db"
	"github.com/PauloHFS/goth/internal/features/jobs"
)

// AsyncWebhookHandler é um handler assíncrono que salva webhooks no banco e enfileira para processamento
type AsyncWebhookHandler struct {
	queries      *db.Queries
	jobQueue     jobs.JobQueue
	webhookToken string
	hmacSecret   string
}

func NewAsyncWebhookHandler(queries *db.Queries, jobQueue jobs.JobQueue, webhookToken, hmacSecret string) *AsyncWebhookHandler {
	return &AsyncWebhookHandler{
		queries:      queries,
		jobQueue:     jobQueue,
		webhookToken: webhookToken,
		hmacSecret:   hmacSecret,
	}
}

func (h *AsyncWebhookHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	receivedToken := r.Header.Get("X-Asaas-Token")
	if receivedToken != h.webhookToken {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if h.hmacSecret != "" {
		if err := h.validateHMACSignature(r); err != nil {
			http.Error(w, "Invalid signature", http.StatusUnauthorized)
			return
		}
	}

	const maxBodySize = 1 << 20
	r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read body", http.StatusBadRequest)
		return
	}

	var payload AsaasWebhookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		http.Error(w, "Invalid payload", http.StatusBadRequest)
		return
	}

	// Gera um ID único para o evento baseado no tipo + ID do recurso
	eventID := h.generateEventID(payload)
	if eventID == "" {
		http.Error(w, "Invalid event payload", http.StatusBadRequest)
		return
	}

	// Serializa headers para JSON
	headers, _ := json.Marshal(map[string]string{
		"X-Asaas-Token": receivedToken,
		"Content-Type":  r.Header.Get("Content-Type"),
	})

	// Cria webhook com idempotência (unique constraint em source + external_id)
	webhook, err := h.queries.CreateWebhookWithIdempotency(r.Context(), db.CreateWebhookWithIdempotencyParams{
		Source:     "asaas",
		ExternalID: sql.NullString{String: eventID, Valid: true},
		Payload:    body,
		Headers:    headers,
	})
	if err != nil {
		// Se for erro de unique constraint, o webhook já foi processado
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			w.WriteHeader(http.StatusOK)
			return
		}
		http.Error(w, "Failed to store webhook", http.StatusInternalServerError)
		return
	}

	// Gera idempotency key para o job
	idempotencyKey := "asaas:" + eventID

	// Enfileira para processamento assíncrono (usa o mesmo job type do webhook genérico)
	jobPayload, _ := json.Marshal(map[string]interface{}{
		"webhook_id": webhook.ID,
		"source":     "asaas",
		"event_id":   eventID,
	})

	err = h.jobQueue.EnqueueWithIdempotency(r.Context(), "process_webhook", jobPayload, "", idempotencyKey)
	if err != nil {
		// Se o job já existir, retorna 200 OK
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			w.WriteHeader(http.StatusOK)
			return
		}
	}

	w.WriteHeader(http.StatusOK)
}

func (h *AsyncWebhookHandler) generateEventID(payload AsaasWebhookPayload) string {
	if payload.Payment != nil && payload.Payment.ID != "" {
		return "payment:" + payload.Payment.ID + ":" + payload.Event
	}
	if payload.Subscription != nil && payload.Subscription.ID != "" {
		return "subscription:" + payload.Subscription.ID + ":" + payload.Event
	}
	return ""
}

func (h *AsyncWebhookHandler) validateHMACSignature(r *http.Request) error {
	receivedSignature := r.Header.Get("X-Signature")
	if receivedSignature == "" {
		receivedSignature = r.Header.Get("X-Hub-Signature-256")
	}

	if receivedSignature == "" {
		return nil // HMAC não configurado
	}

	// Remove prefixo "sha256=" se presente
	if len(receivedSignature) > 7 && receivedSignature[:7] == "sha256=" {
		receivedSignature = receivedSignature[7:]
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}

	r.Body = io.NopCloser(&readCloser{reader: &byteReader{data: body}})

	mac := hmac.New(sha256.New, []byte(h.hmacSecret))
	mac.Write(body)
	expectedSignature := hex.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(receivedSignature), []byte(expectedSignature)) {
		return &signatureError{"signature mismatch"}
	}

	return nil
}

// byteReader é um helper para implementar io.ReadCloser
type byteReader struct {
	data []byte
	pos  int
}

func (r *byteReader) Read(p []byte) (n int, err error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	n = copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}

type readCloser struct {
	reader *byteReader
}

func (rc *readCloser) Read(p []byte) (n int, err error) {
	return rc.reader.Read(p)
}

func (rc *readCloser) Close() error {
	return nil
}
