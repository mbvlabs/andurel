-- name: GetUser :one
SELECT *
FROM users
WHERE id = $1
LIMIT 1;

-- name: GetUserByEmail :one
SELECT *
FROM users
WHERE email = $1
LIMIT 1;

-- name: GetUserForUpdate :one
SELECT *
FROM users
WHERE id = $1
FOR UPDATE;

-- name: ListUsers :many
-- @order id
SELECT *
FROM users;

-- name: CountUsers :one
SELECT count(*)
FROM users;

-- name: CreateUser :one
INSERT INTO users (
	id,
	created_at,
	updated_at,
	email,
	email_validated_at,
	password,
	is_admin,
	session_version
) VALUES (
	$1, $2, $3, $4, $5, $6, $7, $8
)
RETURNING *;

-- name: UpdateUser :one
UPDATE users
SET
	email = $2,
	email_validated_at = $3,
	password = $4,
	is_admin = $5,
	session_version = $6,
	updated_at = $7
WHERE id = $1
RETURNING *;

-- name: DeleteUser :exec
DELETE FROM users
WHERE id = $1;

-- name: BumpSessionVersion :exec
UPDATE users
SET session_version = session_version + 1
WHERE id = $1;
