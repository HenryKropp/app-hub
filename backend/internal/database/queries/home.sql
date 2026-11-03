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

-- name: ListMyAttentionRuns :many
SELECT r.id, r.app_id, a.name AS app_name, r.status, r.created_at, r.error_message
FROM runs r
JOIN apps a ON a.id = r.app_id
WHERE r.person_id = $1 AND r.dismissed_at IS NULL
  AND r.created_at > now() - interval '7 days'
  AND (r.status = 'error' OR (r.status = 'waiting' AND r.created_at < now() - interval '5 minutes'))
ORDER BY r.created_at DESC
LIMIT 10;

-- name: CountAppErrorsSince :many
SELECT app_id, count(*)::bigint AS errors
FROM runs
WHERE status = 'error' AND created_at > sqlc.arg(since)::timestamptz
GROUP BY app_id;

-- name: ListMyRecentApps :many
SELECT app_id, max(created_at)::timestamptz AS used_at
FROM runs
WHERE person_id = $1 AND created_at > now() - interval '30 days'
GROUP BY app_id;

-- name: ListMyRecentArtifacts :many
SELECT artifact_id, max(opened_at)::timestamptz AS used_at
FROM artifact_opens
WHERE person_id = $1 AND opened_at > now() - interval '30 days'
GROUP BY artifact_id;
