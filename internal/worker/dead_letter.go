package worker

import (
	"context"
	"database/sql"
	"log/slog"
	"time"

	"github.com/PauloHFS/goth/internal/db"
	"github.com/PauloHFS/goth/internal/metrics"
)

const (
	MaxJobAttempts   = 5
	DLQRetentionDays = 14
)

type DeadLetterQueue struct {
	queries *db.Queries
	db      *sql.DB
	logger  *slog.Logger
}

func NewDeadLetterQueue(queries *db.Queries, db *sql.DB, logger *slog.Logger) *DeadLetterQueue {
	return &DeadLetterQueue{
		queries: queries,
		db:      db,
		logger:  logger,
	}
}

func (dlq *DeadLetterQueue) ShouldMoveToDLQ(job db.Job) bool {
	if job.AttemptCount >= MaxJobAttempts {
		return true
	}

	if job.CreatedAt.Valid && time.Since(job.CreatedAt.Time) > 24*time.Hour {
		return true
	}

	return false
}

func (dlq *DeadLetterQueue) Move(ctx context.Context, job db.Job, lastErr error) error {
	tx, err := dlq.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	qtx := dlq.queries.WithTx(tx)

	if err := qtx.MoveToDeadLetter(ctx, job.ID); err != nil {
		return err
	}

	if err := qtx.DeleteJob(ctx, job.ID); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	metrics.JobsDeadLetter.WithLabelValues(string(job.Type)).Inc()

	dlq.logger.ErrorContext(ctx, "job moved to dead letter queue",
		slog.Int64("job_id", int64(job.ID)),
		slog.String("type", string(job.Type)),
		slog.Int64("attempts", job.AttemptCount),
		slog.String("error", lastErr.Error()),
	)

	return nil
}

func (dlq *DeadLetterQueue) Reprocess(ctx context.Context, dlqJobID int64) (*db.Job, error) {
	job, err := dlq.queries.ReprocessDeadLetterJob(ctx, dlqJobID)
	if err != nil {
		return nil, err
	}

	if err := dlq.queries.DeleteDeadLetterJob(ctx, dlqJobID); err != nil {
		return nil, err
	}

	dlq.logger.InfoContext(ctx, "job reprocessed from DLQ",
		slog.Int64("dlq_id", dlqJobID),
		slog.Int64("new_job_id", int64(job.ID)),
	)

	return &job, nil
}

func (dlq *DeadLetterQueue) Delete(ctx context.Context, id int64) error {
	return dlq.queries.DeleteDeadLetterJob(ctx, id)
}

func (dlq *DeadLetterQueue) Cleanup(ctx context.Context) error {
	return dlq.queries.CleanupDeadLetterJobs(ctx)
}

func (dlq *DeadLetterQueue) Stats(ctx context.Context) (map[string]int64, error) {
	total, err := dlq.queries.CountDeadLetterJobs(ctx)
	if err != nil {
		return nil, err
	}

	stats := map[string]int64{
		"total": total,
	}

	for _, jobType := range []string{"send_email", "send_verification_email", "send_password_reset_email", "process_ai", "process_webhook"} {
		count, _ := dlq.queries.CountDeadLetterJobsByType(ctx, jobType)
		stats[jobType] = count
	}

	return stats, nil
}
