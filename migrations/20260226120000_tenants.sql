-- +goose Up
PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS tenants (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    settings JSON NOT NULL DEFAULT '{}',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Seed
INSERT OR IGNORE INTO tenants (id, name, settings) VALUES ('default', 'Default Tenant', '{}');

-- +goose Down
DROP TABLE IF EXISTS tenants;
