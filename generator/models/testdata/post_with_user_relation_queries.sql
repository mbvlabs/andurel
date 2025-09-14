-- name: QueryPostByID :one
select * from posts where id=$1;

-- name: QueryPosts :many
select * from posts;

-- name: QueryAllPosts :many
select * from posts;

-- name: InsertPost :one
insert into
    posts (id, user_id, title, content, published, created_at, updated_at)
values
    ($1, $2, $3, $4, $5, now(), now())
returning *;

-- name: UpdatePost :one
update posts
    set user_id=$2, title=$3, content=$4, published=$5, updated_at=now()
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

