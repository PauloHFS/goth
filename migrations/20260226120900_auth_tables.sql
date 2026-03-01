-- +goose Up
PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS email_verifications (
    email TEXT NOT NULL PRIMARY KEY,
    token TEXT NOT NULL,
    expires_at DATETIME NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS password_resets (
    email TEXT NOT NULL,
    token_hash TEXT NOT NULL PRIMARY KEY,
    expires_at DATETIME NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Indexes
CREATE INDEX IF NOT EXISTS idx_email_verifications_expires_at ON email_verifications(expires_at);
CREATE INDEX IF NOT EXISTS idx_password_resets_email ON password_resets(email);
CREATE INDEX IF NOT EXISTS idx_password_resets_expires_at ON password_resets(expires_at);

-- +goose Down
DROP INDEX IF EXISTS idx_password_resets_expires_at;
DROP INDEX IF EXISTS idx_password_resets_email;
DROP INDEX IF EXISTS idx_email_verifications_expires_at;
DROP TABLE IF EXISTS password_resets;
DROP TABLE IF EXISTS email_verifications;
