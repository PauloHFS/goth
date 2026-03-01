-- Auth Feature Queries

-- name: GetUserByEmail :one
SELECT * FROM users WHERE tenant_id = ? AND email = ? LIMIT 1;

-- name: GetUserByID :one
SELECT * FROM users WHERE id = ? LIMIT 1;

-- name: CreateUser :one
INSERT INTO users (tenant_id, email, password_hash, role_id) 
VALUES (?, ?, ?, ?) RETURNING *;

-- name: UpdateUserPassword :exec
UPDATE users SET password_hash = ? WHERE email = ?;

-- name: UpdateUserAvatar :exec
UPDATE users SET avatar_url = ? WHERE id = ?;

-- name: VerifyUser :exec
UPDATE users SET is_verified = TRUE WHERE email = ?;

-- name: GetUserByProvider :one
SELECT * FROM users WHERE tenant_id = ? AND provider = ? AND provider_id = ? LIMIT 1;

-- name: UpdateUserWithOAuth :one
UPDATE users 
SET provider = ?, provider_id = ?, avatar_url = ?, is_verified = ?
WHERE tenant_id = ? AND email = ?
RETURNING *;

-- name: CreateUserWithOAuth :one
INSERT INTO users (tenant_id, email, provider, provider_id, password_hash, role_id, is_verified, avatar_url)
VALUES (?, ?, ?, ?, ?, ?, ?, ?) RETURNING *;

-- Email Verification

-- name: UpsertEmailVerification :exec
INSERT INTO email_verifications (email, token, expires_at) 
VALUES (?, ?, ?)
ON CONFLICT(email) DO UPDATE SET 
    token = excluded.token,
    expires_at = excluded.expires_at;

-- name: GetEmailVerificationByToken :one
SELECT * FROM email_verifications WHERE token = ? LIMIT 1;

-- name: DeleteEmailVerification :exec
DELETE FROM email_verifications WHERE email = ?;

-- Password Reset

-- name: UpsertPasswordReset :exec
INSERT INTO password_resets (email, token_hash, expires_at) 
VALUES (?, ?, ?)
ON CONFLICT(token_hash) DO UPDATE SET 
    email = excluded.email,
    expires_at = excluded.expires_at;

-- name: GetPasswordResetByToken :one
SELECT * FROM password_resets WHERE token_hash = ? LIMIT 1;

-- name: DeletePasswordReset :exec
DELETE FROM password_resets WHERE email = ?;
