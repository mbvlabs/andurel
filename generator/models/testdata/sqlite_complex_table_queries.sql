-- name: QueryProductByID :one
select * from products where id=?;

-- name: QueryProducts :many
select * from products;

-- name: QueryAllProducts :many
select * from products;

-- name: InsertProduct :one
insert into
    products (id, int_field, integer_field, tinyint_field, smallint_field, mediumint_field, bigint_field, unsigned_bigint_field, int2_field, int8_field, boolean_field, bool_field, character_field, varchar_field, varying_character_field, nchar_field, native_character_field, nvarchar_field, text_field, clob_field, char_field, real_field, double_field, double_precision_field, float_field, numeric_field, decimal_field, dec_field, blob_field, date_as_text, datetime_as_text, timestamp_field, time_field, required_text, required_int, default_text, default_int, default_real, default_bool, default_timestamp, positive_int, email_text)
values
    (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
returning *;

-- name: UpdateProduct :one
update products
    set int_field=?, integer_field=?, tinyint_field=?, smallint_field=?, mediumint_field=?, bigint_field=?, unsigned_bigint_field=?, int2_field=?, int8_field=?, boolean_field=?, bool_field=?, character_field=?, varchar_field=?, varying_character_field=?, nchar_field=?, native_character_field=?, nvarchar_field=?, text_field=?, clob_field=?, char_field=?, real_field=?, double_field=?, double_precision_field=?, float_field=?, numeric_field=?, decimal_field=?, dec_field=?, blob_field=?, date_as_text=?, datetime_as_text=?, timestamp_field=?, time_field=?, required_text=?, required_int=?, default_text=?, default_int=?, default_real=?, default_bool=?, default_timestamp=?, positive_int=?, email_text=?
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

