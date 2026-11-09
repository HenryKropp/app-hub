-- name: GetApp :one
SELECT * FROM apps WHERE id = $1;

-- name: ListAppIdentities :many
SELECT id, slug, name, summary, folder_path, archived_at FROM apps;

-- name: SlugExists :one
SELECT EXISTS(SELECT 1 FROM apps WHERE slug = $1 AND folder_path <> $2);

-- name: GetAppByFolder :one
SELECT * FROM apps WHERE folder_path = $1;

-- name: ListApps :many
SELECT a.id, a.slug, a.name, a.summary, a.kind, a.language, a.area, a.area_id, a.restricted, a.folder_path,
       a.owner_id, o.name AS owner_name, a.backup_id, a.created_at, a.updated_at,
       COALESCE(a.manifest->>'entrypoint', '')::text AS entrypoint
FROM apps a
JOIN people o ON o.id = a.owner_id
WHERE a.archived_at IS NULL
ORDER BY a.name;

-- name: UpsertApp :one
INSERT INTO apps (slug, name, summary, kind, language, area, area_id, folder_path, owner_id, backup_id, manifest, created_by)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
ON CONFLICT (folder_path) DO UPDATE SET
    name = EXCLUDED.name,
    summary = EXCLUDED.summary,
    kind = EXCLUDED.kind,
    language = EXCLUDED.language,
    area = EXCLUDED.area,
    area_id = EXCLUDED.area_id,
    owner_id = EXCLUDED.owner_id,
    backup_id = EXCLUDED.backup_id,
    manifest = EXCLUDED.manifest,
    updated_at = now()
RETURNING *;

-- name: ListAppLinks :many
SELECT id, slug, name, area_id, owner_id, backup_id, restricted,
       COALESCE(manifest->'depends_on', '[]'::jsonb) AS depends_on
FROM apps WHERE archived_at IS NULL ORDER BY name;

-- name: ListArchivedApps :many
SELECT a.id, a.name, a.summary, a.kind, a.area, a.area_id, a.restricted, a.owner_id, a.backup_id,
       o.name AS owner_name, a.archived_at::timestamptz AS archived_at,
       COALESCE(b.name, '')::text AS archived_by_name
FROM apps a
JOIN people o ON o.id = a.owner_id
LEFT JOIN people b ON b.id = a.archived_by
WHERE a.archived_at IS NOT NULL
ORDER BY a.archived_at DESC;

-- name: ArchiveApp :one
UPDATE apps SET archived_at = now(), archived_by = $2
WHERE id = $1 AND archived_at IS NULL
RETURNING *;

-- name: UnarchiveApp :one
UPDATE apps SET archived_at = NULL, archived_by = NULL
WHERE id = $1 AND archived_at IS NOT NULL
RETURNING *;

-- name: ListArchivedBefore :many
SELECT id FROM apps WHERE archived_at IS NOT NULL AND archived_at < $1 ORDER BY id;

-- name: PurgeArchivedApp :execrows
-- Deletes the app for good (everything that hangs on it goes by cascade);
-- only while it is still archived since before the cutoff.
DELETE FROM apps WHERE id = $1 AND archived_at IS NOT NULL AND archived_at < $2;

-- name: AppArchived :one
SELECT (archived_at IS NOT NULL)::bool AS archived FROM apps WHERE id = $1;
