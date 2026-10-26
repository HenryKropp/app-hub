-- name: CountPeople :one
SELECT count(*) FROM people;

-- name: GetPersonByEmail :one
SELECT * FROM people WHERE email = $1;

-- name: GetPerson :one
SELECT * FROM people WHERE id = $1;

-- name: CreateSession :exec
INSERT INTO sessions (token, person_id, expires_at) VALUES (encode(sha256(convert_to($1::text,'UTF8')),'hex'), $2, $3);

-- name: GetSessionPerson :one
SELECT p.* FROM sessions s
JOIN people p ON p.id = s.person_id
WHERE s.token = encode(sha256(convert_to($1::text,'UTF8')),'hex') AND s.expires_at > now();

-- name: DeleteSession :exec
DELETE FROM sessions WHERE token = encode(sha256(convert_to($1::text,'UTF8')),'hex');

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
SELECT p.id, p.name, p.email, p.role_id, r.name AS role_name, p.created_at,
       COALESCE((SELECT array_agg(m.area_id ORDER BY m.area_id) FROM area_members m WHERE m.person_id = p.id), '{}')::bigint[] AS area_ids
FROM people p
JOIN roles r ON r.id = p.role_id
ORDER BY p.name;

-- name: SetPersonRole :exec
UPDATE people SET role_id = $2 WHERE id = $1;
