package admin

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/PauloHFS/goth/internal/db"
	httpErr "github.com/PauloHFS/goth/internal/platform/http"
)

// AdminHandler gerencia rotas de administração
type AdminHandler struct {
	queries *db.Queries
}

func NewAdminHandler(queries *db.Queries) *AdminHandler {
	return &AdminHandler{queries: queries}
}

// DLQJob representa um job na dead letter queue
type DLQJob struct {
	ID           int64           `json:"id"`
	JobID        int64           `json:"job_id"`
	OriginalType string          `json:"original_type"`
	ErrorMessage string          `json:"error_message"`
	AttemptCount int64           `json:"attempt_count"`
	FailedAt     string          `json:"failed_at"`
	TenantID     sql.NullString  `json:"tenant_id,omitempty"`
	Metadata     json.RawMessage `json:"metadata,omitempty"`
}

// DLQListResponse resposta para listagem de DLQ
type DLQListResponse struct {
	Jobs  []DLQJob `json:"jobs"`
	Total int64    `json:"total"`
	Page  int      `json:"page"`
	Limit int      `json:"limit"`
}

// RegisterRoutes registra as rotas de admin
func (h *AdminHandler) RegisterRoutes(mux *http.ServeMux) {
	// DLQ endpoints
	mux.HandleFunc("GET /admin/dlq", h.listDLQ)
	mux.HandleFunc("GET /admin/dlq/{id}", h.getDLQJob)
	mux.HandleFunc("POST /admin/dlq/{id}/retry", h.retryDLQJob)
	mux.HandleFunc("DELETE /admin/dlq/{id}", h.deleteDLQJob)
	mux.HandleFunc("POST /admin/dlq/retry-all", h.retryAllDLQ)

	// Worker stats endpoints
	mux.HandleFunc("GET /admin/workers", h.getWorkerStats)
	mux.HandleFunc("GET /admin/circuit-breakers", h.getCircuitBreakers)
}

// listDLQ lista jobs na dead letter queue
func (h *AdminHandler) listDLQ(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page <= 0 {
		page = 1
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}

	offset := (page - 1) * limit

	ctx := r.Context()
	jobs, err := h.queries.GetDeadLetterJobs(ctx, db.GetDeadLetterJobsParams{
		Limit:  int64(limit),
		Offset: int64(offset),
	})
	if err != nil {
		httpErr.HandleError(w, r, err, "list_dlq_jobs")
		return
	}

	total, err := h.queries.GetDeadLetterJobCount(ctx)
	if err != nil {
		httpErr.HandleError(w, r, err, "get_dlq_count")
		return
	}

	dlqJobs := make([]DLQJob, len(jobs))
	for i, job := range jobs {
		dlqJobs[i] = DLQJob{
			ID:           job.ID,
			JobID:        job.JobID,
			OriginalType: job.OriginalType,
			ErrorMessage: job.ErrorMessage,
			AttemptCount: job.AttemptCount,
			FailedAt:     job.FailedAt.String(),
			TenantID:     job.TenantID,
			Metadata:     job.Metadata,
		}
	}

	resp := DLQListResponse{
		Jobs:  dlqJobs,
		Total: total,
		Page:  page,
		Limit: limit,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		httpErr.HandleError(w, r, err, "encode_response")
		return
	}
}

// getDLQJob retorna um job específico da DLQ
func (h *AdminHandler) getDLQJob(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		httpErr.HandleError(w, r, httpErr.NewValidationError("invalid ID", nil), "parse_dlq_id")
		return
	}

	ctx := r.Context()
	jobs, err := h.queries.GetDeadLetterJobs(ctx, db.GetDeadLetterJobsParams{
		Limit:  100,
		Offset: 0,
	})
	if err != nil {
		httpErr.HandleError(w, r, err, "get_dlq_job")
		return
	}

	var foundJob *db.DeadLetterJob
	for _, job := range jobs {
		if job.ID == id {
			foundJob = &job
			break
		}
	}

	if foundJob == nil {
		httpErr.HandleError(w, r, httpErr.NewNotFoundError("job"), "get_dlq_job")
		return
	}

	resp := DLQJob{
		ID:           foundJob.ID,
		JobID:        foundJob.JobID,
		OriginalType: foundJob.OriginalType,
		ErrorMessage: foundJob.ErrorMessage,
		AttemptCount: foundJob.AttemptCount,
		FailedAt:     foundJob.FailedAt.String(),
		TenantID:     foundJob.TenantID,
		Metadata:     foundJob.Metadata,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		httpErr.HandleError(w, r, err, "encode_response")
		return
	}
}

// retryDLQJob retrya um job específico da DLQ
func (h *AdminHandler) retryDLQJob(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		httpErr.HandleError(w, r, httpErr.NewValidationError("invalid ID", nil), "parse_dlq_id")
		return
	}

	ctx := r.Context()

	// Busca o job
	jobs, err := h.queries.GetDeadLetterJobs(ctx, db.GetDeadLetterJobsParams{
		Limit:  1,
		Offset: 0,
	})
	if err != nil {
		httpErr.HandleError(w, r, err, "get_dlq_job")
		return
	}

	var foundJob *db.DeadLetterJob
	for _, job := range jobs {
		if job.ID == id {
			foundJob = &job
			break
		}
	}

	if foundJob == nil {
		httpErr.HandleError(w, r, httpErr.NewNotFoundError("job"), "get_dlq_job")
		return
	}

	// Re-enfileira o job
	_, err = h.queries.RetryDeadLetterJob(ctx, db.RetryDeadLetterJobParams{
		TenantID:       foundJob.TenantID,
		Type:           foundJob.OriginalType,
		Payload:        foundJob.OriginalPayload,
		RunAt:          sql.NullTime{Time: foundJob.FailedAt, Valid: true},
		AttemptCount:   sql.NullInt64{Int64: 0, Valid: true},
		TimeoutSeconds: sql.NullInt64{Int64: 300, Valid: true},
	})
	if err != nil {
		httpErr.HandleError(w, r, err, "retry_dlq_job")
		return
	}

	// Remove da DLQ
	err = h.queries.DeleteDeadLetterJob(ctx, id)
	if err != nil {
		// Log error but don't fail - job was already requeued
		httpErr.HandleError(w, r, err, "delete_dlq_job")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]string{"status": "success", "message": "Job requeued"}); err != nil {
		httpErr.HandleError(w, r, err, "encode_response")
		return
	}
}

// deleteDLQJob remove um job da DLQ
func (h *AdminHandler) deleteDLQJob(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		httpErr.HandleError(w, r, httpErr.NewValidationError("invalid ID", nil), "parse_dlq_id")
		return
	}

	ctx := r.Context()
	err = h.queries.DeleteDeadLetterJob(ctx, id)
	if err != nil {
		httpErr.HandleError(w, r, err, "delete_dlq_job")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]string{"status": "success", "message": "Job deleted"}); err != nil {
		httpErr.HandleError(w, r, err, "encode_response")
		return
	}
}

// retryAllDLQ retrya todos os jobs da DLQ
func (h *AdminHandler) retryAllDLQ(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	jobs, err := h.queries.GetDeadLetterJobs(ctx, db.GetDeadLetterJobsParams{
		Limit:  1000,
		Offset: 0,
	})
	if err != nil {
		httpErr.HandleError(w, r, err, "list_dlq_jobs")
		return
	}

	retried := 0
	failed := 0

	for _, job := range jobs {
		_, err := h.queries.RetryDeadLetterJob(ctx, db.RetryDeadLetterJobParams{
			TenantID:       job.TenantID,
			Type:           job.OriginalType,
			Payload:        job.OriginalPayload,
			RunAt:          sql.NullTime{Time: job.FailedAt, Valid: true},
			AttemptCount:   sql.NullInt64{Int64: 0, Valid: true},
			TimeoutSeconds: sql.NullInt64{Int64: 300, Valid: true},
		})
		if err != nil {
			failed++
			continue
		}

		_ = h.queries.DeleteDeadLetterJob(ctx, job.ID)
		retried++
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "success",
		"retried": retried,
		"failed":  failed,
	}); err != nil {
		httpErr.HandleError(w, r, err, "encode_response")
		return
	}
}

// WorkerStatsResponse resposta para stats de workers
type WorkerStatsResponse struct {
	ActiveWorkers   int32                  `json:"active_workers"`
	PendingJobs     int64                  `json:"pending_jobs"`
	ProcessingJobs  int64                  `json:"processing_jobs"`
	CircuitBreakers []CircuitBreakerStatus `json:"circuit_breakers"`
}

type CircuitBreakerStatus struct {
	JobType  string `json:"job_type"`
	State    string `json:"state"`
	Failures int64  `json:"failures"`
}

// getWorkerStats retorna stats dos workers
func (h *AdminHandler) getWorkerStats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get pending jobs count
	pending, _ := h.queries.GetPendingJobCount(ctx)

	resp := WorkerStatsResponse{
		PendingJobs: pending,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		httpErr.HandleError(w, r, err, "encode_response")
		return
	}
}

// getCircuitBreakers retorna estado dos circuit breakers
func (h *AdminHandler) getCircuitBreakers(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	states, err := h.queries.ListCircuitBreakerStates(ctx)
	if err != nil {
		httpErr.HandleError(w, r, err, "list_circuit_breakers")
		return
	}

	resp := make([]CircuitBreakerStatus, len(states))
	for i, state := range states {
		resp[i] = CircuitBreakerStatus{
			JobType:  state.JobType,
			State:    state.State,
			Failures: state.FailureCount,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		httpErr.HandleError(w, r, err, "encode_response")
		return
	}
}
