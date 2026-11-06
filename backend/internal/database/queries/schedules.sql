-- name: CreateSchedule :one
INSERT INTO schedules (app_id, person_id, computer_id, name, frequency, inputs, files, output_folder, next_run_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: UpdateSchedule :one
UPDATE schedules SET computer_id = $2, name = $3, frequency = $4, inputs = $5, files = $6, output_folder = $7,
    next_run_at = $8, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: GetSchedule :one
SELECT * FROM schedules WHERE id = $1;

-- name: ListAppSchedules :many
SELECT s.*, p.name AS person_name, c.name AS computer_name, a.name AS app_name,
       COALESCE(r.status, '') AS last_run_status
FROM schedules s
JOIN people p ON p.id = s.person_id
JOIN computers c ON c.id = s.computer_id
JOIN apps a ON a.id = s.app_id
LEFT JOIN runs r ON r.id = s.last_run_id
WHERE s.app_id = $1
ORDER BY s.created_at;

-- name: ListPersonSchedules :many
SELECT s.*, p.name AS person_name, c.name AS computer_name, a.name AS app_name,
       COALESCE(r.status, '') AS last_run_status
FROM schedules s
JOIN people p ON p.id = s.person_id
JOIN computers c ON c.id = s.computer_id
JOIN apps a ON a.id = s.app_id
LEFT JOIN runs r ON r.id = s.last_run_id
WHERE s.person_id = $1
ORDER BY s.created_at;

-- name: DeleteSchedule :exec
DELETE FROM schedules WHERE id = $1;

-- name: SetScheduleActive :exec
UPDATE schedules SET active = $2, paused_reason = $3, updated_at = now() WHERE id = $1;

-- name: ResumeSchedule :exec
UPDATE schedules SET active = true, paused_reason = '', situation = '', failures = 0, next_run_at = $2, updated_at = now()
WHERE id = $1;

-- name: DueSchedules :many
SELECT * FROM schedules WHERE active AND next_run_at <= $1 ORDER BY next_run_at LIMIT 50;

-- name: ClaimSchedule :execrows
UPDATE schedules SET next_run_at = sqlc.arg(new_next) WHERE id = sqlc.arg(id) AND active AND next_run_at = sqlc.arg(old_next);

-- name: SetScheduleSituation :exec
UPDATE schedules SET situation = $2 WHERE id = $1;

-- name: RecordScheduleLaunch :exec
UPDATE schedules SET last_run_id = $2, last_run_at = now(), situation = '' WHERE id = $1;

-- name: RecordScheduleOutcome :one
-- failed=true adds one failure; false clears the count.
UPDATE schedules SET failures = CASE WHEN sqlc.arg(failed)::boolean THEN failures + 1 ELSE 0 END
WHERE id = sqlc.arg(id)
RETURNING failures, active;

-- name: ListMyPausedSchedules :many
SELECT s.id, s.app_id, a.name AS app_name, s.paused_reason, s.updated_at
FROM schedules s JOIN apps a ON a.id = s.app_id
WHERE s.person_id = $1 AND NOT s.active AND s.paused_reason <> ''
ORDER BY s.updated_at DESC
LIMIT 10;

-- name: ScheduleIDForRun :one
SELECT schedule_id FROM runs WHERE id = $1;

-- name: GetScheduleRow :one
SELECT s.*, p.name AS person_name, c.name AS computer_name, a.name AS app_name,
       COALESCE(r.status, '') AS last_run_status
FROM schedules s
JOIN people p ON p.id = s.person_id
JOIN computers c ON c.id = s.computer_id
JOIN apps a ON a.id = s.app_id
LEFT JOIN runs r ON r.id = s.last_run_id
WHERE s.id = $1;
