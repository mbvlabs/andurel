package models

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/example/myapp/models/internal/db"
)

type User struct {
	ID        uuid.UUID
	Email     string
	Name      string
	Age       int32
	IsActive  bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

func FindUser(
	ctx context.Context,
	dbtx db.DBTX,
	id uuid.UUID,
) (User, error) {
	row, err := db.New().QueryUserByID(ctx, dbtx, id)
	if err != nil {
		return User{}, err
	}

	return rowToUser(row), nil
}

type CreateUserPayload struct {
	Email    string
	Name     string
	Age      int32
	IsActive bool
}

func CreateUser(
	ctx context.Context,
	dbtx db.DBTX,
	data CreateUserPayload,
) (User, error) {
	if err := validate.Struct(data); err != nil {
		return User{}, errors.Join(ErrDomainValidation, err)
	}

	row, err := db.New().InsertUser(ctx, dbtx, db.InsertUserParams{
		ID:       uuid.New(),
		Email:    data.Email,
		Name:     data.Name,
		Age:      pgtype.Int4{Int32: data.Age, Valid: true},
		IsActive: pgtype.Bool{Bool: data.IsActive, Valid: true},
	})
	if err != nil {
		return User{}, err
	}

	return rowToUser(row), nil
}

type UpdateUserPayload struct {
	ID        uuid.UUID
	Email     string
	Name      string
	Age       int32
	IsActive  bool
	UpdatedAt time.Time
}

func UpdateUser(
	ctx context.Context,
	dbtx db.DBTX,
	data UpdateUserPayload,
) (User, error) {
	if err := validate.Struct(data); err != nil {
		return User{}, errors.Join(ErrDomainValidation, err)
	}

	currentRow, err := db.New().QueryUserByID(ctx, dbtx, data.ID)
	if err != nil {
		return User{}, err
	}

	payload := db.UpdateUserParams{
		ID:       data.ID,
		Email:    currentRow.Email,
		Name:     currentRow.Name,
		Age:      currentRow.Age,
		IsActive: currentRow.IsActive,
	}
	if true {
		payload.Email = data.Email
	}
	if true {
		payload.Name = data.Name
	}
	if true {
		payload.Age = pgtype.Int4{Int32: data.Age, Valid: true}
	}
	if true {
		payload.IsActive = pgtype.Bool{Bool: data.IsActive, Valid: true}
	}

	row, err := db.New().UpdateUser(ctx, dbtx, payload)
	if err != nil {
		return User{}, err
	}

	return rowToUser(row), nil
}

func DestroyUser(
	ctx context.Context,
	dbtx db.DBTX,
	id uuid.UUID,
) error {
	return db.New().DeleteUser(ctx, dbtx, id)
}

func AllUsers(
	ctx context.Context,
	dbtx db.DBTX,
) ([]User, error) {
	rows, err := db.New().QueryAllUsers(ctx, dbtx)
	if err != nil {
		return nil, err
	}

	users := make([]User, len(rows))
	for i, row := range rows {
		users[i] = rowToUser(row)
	}

	return users, nil
}

type PaginatedUsers struct {
	Users      []User
	TotalCount int64
	Page       int64
	PageSize   int64
	TotalPages int64
}

func PaginateUsers(
	ctx context.Context,
	dbtx db.DBTX,
	page int64,
	pageSize int64,
) (PaginatedUsers, error) {
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

	totalCount, err := db.New().CountUsers(ctx, dbtx)
	if err != nil {
		return PaginatedUsers{}, err
	}

	rows, err := db.New().QueryPaginatedUsers(
		ctx,
		dbtx,
		db.QueryPaginatedUsersParams{
			Limit:  pageSize,
			Offset: offset,
		},
	)
	if err != nil {
		return PaginatedUsers{}, err
	}

	users := make([]User, len(rows))
	for i, row := range rows {
		users[i] = rowToUser(row)
	}

	totalPages := (totalCount + int64(pageSize) - 1) / int64(pageSize)

	return PaginatedUsers{
		Users:      users,
		TotalCount: totalCount,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	}, nil
}

func rowToUser(row db.User) User {
	return User{
		ID:        row.ID,
		Email:     row.Email,
		Name:      row.Name,
		Age:       row.Age.Int32,
		IsActive:  row.IsActive.Bool,
		CreatedAt: row.CreatedAt.Time,
		UpdatedAt: row.UpdatedAt.Time,
	}
}
