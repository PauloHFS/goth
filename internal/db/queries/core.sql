-- name: GetTenantByID :one
SELECT * FROM tenants WHERE id = ? LIMIT 1;

-- name: GetUserByEmail :one
SELECT * FROM users WHERE tenant_id = ? AND email = ? LIMIT 1;

-- name: GetUserByID :one
SELECT * FROM users WHERE id = ? LIMIT 1;

-- name: CreateUser :one
INSERT INTO users (tenant_id, email, password_hash, role_id) 
VALUES (?, ?, ?, ?) RETURNING *;

-- name: CreateJob :one
INSERT INTO jobs (tenant_id, type, payload, run_at) VALUES (?, ?, ?, ?) RETURNING *;

-- name: PickNextJob :one
UPDATE jobs 
SET status = 'processing', updated_at = CURRENT_TIMESTAMP
WHERE id = (
    SELECT id FROM jobs 
    WHERE status = 'pending' AND run_at <= CURRENT_TIMESTAMP 
    ORDER BY run_at ASC LIMIT 1
) RETURNING *;

-- name: CompleteJob :exec
UPDATE jobs 
SET status = 'completed', updated_at = CURRENT_TIMESTAMP 
WHERE id = ?;

-- name: FailJob :exec
UPDATE jobs SET status = 'failed', last_error = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?;

-- name: RescueZombies :exec
UPDATE jobs 
SET status = 'pending', attempt_count = attempt_count + 1 
WHERE status = 'processing' AND updated_at < datetime('now', '-5 minutes');

-- name: RecordJobProcessed :exec
INSERT INTO processed_jobs (job_id) VALUES (?)
ON CONFLICT(job_id) DO UPDATE SET processed_at = CURRENT_TIMESTAMP;

-- name: IsJobProcessed :one
SELECT EXISTS(SELECT 1 FROM processed_jobs WHERE job_id = ?);

-- name: CreateWebhook :one
INSERT INTO webhooks (source, external_id, payload, headers) 
VALUES (?, ?, ?, ?) RETURNING *;

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

-- name: UpdateUserPassword :exec
UPDATE users SET password_hash = ? WHERE email = ?;

-- name: UpdateUserAvatar :exec
UPDATE users SET avatar_url = ? WHERE id = ?;

-- name: ListUsersPaginated :many
SELECT * FROM users 
WHERE tenant_id = ? 
AND (email LIKE '%' || ? || '%' OR ? = '')
ORDER BY created_at DESC 
LIMIT ? OFFSET ?;

-- name: CountUsers :one
SELECT COUNT(*) FROM users WHERE tenant_id = ?;

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

-- name: VerifyUser :exec
UPDATE users SET is_verified = TRUE WHERE email = ?;

-- === DEAD LETTER QUEUE ===

-- name: MoveToDeadLetter :exec
INSERT INTO dead_letter_jobs (original_job_id, tenant_id, type, payload, attempt_count, last_error)
SELECT jobs.id, jobs.tenant_id, jobs.type, jobs.payload, jobs.attempt_count, jobs.last_error FROM jobs WHERE jobs.id = ?;

-- name: DeleteJob :exec
DELETE FROM jobs WHERE id = ?;

-- name: ListDeadLetterJobs :many
SELECT * FROM dead_letter_jobs 
WHERE (?1 = '' OR type = ?1) 
ORDER BY failed_at DESC 
LIMIT ?2 OFFSET ?3;

-- name: GetDeadLetterJob :one
SELECT * FROM dead_letter_jobs WHERE id = ?;

-- name: ReprocessDeadLetterJob :one
INSERT INTO jobs (tenant_id, type, payload, run_at)
SELECT tenant_id, type, payload, CURRENT_TIMESTAMP FROM dead_letter_jobs WHERE dead_letter_jobs.id = ?
RETURNING *;

-- name: DeleteDeadLetterJob :exec
DELETE FROM dead_letter_jobs WHERE id = ?;

-- name: CountDeadLetterJobs :one
SELECT COUNT(*) FROM dead_letter_jobs;

-- name: CountDeadLetterJobsByType :one
SELECT COUNT(*) FROM dead_letter_jobs WHERE type = ?;

-- name: CleanupDeadLetterJobs :exec
DELETE FROM dead_letter_jobs 
WHERE failed_at < datetime('now', '-14 days');

-- === JOBS MANAGEMENT ===

-- name: ListJobs :many
SELECT * FROM jobs 
WHERE (?1 = '' OR status = ?1)
ORDER BY created_at DESC 
LIMIT ?2 OFFSET ?3;

-- name: CountJobs :one
SELECT COUNT(*) FROM jobs WHERE (? = '' OR status = ?);

-- name: CancelJob :exec
UPDATE jobs SET status = 'cancelled', updated_at = CURRENT_TIMESTAMP WHERE id = ? AND status = 'pending';

-- name: RetryJob :exec
UPDATE jobs SET status = 'pending', attempt_count = 0, last_error = NULL, updated_at = CURRENT_TIMESTAMP WHERE id = ?;
