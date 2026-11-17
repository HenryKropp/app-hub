-- name: CreateExternalLink :one
INSERT INTO external_links (token, artifact_id, version_id, kind, recipient_email, created_by, expires_at)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: GetExternalLink :one
SELECT * FROM external_links WHERE id = $1;

-- name: GetExternalLinkByToken :one
SELECT * FROM external_links WHERE token = $1;

-- name: ListArtifactExternalLinks :many
SELECT l.*, COALESCE(p.name, '')::text AS created_by_name, v.number AS version_number,
       (SELECT count(*) FROM external_link_opens o WHERE o.link_id = l.id)::int AS opens,
       (SELECT max(o.opened_at) FROM external_link_opens o WHERE o.link_id = l.id)::timestamptz AS last_opened_at
FROM external_links l
LEFT JOIN people p ON p.id = l.created_by
LEFT JOIN artifact_versions v ON v.id = l.version_id
WHERE l.artifact_id = $1
ORDER BY l.created_at DESC, l.id DESC;

-- name: ListCompanyExternalLinks :many
SELECT l.*, COALESCE(p.name, '')::text AS created_by_name, v.number AS version_number, a.name AS artifact_name,
       (SELECT count(*) FROM external_link_opens o WHERE o.link_id = l.id)::int AS opens,
       (SELECT max(o.opened_at) FROM external_link_opens o WHERE o.link_id = l.id)::timestamptz AS last_opened_at
FROM external_links l
JOIN artifacts a ON a.id = l.artifact_id
LEFT JOIN people p ON p.id = l.created_by
LEFT JOIN artifact_versions v ON v.id = l.version_id
ORDER BY l.created_at DESC, l.id DESC;

-- name: CancelExternalLink :execrows
UPDATE external_links SET cancelled_at = now(), cancelled_by = $2 WHERE id = $1 AND cancelled_at IS NULL;

-- name: CreateExternalLinkCode :one
INSERT INTO external_link_codes (link_id, code_hash, expires_at) VALUES ($1, $2, $3) RETURNING *;

-- name: ReserveExternalLinkCodeTry :one
-- Takes one of the code's tries before comparing, in one statement, so a burst
-- of simultaneous guesses can never compare more than max_tries times.
UPDATE external_link_codes SET wrong_tries = wrong_tries + 1
WHERE id = (SELECT c.id FROM external_link_codes c WHERE c.link_id = $1 ORDER BY c.id DESC LIMIT 1)
  AND used_at IS NULL AND expires_at > now() AND wrong_tries < sqlc.arg(max_tries)::int
RETURNING id, code_hash, wrong_tries;

-- name: UseExternalLinkCode :execrows
UPDATE external_link_codes SET used_at = now() WHERE id = $1 AND used_at IS NULL AND expires_at > now();

-- name: ExternalLinkCodeUseToday :one
-- What one link spent in the last 24 hours: codes sent and tries taken.
SELECT count(*)::int AS codes, COALESCE(sum(wrong_tries), 0)::int AS tries
FROM external_link_codes WHERE link_id = $1 AND created_at > now() - interval '24 hours';

-- name: CountCompanyLinkCodesToday :one
SELECT count(*)::int FROM external_link_codes WHERE created_at > now() - interval '24 hours';

-- name: CreateExternalLinkSession :exec
INSERT INTO external_link_sessions (link_id, secret_hash, email, expires_at) VALUES ($1, $2, $3, $4);

-- name: GetExternalLinkSession :one
SELECT * FROM external_link_sessions WHERE secret_hash = $1 AND link_id = $2 AND expires_at > now();

-- name: RecordExternalLinkOpen :exec
INSERT INTO external_link_opens (link_id, email) VALUES ($1, $2);

-- name: GetArtifactVersionByID :one
SELECT * FROM artifact_versions WHERE id = $1;
