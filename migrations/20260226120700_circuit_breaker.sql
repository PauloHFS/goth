-- +goose Up
PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS circuit_breaker_states (
    job_type TEXT PRIMARY KEY,
    state TEXT NOT NULL DEFAULT 'closed',
    failure_count INTEGER NOT NULL DEFAULT 0,
    success_count INTEGER NOT NULL DEFAULT 0,
    last_failure_at DATETIME,
    last_success_at DATETIME,
    opened_at DATETIME,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Seed
INSERT OR IGNORE INTO circuit_breaker_states (job_type, state)
VALUES
    ('send_email', 'closed'),
    ('send_password_reset_email', 'closed'),
    ('send_verification_email', 'closed'),
    ('process_ai', 'closed'),
    ('process_webhook', 'closed'),
    ('process_asaas_webhook', 'closed');

-- +goose Down
DROP TABLE IF EXISTS circuit_breaker_states;
