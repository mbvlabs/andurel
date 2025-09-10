-- name: QueryPostByID :one
select * from posts where id=$1;

-- name: QueryPosts :many
select * from posts;

-- name: QueryAllPosts :many
select * from posts;

-- name: InsertPost :one
insert into
    posts (id, title, created_at, author_id, published_at)
values
    ($1, $2, now(), $3, $4)
returning *;

-- name: UpdatePost :one
update posts
    set title=$2, author_id=$3, published_at=$4
where id = $1
returning *;

-- name: DeletePost :exec
delete from posts where id=$1;

-- name: QueryPaginatedPosts :many
select * from posts 
order by created_at desc 
limit sqlc.arg('limit')::bigint offset sqlc.arg('offset')::bigint;

-- name: CountPosts :one
select count(*) from posts;

