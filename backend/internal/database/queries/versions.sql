-- name: LatestAppVersion :one
SELECT * FROM app_versions WHERE app_id = $1 ORDER BY number DESC LIMIT 1;

-- name: GetAppVersion :one
SELECT v.*, p.name AS author_name
FROM app_versions v LEFT JOIN people p ON p.id = v.author_id
WHERE v.app_id = $1 AND v.number = $2;

-- name: InsertAppVersion :one
INSERT INTO app_versions (app_id, number, origin, restored_from, author_id, author_probable, files, summary, files_changed_at)
VALUES (sqlc.arg(app_id),
        (SELECT COALESCE(MAX(number), 0) + 1 FROM app_versions WHERE app_id = sqlc.arg(app_id)),
        sqlc.arg(origin), sqlc.narg(restored_from), sqlc.narg(author_id), sqlc.arg(author_probable),
        sqlc.arg(files), sqlc.arg(summary), sqlc.narg(files_changed_at))
RETURNING *;

-- name: PutCodeBlob :exec
INSERT INTO code_blobs (sha256, content, size) VALUES ($1, $2, $3) ON CONFLICT (sha256) DO NOTHING;

-- name: GetCodeBlob :one
SELECT content FROM code_blobs WHERE sha256 = $1;

-- name: DescribeAppVersion :exec
UPDATE app_versions
SET description = sqlc.arg(description), author_id = sqlc.narg(author_id), author_probable = false,
    described_by = sqlc.arg(described_by), described_at = now()
WHERE app_id = sqlc.arg(app_id) AND number = sqlc.arg(number);

-- name: ListVersionsBefore :many
SELECT v.number, v.origin, v.restored_from, v.author_probable, v.description, v.summary,
       v.files_changed_at, v.created_at, p.name AS author_name
FROM app_versions v LEFT JOIN people p ON p.id = v.author_id
WHERE v.app_id = sqlc.arg(app_id) AND v.created_at < sqlc.arg(before)
ORDER BY v.created_at DESC
LIMIT sqlc.arg(lim);

-- name: ListRunsBefore :many
SELECT r.id, r.status, r.created_at, p.name AS person_name, v.number AS version_number
FROM runs r
JOIN people p ON p.id = r.person_id
LEFT JOIN app_versions v ON v.id = r.version_id
WHERE r.app_id = sqlc.arg(app_id) AND r.created_at < sqlc.arg(before)
  AND (sqlc.arg(see_all)::boolean OR r.person_id = sqlc.arg(person_id))
ORDER BY r.created_at DESC
LIMIT sqlc.arg(lim);

-- name: InsertAppChange :exec
INSERT INTO app_changes (app_id, person_id, changes) VALUES ($1, $2, $3);

-- name: ListChangesBefore :many
SELECT c.changes, c.created_at, COALESCE(p.name, 'Ziv')::text AS person_name
FROM app_changes c LEFT JOIN people p ON p.id = c.person_id
WHERE c.app_id = sqlc.arg(app_id) AND c.created_at < sqlc.arg(before)
ORDER BY c.created_at DESC
LIMIT sqlc.arg(lim);

-- name: AppHasActiveRun :one
SELECT EXISTS(SELECT 1 FROM runs WHERE app_id = $1 AND status IN ('waiting', 'running'));

-- name: GetAppVersionByID :one
SELECT * FROM app_versions WHERE id = $1;
