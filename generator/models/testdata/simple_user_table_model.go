package models

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

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
	row, err := queries.QueryUserByID(ctx, dbtx, id)
	if err != nil {
		return User{}, err
	}

	return rowToUser(row), nil
}

type CreateUserData struct {
	Email    string
	Name     string
	Age      int32
	IsActive bool
}

func CreateUser(
	ctx context.Context,
	dbtx db.DBTX,
	data CreateUserData,
) (User, error) {
	if err := validate.Struct(data); err != nil {
		return User{}, errors.Join(ErrDomainValidation, err)
	}

	params := db.CreateInsertUserParams()
	row, err := queries.InsertUser(ctx, dbtx, params)
	if err != nil {
		return User{}, err
	}

	return rowToUser(row), nil
}

type UpdateUserData struct {
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
	data UpdateUserData,
) (User, error) {
	if err := validate.Struct(data); err != nil {
		return User{}, errors.Join(ErrDomainValidation, err)
	}

	currentRow, err := queries.QueryUserByID(ctx, dbtx, data.ID)
	if err != nil {
		return User{}, err
	}

	params := db.CreateUpdateUserParams()

	row, err := queries.UpdateUser(ctx, dbtx, params)
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
	return queries.DeleteUser(ctx, dbtx, id)
}

func AllUsers(
	ctx context.Context,
	dbtx db.DBTX,
) ([]User, error) {
	rows, err := queries.QueryAllUsers(ctx, dbtx)
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

	totalCount, err := queries.CountUsers(ctx, dbtx)
	if err != nil {
		return PaginatedUsers{}, err
	}

	rows, err := queries.QueryPaginatedUsers(
		ctx,
		dbtx,
		db.CreateQueryPaginatedUsersParams(pageSize, offset),
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
