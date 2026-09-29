-- name: CountPeople :one
SELECT count(*) FROM people;

-- name: CreatePerson :one
INSERT INTO people (name, email, password_hash, role)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: UpsertDevPerson :one
INSERT INTO people (name, email, password_hash, role)
VALUES ($1, $2, '', 'admin')
ON CONFLICT (email) DO UPDATE SET email = EXCLUDED.email
RETURNING *;

-- name: GetPersonByEmail :one
SELECT * FROM people WHERE email = $1;

-- name: GetPerson :one
SELECT * FROM people WHERE id = $1;

-- name: ListPeople :many
SELECT id, name, email, role, created_at FROM people ORDER BY name;

-- name: CreateSession :exec
INSERT INTO sessions (token, person_id, expires_at) VALUES ($1, $2, $3);

-- name: GetSessionPerson :one
SELECT p.* FROM sessions s
JOIN people p ON p.id = s.person_id
WHERE s.token = $1 AND s.expires_at > now();

-- name: DeleteSession :exec
DELETE FROM sessions WHERE token = $1;
