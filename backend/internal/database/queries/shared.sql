-- name: GetSharedApp :one
SELECT * FROM shared_apps WHERE app_id = $1;

-- name: GetSharedAppBySlug :one
SELECT * FROM shared_apps WHERE slug = $1 AND unshared_at IS NULL;

-- name: ListSharedApps :many
SELECT s.*, a.name AS app_name, a.owner_id, a.backup_id
FROM shared_apps s JOIN apps a ON a.id = s.app_id
WHERE s.unshared_at IS NULL
ORDER BY a.name;

-- name: UpsertSharedApp :one
INSERT INTO shared_apps (app_id, slug, always_on, idle_minutes, memory_mb, cpus, health_path, port, wants_db)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
ON CONFLICT (app_id) DO UPDATE SET
    always_on = EXCLUDED.always_on, idle_minutes = EXCLUDED.idle_minutes, memory_mb = EXCLUDED.memory_mb,
    cpus = EXCLUDED.cpus, health_path = EXCLUDED.health_path, port = EXCLUDED.port,
    -- never off while the app has a database
    wants_db = EXCLUDED.wants_db OR shared_apps.db_name <> '',
    -- shared again after "Deixar de compartilhar": starts over as a draft
    status = CASE WHEN shared_apps.unshared_at IS NULL THEN shared_apps.status ELSE 'draft' END,
    status_message = CASE WHEN shared_apps.unshared_at IS NULL THEN shared_apps.status_message ELSE '' END,
    stopped_by = CASE WHEN shared_apps.unshared_at IS NULL THEN shared_apps.stopped_by END,
    unshared_at = NULL
RETURNING *;

-- name: SetSharedStatus :exec
UPDATE shared_apps
SET status = sqlc.arg(status), status_message = sqlc.arg(status_message), status_changed_at = now(),
    stopped_by = sqlc.narg(stopped_by)
WHERE app_id = sqlc.arg(app_id);

-- name: SetSharedDeployments :exec
UPDATE shared_apps SET current_deployment_id = sqlc.narg(current_id), previous_deployment_id = sqlc.narg(previous_id)
WHERE app_id = sqlc.arg(app_id);

-- name: SetSharedAcks :exec
UPDATE shared_apps SET acks = $2 WHERE app_id = $1;

-- name: SetSharedDBName :exec
UPDATE shared_apps SET db_name = $2 WHERE app_id = $1;

-- name: SetSharedRestarts :exec
UPDATE shared_apps SET restart_count = $2, restart_window_start = $3 WHERE app_id = $1;

-- name: TouchSharedAccess :exec
UPDATE shared_apps SET last_access_at = $2 WHERE app_id = $1 AND (last_access_at IS NULL OR last_access_at < $2);

-- name: SetSharedAccessNow :exec
UPDATE shared_apps SET last_access_at = now() WHERE app_id = $1;

-- name: MarkUnshared :exec
UPDATE shared_apps SET unshared_at = now(), status = 'stopped', current_deployment_id = NULL, previous_deployment_id = NULL
WHERE app_id = $1;

-- name: ListUnsharedBefore :many
SELECT * FROM shared_apps WHERE unshared_at IS NOT NULL AND unshared_at < $1;

-- name: DeleteSharedApp :exec
DELETE FROM shared_apps WHERE app_id = $1;

-- name: InsertDeployment :one
INSERT INTO shared_deployments (app_id, version_id, kind, recipe, test_db_name, created_by, expires_at)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: GetDeployment :one
SELECT * FROM shared_deployments WHERE id = $1;

-- name: FinishDeployment :execrows
UPDATE shared_deployments
SET status = sqlc.arg(status), image_id = sqlc.arg(image_id), container_id = sqlc.arg(container_id),
    port = sqlc.arg(port), build_log = sqlc.arg(build_log), error_message = sqlc.arg(error_message), finished_at = now()
WHERE id = sqlc.arg(id) AND status = 'building';

-- name: FailFinishedDeployment :exec
-- Undo a deployment this same job had already recorded as live/ready (the swap
-- after it could not be recorded): it never became the version in the air.
UPDATE shared_deployments SET status = 'failed', image_id = '', container_id = '', error_message = $2
WHERE id = $1 AND status IN ('live', 'ready');

-- name: SetDeploymentStatus :exec
UPDATE shared_deployments SET status = $2 WHERE id = $1;

-- name: SetDeploymentContainer :exec
UPDATE shared_deployments SET container_id = $2 WHERE id = $1;

-- name: SetDeploymentTestDB :exec
UPDATE shared_deployments SET test_db_name = $2 WHERE id = $1;

-- name: ListAllOpenTests :many
SELECT * FROM shared_deployments WHERE kind = 'test' AND status IN ('building','ready');

-- name: ListOpenTests :many
SELECT * FROM shared_deployments WHERE app_id = $1 AND kind = 'test' AND status IN ('building','ready');

-- name: ListDeployments :many
SELECT d.*, v.number AS version_number, p.name AS created_by_name
FROM shared_deployments d
LEFT JOIN app_versions v ON v.id = d.version_id
LEFT JOIN people p ON p.id = d.created_by
WHERE d.app_id = $1 ORDER BY d.id DESC LIMIT $2;

-- name: ListExpiredTests :many
SELECT * FROM shared_deployments
WHERE kind = 'test' AND status IN ('ready','building') AND expires_at < $1;

-- name: ListLabelledContainersExpected :many
-- Containers the Hub expects to exist: live deployments of shared apps that are not unshared.
SELECT d.container_id FROM shared_deployments d
JOIN shared_apps s ON s.current_deployment_id = d.id
WHERE s.unshared_at IS NULL AND d.container_id <> ''
UNION
SELECT container_id FROM shared_deployments WHERE kind = 'test' AND status = 'ready' AND container_id <> '';

-- name: ListAppConfig :many
SELECT * FROM app_config WHERE app_id = $1 ORDER BY name;

-- name: UpsertAppConfig :exec
INSERT INTO app_config (app_id, name, is_secret, value_plain, value_cipher, updated_by, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, now())
ON CONFLICT (app_id, name) DO UPDATE SET
    is_secret = EXCLUDED.is_secret, value_plain = EXCLUDED.value_plain, value_cipher = EXCLUDED.value_cipher,
    updated_by = EXCLUDED.updated_by, updated_at = now();

-- name: DeleteAppConfig :exec
DELETE FROM app_config WHERE app_id = $1 AND name = $2;

-- name: InsertBackup :one
INSERT INTO shared_backups (app_id, kind, path, size_bytes, deployment_id, created_by, expires_at, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: GetBackup :one
SELECT * FROM shared_backups WHERE id = $1 AND app_id = $2;

-- name: ListBackups :many
SELECT b.*, p.name AS created_by_name, v.number AS version_number
FROM shared_backups b
LEFT JOIN people p ON p.id = b.created_by
LEFT JOIN shared_deployments d ON d.id = b.deployment_id
LEFT JOIN app_versions v ON v.id = d.version_id
WHERE b.app_id = $1 ORDER BY b.created_at DESC;

-- name: LatestPreDeployBackup :one
SELECT * FROM shared_backups WHERE app_id = $1 AND kind = 'pre_deploy' AND deployment_id = $2
ORDER BY created_at DESC LIMIT 1;

-- name: LastDailyBackup :one
SELECT * FROM shared_backups WHERE app_id = $1 AND kind = 'daily' ORDER BY created_at DESC LIMIT 1;

-- name: ListAllBackups :many
SELECT * FROM shared_backups ORDER BY id;

-- name: ListExpiredBackups :many
SELECT * FROM shared_backups WHERE expires_at < $1;

-- name: DeleteBackup :exec
DELETE FROM shared_backups WHERE id = $1;

-- name: DeleteBackupsOfApp :exec
DELETE FROM shared_backups WHERE app_id = $1;

-- name: SumBackupBytes :many
SELECT app_id, COALESCE(SUM(size_bytes), 0)::bigint AS total FROM shared_backups GROUP BY app_id;

-- name: InsertUsage :exec
INSERT INTO shared_usage (app_id, at, mem_bytes, cpu_pct) VALUES ($1, $2, $3, $4)
ON CONFLICT (app_id, at) DO NOTHING;

-- name: UsageSince :many
SELECT * FROM shared_usage WHERE app_id = $1 AND at >= $2 ORDER BY at;

-- name: UsageTotalsSince :many
-- Minutes on, average and peak memory, average CPU per app since a moment.
SELECT app_id, COUNT(*)::bigint AS minutes, COALESCE(AVG(mem_bytes), 0)::bigint AS avg_mem,
       COALESCE(MAX(mem_bytes), 0)::bigint AS max_mem, COALESCE(AVG(cpu_pct), 0)::real AS avg_cpu
FROM shared_usage WHERE at >= $1 GROUP BY app_id;

-- name: DeleteUsageBefore :exec
DELETE FROM shared_usage WHERE at < $1;

-- name: GetServerSettings :one
SELECT * FROM server_settings WHERE id = 1;

-- name: SetServerCost :exec
UPDATE server_settings SET monthly_cost_cents = $1, updated_by = $2, updated_at = now() WHERE id = 1;

-- name: InsertHubChange :exec
-- An event in the app's timeline without a person (the Hub itself).
INSERT INTO app_changes (app_id, person_id, changes) VALUES ($1, NULL, $2);

-- name: ListBuildingDeployments :many
-- Deployments still marked "building": after a Hub restart those are leftovers.
SELECT * FROM shared_deployments WHERE status = 'building';
