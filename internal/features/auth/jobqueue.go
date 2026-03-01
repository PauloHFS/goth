package auth

import (
	"context"
	"database/sql"
	"time"

	"github.com/PauloHFS/goth/internal/db"
)

// jobQueueImpl implementação concreta do JobQueue
type jobQueueImpl struct {
	db      *sql.DB
	queries *db.Queries
}

// NewJobQueue cria uma nova instância do JobQueue
func NewJobQueue(dbConn *sql.DB, queries *db.Queries) JobQueue {
	return &jobQueueImpl{
		db:      dbConn,
		queries: queries,
	}
}

// Enqueue adiciona um job à fila
func (q *jobQueueImpl) Enqueue(ctx context.Context, jobType string, payload []byte, tenantID string) error {
	tx, err := q.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	qtx := q.queries.WithTx(tx)

	_, err = qtx.CreateJob(ctx, db.CreateJobParams{
		TenantID: sql.NullString{String: tenantID, Valid: tenantID != ""},
		Type:     jobType,
		Payload:  payload,
		RunAt:    sql.NullTime{Time: time.Now(), Valid: true},
	})
	if err != nil {
		return err
	}

	return tx.Commit()
}
