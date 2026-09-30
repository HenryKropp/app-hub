-- name: CreateRun :one
INSERT INTO runs (app_id, person_id, computer_id, output_folder)
VALUES ($1, $2, $3, $4)
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
SELECT r.*, a.name AS app_name, a.manifest AS app_manifest, p.name AS person_name, c.name AS computer_name
FROM runs r
JOIN apps a ON a.id = r.app_id
JOIN people p ON p.id = r.person_id
JOIN computers c ON c.id = r.computer_id
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
SELECT * FROM run_files WHERE run_id = $1 ORDER BY group_index < 0, group_index, rel_path;

-- name: GetRunFile :one
SELECT * FROM run_files WHERE id = $1 AND run_id = $2;
