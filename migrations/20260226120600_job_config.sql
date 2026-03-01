-- +goose Up
PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS job_type_configs (
    job_type TEXT PRIMARY KEY,
    timeout_seconds INTEGER NOT NULL DEFAULT 300,
    max_attempts INTEGER NOT NULL DEFAULT 5,
    backoff_base_seconds INTEGER NOT NULL DEFAULT 30,
    backoff_max_seconds INTEGER NOT NULL DEFAULT 600,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Seed
INSERT OR IGNORE INTO job_type_configs (job_type, timeout_seconds, max_attempts, backoff_base_seconds, backoff_max_seconds)
VALUES
    ('send_email', 60, 5, 30, 600),
    ('send_password_reset_email', 60, 5, 30, 600),
    ('send_verification_email', 60, 5, 30, 600),
    ('process_ai', 120, 3, 60, 300),
    ('process_webhook', 300, 5, 30, 600),
    ('process_asaas_webhook', 300, 5, 30, 600);

-- +goose Down
DROP TABLE IF EXISTS job_type_configs;
