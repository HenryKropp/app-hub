-- name: CreatePairing :exec
INSERT INTO pairings (code, secret_hash, computer_name, expires_at)
VALUES ($1, $2, $3, now() + interval '10 minutes');

-- name: GetPairing :one
SELECT code, computer_name, person_id, (expires_at <= now())::boolean AS expired
FROM pairings WHERE code = $1;

-- name: ApprovePairing :execrows
UPDATE pairings SET person_id = $2
WHERE code = $1 AND person_id IS NULL AND expires_at > now();

-- name: GetPairingStateBySecret :one
SELECT person_id, (expires_at <= now())::boolean AS expired
FROM pairings WHERE secret_hash = $1;

-- name: ConsumePairing :one
DELETE FROM pairings
WHERE secret_hash = $1 AND person_id IS NOT NULL AND expires_at > now()
RETURNING computer_name, person_id;

-- name: CreateComputer :one
INSERT INTO computers (person_id, name, token_hash, last_seen_at)
VALUES ($1, $2, $3, now())
RETURNING *;

-- name: GetComputerByToken :one
SELECT * FROM computers WHERE token_hash = $1 AND revoked_at IS NULL;

-- name: GetComputer :one
SELECT * FROM computers WHERE id = $1;

-- name: TouchComputer :exec
UPDATE computers SET last_seen_at = now() WHERE id = $1;

-- name: HelloComputer :exec
UPDATE computers SET last_seen_at = now(), name = $2, agent_version = $3, can_update = $4, can_stop = $5 WHERE id = $1;

-- name: ListPersonComputers :many
SELECT *, (last_seen_at > now() - interval '60 seconds')::boolean AS connected
FROM computers
WHERE person_id = $1 AND revoked_at IS NULL
ORDER BY last_seen_at DESC NULLS LAST;

-- name: ConnectedComputerForPerson :one
SELECT * FROM computers
WHERE person_id = $1 AND revoked_at IS NULL AND last_seen_at > now() - interval '60 seconds'
ORDER BY last_seen_at DESC
LIMIT 1;

-- name: IsComputerConnected :one
SELECT (revoked_at IS NULL AND last_seen_at > now() - interval '60 seconds')::boolean
FROM computers WHERE id = $1;

-- name: RevokeComputer :exec
UPDATE computers SET revoked_at = now() WHERE id = $1;

-- name: CreateJob :one
INSERT INTO agent_jobs (computer_id, run_id, kind, payload)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: ClaimJob :one
UPDATE agent_jobs SET status = 'taken', taken_at = now()
WHERE id = (
    SELECT j.id FROM agent_jobs j
    WHERE j.computer_id = $1 AND j.status = 'pending' AND j.created_at > now() - interval '2 minutes'
    ORDER BY j.id
    LIMIT 1
    FOR UPDATE SKIP LOCKED
)
RETURNING *;

-- name: GetJob :one
SELECT * FROM agent_jobs WHERE id = $1;

-- name: FinishJob :exec
UPDATE agent_jobs SET status = $2, result = $3, done_at = now() WHERE id = $1;

-- name: ExpireStaleRuns :exec
UPDATE runs SET status = 'interrupted', finished_at = now(),
    error_message = 'O computador não abriu o app a tempo. Confira se ele está ligado e tente de novo.'
WHERE status = 'waiting' AND id IN (
    SELECT j.run_id FROM agent_jobs j
    WHERE j.run_id IS NOT NULL AND j.kind = 'run' AND (
        (j.status = 'pending' AND j.created_at <= now() - interval '2 minutes')
        -- claimed but the agent never said "started" (connection lost)
        OR (j.status = 'taken' AND j.taken_at <= now() - interval '1 minute')
    )
);

-- name: ExpireStaleJobs :exec
UPDATE agent_jobs SET status = 'failed', done_at = now()
WHERE status = 'pending' AND created_at <= now() - interval '2 minutes';

-- name: InterruptTakenRuns :exec
UPDATE runs SET status = 'interrupted', finished_at = now(),
    error_message = 'O Agente reiniciou com o app aberto (o computador pode ter sido desligado).'
WHERE status IN ('waiting', 'running') AND id IN (
    SELECT j.run_id FROM agent_jobs j
    WHERE j.computer_id = $1 AND j.status = 'taken' AND j.run_id IS NOT NULL
);

-- name: CancelPendingRunJob :execrows
-- A run the agent has not taken yet can be called off in the Hub alone.
UPDATE agent_jobs SET status = 'failed', payload = payload - 'script', done_at = now()
WHERE run_id = $1 AND kind = 'run' AND status = 'pending';

-- name: FailTakenJobs :exec
UPDATE agent_jobs SET status = 'failed', done_at = now()
WHERE computer_id = $1 AND status = 'taken';

-- name: CountActiveUpdates :one
SELECT count(*) FROM agent_jobs
WHERE computer_id = $1 AND kind = 'update' AND status IN ('pending', 'taken');

-- name: CountFailedUpdates :one
-- Only updates the agent took count; one that expired in the queue does not.
SELECT count(*) FROM agent_jobs
WHERE computer_id = sqlc.arg(computer_id) AND kind = 'update' AND status = 'failed'
  AND taken_at IS NOT NULL AND payload->>'version' = sqlc.arg(target_version)::text;

-- name: ResolveTakenUpdates :exec
-- Called on hello: the agent restarted, so a taken update either worked
-- (it now reports the target version) or was rolled back. An update marked
-- done in the last 10 minutes whose version the agent no longer runs
-- connected and then died, so it counts as failed.
UPDATE agent_jobs
SET status = CASE WHEN payload->>'version' = sqlc.arg(hello_version)::text THEN 'done' ELSE 'failed' END,
    done_at = now()
WHERE computer_id = sqlc.arg(computer_id) AND kind = 'update'
  AND (status = 'taken'
       OR (status = 'done' AND done_at > now() - interval '10 minutes'
           AND payload->>'version' <> sqlc.arg(hello_version)::text));

-- name: LatestUpdateJob :one
SELECT * FROM agent_jobs WHERE computer_id = $1 AND kind = 'update' ORDER BY id DESC LIMIT 1;
