-- name: QueryProductByID :one
select * from products where id=$1;

-- name: QueryProducts :many
select * from products;

-- name: QueryAllProducts :many
select * from products;

-- name: InsertProduct :one
insert into
    products (id, name, price, description, category_id, in_stock, metadata, created_at, updated_at)
values
    ($1, $2, $3, $4, $5, $6, $7, now(), now())
returning *;

-- name: UpdateProduct :one
update products
    set name=$2, price=$3, description=$4, category_id=$5, in_stock=$6, metadata=$7, updated_at=now()
where id = $1
returning *;

-- name: DeleteProduct :exec
delete from products where id=$1;

-- name: QueryPaginatedProducts :many
select * from products 
order by created_at desc 
limit sqlc.arg('limit')::bigint offset sqlc.arg('offset')::bigint;

-- name: CountProducts :one
select count(*) from products;

