-- +goose Up
PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS webhooks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    source TEXT NOT NULL,
    external_id TEXT,
    payload JSON NOT NULL,
    headers JSON NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending' CHECK(status IN ('pending', 'processed', 'failed')),
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS processed_webhook_events (
    event_source TEXT NOT NULL,
    event_id TEXT NOT NULL,
    webhook_id INTEGER REFERENCES webhooks(id) ON DELETE CASCADE,
    processed_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (event_source, event_id)
);

-- Indexes
CREATE INDEX IF NOT EXISTS idx_webhooks_source_status ON webhooks(source, status);
CREATE INDEX IF NOT EXISTS idx_webhooks_external_id ON webhooks(external_id);
CREATE UNIQUE INDEX IF NOT EXISTS udx_webhooks_source_external_id ON webhooks(source, external_id) WHERE external_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_processed_webhook_events_source ON processed_webhook_events(event_source);
CREATE INDEX IF NOT EXISTS idx_processed_webhook_events_processed_at ON processed_webhook_events(processed_at);

-- +goose Down
DROP INDEX IF EXISTS idx_processed_webhook_events_processed_at;
DROP INDEX IF EXISTS idx_processed_webhook_events_source;
DROP INDEX IF EXISTS udx_webhooks_source_external_id;
DROP INDEX IF EXISTS idx_webhooks_external_id;
DROP INDEX IF EXISTS idx_webhooks_source_status;
DROP TABLE IF EXISTS processed_webhook_events;
DROP TABLE IF EXISTS webhooks;
