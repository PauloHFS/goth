package worker

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/PauloHFS/goth/internal/db"
)

// DBWriter gerencia writes serializados no banco (fan-in)
type DBWriter struct {
	db        *sql.DB
	queries   *db.Queries
	logger    *slog.Logger
	writeChan chan writeRequest
	wg        sync.WaitGroup
	ctx       context.Context
	cancel    context.CancelFunc
}

type writeRequest struct {
	kind     writeKind
	jobID    int64
	params   interface{}
	resultCh chan writeResult
}

type writeResult struct {
	err error
}

type writeKind string

const (
	writeCompleteJob        writeKind = "complete"
	writeFailJob            writeKind = "fail"
	writeRecordProcessed    writeKind = "record_processed"
	writeUpdateNextRetry    writeKind = "update_next_retry"
	writeMoveToDLQ          writeKind = "move_to_dlq"
	writeUpdateCircuitState writeKind = "update_circuit_state"
)

// NewDBWriter cria um novo DB writer com fan-in
func NewDBWriter(dbConn *sql.DB, queries *db.Queries, logger *slog.Logger) *DBWriter {
	ctx, cancel := context.WithCancel(context.Background())

	w := &DBWriter{
		db:        dbConn,
		queries:   queries,
		logger:    logger,
		writeChan: make(chan writeRequest, 1000), // Buffer de 1000 writes
		ctx:       ctx,
		cancel:    cancel,
	}

	// Inicia o writer goroutine
	go w.run()

	return w
}

// run é a goroutine principal que processa writes serializados
func (w *DBWriter) run() {
	w.wg.Add(1)
	defer w.wg.Done()

	for {
		select {
		case <-w.ctx.Done():
			return
		case req := <-w.writeChan:
			w.processWrite(req)
		}
	}
}

// processWrite processa um write request
func (w *DBWriter) processWrite(req writeRequest) {
	var err error

	switch req.kind {
	case writeCompleteJob:
		err = w.queries.CompleteJob(w.ctx, req.jobID)

	case writeFailJob:
		params, ok := req.params.(db.FailJobParams)
		if ok {
			err = w.queries.FailJob(w.ctx, params)
		} else {
			err = fmt.Errorf("invalid params for FailJob")
		}

	case writeRecordProcessed:
		err = w.queries.RecordJobProcessed(w.ctx, req.jobID)

	case writeUpdateNextRetry:
		params, ok := req.params.(db.UpdateJobNextRetryParams)
		if ok {
			err = w.queries.UpdateJobNextRetry(w.ctx, params)
		} else {
			err = fmt.Errorf("invalid params for UpdateJobNextRetry")
		}

	case writeMoveToDLQ:
		params, ok := req.params.(db.CreateDeadLetterJobParams)
		if ok {
			_, err = w.queries.CreateDeadLetterJob(w.ctx, params)
		} else {
			err = fmt.Errorf("invalid params for CreateDeadLetterJob")
		}

	case writeUpdateCircuitState:
		params, ok := req.params.(db.UpsertCircuitBreakerStateParams)
		if ok {
			_, err = w.queries.UpsertCircuitBreakerState(w.ctx, params)
		} else {
			err = fmt.Errorf("invalid params for UpsertCircuitBreakerState")
		}
	}

	// Envia resultado
	if req.resultCh != nil {
		req.resultCh <- writeResult{err: err}
	}

	// Log errors
	if err != nil {
		w.logger.Error("db write failed",
			slog.String("kind", string(req.kind)),
			slog.Int64("job_id", req.jobID),
			slog.String("error", err.Error()),
		)
	}
}

// CompleteJob marca um job como completado
func (w *DBWriter) CompleteJob(ctx context.Context, jobID int64) error {
	return w.submitWrite(ctx, writeRequest{
		kind:     writeCompleteJob,
		jobID:    jobID,
		resultCh: make(chan writeResult, 1),
	})
}

// FailJob marca um job como falhado
func (w *DBWriter) FailJob(ctx context.Context, params db.FailJobParams) error {
	return w.submitWrite(ctx, writeRequest{
		kind:     writeFailJob,
		params:   params,
		resultCh: make(chan writeResult, 1),
	})
}

// RecordJobProcessed registra que um job foi processado
func (w *DBWriter) RecordJobProcessed(ctx context.Context, jobID int64) error {
	return w.submitWrite(ctx, writeRequest{
		kind:     writeRecordProcessed,
		jobID:    jobID,
		resultCh: make(chan writeResult, 1),
	})
}

// UpdateJobNextRetry atualiza o próximo retry de um job
func (w *DBWriter) UpdateJobNextRetry(ctx context.Context, params db.UpdateJobNextRetryParams) error {
	return w.submitWrite(ctx, writeRequest{
		kind:     writeUpdateNextRetry,
		params:   params,
		resultCh: make(chan writeResult, 1),
	})
}

// CreateDeadLetterJob move um job para dead letter queue
func (w *DBWriter) CreateDeadLetterJob(ctx context.Context, params db.CreateDeadLetterJobParams) error {
	return w.submitWrite(ctx, writeRequest{
		kind:     writeMoveToDLQ,
		params:   params,
		resultCh: make(chan writeResult, 1),
	})
}

// UpsertCircuitBreakerState atualiza estado do circuit breaker
func (w *DBWriter) UpsertCircuitBreakerState(ctx context.Context, params db.UpsertCircuitBreakerStateParams) error {
	return w.submitWrite(ctx, writeRequest{
		kind:     writeUpdateCircuitState,
		params:   params,
		resultCh: make(chan writeResult, 1),
	})
}

// submitWrite submete um write request e espera resultado
func (w *DBWriter) submitWrite(ctx context.Context, req writeRequest) error {
	// Tenta enviar para o channel
	select {
	case <-ctx.Done():
		return ctx.Err()
	case w.writeChan <- req:
		// Enviado com sucesso, espera resultado
	}

	// Espera resultado com timeout
	select {
	case <-ctx.Done():
		return ctx.Err()
	case result := <-req.resultCh:
		return result.err
	case <-time.After(10 * time.Second):
		return fmt.Errorf("timeout waiting for db write")
	}
}

// QueueLength retorna tamanho da fila de writes
func (w *DBWriter) QueueLength() int {
	return len(w.writeChan)
}

// Stop para o DB writer
func (w *DBWriter) Stop() {
	w.cancel()
	w.wg.Wait()
}

// Stats retorna estatísticas do writer
type DBWriterStats struct {
	QueueLength int
}

func (w *DBWriter) Stats() DBWriterStats {
	return DBWriterStats{
		QueueLength: len(w.writeChan),
	}
}
