-- name: QueryCategoryByID :one
select * from categories where id=$1;

-- name: QueryCategorys :many
select * from categories;

-- name: QueryAllCategorys :many
select * from categories;

-- name: InsertCategory :one
insert into
    categories (id, name, slug, description, parent_id, sort_order, is_active, created_at, updated_at)
values
    ($1, $2, $3, $4, $5, $6, $7, now(), now())
returning *;

-- name: UpdateCategory :one
update categories
    set name=$2, slug=$3, description=$4, parent_id=$5, sort_order=$6, is_active=$7, updated_at=now()
where id = $1
returning *;

-- name: DeleteCategory :exec
delete from categories where id=$1;

-- name: QueryPaginatedCategorys :many
select * from categories 
order by created_at desc 
limit sqlc.arg('limit')::bigint offset sqlc.arg('offset')::bigint;

-- name: CountCategorys :one
select count(*) from categories;

