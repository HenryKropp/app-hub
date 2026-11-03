-- name: AddFavorite :exec
INSERT INTO favorites (person_id, kind, item_id) VALUES ($1, $2, $3)
ON CONFLICT DO NOTHING;

-- name: RemoveFavorite :exec
DELETE FROM favorites WHERE person_id = $1 AND kind = $2 AND item_id = $3;

-- name: ListFavorites :many
SELECT kind, item_id, created_at FROM favorites WHERE person_id = $1 ORDER BY created_at ASC;

-- name: DeleteItemFavorites :exec
DELETE FROM favorites WHERE kind = $1 AND item_id = $2;

-- name: RecordArtifactOpen :exec
INSERT INTO artifact_opens (artifact_id, person_id)
SELECT sqlc.arg(artifact_id)::bigint, sqlc.arg(person_id)::bigint
WHERE NOT EXISTS (
    SELECT 1 FROM artifact_opens
    WHERE artifact_id = sqlc.arg(artifact_id)::bigint AND person_id = sqlc.arg(person_id)::bigint
      AND opened_at > clock_timestamp() - interval '30 minutes'
);
