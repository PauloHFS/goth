-- +goose Up
PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS job_metrics_hourly (
    hour TEXT NOT NULL PRIMARY KEY,
    job_type TEXT NOT NULL,
    total_processed INTEGER DEFAULT 0,
    total_succeeded INTEGER DEFAULT 0,
    total_failed INTEGER DEFAULT 0,
    avg_duration_ms REAL DEFAULT 0,
    min_duration_ms REAL DEFAULT 0,
    max_duration_ms REAL DEFAULT 0,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Indexes
CREATE INDEX IF NOT EXISTS idx_job_metrics_hour ON job_metrics_hourly(hour);
CREATE INDEX IF NOT EXISTS idx_job_metrics_type ON job_metrics_hourly(job_type);

-- +goose Down
DROP INDEX IF EXISTS idx_job_metrics_type;
DROP INDEX IF EXISTS idx_job_metrics_hour;
DROP TABLE IF EXISTS job_metrics_hourly;
