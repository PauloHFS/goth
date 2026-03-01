-- User Feature Queries

-- name: ListUsersPaginated :many
SELECT * FROM users 
WHERE tenant_id = ? 
AND (email LIKE '%' || ? || '%' OR ? = '')
ORDER BY created_at DESC 
LIMIT ? OFFSET ?;

-- name: CountUsers :one
SELECT COUNT(*) FROM users WHERE tenant_id = ?;

-- name: GetUserByID :one
SELECT * FROM users WHERE id = ? LIMIT 1;

-- name: UpdateUserAvatar :exec
UPDATE users SET avatar_url = ? WHERE id = ?;
