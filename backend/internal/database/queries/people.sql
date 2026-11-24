-- name: CountPeople :one
SELECT count(*) FROM people;

-- name: GetPersonByEmail :one
SELECT * FROM people WHERE email = $1;

-- name: GetPerson :one
SELECT * FROM people WHERE id = $1;

-- name: CreateSession :exec
INSERT INTO sessions (token, person_id, expires_at) VALUES (encode(sha256(convert_to(sqlc.arg(token)::text,'UTF8')),'hex'), sqlc.arg(person_id), sqlc.arg(expires_at));

-- name: GetSessionPerson :one
SELECT p.* FROM sessions s
JOIN people p ON p.id = s.person_id
WHERE s.token = encode(sha256(convert_to(sqlc.arg(token)::text,'UTF8')),'hex') AND s.expires_at > now()
  AND s.last_used_at > now() - interval '7 days' AND p.disabled_at IS NULL;

-- name: TouchSession :exec
-- At most once an hour, so a busy page does not write on every request.
UPDATE sessions SET last_used_at = now()
WHERE token = encode(sha256(convert_to(sqlc.arg(token)::text,'UTF8')),'hex') AND last_used_at < now() - interval '1 hour';

-- name: DeletePersonSessions :exec
DELETE FROM sessions WHERE person_id = $1;

-- name: SetPersonDisabled :execrows
UPDATE people SET disabled_at = CASE WHEN sqlc.arg(disabled)::boolean THEN COALESCE(disabled_at, now()) END
WHERE id = sqlc.arg(id);

-- name: DeleteSession :exec
DELETE FROM sessions WHERE token = encode(sha256(convert_to(sqlc.arg(token)::text,'UTF8')),'hex');

-- name: CreatePerson :one
INSERT INTO people (name, email, password_hash, role_id)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: UpsertDevPerson :one
INSERT INTO people (name, email, password_hash, role_id)
VALUES ($1, $2, '', (SELECT id FROM roles WHERE key = 'admin'))
ON CONFLICT (email) DO UPDATE SET email = EXCLUDED.email
RETURNING *;

-- name: ListPeople :many
SELECT p.id, p.name, p.email, p.role_id, r.name AS role_name, p.created_at, p.disabled_at,
       COALESCE((SELECT array_agg(m.area_id ORDER BY m.area_id) FROM area_members m WHERE m.person_id = p.id), '{}')::bigint[] AS area_ids
FROM people p
JOIN roles r ON r.id = p.role_id
ORDER BY p.name;

-- name: SetPersonRole :exec
UPDATE people SET role_id = $2 WHERE id = $1;
