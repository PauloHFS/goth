-- +goose Up
PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS roles (
    id TEXT PRIMARY KEY,
    permissions JSON NOT NULL
);

-- Seed
INSERT OR IGNORE INTO roles (id, permissions) VALUES ('admin', '["read", "write", "delete"]');
INSERT OR IGNORE INTO roles (id, permissions) VALUES ('user', '["read", "write"]');

-- +goose Down
DROP TABLE IF EXISTS roles;
