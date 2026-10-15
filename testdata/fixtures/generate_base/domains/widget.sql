-- name: GetWidget :one
SELECT *
FROM widgets
WHERE id = $1
LIMIT 1;

-- name: ListWidgets :many
-- @order id
SELECT *
FROM widgets;

-- name: CreateWidget :one
INSERT INTO widgets (
	id,
	name,
	created_at,
	updated_at
) VALUES (
	$1, $2, $3, $4
)
RETURNING *;

-- name: DeleteWidget :exec
DELETE FROM widgets
WHERE id = $1;
