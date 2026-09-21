-- name: GetToken :one
SELECT *
FROM tokens
WHERE id = $1
LIMIT 1;

-- name: GetTokenByScopeAndHash :one
SELECT *
FROM tokens
WHERE scope = $1
	AND hash = $2
LIMIT 1;

-- name: ListTokens :many
-- @order id
SELECT *
FROM tokens;

-- name: CountTokens :one
SELECT count(*)
FROM tokens;

-- name: CreateToken :one
INSERT INTO tokens (
	id,
	created_at,
	updated_at,
	scope,
	expires_at,
	hash,
	meta_data
) VALUES (
	$1, $2, $3, $4, $5, $6, $7
)
RETURNING *;

-- name: DeleteToken :exec
DELETE FROM tokens
WHERE id = $1;
