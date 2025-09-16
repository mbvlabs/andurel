package models

import (
	"bob-sqlite/models/internal/db"
	"context"
	"errors"
	"time"

	"github.com/aarondl/opt/omit"
	"github.com/aarondl/opt/omitnull"
	"github.com/google/uuid"
	"github.com/stephenafamo/bob"
)

type User struct {
	ID              string
	CreatedAt       time.Time
	UpdatedAt       time.Time
	Email           string
	EmailVerifiedAt time.Time
	Password        []byte
	IsAdmin         bool
}

func FindUser(
	ctx context.Context,
	exec bob.Executor,
	id uuid.UUID,
) (User, error) {
	row, err := db.FindUser(ctx, exec, id.String())
	if err != nil {
		return User{}, err
	}

	return rowToUser(*row), nil
}

type CreateUserData struct {
	Email           string
	EmailVerifiedAt time.Time
	Password        []byte
	IsAdmin         bool
}

func CreateUser(
	ctx context.Context,
	exec bob.Executor,
	data CreateUserData,
) (User, error) {
	if err := validate.Struct(data); err != nil {
		return User{}, errors.Join(ErrDomainValidation, err)
	}

	setter := &db.UserSetter{
		Email:           omit.From(data.Email),
		EmailVerifiedAt: omitnull.From(data.EmailVerifiedAt),
		Password:        omit.From(data.Password),
		IsAdmin:         omit.From(data.IsAdmin),
		ID:              omit.From(uuid.New().String()),
		CreatedAt:       omit.From(time.Now()),
		UpdatedAt:       omit.From(time.Now()),
	}

	row, err := db.Users.Insert(setter).One(ctx, exec)
	if err != nil {
		return User{}, err
	}

	return rowToUser(*row), nil
}

type UpdateUserData struct {
	ID              uuid.UUID
	UpdatedAt       time.Time
	Email           string
	EmailVerifiedAt time.Time
	Password        []byte
	IsAdmin         bool
}

func UpdateUser(
	ctx context.Context,
	exec bob.Executor,
	data UpdateUserData,
) (User, error) {
	if err := validate.Struct(data); err != nil {
		return User{}, errors.Join(ErrDomainValidation, err)
	}

	// First get the existing record
	existing, err := db.FindUser(ctx, exec, data.ID.String())
	if err != nil {
		return User{}, err
	}

	setter := &db.UserSetter{
		Email:           omit.From(data.Email),
		EmailVerifiedAt: omitnull.From(data.EmailVerifiedAt),
		Password:        omit.From(data.Password),
		IsAdmin:         omit.From(data.IsAdmin),
	}

	err = existing.Update(ctx, exec, setter)
	if err != nil {
		return User{}, err
	}

	return rowToUser(*existing), nil
}

func DestroyUser(
	ctx context.Context,
	exec bob.Executor,
	id uuid.UUID,
) error {
	existing, err := db.FindUser(ctx, exec, id.String())
	if err != nil {
		return err
	}

	return existing.Delete(ctx, exec)
}

func AllUsers(
	ctx context.Context,
	exec bob.Executor,
) ([]User, error) {
	rows, err := db.Users.Query().All(ctx, exec)
	if err != nil {
		return nil, err
	}

	users := make([]User, len(rows))
	for i, row := range rows {
		users[i] = rowToUser(*row)
	}

	return users, nil
}

func rowToUser(row db.User) User {
	return User{
		ID:              row.ID,
		CreatedAt:       row.CreatedAt,
		UpdatedAt:       row.UpdatedAt,
		Email:           row.Email,
		EmailVerifiedAt: row.EmailVerifiedAt.GetOrZero(),
		Password:        row.Password,
		IsAdmin:         row.IsAdmin,
	}
}
