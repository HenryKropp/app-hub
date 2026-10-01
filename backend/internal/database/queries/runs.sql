-- name: CreateRun :one
INSERT INTO runs (app_id, person_id, computer_id, output_folder, version_id)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: StartRun :exec
UPDATE runs SET status = 'running', started_at = now(), app_version = $2
WHERE id = $1 AND status = 'waiting';

-- name: FinishRun :exec
UPDATE runs SET status = $2, exit_code = $3, error_message = $4, finished_at = now(),
    started_at = COALESCE(started_at, now())
WHERE id = $1 AND status IN ('waiting', 'running');

-- name: AddRunFile :exec
INSERT INTO run_files (run_id, rel_path, size, modified_at, change, group_index, group_label)
VALUES ($1, $2, $3, $4, $5, $6, $7);

-- name: GetRun :one
SELECT r.*, a.name AS app_name, a.manifest AS app_manifest, p.name AS person_name, c.name AS computer_name,
       v.number AS version_number
FROM runs r
JOIN apps a ON a.id = r.app_id
JOIN people p ON p.id = r.person_id
JOIN computers c ON c.id = r.computer_id
LEFT JOIN app_versions v ON v.id = r.version_id
WHERE r.id = $1;

-- name: ListAppRuns :many
SELECT r.id, r.status, r.created_at, r.started_at, r.finished_at, r.person_id, p.name AS person_name,
       (SELECT count(*) FROM run_files f WHERE f.run_id = r.id) AS file_count
FROM runs r
JOIN people p ON p.id = r.person_id
WHERE r.app_id = sqlc.arg(app_id) AND (sqlc.arg(see_all)::boolean OR r.person_id = sqlc.arg(person_id))
ORDER BY r.id DESC
LIMIT 20;

-- name: ListRunFiles :many
SELECT * FROM run_files WHERE run_id = $1 ORDER BY rel_path;

-- name: GetRunFile :one
SELECT * FROM run_files WHERE id = $1 AND run_id = $2;

-- name: ListActiveRuns :many
SELECT r.id, r.app_id, a.name AS app_name, a.folder_path AS app_folder_path, r.status, r.error_message, r.created_at, r.finished_at,
       (SELECT count(*) FROM run_files f WHERE f.run_id = r.id) AS file_count
FROM runs r
JOIN apps a ON a.id = r.app_id
WHERE r.person_id = $1 AND r.dismissed_at IS NULL
ORDER BY r.id DESC
LIMIT 10;

-- name: DismissRun :execrows
UPDATE runs SET dismissed_at = now()
WHERE id = $1 AND person_id = $2 AND dismissed_at IS NULL;

-- name: ListRuns :many
SELECT r.id, r.app_id, a.name AS app_name, r.status, r.output_folder, r.created_at, r.started_at, r.finished_at,
       r.person_id, p.name AS person_name,
       (SELECT count(*) FROM run_files f WHERE f.run_id = r.id) AS file_count
FROM runs r
JOIN apps a ON a.id = r.app_id
JOIN people p ON p.id = r.person_id
WHERE (sqlc.arg(see_all)::boolean OR r.person_id = sqlc.arg(person_id))
  AND (sqlc.arg(before)::bigint = 0 OR r.id < sqlc.arg(before)::bigint)
ORDER BY r.id DESC
LIMIT sqlc.arg(lim);

-- name: GetRunFolder :one
SELECT folder FROM run_folders WHERE person_id = $1 AND app_id = $2;

-- name: ListRunFolders :many
SELECT app_id, folder FROM run_folders WHERE person_id = $1;

-- name: SetRunFolder :exec
INSERT INTO run_folders (person_id, app_id, folder) VALUES ($1, $2, $3)
ON CONFLICT (person_id, app_id) DO UPDATE SET folder = EXCLUDED.folder, updated_at = now();

-- name: ClearRunFolder :exec
DELETE FROM run_folders WHERE person_id = $1 AND app_id = $2;
