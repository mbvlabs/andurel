-- name: QueryUserByID :one
select * from users where id=$1;

-- name: QueryUsers :many
select * from users;

-- name: QueryAllUsers :many
select * from users;

-- name: InsertUser :one
insert into
    users (id, email, name, age, is_active, created_at, updated_at)
values
    ($1, $2, $3, $4, $5, $6, $7)
returning *;

-- name: UpdateUser :one
update users
    set email=$2, name=$3, age=$4, is_active=$5, updated_at=$6
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

