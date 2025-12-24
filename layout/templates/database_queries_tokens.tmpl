-- name: QueryTokenByID :one
select * from tokens where id=$1;

-- name: QueryTokens :many
select * from tokens;

-- name: InsertToken :one
insert into
    tokens (id, created_at, updated_at, scope, expires_at, hash, meta_data)
values
    ($1, now(), now(), $2, $3, $4, $5)
returning *;

-- name: UpdateToken :one
update tokens
    set updated_at=now(), scope=$2, expires_at=$3, hash=$4, meta_data=$5
where id = $1
returning *;

-- name: DeleteToken :exec
delete from tokens where id=$1;

-- name: QueryPaginatedTokens :many
select * from tokens
order by created_at desc
limit sqlc.arg('limit')::bigint offset sqlc.arg('offset')::bigint;

-- name: CountTokens :one
select count(*) from tokens;

-- name: QueryTokenByScopeAndHash :one
select * from tokens where scope=$1 and hash=$2 limit 1;
