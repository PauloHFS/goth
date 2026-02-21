package worker

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/PauloHFS/goth/internal/config"
	"github.com/PauloHFS/goth/internal/db"
	"github.com/PauloHFS/goth/internal/logging"
	"github.com/PauloHFS/goth/internal/mailer"
	"github.com/PauloHFS/goth/internal/metrics"
	"github.com/PauloHFS/goth/internal/web"
)

type Processor struct {
	config      *config.Config
	db          *sql.DB
	queries     *db.Queries
	logger      *slog.Logger
	mailer      *mailer.Mailer
	wg          sync.WaitGroup
	jobNotify   chan struct{}
	rateLimiter *JobRateLimiter
	dlq         *DeadLetterQueue
}

func New(cfg *config.Config, dbConn *sql.DB, q *db.Queries, l *slog.Logger) *Processor {
	return &Processor{
		config:      cfg,
		db:          dbConn,
		queries:     q,
		logger:      l,
		mailer:      mailer.New(cfg),
		jobNotify:   make(chan struct{}, 1),
		rateLimiter: NewJobRateLimiter(),
		dlq:         NewDeadLetterQueue(q, dbConn, l),
	}
}

func (p *Processor) Start(ctx context.Context) {
	p.logger.Info("worker started")
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			p.logger.Info("worker stopping: waiting for active jobs to finish")
			p.wg.Wait()
			return
		case <-p.jobNotify:
			p.processNext(ctx)
		case <-ticker.C:
			p.processNext(ctx)
		}
	}
}

func (p *Processor) NotifyNewJob() {
	select {
	case p.jobNotify <- struct{}{}:
	default:
	}
}

func (p *Processor) Wait() {
	p.wg.Wait()
}

func (p *Processor) GetDLQ() *DeadLetterQueue {
	return p.dlq
}

func (p *Processor) GetRateLimiter() *JobRateLimiter {
	return p.rateLimiter
}

func (p *Processor) processNext(ctx context.Context) {
	job, err := p.queries.PickNextJob(ctx)
	if err != nil {
		return
	}

	p.wg.Add(1)
	go p.processJob(ctx, job)
}

func (p *Processor) processJob(ctx context.Context, job db.Job) {
	defer p.wg.Done()

	start := time.Now()

	ctx, event := logging.NewEventContext(ctx)
	event.Add(
		slog.Int64("job_id", int64(job.ID)),
		slog.String("job_type", string(job.Type)),
		slog.Int64("attempt", job.AttemptCount),
	)

	processed, err := p.queries.IsJobProcessed(ctx, job.ID)
	if err == nil && processed == 1 {
		p.logger.InfoContext(ctx, "job already processed, skipping", event.Attrs()...)
		_ = p.queries.CompleteJob(ctx, job.ID)
		return
	}

	if err := p.rateLimiter.Acquire(ctx, string(job.Type)); err != nil {
		p.logger.WarnContext(ctx, "rate limit wait cancelled", "error", err.Error())
		return
	}
	defer p.rateLimiter.Release(string(job.Type))

	errProcessing := p.handleJob(ctx, job)

	if retryAfter := GetRetryAfterDuration(errProcessing); retryAfter > 0 && IsExternalRateLimitError(errProcessing) {
		p.logger.WarnContext(ctx, "external rate limit detected, backing off",
			"retry_after", retryAfter.String(),
			"error", errProcessing.Error(),
		)
		time.Sleep(retryAfter)
		errProcessing = nil
	}

	if errProcessing != nil {
		p.handleFailure(ctx, job, errProcessing, start, event)
		return
	}

	p.handleSuccess(ctx, job, start, event)

	if job.TenantID.Valid {
		var userID int64
		if _, err := fmt.Sscanf(job.TenantID.String, "%d", &userID); err == nil && userID > 0 {
			web.BroadcastToUser(userID, "job_completed", string(job.Type))
			return
		}
	}

	web.Broadcast("job_completed", string(job.Type))
}

func (p *Processor) handleJob(ctx context.Context, job db.Job) error {
	switch job.Type {
	case "send_email":
		return p.handleSendEmail(ctx, job.Payload)
	case "send_password_reset_email":
		return p.handleSendPasswordResetEmail(ctx, job.Payload)
	case "send_verification_email":
		return p.handleSendVerificationEmail(ctx, job.Payload)
	case "process_ai":
		return p.handleProcessAI(ctx, job.Payload)
	case "process_webhook":
		return p.handleProcessWebhook(ctx, job.Payload)
	default:
		p.logger.WarnContext(ctx, "unknown job type", "type", job.Type)
		return fmt.Errorf("unknown job type: %s", job.Type)
	}
}

func (p *Processor) handleFailure(ctx context.Context, job db.Job, errProcessing error, start time.Time, event *logging.Event) {
	metrics.JobDuration.WithLabelValues(string(job.Type), "failed").Observe(time.Since(start).Seconds())

	if p.dlq.ShouldMoveToDLQ(job) {
		if err := p.dlq.Move(ctx, job, errProcessing); err != nil {
			p.logger.ErrorContext(ctx, "failed to move job to DLQ", "error", err.Error())
		}
		return
	}

	if err := p.queries.FailJob(ctx, db.FailJobParams{
		LastError: sql.NullString{String: errProcessing.Error(), Valid: true},
		ID:        job.ID,
	}); err != nil {
		p.logger.ErrorContext(ctx, "failed to record job failure in db", "error", err.Error())
	}

	if IsRetryableError(errProcessing) {
		backoff := FullJitter(int(job.AttemptCount), DefaultBackoffConfig)
		p.logger.InfoContext(ctx, "retryable error, will retry",
			"backoff", backoff.String(),
			"error", errProcessing.Error(),
		)
		metrics.JobRetries.WithLabelValues(string(job.Type)).Inc()
	}

	p.logger.ErrorContext(ctx, "job processing failed",
		append(event.Attrs(), slog.String("error", errProcessing.Error()))...)
}

func (p *Processor) handleSuccess(ctx context.Context, job db.Job, start time.Time, event *logging.Event) {
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		p.logger.ErrorContext(ctx, "failed to start transaction", "error", err.Error())
		return
	}
	defer func() { _ = tx.Rollback() }()

	qtx := p.queries.WithTx(tx)

	if err := qtx.RecordJobProcessed(ctx, job.ID); err != nil {
		p.logger.ErrorContext(ctx, "failed to record job processed", "error", err.Error())
		return
	}

	if err := qtx.CompleteJob(ctx, job.ID); err != nil {
		p.logger.ErrorContext(ctx, "failed to complete job", "error", err.Error())
		return
	}

	if err := tx.Commit(); err != nil {
		p.logger.ErrorContext(ctx, "failed to commit transaction", "error", err.Error())
		return
	}

	duration := time.Since(start)
	metrics.JobDuration.WithLabelValues(string(job.Type), "success").Observe(duration.Seconds())
	event.Add(slog.Float64("duration_ms", float64(duration.Nanoseconds())/1e6))

	p.logger.InfoContext(ctx, "job completed", event.Attrs()...)
}

func (p *Processor) handleSendEmail(ctx context.Context, payload json.RawMessage) error {
	var data struct {
		To      string `json:"to"`
		Subject string `json:"subject"`
		Body    string `json:"body"`
	}

	if err := json.Unmarshal(payload, &data); err != nil {
		return err
	}

	return p.mailer.Send(data.To, data.Subject, data.Body)
}

func (p *Processor) handleSendVerificationEmail(ctx context.Context, payload json.RawMessage) error {
	var data struct {
		Email string `json:"email"`
		Token string `json:"token"`
	}

	if err := json.Unmarshal(payload, &data); err != nil {
		return err
	}

	verifyURL := p.config.BaseURL + "/verify-email?token=" + data.Token
	subject := "Verifique seu E-mail"
	body := "Olá,\n\nBem-vindo! Clique no link abaixo para verificar seu e-mail:\n\n" + verifyURL

	return p.mailer.Send(data.Email, subject, body)
}

func (p *Processor) handleSendPasswordResetEmail(ctx context.Context, payload json.RawMessage) error {
	var data struct {
		Email string `json:"email"`
		Token string `json:"token"`
	}

	if err := json.Unmarshal(payload, &data); err != nil {
		return err
	}

	resetURL := p.config.BaseURL + "/reset-password?token=" + data.Token
	subject := "Recuperação de Senha"
	body := "Olá,\n\nClique no link abaixo para redefinir sua senha:\n\n" +
		resetURL + "\n\nEste link expira em 1 hora."

	return p.mailer.Send(data.Email, subject, body)
}

func (p *Processor) handleProcessAI(ctx context.Context, payload json.RawMessage) error {
	var data struct {
		Prompt string `json:"prompt"`
	}

	if err := json.Unmarshal(payload, &data); err != nil {
		return err
	}

	p.logger.InfoContext(ctx, "AI processing started", "prompt", data.Prompt)
	time.Sleep(2 * time.Second)

	return nil
}

func (p *Processor) handleProcessWebhook(ctx context.Context, payload json.RawMessage) error {
	var data struct {
		WebhookID int64 `json:"webhook_id"`
	}

	if err := json.Unmarshal(payload, &data); err != nil {
		return err
	}

	p.logger.InfoContext(ctx, "processing webhook event", "webhook_id", data.WebhookID)

	return nil
}
