-- Custom comment that should be replaced

-- name: QueryUserByID :one
select * from users where id=$1;

-- name: QueryUsers :many
select * from users;

-- Custom query that should be preserved
-- name: CustomQuery :many  
select * from users where custom_field = $1;

-- name: InsertUser :one
insert into
    users (id, email, name, age, is_active, created_at, updated_at)
values
    ($1, $2, $3, $4, $5, now(), now())
returning *;

-- Another custom query
-- name: FindUsersByEmail :many
select * from users where email like $1;

-- name: UpdateUser :one
update users
    set email=$2, name=$3, age=$4, is_active=$5, updated_at=now()
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

-- name: QueryAllUsers :many
select * from users;
