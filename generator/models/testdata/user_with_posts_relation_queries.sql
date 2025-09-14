-- name: QueryUserByID :one
select * from users where id=$1;

-- name: QueryUsers :many
select * from users;

-- name: QueryAllUsers :many
select * from users;

-- name: InsertUser :one
insert into
    users (id, name, email, created_at, updated_at)
values
    ($1, $2, $3, now(), now())
returning *;

-- name: UpdateUser :one
update users
    set name=$2, email=$3, updated_at=now()
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

