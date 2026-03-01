-- +goose Up
PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS dead_letter_jobs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    job_id INTEGER NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
    original_type TEXT NOT NULL,
    original_payload JSON NOT NULL,
    error_message TEXT NOT NULL,
    attempt_count INTEGER NOT NULL,
    failed_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    tenant_id TEXT,
    metadata JSON
);

-- Indexes
CREATE INDEX IF NOT EXISTS idx_dead_letter_jobs_failed_at ON dead_letter_jobs(failed_at DESC);
CREATE INDEX IF NOT EXISTS idx_dead_letter_jobs_type ON dead_letter_jobs(original_type);
CREATE INDEX IF NOT EXISTS idx_dead_letter_jobs_tenant ON dead_letter_jobs(tenant_id);

-- +goose Down
DROP INDEX IF EXISTS idx_dead_letter_jobs_tenant;
DROP INDEX IF EXISTS idx_dead_letter_jobs_type;
DROP INDEX IF EXISTS idx_dead_letter_jobs_failed_at;
DROP TABLE IF EXISTS dead_letter_jobs;
