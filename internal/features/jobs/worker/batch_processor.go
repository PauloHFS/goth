package worker

import (
	"context"
	"database/sql"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/PauloHFS/goth/internal/db"
	"github.com/PauloHFS/goth/internal/platform/mailer"
)

// BatchProcessor processa jobs em lote para eficiência
type BatchProcessor struct {
	mailer     *mailer.Mailer
	queries    *db.Queries
	db         *sql.DB
	logger     *slog.Logger
	batchSize  int
	batchDelay time.Duration
}

// NewBatchProcessor cria um novo processador de batches
func NewBatchProcessor(db *sql.DB, mailer *mailer.Mailer, queries *db.Queries, logger *slog.Logger) *BatchProcessor {
	return &BatchProcessor{
		db:         db,
		mailer:     mailer,
		queries:    queries,
		logger:     logger,
		batchSize:  50,                     // Processa até 50 emails por batch
		batchDelay: 500 * time.Millisecond, // Espera até 500ms para formar batch
	}
}

// EmailBatchJob representa um job de email em batch
type EmailBatchJob struct {
	To      string `json:"to"`
	Subject string `json:"subject"`
	Body    string `json:"body"`
	JobID   int64  `json:"job_id"`
}

// ProcessEmailBatch processa múltiplos jobs de email em lote
func (bp *BatchProcessor) ProcessEmailBatch(ctx context.Context, jobIDs []int64) error {
	if len(jobIDs) == 0 {
		return nil
	}

	bp.logger.InfoContext(ctx, "processing email batch",
		slog.Int("batch_size", len(jobIDs)),
	)

	// Busca todos os jobs do batch
	jobs := make([]db.Job, 0, len(jobIDs))
	for _, jobID := range jobIDs {
		job, err := bp.queries.GetJobByID(ctx, jobID)
		if err != nil {
			bp.logger.WarnContext(ctx, "failed to get job for batch",
				slog.Int64("job_id", jobID),
				slog.String("error", err.Error()),
			)
			continue
		}
		jobs = append(jobs, job)
	}

	if len(jobs) == 0 {
		return nil
	}

	// Agrupa emails por destinatário para evitar duplicatas
	emailMap := make(map[string][]EmailBatchJob)
	for _, job := range jobs {
		var emailJob EmailBatchJob
		if err := json.Unmarshal(job.Payload, &emailJob); err != nil {
			bp.logger.WarnContext(ctx, "failed to parse email job payload",
				slog.Int64("job_id", job.ID),
				slog.String("error", err.Error()),
			)
			continue
		}
		emailJob.JobID = job.ID
		emailMap[emailJob.To] = append(emailMap[emailJob.To], emailJob)
	}

	// Processa emails em paralelo por destinatário
	var wg sync.WaitGroup
	errChan := make(chan error, len(emailMap))

	for to, emails := range emailMap {
		wg.Add(1)
		go func(to string, emails []EmailBatchJob) {
			defer wg.Done()

			// Envia todos os emails para este destinatário
			for _, email := range emails {
				err := bp.mailer.Send(ctx, email.To, email.Subject, email.Body)
				if err != nil {
					bp.logger.ErrorContext(ctx, "failed to send email in batch",
						slog.String("to", to),
						slog.Int64("job_id", email.JobID),
						slog.String("error", err.Error()),
					)
					errChan <- err
				}
			}
		}(to, emails)
	}

	wg.Wait()
	close(errChan)

	// Verifica se houve erros
	var hasError bool
	for err := range errChan {
		if err != nil {
			hasError = true
		}
	}

	if hasError {
		return sql.ErrTxDone // Error genérico para indicar falha parcial
	}

	bp.logger.InfoContext(ctx, "email batch completed successfully",
		slog.Int("emails_sent", len(jobs)),
	)

	return nil
}

// MarkJobsAsProcessed marca todos os jobs do batch como processados
func (bp *BatchProcessor) MarkJobsAsProcessed(ctx context.Context, jobIDs []int64) error {
	tx, err := bp.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	qtx := bp.queries.WithTx(tx)

	for _, jobID := range jobIDs {
		if err := qtx.RecordJobProcessed(ctx, jobID); err != nil {
			bp.logger.WarnContext(ctx, "failed to mark job as processed",
				slog.Int64("job_id", jobID),
				slog.String("error", err.Error()),
			)
		}
		if err := qtx.CompleteJob(ctx, jobID); err != nil {
			bp.logger.WarnContext(ctx, "failed to complete job",
				slog.Int64("job_id", jobID),
				slog.String("error", err.Error()),
			)
		}
	}

	return tx.Commit()
}
