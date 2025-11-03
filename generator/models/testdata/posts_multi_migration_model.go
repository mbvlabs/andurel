package models

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/example/blog/models/internal/db"
)

type Post struct {
	ID          uuid.UUID
	Title       string
	CreatedAt   time.Time
	AuthorID    int32
	PublishedAt time.Time
}

func FindPost(
	ctx context.Context,
	dbtx db.DBTX,
	id uuid.UUID,
) (Post, error) {
	row, err := queries.QueryPostByID(ctx, dbtx, id)
	if err != nil {
		return Post{}, err
	}

	return rowToPost(row), nil
}

type CreatePostData struct {
	Title       string
	AuthorID    int32
	PublishedAt time.Time
}

func CreatePost(
	ctx context.Context,
	dbtx db.DBTX,
	data CreatePostData,
) (Post, error) {
	if err := validate.Struct(data); err != nil {
		return Post{}, errors.Join(ErrDomainValidation, err)
	}

	params := db.NewInsertPostParams()
	row, err := queries.InsertPost(ctx, dbtx, params)
	if err != nil {
		return Post{}, err
	}

	return rowToPost(row), nil
}

type UpdatePostData struct {
	ID          uuid.UUID
	Title       string
	AuthorID    int32
	PublishedAt time.Time
}

func UpdatePost(
	ctx context.Context,
	dbtx db.DBTX,
	data UpdatePostData,
) (Post, error) {
	if err := validate.Struct(data); err != nil {
		return Post{}, errors.Join(ErrDomainValidation, err)
	}

	currentRow, err := queries.QueryPostByID(ctx, dbtx, data.ID)
	if err != nil {
		return Post{}, err
	}

	params := db.NewUpdatePostParams()

	row, err := queries.UpdatePost(ctx, dbtx, params)
	if err != nil {
		return Post{}, err
	}

	return rowToPost(row), nil
}

func DestroyPost(
	ctx context.Context,
	dbtx db.DBTX,
	id uuid.UUID,
) error {
	return queries.DeletePost(ctx, dbtx, id)
}

func AllPosts(
	ctx context.Context,
	dbtx db.DBTX,
) ([]Post, error) {
	rows, err := queries.QueryAllPosts(ctx, dbtx)
	if err != nil {
		return nil, err
	}

	posts := make([]Post, len(rows))
	for i, row := range rows {
		posts[i] = rowToPost(row)
	}

	return posts, nil
}

type PaginatedPosts struct {
	Posts      []Post
	TotalCount int64
	Page       int64
	PageSize   int64
	TotalPages int64
}

func PaginatePosts(
	ctx context.Context,
	dbtx db.DBTX,
	page int64,
	pageSize int64,
) (PaginatedPosts, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 10
	}
	if pageSize > 100 {
		pageSize = 100
	}

	offset := (page - 1) * pageSize

	totalCount, err := queries.CountPosts(ctx, dbtx)
	if err != nil {
		return PaginatedPosts{}, err
	}

	rows, err := queries.QueryPaginatedPosts(
		ctx,
		dbtx,
		db.NewQueryPaginatedPostsParams(pageSize, offset),
	)
	if err != nil {
		return PaginatedPosts{}, err
	}

	posts := make([]Post, len(rows))
	for i, row := range rows {
		posts[i] = rowToPost(row)
	}

	totalPages := (totalCount + int64(pageSize) - 1) / int64(pageSize)

	return PaginatedPosts{
		Posts:      posts,
		TotalCount: totalCount,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	}, nil
}

func rowToPost(row db.Post) Post {
	return Post{
		ID:          row.ID,
		Title:       row.Title,
		CreatedAt:   row.CreatedAt.Time,
		AuthorID:    row.AuthorID.Int32,
		PublishedAt: row.PublishedAt.Time,
	}
}
