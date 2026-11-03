-- name: CreateArtifact :one
INSERT INTO artifacts (name, summary, kind, tags, area_id, owner_id, backup_id, restricted, origin, created_by)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING *;

-- name: GetArtifact :one
SELECT * FROM artifacts WHERE id = $1;

-- name: ListArtifacts :many
SELECT a.id, a.name, a.summary, a.kind, a.tags, a.area_id, ar.name AS area_name,
       a.owner_id, o.name AS owner_name, a.backup_id, a.restricted, a.origin,
       a.created_by, a.created_at, a.updated_at,
       COALESCE((SELECT max(v.number) FROM artifact_versions v WHERE v.artifact_id = a.id), 0)::int AS current_version
FROM artifacts a
JOIN areas ar ON ar.id = a.area_id
JOIN people o ON o.id = a.owner_id
ORDER BY a.name;

-- name: UpdateArtifactFicha :one
UPDATE artifacts SET name = $2, summary = $3, tags = $4, updated_at = now() WHERE id = $1 RETURNING *;

-- name: UpdateArtifactCare :one
UPDATE artifacts SET owner_id = $2, backup_id = $3, area_id = $4, updated_at = now() WHERE id = $1 RETURNING *;

-- name: LockArtifact :exec
SELECT id FROM artifacts WHERE id = $1 FOR UPDATE;

-- name: SetArtifactKind :exec
UPDATE artifacts SET kind = $2, updated_at = now() WHERE id = $1;

-- name: TouchArtifact :exec
UPDATE artifacts SET updated_at = now() WHERE id = $1;

-- name: DeleteArtifact :exec
DELETE FROM artifacts WHERE id = $1;

-- name: CreateArtifactVersion :one
INSERT INTO artifact_versions (artifact_id, number, kind, url, blob_sha256, file_name, size, note, restored_from, author_id, embeddable)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
RETURNING *;

-- name: ListArtifactVersions :many
SELECT v.id, v.artifact_id, v.number, v.kind, v.url, v.blob_sha256, v.file_name, v.size, v.note,
       v.restored_from, v.author_id, p.name AS author_name, v.created_at, v.embeddable
FROM artifact_versions v
LEFT JOIN people p ON p.id = v.author_id
WHERE v.artifact_id = $1
ORDER BY v.number DESC;

-- name: GetArtifactVersion :one
SELECT * FROM artifact_versions WHERE artifact_id = $1 AND number = $2;

-- name: LatestArtifactVersion :one
SELECT * FROM artifact_versions WHERE artifact_id = $1 ORDER BY number DESC LIMIT 1;

-- name: SetArtifactRestricted :exec
UPDATE artifacts SET restricted = $2 WHERE id = $1;

-- name: ClearArtifactViewers :exec
DELETE FROM artifact_viewers WHERE artifact_id = $1;

-- name: AddArtifactViewerArea :exec
INSERT INTO artifact_viewers (artifact_id, area_id) VALUES ($1, $2) ON CONFLICT DO NOTHING;

-- name: AddArtifactViewerPerson :exec
INSERT INTO artifact_viewers (artifact_id, person_id) VALUES ($1, $2) ON CONFLICT DO NOTHING;

-- name: ListArtifactViewers :many
SELECT v.area_id, ar.name AS area_name, v.person_id, p.name AS person_name
FROM artifact_viewers v
LEFT JOIN areas ar ON ar.id = v.area_id
LEFT JOIN people p ON p.id = v.person_id
WHERE v.artifact_id = $1
ORDER BY ar.name, p.name;

-- name: ListArtifactGrants :many
SELECT g.person_id, p.name AS person_name, g.permission
FROM artifact_grants g JOIN people p ON p.id = g.person_id
WHERE g.artifact_id = $1 ORDER BY p.name, g.permission;

-- name: ClearPersonArtifactGrants :exec
DELETE FROM artifact_grants WHERE artifact_id = $1 AND person_id = $2;

-- name: AddArtifactGrant :exec
INSERT INTO artifact_grants (artifact_id, person_id, permission) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING;

-- name: ListPersonArtifactGrants :many
SELECT artifact_id, permission FROM artifact_grants WHERE person_id = $1;

-- name: ListViewerArtifactIDs :many
SELECT DISTINCT v.artifact_id FROM artifact_viewers v
WHERE v.person_id = sqlc.arg(person_id)
   OR v.area_id IN (SELECT m.area_id FROM area_members m WHERE m.person_id = sqlc.arg(person_id));

-- name: DisableAppApprovalByDefault :exec
UPDATE security_settings SET require_app_approval = FALSE WHERE id;
