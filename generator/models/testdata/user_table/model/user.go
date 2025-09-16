package models

import (
	"context"
	"errors"
	"github.com/aarondl/opt/omit"
	"github.com/aarondl/opt/omitnull"
	"github.com/google/uuid"
	"github.com/stephenafamo/bob"
	"time"

	"bob-new/models/internal/db"
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
	exec bob.Executor,
	id uuid.UUID,
) (User, error) {
	row, err := db.FindUser(ctx, exec, id)
	if err != nil {
		return User{}, err
	}

	return rowToUser(*row), nil
}

type CreateUserData struct {
	Email    string
	Name     string
	Age      int32
	IsActive bool
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
		Email:     omit.From(data.Email),
		Name:      omit.From(data.Name),
		Age:       omitnull.From(data.Age),
		IsActive:  omitnull.From(data.IsActive),
		ID:        omit.From(uuid.New()),
		CreatedAt: omitnull.From(time.Now()),
		UpdatedAt: omitnull.From(time.Now()),
	}

	row, err := db.Users.Insert(setter).One(ctx, exec)
	if err != nil {
		return User{}, err
	}

	return rowToUser(*row), nil
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
	exec bob.Executor,
	data UpdateUserData,
) (User, error) {
	if err := validate.Struct(data); err != nil {
		return User{}, errors.Join(ErrDomainValidation, err)
	}

	// First get the existing record
	existing, err := db.FindUser(ctx, exec, data.ID)
	if err != nil {
		return User{}, err
	}

	setter := &db.UserSetter{
		Email:    omit.From(data.Email),
		Name:     omit.From(data.Name),
		Age:      omitnull.From(data.Age),
		IsActive: omitnull.From(data.IsActive),
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
	existing, err := db.FindUser(ctx, exec, id)
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
		ID:        row.ID,
		Email:     row.Email,
		Name:      row.Name,
		Age:       row.Age.GetOrZero(),
		IsActive:  row.IsActive.GetOrZero(),
		CreatedAt: row.CreatedAt.GetOrZero(),
		UpdatedAt: row.UpdatedAt.GetOrZero(),
	}
}
