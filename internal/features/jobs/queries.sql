-- Jobs Feature Queries

-- name: PickNextJob :one
UPDATE jobs
SET status = 'processing', updated_at = CURRENT_TIMESTAMP
WHERE id = (
    SELECT id FROM jobs
    WHERE status = 'pending'
    AND run_at <= CURRENT_TIMESTAMP
    AND (next_retry_at IS NULL OR next_retry_at <= CURRENT_TIMESTAMP)
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

-- name: CreateJob :one
INSERT INTO jobs (tenant_id, type, payload, run_at) VALUES (?, ?, ?, ?) RETURNING *;

-- name: RecordJobProcessed :exec
INSERT INTO processed_jobs (job_id) VALUES (?)
ON CONFLICT(job_id) DO UPDATE SET processed_at = CURRENT_TIMESTAMP;

-- name: IsJobProcessed :one
SELECT EXISTS(SELECT 1 FROM processed_jobs WHERE job_id = ?);

-- name: UpdateJobNextRetry :exec
UPDATE jobs SET next_retry_at = ? WHERE id = ?;
