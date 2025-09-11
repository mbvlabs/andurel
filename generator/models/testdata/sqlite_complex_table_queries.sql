-- name: QueryProductByID :one
select * from products where id=?;

-- name: QueryProducts :many
select * from products;

-- name: QueryAllProducts :many
select * from products;

-- name: InsertProduct :one
insert into
    products (id, uuid, name, description, price, weight, quantity, in_stock, tags, metadata, created_date, created_at, updated_at, category_id, is_featured, discount_rate)
values
    (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, datetime('now'), datetime('now'), ?, ?, ?)
returning *;

-- name: UpdateProduct :one
update products
    set uuid=?, name=?, description=?, price=?, weight=?, quantity=?, in_stock=?, tags=?, metadata=?, created_date=?, updated_at=datetime('now'), category_id=?, is_featured=?, discount_rate=?
where id = ?
returning *;

-- name: DeleteProduct :exec
delete from products where id=?;

-- name: QueryPaginatedProducts :many
select * from products 
order by created_at desc 
limit ? offset ?;

-- name: CountProducts :one
select count(*) from products;

