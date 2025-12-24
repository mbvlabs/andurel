-- name: QueryUserByID :one
select * from users where id=$1;

-- name: QueryUserByEmail :one
select * from users where email=$1;

-- name: QueryUsers :many
select * from users;

-- name: InsertUser :one
insert into
    users (id, created_at, updated_at, email, email_validated_at, password, is_admin)
values
    ($1, now(), now(), $2, $3, $4, $5)
returning *;

-- name: UpdateUser :one
update users
    set updated_at=now(), email=$2, email_validated_at=$3, password=$4, is_admin=$5
where id = $1
returning *;

-- name: DeleteUser :exec
delete from users where id=$1;

-- name: QueryPaginatedUsers :many
select * from users
order by created_at desc
limit sqlc.arg('limit')::bigint offset sqlc.arg('offset')::bigint;

-- name: CountUsers :one
select count(*) from users;
