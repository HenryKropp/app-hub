-- name: InsertNotice :exec
INSERT INTO notices (app_id, version, text) VALUES ($1, $2, $3);

-- name: ListNotices :many
-- The notices of the apps the person owns; a company admin sees them all.
SELECT n.*, a.name AS app_name FROM notices n JOIN apps a ON a.id = n.app_id
WHERE a.archived_at IS NULL AND (sqlc.arg(admin)::boolean OR a.owner_id = sqlc.arg(person_id))
ORDER BY n.created_at DESC, n.id DESC LIMIT 20;

-- name: GetNotice :one
SELECT * FROM notices WHERE id = $1;

-- name: ReadNotice :exec
UPDATE notices SET read_at = now() WHERE id = $1 AND read_at IS NULL;
