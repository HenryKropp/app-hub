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
WHERE r.person_id = $1 AND r.dismissed_at IS NULL AND r.origin <> 'schedule'
  AND r.created_at > now() - interval '7 days'
  AND (r.status IN ('error', 'interrupted') OR (r.status = 'waiting' AND r.created_at < now() - interval '5 minutes'))
ORDER BY r.created_at DESC
LIMIT 10;

-- name: CountAppErrorsSince :many
SELECT app_id, count(*)::bigint AS errors, max(created_at)::timestamptz AS last_at
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

-- name: TeamPeople :many
SELECT p.id, p.name
FROM people p
WHERE sqlc.arg(all_areas)::bool
   OR p.id IN (SELECT m.person_id FROM area_members m WHERE m.area_id = ANY(sqlc.arg(area_ids)::bigint[]))
   OR p.id IN (SELECT l.person_id FROM area_leaders l WHERE l.area_id = ANY(sqlc.arg(area_ids)::bigint[]))
ORDER BY p.name;

-- name: TeamUsage :many
SELECT p.id AS person_id,
       (SELECT count(*) FROM runs r JOIN apps a ON a.id = r.app_id
         WHERE r.person_id = p.id AND r.created_at > now() - interval '7 days'
           AND (sqlc.arg(all_areas)::bool OR a.area_id = ANY(sqlc.arg(area_ids)::bigint[])))::bigint AS runs_7d,
       (SELECT count(*) FROM artifact_opens o JOIN artifacts x ON x.id = o.artifact_id
         WHERE o.person_id = p.id AND o.opened_at > now() - interval '7 days'
           AND (sqlc.arg(all_areas)::bool OR x.area_id = ANY(sqlc.arg(area_ids)::bigint[])))::bigint AS opens_7d,
       (EXISTS (SELECT 1 FROM runs r WHERE r.person_id = p.id)
        OR EXISTS (SELECT 1 FROM artifact_opens o WHERE o.person_id = p.id))::bool AS ever_used
FROM people p
WHERE p.id = ANY(sqlc.arg(person_ids)::bigint[]);

-- name: TeamCreations :many
SELECT * FROM (
    SELECT 'app'::text AS kind, a.id, a.name, a.area_id, a.owner_id, a.backup_id, a.restricted,
           COALESCE(pe.name, '')::text AS person_name, 'created'::text AS event, 1::int AS version, a.created_at AS at
    FROM apps a LEFT JOIN people pe ON pe.id = a.created_by
    WHERE a.created_at > now() - interval '14 days'
      AND (sqlc.arg(all_areas)::bool OR a.area_id = ANY(sqlc.arg(area_ids)::bigint[]))
    UNION ALL
    SELECT 'app', a.id, a.name, a.area_id, a.owner_id, a.backup_id, a.restricted,
           COALESCE(pe.name, ''), 'new_version', v.number, v.created_at
    FROM app_versions v JOIN apps a ON a.id = v.app_id LEFT JOIN people pe ON pe.id = v.author_id
    WHERE v.number > 1 AND v.created_at > now() - interval '14 days'
      AND (sqlc.arg(all_areas)::bool OR a.area_id = ANY(sqlc.arg(area_ids)::bigint[]))
    UNION ALL
    SELECT 'artifact', a.id, a.name, a.area_id, a.owner_id, a.backup_id, a.restricted,
           COALESCE(pe.name, ''), 'created', 1, a.created_at
    FROM artifacts a LEFT JOIN people pe ON pe.id = a.created_by
    WHERE a.created_at > now() - interval '14 days'
      AND (sqlc.arg(all_areas)::bool OR a.area_id = ANY(sqlc.arg(area_ids)::bigint[]))
    UNION ALL
    SELECT 'artifact', a.id, a.name, a.area_id, a.owner_id, a.backup_id, a.restricted,
           COALESCE(pe.name, ''), 'new_version', v.number, v.created_at
    FROM artifact_versions v JOIN artifacts a ON a.id = v.artifact_id LEFT JOIN people pe ON pe.id = v.author_id
    WHERE v.number > 1 AND v.created_at > now() - interval '14 days'
      AND (sqlc.arg(all_areas)::bool OR a.area_id = ANY(sqlc.arg(area_ids)::bigint[]))
) e
ORDER BY at DESC
LIMIT 30;

-- name: ListPendingInvitations :many
SELECT id, email, created_at, (expires_at < now())::bool AS expired
FROM membership_invitations
WHERE accepted_at IS NULL AND revoked_at IS NULL
ORDER BY created_at DESC;

-- name: ListPendingRequests :many
SELECT id, name, email, created_at FROM membership_requests WHERE status = 'pending' ORDER BY created_at DESC;

-- name: ListPeopleWithoutArea :many
SELECT p.id, p.name FROM people p
WHERE NOT EXISTS (SELECT 1 FROM area_members m WHERE m.person_id = p.id)
  AND NOT EXISTS (SELECT 1 FROM area_leaders l WHERE l.person_id = p.id)
ORDER BY p.name;
