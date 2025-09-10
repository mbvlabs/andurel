package models

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/example/blog/models/internal/db"
)

type Post struct {
	ID          uuid.UUID
	Title       string
	CreatedAt   time.Time
	AuthorId    int32
	PublishedAt time.Time
}

func FindPost(
	ctx context.Context,
	dbtx db.DBTX,
	id uuid.UUID,
) (Post, error) {
	row, err := db.New().QueryPostByID(ctx, dbtx, id)
	if err != nil {
		return Post{}, err
	}

	return rowToPost(row), nil
}

type CreatePostPayload struct {
	Title       string
	AuthorId    int32
	PublishedAt time.Time
}

func CreatePost(
	ctx context.Context,
	dbtx db.DBTX,
	data CreatePostPayload,
) (Post, error) {
	if err := validate.Struct(data); err != nil {
		return Post{}, errors.Join(ErrDomainValidation, err)
	}

	row, err := db.New().InsertPost(ctx, dbtx, db.InsertPostParams{
		ID:          uuid.New(),
		Title:       data.Title,
		AuthorId:    pgtype.Int4{Int32: data.AuthorId, Valid: true},
		PublishedAt: pgtype.Timestamptz{Time: data.PublishedAt, Valid: true},
	})
	if err != nil {
		return Post{}, err
	}

	return rowToPost(row), nil
}

type UpdatePostPayload struct {
	ID          uuid.UUID
	Title       string
	AuthorId    int32
	PublishedAt time.Time
}

func UpdatePost(
	ctx context.Context,
	dbtx db.DBTX,
	data UpdatePostPayload,
) (Post, error) {
	if err := validate.Struct(data); err != nil {
		return Post{}, errors.Join(ErrDomainValidation, err)
	}

	currentRow, err := db.New().QueryPostByID(ctx, dbtx, data.ID)
	if err != nil {
		return Post{}, err
	}

	payload := db.UpdatePostParams{
		ID:          data.ID,
		Title:       currentRow.Title,
		AuthorId:    currentRow.AuthorId,
		PublishedAt: currentRow.PublishedAt,
	}
	if true {
		payload.Title = data.Title
	}
	if true {
		payload.AuthorId = pgtype.Int4{Int32: data.AuthorId, Valid: true}
	}
	if true {
		payload.PublishedAt = pgtype.Timestamptz{Time: data.PublishedAt, Valid: true}
	}

	row, err := db.New().UpdatePost(ctx, dbtx, payload)
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
	return db.New().DeletePost(ctx, dbtx, id)
}

func AllPosts(
	ctx context.Context,
	dbtx db.DBTX,
) ([]Post, error) {
	rows, err := db.New().QueryAllPosts(ctx, dbtx)
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

	totalCount, err := db.New().CountPosts(ctx, dbtx)
	if err != nil {
		return PaginatedPosts{}, err
	}

	rows, err := db.New().QueryPaginatedPosts(
		ctx,
		dbtx,
		db.QueryPaginatedPostsParams{
			Limit:  pageSize,
			Offset: offset,
		},
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
		AuthorId:    row.AuthorId.Int32,
		PublishedAt: row.PublishedAt.Time,
	}
}
