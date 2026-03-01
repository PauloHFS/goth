package jobs

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/PauloHFS/goth/internal/db"
)

// NotifyFunc é uma função callback para notificar novos jobs
type NotifyFunc func()

type JobQueue interface {
	Enqueue(ctx context.Context, jobType string, payload []byte, tenantID string) error
	EnqueueWithIdempotency(ctx context.Context, jobType string, payload []byte, tenantID string, idempotencyKey string) error
	NotifyNewJob()
}

type jobQueue struct {
	db     *sql.DB
	q      *db.Queries
	notify NotifyFunc // Callback para notificação
}

func NewJobQueue(dbConn *sql.DB) JobQueue {
	return &jobQueue{db: dbConn, q: db.New(dbConn)}
}

// NewJobQueueWithNotify cria JobQueue com callback de notificação
func NewJobQueueWithNotify(dbConn *sql.DB, notify NotifyFunc) JobQueue {
	return &jobQueue{db: dbConn, q: db.New(dbConn), notify: notify}
}

func (q *jobQueue) Enqueue(ctx context.Context, jobType string, payload []byte, tenantID string) error {
	_, err := q.q.CreateJob(ctx, db.CreateJobParams{
		TenantID:       sql.NullString{String: tenantID, Valid: tenantID != ""},
		Type:           jobType,
		Payload:        payload,
		RunAt:          sql.NullTime{Time: time.Now(), Valid: true},
		IdempotencyKey: sql.NullString{},
	})
	if err != nil {
		return err
	}

	// Notifica workers
	q.NotifyNewJob()

	return nil
}

func (q *jobQueue) EnqueueWithIdempotency(ctx context.Context, jobType string, payload []byte, tenantID string, idempotencyKey string) error {
	_, err := q.q.CreateJobWithIdempotency(ctx, db.CreateJobWithIdempotencyParams{
		TenantID:       sql.NullString{String: tenantID, Valid: tenantID != ""},
		Type:           jobType,
		Payload:        payload,
		RunAt:          sql.NullTime{Time: time.Now(), Valid: true},
		IdempotencyKey: sql.NullString{String: idempotencyKey, Valid: idempotencyKey != ""},
	})
	if err != nil {
		return err
	}

	// Notifica workers
	q.NotifyNewJob()

	return nil
}

// NotifyNewJob notifica os workers que há um novo job
func (q *jobQueue) NotifyNewJob() {
	if q.notify != nil {
		q.notify()
	}
}

type JobRepository interface {
	PickNext(ctx context.Context) (db.Job, error)
	Complete(ctx context.Context, id int64) error
	Fail(ctx context.Context, id int64, errMsg string) error
	Create(ctx context.Context, params CreateJobParams) (db.Job, error)
	RecordProcessed(ctx context.Context, jobID int64) error
	IsProcessed(ctx context.Context, jobID int64) (bool, error)
}

type CreateJobParams struct {
	TenantID string
	Type     string
	Payload  json.RawMessage
	RunAt    time.Time
}

type repository struct {
	db *sql.DB
	q  *db.Queries
}

func NewRepository(dbConn *sql.DB) JobRepository {
	return &repository{
		db: dbConn,
		q:  db.New(dbConn),
	}
}

func (r *repository) PickNext(ctx context.Context) (db.Job, error) {
	return r.q.PickNextJob(ctx)
}

func (r *repository) Complete(ctx context.Context, id int64) error {
	return r.q.CompleteJob(ctx, id)
}

func (r *repository) Fail(ctx context.Context, id int64, errMsg string) error {
	return r.q.FailJob(ctx, db.FailJobParams{
		LastError: sql.NullString{String: errMsg, Valid: true},
		ID:        id,
	})
}

func (r *repository) Create(ctx context.Context, params CreateJobParams) (db.Job, error) {
	return r.q.CreateJob(ctx, db.CreateJobParams{
		TenantID: sql.NullString{String: params.TenantID, Valid: params.TenantID != ""},
		Type:     params.Type,
		Payload:  params.Payload,
		RunAt:    sql.NullTime{Time: params.RunAt, Valid: !params.RunAt.IsZero()},
	})
}

func (r *repository) RecordProcessed(ctx context.Context, jobID int64) error {
	return r.q.RecordJobProcessed(ctx, jobID)
}

func (r *repository) IsProcessed(ctx context.Context, jobID int64) (bool, error) {
	processed, err := r.q.IsJobProcessed(ctx, jobID)
	return processed == 1, err
}
