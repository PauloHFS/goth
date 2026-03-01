-- +goose Up
PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS jobs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id TEXT REFERENCES tenants(id) ON DELETE SET NULL,
    type TEXT NOT NULL,
    payload JSON NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending' CHECK(status IN ('pending', 'processing', 'completed', 'failed')),
    idempotency_key TEXT UNIQUE,
    attempt_count INTEGER DEFAULT 0 CHECK(attempt_count >= 0),
    max_attempts INTEGER DEFAULT 3 CHECK(max_attempts > 0),
    last_error TEXT,
    run_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    next_retry_at DATETIME,
    worker_id TEXT,
    started_at DATETIME,
    completed_at DATETIME,
    timeout_seconds INTEGER DEFAULT 300,
    priority INTEGER DEFAULT 5,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS processed_jobs (
    job_id INTEGER PRIMARY KEY REFERENCES jobs(id) ON DELETE CASCADE,
    processed_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Indexes
CREATE INDEX IF NOT EXISTS idx_jobs_status_run ON jobs(status, run_at);
CREATE INDEX IF NOT EXISTS idx_jobs_tenant ON jobs(tenant_id);
CREATE INDEX IF NOT EXISTS idx_jobs_idempotency_key ON jobs(idempotency_key) WHERE idempotency_key IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_jobs_next_retry ON jobs(next_retry_at) WHERE next_retry_at IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_jobs_worker_id ON jobs(worker_id) WHERE worker_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_jobs_started_at ON jobs(started_at) WHERE started_at IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_jobs_completed_at ON jobs(completed_at) WHERE completed_at IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_jobs_status_created ON jobs(status, created_at);
CREATE INDEX IF NOT EXISTS idx_jobs_priority_run ON jobs(priority, run_at) WHERE status = 'pending';
CREATE INDEX IF NOT EXISTS idx_processed_jobs_processed_at ON processed_jobs(processed_at);

-- Triggers
DROP TRIGGER IF EXISTS jobs_update_started_at;
-- +goose StatementBegin
CREATE TRIGGER jobs_update_started_at AFTER UPDATE OF status ON jobs
WHEN NEW.status = 'processing' AND OLD.status != 'processing'
BEGIN
    UPDATE jobs SET started_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
END;
-- +goose StatementEnd

DROP TRIGGER IF EXISTS jobs_update_completed_at;
-- +goose StatementBegin
CREATE TRIGGER jobs_update_completed_at AFTER UPDATE OF status ON jobs
WHEN NEW.status IN ('completed', 'failed') AND OLD.status != 'completed'
BEGIN
    UPDATE jobs SET completed_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
END;
-- +goose StatementEnd

-- +goose Down
DROP TRIGGER IF EXISTS jobs_update_completed_at;
DROP TRIGGER IF EXISTS jobs_update_started_at;
DROP INDEX IF EXISTS idx_jobs_priority_run;
DROP INDEX IF EXISTS idx_jobs_status_created;
DROP INDEX IF EXISTS idx_jobs_completed_at;
DROP INDEX IF EXISTS idx_jobs_started_at;
DROP INDEX IF EXISTS idx_jobs_worker_id;
DROP INDEX IF EXISTS idx_jobs_next_retry;
DROP INDEX IF EXISTS idx_jobs_idempotency_key;
DROP INDEX IF EXISTS idx_jobs_tenant;
DROP INDEX IF EXISTS idx_jobs_status_run;
DROP INDEX IF EXISTS idx_processed_jobs_processed_at;
DROP TABLE IF EXISTS processed_jobs;
DROP TABLE IF EXISTS jobs;
