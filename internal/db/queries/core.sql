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
INSERT INTO jobs (tenant_id, type, payload, run_at, idempotency_key) 
VALUES (?, ?, ?, ?, ?) 
RETURNING *;

-- name: CreateJobWithIdempotency :one
INSERT INTO jobs (tenant_id, type, payload, run_at, idempotency_key) 
VALUES (?, ?, ?, ?, ?) 
ON CONFLICT(idempotency_key) DO UPDATE SET 
    updated_at = CURRENT_TIMESTAMP
RETURNING *;

-- name: PickNextJob :one
UPDATE jobs
SET status = 'processing', updated_at = CURRENT_TIMESTAMP, started_at = CURRENT_TIMESTAMP
WHERE id = (
    SELECT id FROM jobs
    WHERE status = 'pending'
    AND run_at <= CURRENT_TIMESTAMP
    AND (next_retry_at IS NULL OR next_retry_at <= CURRENT_TIMESTAMP)
    ORDER BY priority ASC, run_at ASC LIMIT 1
) RETURNING *;

-- name: PickNextJobWithWorker :one
UPDATE jobs
SET status = 'processing', updated_at = CURRENT_TIMESTAMP, started_at = CURRENT_TIMESTAMP, worker_id = ?
WHERE id = (
    SELECT id FROM jobs
    WHERE status = 'pending'
    AND run_at <= CURRENT_TIMESTAMP
    AND (next_retry_at IS NULL OR next_retry_at <= CURRENT_TIMESTAMP)
    ORDER BY priority ASC, run_at ASC LIMIT 1
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

-- name: GetJobByID :one
SELECT * FROM jobs WHERE id = ? LIMIT 1;

-- name: CreateWebhook :one
INSERT INTO webhooks (source, external_id, payload, headers)
VALUES (?, ?, ?, ?) RETURNING *;

-- name: CreateWebhookWithIdempotency :one
INSERT INTO webhooks (source, external_id, payload, headers, status)
VALUES (?, ?, ?, ?, 'pending')
ON CONFLICT(source, external_id) DO UPDATE SET
    payload = excluded.payload,
    headers = excluded.headers,
    updated_at = CURRENT_TIMESTAMP
RETURNING *;

-- name: GetWebhookByID :one
SELECT * FROM webhooks WHERE id = ? LIMIT 1;

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

-- Asaas Customers

-- name: CreateAsaasCustomer :one
INSERT INTO asaas_customers (id, tenant_id, user_id, email, cpf_cnpj, name, asaas_data)
VALUES (?, ?, ?, ?, ?, ?, ?) RETURNING *;

-- name: GetAsaasCustomerByID :one
SELECT * FROM asaas_customers WHERE id = ? LIMIT 1;

-- name: GetAsaasCustomerByEmail :one
SELECT * FROM asaas_customers WHERE tenant_id = ? AND email = ? LIMIT 1;

-- name: UpdateAsaasCustomer :one
UPDATE asaas_customers SET cpf_cnpj = ?, name = ?, asaas_data = ?, updated_at = CURRENT_TIMESTAMP
WHERE id = ? RETURNING *;

-- Asaas Payments

-- name: CreateAsaasPayment :one
INSERT INTO asaas_payments (id, tenant_id, customer_id, user_id, amount, billing_type, status, due_date, invoice_url, invoice_id, pix_qr_code, pix_copy_paste, asaas_response)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) RETURNING *;

-- name: GetAsaasPaymentByID :one
SELECT * FROM asaas_payments WHERE id = ? LIMIT 1;

-- name: UpdateAsaasPaymentStatus :one
UPDATE asaas_payments SET status = ?, payment_date = ?, updated_at = CURRENT_TIMESTAMP
WHERE id = ? RETURNING *;

-- name: ListAsaasPaymentsByCustomer :many
SELECT * FROM asaas_payments WHERE customer_id = ? ORDER BY created_at DESC;

-- name: ListAsaasPaymentsByUser :many
SELECT * FROM asaas_payments WHERE user_id = ? ORDER BY created_at DESC;

-- Asaas Subscriptions

-- name: CreateAsaasSubscription :one
INSERT INTO asaas_subscriptions (id, tenant_id, customer_id, user_id, plan_value, billing_type, cycle_type, status, next_billing_date, asaas_response)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?) RETURNING *;

-- name: GetAsaasSubscriptionByID :one
SELECT * FROM asaas_subscriptions WHERE id = ? LIMIT 1;

-- name: UpdateAsaasSubscription :one
UPDATE asaas_subscriptions SET status = ?, next_billing_date = ?, asaas_response = ?, updated_at = CURRENT_TIMESTAMP
WHERE id = ? RETURNING *;

-- name: ListAsaasSubscriptionsByCustomer :many
SELECT * FROM asaas_subscriptions WHERE customer_id = ? ORDER BY created_at DESC;

-- name: ListAsaasSubscriptionsByUser :many
SELECT * FROM asaas_subscriptions WHERE user_id = ? ORDER BY created_at DESC;

-- name: GetActiveSubscriptionByUser :one
SELECT * FROM asaas_subscriptions WHERE user_id = ? AND status = 'ACTIVE' LIMIT 1;

-- OAuth2 Queries

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

-- name: UpdateJobNextRetry :exec
UPDATE jobs SET next_retry_at = ? WHERE id = ?;

-- Processed Webhook Events (Idempotency)

-- name: CreateProcessedWebhookEvent :exec
INSERT INTO processed_webhook_events (event_source, event_id, webhook_id)
VALUES (?, ?, ?)
ON CONFLICT(event_source, event_id) DO UPDATE SET
    processed_at = CURRENT_TIMESTAMP;

-- name: IsWebhookEventProcessed :one
SELECT EXISTS(SELECT 1 FROM processed_webhook_events WHERE event_source = ? AND event_id = ?);

-- name: GetProcessedWebhookEvent :one
SELECT * FROM processed_webhook_events WHERE event_source = ? AND event_id = ? LIMIT 1;

-- Dead Letter Queue

-- name: CreateDeadLetterJob :one
INSERT INTO dead_letter_jobs (job_id, original_type, original_payload, error_message, attempt_count, tenant_id, metadata)
VALUES (?, ?, ?, ?, ?, ?, ?) RETURNING *;

-- name: GetDeadLetterJobs :many
SELECT * FROM dead_letter_jobs ORDER BY failed_at DESC LIMIT ? OFFSET ?;

-- name: GetDeadLetterJobCount :one
SELECT COUNT(*) FROM dead_letter_jobs;

-- name: DeleteDeadLetterJob :exec
DELETE FROM dead_letter_jobs WHERE id = ?;

-- name: RetryDeadLetterJob :one
INSERT INTO jobs (tenant_id, type, payload, status, run_at, attempt_count, timeout_seconds)
VALUES (?, ?, ?, 'pending', ?, ?, ?) RETURNING *;

-- Job Type Configs

-- name: GetJobTypeConfig :one
SELECT * FROM job_type_configs WHERE job_type = ?;

-- name: UpsertJobTypeConfig :one
INSERT INTO job_type_configs (job_type, timeout_seconds, max_attempts, backoff_base_seconds, backoff_max_seconds, enabled)
VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT(job_type) DO UPDATE SET
    timeout_seconds = excluded.timeout_seconds,
    max_attempts = excluded.max_attempts,
    backoff_base_seconds = excluded.backoff_base_seconds,
    backoff_max_seconds = excluded.backoff_max_seconds,
    enabled = excluded.enabled,
    updated_at = CURRENT_TIMESTAMP
RETURNING *;

-- name: ListJobTypeConfigs :many
SELECT * FROM job_type_configs ORDER BY job_type;

-- Job Metrics Hourly

-- name: UpsertJobMetricsHourly :exec
INSERT INTO job_metrics_hourly (hour, job_type, total_processed, total_succeeded, total_failed, avg_duration_ms, min_duration_ms, max_duration_ms)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(hour, job_type) DO UPDATE SET
    total_processed = job_metrics_hourly.total_processed + excluded.total_processed,
    total_succeeded = job_metrics_hourly.total_succeeded + excluded.total_succeeded,
    total_failed = job_metrics_hourly.total_failed + excluded.total_failed,
    avg_duration_ms = (job_metrics_hourly.avg_duration_ms * job_metrics_hourly.total_processed + excluded.avg_duration_ms) / (job_metrics_hourly.total_processed + 1),
    min_duration_ms = MIN(job_metrics_hourly.min_duration_ms, excluded.min_duration_ms),
    max_duration_ms = MAX(job_metrics_hourly.max_duration_ms, excluded.max_duration_ms),
    updated_at = CURRENT_TIMESTAMP;

-- name: GetJobMetricsHourly :many
SELECT * FROM job_metrics_hourly WHERE hour >= ? ORDER BY hour DESC LIMIT ?;

-- Job Timeout Management

-- name: GetStaleProcessingJobs :many
SELECT * FROM jobs 
WHERE status = 'processing' 
AND started_at < datetime('now', '-' || CAST(? AS TEXT) || ' seconds')
ORDER BY started_at ASC;

-- name: TimeoutJob :exec
UPDATE jobs 
SET status = 'failed', 
    last_error = 'Job timed out',
    updated_at = CURRENT_TIMESTAMP,
    completed_at = CURRENT_TIMESTAMP
WHERE id = ?;

-- Job Notification (para otimizar polling)

-- name: GetPendingJobCount :one
SELECT COUNT(*) FROM jobs WHERE status = 'pending' AND run_at <= CURRENT_TIMESTAMP;

-- Circuit Breaker

-- name: GetCircuitBreakerState :one
SELECT * FROM circuit_breaker_states WHERE job_type = ?;

-- name: UpsertCircuitBreakerState :one
INSERT INTO circuit_breaker_states (job_type, state, failure_count, success_count, last_failure_at, last_success_at, opened_at)
VALUES (?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(job_type) DO UPDATE SET
    state = excluded.state,
    failure_count = excluded.failure_count,
    success_count = excluded.success_count,
    last_failure_at = excluded.last_failure_at,
    last_success_at = excluded.last_success_at,
    opened_at = excluded.opened_at,
    updated_at = CURRENT_TIMESTAMP
RETURNING *;

-- name: UpdateCircuitBreakerFailure :exec
UPDATE circuit_breaker_states SET
    failure_count = failure_count + 1,
    last_failure_at = CURRENT_TIMESTAMP,
    updated_at = CURRENT_TIMESTAMP
WHERE job_type = ?;

-- name: UpdateCircuitBreakerSuccess :exec
UPDATE circuit_breaker_states SET
    failure_count = 0,
    success_count = success_count + 1,
    last_success_at = CURRENT_TIMESTAMP,
    state = 'closed',
    updated_at = CURRENT_TIMESTAMP
WHERE job_type = ?;

-- name: UpdateCircuitBreakerOpen :exec
UPDATE circuit_breaker_states SET
    state = 'open',
    opened_at = CURRENT_TIMESTAMP,
    updated_at = CURRENT_TIMESTAMP
WHERE job_type = ?;

-- name: ListCircuitBreakerStates :many
SELECT * FROM circuit_breaker_states ORDER BY job_type;
