-- name: CreateMCPConnection :one
INSERT INTO mcp_connections (person_id, kind, name, client_id, token_hash, access_expires_at, refresh_hash, refresh_expires_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: GetMCPConnectionByTokenHash :one
SELECT * FROM mcp_connections WHERE token_hash = $1;

-- name: TouchMCPConnection :exec
UPDATE mcp_connections SET last_used_at = now()
WHERE id = $1 AND (last_used_at IS NULL OR last_used_at < now() - interval '1 minute');

-- name: ListMCPConnections :many
SELECT * FROM mcp_connections WHERE person_id = $1 AND revoked_at IS NULL ORDER BY id DESC;

-- name: RevokeMCPConnection :execrows
UPDATE mcp_connections SET revoked_at = now()
WHERE id = $1 AND person_id = $2 AND revoked_at IS NULL;
