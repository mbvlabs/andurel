-- name: QueryUserByID :one
select * from users where id=$1;

-- name: QueryUsers :many
select * from users;

-- name: QueryAllUsers :many
select * from users;

-- name: InsertUser :one
insert into
    users (id, created_at, updated_at, email, email_verified_at, password, is_admin, age, zipcode)
values
    ($1, now(), now(), $2, $3, $4, $5, $6, $7)
returning *;

-- name: UpdateUser :one
update users
    set updated_at=now(), email=$2, email_verified_at=$3, password=$4, is_admin=$5, age=$6, zipcode=$7
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

