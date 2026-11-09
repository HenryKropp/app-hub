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
SELECT * FROM mcp_connections WHERE person_id = $1 AND revoked_at IS NULL
  AND (refresh_expires_at IS NULL OR refresh_expires_at > now()) ORDER BY id DESC;

-- name: RevokeMCPConnection :execrows
UPDATE mcp_connections SET revoked_at = now()
WHERE id = $1 AND person_id = $2 AND revoked_at IS NULL;

-- name: CreateMCPClient :one
INSERT INTO mcp_clients (id, name, redirect_uris) VALUES ($1, $2, $3)
RETURNING *;

-- name: GetMCPClient :one
SELECT * FROM mcp_clients WHERE id = $1;

-- name: DeleteStaleMCPAuthRequests :exec
DELETE FROM mcp_auth_requests
WHERE expires_at < now() AND (code_expires_at IS NULL OR code_expires_at < now());

-- name: CreateMCPAuthRequest :exec
INSERT INTO mcp_auth_requests (id, client_id, redirect_uri, code_challenge, state, expires_at)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: GetPendingMCPAuthRequest :one
SELECT r.*, c.name AS client_name
FROM mcp_auth_requests r JOIN mcp_clients c ON c.id = r.client_id
WHERE r.id = $1 AND r.person_id IS NULL AND r.code_hash IS NULL AND r.expires_at > now();

-- name: ApproveMCPAuthRequest :one
UPDATE mcp_auth_requests
SET person_id = $2, code_hash = $3, code_expires_at = $4
WHERE id = $1 AND person_id IS NULL AND code_hash IS NULL AND expires_at > now()
RETURNING *;

-- name: DenyMCPAuthRequest :one
DELETE FROM mcp_auth_requests
WHERE id = $1 AND person_id IS NULL AND code_hash IS NULL AND expires_at > now()
RETURNING *;

-- name: ConsumeMCPAuthCode :one
DELETE FROM mcp_auth_requests WHERE code_hash = $1
RETURNING *;

-- name: GetMCPConnectionByRefreshHash :one
SELECT * FROM mcp_connections WHERE refresh_hash = $1;

-- name: RotateMCPConnectionTokens :execrows
UPDATE mcp_connections
SET token_hash = $3, access_expires_at = $4, refresh_hash = $5, refresh_expires_at = $6
WHERE id = $1 AND refresh_hash = $2 AND revoked_at IS NULL;

-- name: CreateMCPUpload :one
INSERT INTO mcp_uploads (token_hash, person_id, expires_at) VALUES ($1, $2, $3)
RETURNING id;

-- name: CountOpenMCPUploads :one
SELECT count(*) FROM mcp_uploads WHERE person_id = $1 AND expires_at > now() AND used_at IS NULL;

-- name: ReceiveMCPUpload :one
UPDATE mcp_uploads SET content = sqlc.arg(content), size = sqlc.arg(size), received_at = now()
WHERE token_hash = sqlc.arg(token_hash) AND received_at IS NULL AND expires_at > now()
RETURNING id;

-- name: MCPUploadOpen :one
SELECT id FROM mcp_uploads WHERE token_hash = $1 AND received_at IS NULL AND expires_at > now();

-- name: GetMCPUpload :one
SELECT * FROM mcp_uploads WHERE id = $1 AND person_id = $2;

-- name: UseMCPUpload :exec
UPDATE mcp_uploads SET used_at = now(), content = NULL WHERE id = $1;

-- name: DeleteExpiredMCPUploads :exec
DELETE FROM mcp_uploads WHERE expires_at < now() - interval '1 day' OR used_at < now() - interval '1 day';
