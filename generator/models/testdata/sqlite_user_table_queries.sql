-- name: QueryUserByID :one
select * from users where id=?;

-- name: QueryUsers :many
select * from users;

-- name: QueryAllUsers :many
select * from users;

-- name: InsertUser :one
insert into
    users (id, created_at, updated_at, email, email_verified_at, password, is_admin)
values
    (?, datetime('now'), datetime('now'), ?, ?, ?, ?)
returning *;

-- name: UpdateUser :one
update users
    set updated_at=datetime('now'), email=?, email_verified_at=?, password=?, is_admin=?
where id = ?
returning *;

-- name: DeleteUser :exec
delete from users where id=?;

-- name: QueryPaginatedUsers :many
select * from users 
order by created_at desc 
limit ? offset ?;

-- name: CountUsers :one
select count(*) from users;
