-- name: GetRole :one
SELECT * FROM roles WHERE id = $1;

-- name: GetRoleByKey :one
SELECT * FROM roles WHERE key = $1;

-- name: ListRoles :many
SELECT r.id, r.key, r.name, r.permissions, (SELECT count(*) FROM people p WHERE p.role_id = r.id) AS people_count
FROM roles r
ORDER BY r.id;

-- name: CreateRole :one
INSERT INTO roles (name, permissions) VALUES ($1, $2) RETURNING *;

-- name: UpdateRole :one
UPDATE roles SET name = $2, permissions = $3 WHERE id = $1 RETURNING *;

-- name: DeleteRole :exec
DELETE FROM roles WHERE id = $1;

-- name: CountManagersExceptPerson :one
SELECT count(*) FROM people p JOIN roles r ON r.id = p.role_id
WHERE r.permissions @> ARRAY['manage_people','manage_roles']::text[] AND p.id <> $1;

-- name: CountManagersExceptRole :one
SELECT count(*) FROM people p JOIN roles r ON r.id = p.role_id
WHERE r.permissions @> ARRAY['manage_people','manage_roles']::text[] AND r.id <> $1;

-- name: ListLedAreaIDs :many
SELECT area_id FROM area_leaders WHERE person_id = $1;

-- name: ListMemberAreaIDs :many
SELECT area_id FROM area_members WHERE person_id = $1;

-- name: ListPersonAppGrants :many
SELECT app_id, permission FROM app_grants WHERE person_id = $1;

-- name: ListPersonAreaGrants :many
SELECT area_id, permission FROM area_grants WHERE person_id = $1;

-- name: ListViewerAppIDs :many
SELECT DISTINCT v.app_id FROM app_viewers v
WHERE v.person_id = sqlc.arg(person_id)
   OR v.area_id IN (SELECT m.area_id FROM area_members m WHERE m.person_id = sqlc.arg(person_id));

-- name: ListAreas :many
SELECT a.id, a.name, (SELECT count(*) FROM apps x WHERE x.area_id = a.id) AS app_count
FROM areas a ORDER BY a.name;

-- name: ListAreaLeaders :many
SELECT l.area_id, p.id, p.name FROM area_leaders l JOIN people p ON p.id = l.person_id ORDER BY p.name;

-- name: ListAreaMembers :many
SELECT m.area_id, p.id, p.name FROM area_members m JOIN people p ON p.id = m.person_id ORDER BY p.name;

-- name: GetArea :one
SELECT * FROM areas WHERE id = $1;

-- name: GetAreaByName :one
SELECT * FROM areas WHERE name = $1;

-- name: CreateArea :one
INSERT INTO areas (name) VALUES ($1) RETURNING *;

-- name: RenameArea :one
UPDATE areas SET name = $2 WHERE id = $1 RETURNING *;

-- name: RenameAppsArea :exec
UPDATE apps SET area = sqlc.arg(name), manifest = jsonb_set(manifest, '{area}', to_jsonb(sqlc.arg(name)::text))
WHERE area_id = sqlc.arg(area_id);

-- name: DeleteArea :exec
DELETE FROM areas WHERE id = $1;

-- name: ClearAreaLeaders :exec
DELETE FROM area_leaders WHERE area_id = $1;

-- name: AddAreaLeader :exec
INSERT INTO area_leaders (area_id, person_id) VALUES ($1, $2) ON CONFLICT DO NOTHING;

-- name: ClearAreaMembers :exec
DELETE FROM area_members WHERE area_id = $1;

-- name: AddAreaMember :exec
INSERT INTO area_members (area_id, person_id) VALUES ($1, $2) ON CONFLICT DO NOTHING;

-- name: SetAppRestricted :exec
UPDATE apps SET restricted = $2 WHERE id = $1;

-- name: ClearAppViewers :exec
DELETE FROM app_viewers WHERE app_id = $1;

-- name: AddAppViewerArea :exec
INSERT INTO app_viewers (app_id, area_id) VALUES ($1, $2) ON CONFLICT DO NOTHING;

-- name: AddAppViewerPerson :exec
INSERT INTO app_viewers (app_id, person_id) VALUES ($1, $2) ON CONFLICT DO NOTHING;

-- name: ListAppViewers :many
SELECT v.area_id, ar.name AS area_name, v.person_id, p.name AS person_name
FROM app_viewers v
LEFT JOIN areas ar ON ar.id = v.area_id
LEFT JOIN people p ON p.id = v.person_id
WHERE v.app_id = $1
ORDER BY ar.name, p.name;

-- name: ListAppGrants :many
SELECT g.person_id, p.name AS person_name, g.permission
FROM app_grants g JOIN people p ON p.id = g.person_id
WHERE g.app_id = $1 ORDER BY p.name, g.permission;

-- name: ClearPersonAppGrants :exec
DELETE FROM app_grants WHERE app_id = $1 AND person_id = $2;

-- name: AddAppGrant :exec
INSERT INTO app_grants (app_id, person_id, permission) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING;

-- name: ListAreaGrants :many
SELECT g.person_id, p.name AS person_name, g.permission
FROM area_grants g JOIN people p ON p.id = g.person_id
WHERE g.area_id = $1 ORDER BY p.name, g.permission;

-- name: ClearPersonAreaGrants :exec
DELETE FROM area_grants WHERE area_id = $1 AND person_id = $2;

-- name: AddAreaGrant :exec
INSERT INTO area_grants (area_id, person_id, permission) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING;
