package models

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/example/sqlite/models/internal/db"
)

type User struct {
	ID              uuid.UUID
	CreatedAt       time.Time
	UpdatedAt       time.Time
	Email           string
	EmailVerifiedAt time.Time
	Password        []byte
	IsAdmin         int64
}

func FindUser(
	ctx context.Context,
	dbtx db.DBTX,
	id uuid.UUID,
) (User, error) {
	row, err := db.New().QueryUserByID(ctx, dbtx, id.String())
	if err != nil {
		return User{}, err
	}

	result, err := rowToUser(row)
	if err != nil {
		return User{}, err
	}
	return result, nil
}

type CreateUserData struct {
	Email           string
	EmailVerifiedAt time.Time
	Password        []byte
	IsAdmin         int64
}

func CreateUser(
	ctx context.Context,
	dbtx db.DBTX,
	data CreateUserData,
) (User, error) {
	if err := validate.Struct(data); err != nil {
		return User{}, errors.Join(ErrDomainValidation, err)
	}

	params := db.NewInsertUserParams()
	row, err := db.New().InsertUser(ctx, dbtx, params)
	if err != nil {
		return User{}, err
	}

	result, err := rowToUser(row)
	if err != nil {
		return User{}, err
	}
	return result, nil
}

type UpdateUserData struct {
	ID              uuid.UUID
	UpdatedAt       time.Time
	Email           string
	EmailVerifiedAt time.Time
	Password        []byte
	IsAdmin         int64
}

func UpdateUser(
	ctx context.Context,
	dbtx db.DBTX,
	data UpdateUserData,
) (User, error) {
	if err := validate.Struct(data); err != nil {
		return User{}, errors.Join(ErrDomainValidation, err)
	}

	currentRow, err := db.New().QueryUserByID(ctx, dbtx, data.ID.String())
	if err != nil {
		return User{}, err
	}

	params := db.NewUpdateUserParams()

	row, err := db.New().UpdateUser(ctx, dbtx, params)
	if err != nil {
		return User{}, err
	}

	result, err := rowToUser(row)
	if err != nil {
		return User{}, err
	}
	return result, nil
}

func DestroyUser(
	ctx context.Context,
	dbtx db.DBTX,
	id uuid.UUID,
) error {
	return db.New().DeleteUser(ctx, dbtx, id.String())
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
		result, err := rowToUser(row)
		if err != nil {
			return nil, err
		}
		users[i] = result
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
		db.NewQueryPaginatedUsersParams(pageSize, offset),
	)
	if err != nil {
		return PaginatedUsers{}, err
	}

	users := make([]User, len(rows))
	for i, row := range rows {
		result, err := rowToUser(row)
		if err != nil {
			return PaginatedUsers{}, err
		}
		users[i] = result
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

func rowToUser(row db.User) (User, error) {
	id, err := uuid.Parse(row.ID)
	if err != nil {
		return User{}, err
	}

	return User{
		ID:              id,
		CreatedAt:       row.CreatedAt,
		UpdatedAt:       row.UpdatedAt,
		Email:           row.Email,
		EmailVerifiedAt: row.EmailVerifiedAt.Time,
		Password:        row.Password,
		IsAdmin:         row.IsAdmin,
	}, nil
}
