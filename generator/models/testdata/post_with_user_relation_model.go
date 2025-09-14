package models

import (
	"context"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"time"

	"github.com/example/relations/models/internal/db"
)

type Post struct {
	ID        uuid.UUID
	UserId    uuid.UUID
	Title     string
	Content   string
	Published bool
	CreatedAt time.Time
	UpdatedAt time.Time
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

type CreatePostData struct {
	UserId    uuid.UUID
	Title     string
	Content   string
	Published bool
}

func CreatePost(
	ctx context.Context,
	dbtx db.DBTX,
	data CreatePostData,
) (Post, error) {
	if err := validate.Struct(data); err != nil {
		return Post{}, errors.Join(ErrDomainValidation, err)
	}

	params := db.NewInsertPostParams(
		data.UserId,
		data.Title,
		pgtype.Text{String: data.Content, Valid: true},
		data.Published,
	)
	row, err := db.New().InsertPost(ctx, dbtx, params)
	if err != nil {
		return Post{}, err
	}

	return rowToPost(row), nil
}

type UpdatePostData struct {
	ID        uuid.UUID
	UserId    uuid.UUID
	Title     string
	Content   string
	Published bool
	UpdatedAt time.Time
}

func UpdatePost(
	ctx context.Context,
	dbtx db.DBTX,
	data UpdatePostData,
) (Post, error) {
	if err := validate.Struct(data); err != nil {
		return Post{}, errors.Join(ErrDomainValidation, err)
	}

	currentRow, err := db.New().QueryPostByID(ctx, dbtx, data.ID)
	if err != nil {
		return Post{}, err
	}

	params := db.NewUpdatePostParams(
		data.ID,
		func() uuid.UUID {
			if data.UserId != uuid.Nil {
				return data.UserId
			}
			return currentRow.UserId
		}(),
		func() string {
			if true {
				return data.Title
			}
			return currentRow.Title
		}(),
		func() pgtype.Text {
			if true {
				return pgtype.Text{String: data.Content, Valid: true}
			}
			return currentRow.Content
		}(),
		func() bool {
			if true {
				return data.Published
			}
			return currentRow.Published
		}(),
	)

	row, err := db.New().UpdatePost(ctx, dbtx, params)
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
		ID:        row.ID,
		UserId:    row.UserId,
		Title:     row.Title,
		Content:   row.Content.String,
		Published: row.Published,
		CreatedAt: row.CreatedAt.Time,
		UpdatedAt: row.UpdatedAt.Time,
	}
}

// User loads the User that this Post belongs to
func (post Post) User(
	ctx context.Context,
	dbtx db.DBTX,
) (*User, error) {
	// TODO: Implement many-to-one relation loading
	// This would load the User by post.user_id
	return nil, fmt.Errorf("User relation not implemented yet")
}
