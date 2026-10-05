-- name: GetApp :one
SELECT * FROM apps WHERE id = $1;

-- name: ListAppIdentities :many
SELECT id, slug, name, summary, folder_path FROM apps;

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
FROM apps ORDER BY name;
