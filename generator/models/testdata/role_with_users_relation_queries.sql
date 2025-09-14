-- name: QueryRoleByID :one
select * from roles where id=$1;

-- name: QueryRoles :many
select * from roles;

-- name: QueryAllRoles :many
select * from roles;

-- name: InsertRole :one
insert into
    roles (id, name, description, created_at, updated_at)
values
    ($1, $2, $3, now(), now())
returning *;

-- name: UpdateRole :one
update roles
    set name=$2, description=$3, updated_at=now()
where id = $1
returning *;

-- name: DeleteRole :exec
delete from roles where id=$1;

-- name: QueryPaginatedRoles :many
select * from roles 
order by created_at desc 
limit sqlc.arg('limit')::bigint offset sqlc.arg('offset')::bigint;

-- name: CountRoles :one
select count(*) from roles;

