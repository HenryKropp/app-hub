-- name: InsertDraft :one
INSERT INTO app_drafts (app_id, author_id, channel, files, ficha, note)
VALUES (sqlc.arg(app_id), sqlc.narg(author_id), sqlc.arg(channel), sqlc.arg(files), sqlc.narg(ficha), sqlc.arg(note))
RETURNING *;

-- name: GetDraft :one
SELECT * FROM app_drafts WHERE app_id = $1 AND id = $2;

-- name: CancelOpenDrafts :exec
UPDATE app_drafts
SET status = 'cancelada', stage = '', status_reason = sqlc.arg(reason), updated_at = clock_timestamp()
WHERE app_id = sqlc.arg(app_id) AND status IN ('rascunho', 'testando', 'aprovada');

-- name: MarkDraftPublished :exec
UPDATE app_drafts
SET status = 'no_ar', stage = '', version_number = sqlc.arg(version_number), updated_at = clock_timestamp()
WHERE id = sqlc.arg(id);

-- name: SetDraftStage :execrows
-- Moves only from the expected stage, so the clock and the Agente's result
-- never walk the same step twice.
UPDATE app_drafts
SET stage = sqlc.arg(to_stage), stage_since = clock_timestamp(), updated_at = clock_timestamp()
WHERE id = sqlc.arg(id) AND stage = sqlc.arg(from_stage) AND status IN ('testando', 'aprovada');

-- name: StartDraftTest :execrows
UPDATE app_drafts
SET status = 'testando', stage = 'esperando_confirmacao', stage_since = clock_timestamp(), updated_at = clock_timestamp()
WHERE id = $1 AND status = 'rascunho';

-- name: SetDraftTestJob :exec
UPDATE app_drafts
SET test_job_id = sqlc.arg(job_id), test_started_at = clock_timestamp(), updated_at = clock_timestamp()
WHERE id = sqlc.arg(id);

-- name: SaveDraftResult :exec
UPDATE app_drafts
SET test_exit_code = sqlc.arg(exit_code), test_log = sqlc.arg(log), test_checks = sqlc.arg(checks),
    test_files = sqlc.arg(files), updated_at = clock_timestamp()
WHERE id = sqlc.arg(id);

-- name: FailDraft :exec
UPDATE app_drafts
SET status = 'reprovada', stage = '', status_reason = sqlc.arg(reason), updated_at = clock_timestamp()
WHERE id = sqlc.arg(id) AND status IN ('testando', 'aprovada', 'aguardando');

-- name: ApproveDraft :execrows
UPDATE app_drafts
SET status = 'aprovada', stage = 'publicando', stage_since = clock_timestamp(), updated_at = clock_timestamp()
WHERE id = $1 AND status = 'testando' AND stage = 'rodando';

-- name: ListActiveDrafts :many
SELECT * FROM app_drafts WHERE status IN ('testando', 'aprovada') ORDER BY id;

-- name: GetDraftByID :one
SELECT * FROM app_drafts WHERE id = $1;

-- name: OpenDraftForApp :one
SELECT * FROM app_drafts WHERE app_id = $1 AND status IN ('testando', 'aprovada')
ORDER BY id DESC LIMIT 1;

-- name: InsertEvidence :exec
INSERT INTO draft_evidence (draft_id, path, size, content)
VALUES ($1, $2, $3, $4)
ON CONFLICT (draft_id, path) DO UPDATE SET size = EXCLUDED.size, content = EXCLUDED.content;

-- name: GetEvidence :one
SELECT * FROM draft_evidence WHERE draft_id = $1 AND path = $2;

-- name: ListEvidencePaths :many
SELECT path FROM draft_evidence WHERE draft_id = $1 ORDER BY path;

-- name: DeleteOldEvidence :exec
DELETE FROM draft_evidence WHERE created_at < now() - interval '7 days';

-- name: DeleteOldClosedDrafts :exec
DELETE FROM app_drafts WHERE status IN ('cancelada', 'reprovada') AND updated_at < now() - interval '30 days';
