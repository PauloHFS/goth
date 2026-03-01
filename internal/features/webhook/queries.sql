-- Webhook Feature Queries

-- name: CreateWebhook :one
INSERT INTO webhooks (source, external_id, payload, headers) 
VALUES (?, ?, ?, ?) RETURNING *;

-- name: GetWebhookByID :one
SELECT * FROM webhooks WHERE id = ? LIMIT 1;
