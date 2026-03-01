-- +goose Up
PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    email TEXT NOT NULL,
    password_hash TEXT NOT NULL,
    role_id TEXT NOT NULL REFERENCES roles(id) ON DELETE RESTRICT,
    is_verified BOOLEAN NOT NULL DEFAULT FALSE,
    avatar_url TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    provider TEXT,
    provider_id TEXT,
    UNIQUE(tenant_id, email)
);

CREATE TABLE IF NOT EXISTS sessions (
    token TEXT PRIMARY KEY,
    data BLOB NOT NULL CHECK(length(data) <= 65536),
    expiry REAL NOT NULL
);

-- Indexes
CREATE INDEX IF NOT EXISTS sessions_expiry_idx ON sessions(expiry);
CREATE INDEX IF NOT EXISTS idx_users_role_id ON users(role_id);
CREATE INDEX IF NOT EXISTS idx_users_provider ON users(provider, provider_id);
CREATE INDEX IF NOT EXISTS idx_users_email_provider ON users(tenant_id, email, provider);

-- +goose Down
DROP INDEX IF EXISTS idx_users_email_provider;
DROP INDEX IF EXISTS idx_users_provider;
DROP INDEX IF EXISTS idx_users_role_id;
DROP INDEX IF EXISTS sessions_expiry_idx;
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS users;
