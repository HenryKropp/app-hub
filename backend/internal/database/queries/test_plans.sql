-- name: GetTestPlan :one
SELECT * FROM app_test_plans WHERE app_id = $1;

-- name: UpsertTestPlan :one
INSERT INTO app_test_plans (app_id, fingerprint, computer_id, confirmed_by, confirmed_at)
VALUES (sqlc.arg(app_id), sqlc.arg(fingerprint), sqlc.narg(computer_id), sqlc.arg(confirmed_by), now())
ON CONFLICT (app_id) DO UPDATE SET fingerprint = EXCLUDED.fingerprint, computer_id = EXCLUDED.computer_id,
    confirmed_by = EXCLUDED.confirmed_by, confirmed_at = now()
RETURNING *;

-- name: SetTestPlanComputer :execrows
UPDATE app_test_plans SET computer_id = sqlc.narg(computer_id) WHERE app_id = sqlc.arg(app_id);
