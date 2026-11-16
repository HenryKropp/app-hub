-- name: InsertDraft :one
INSERT INTO app_drafts (app_id, author_id, channel, files, ficha, note)
VALUES (sqlc.arg(app_id), sqlc.narg(author_id), sqlc.arg(channel), sqlc.arg(files), sqlc.narg(ficha), sqlc.arg(note))
RETURNING *;

-- name: GetDraft :one
SELECT * FROM app_drafts WHERE app_id = $1 AND id = $2;

-- name: CancelOpenDrafts :exec
UPDATE app_drafts
SET status = 'cancelada', status_reason = sqlc.arg(reason), updated_at = clock_timestamp()
WHERE app_id = sqlc.arg(app_id) AND status IN ('rascunho', 'testando', 'aprovada');

-- name: MarkDraftPublished :exec
UPDATE app_drafts
SET status = 'no_ar', version_number = sqlc.arg(version_number), updated_at = clock_timestamp()
WHERE id = sqlc.arg(id);
