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

type Role struct {
	ID          uuid.UUID
	Name        string
	Description string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func FindRole(
	ctx context.Context,
	dbtx db.DBTX,
	id uuid.UUID,
) (Role, error) {
	row, err := db.New().QueryRoleByID(ctx, dbtx, id)
	if err != nil {
		return Role{}, err
	}

	return rowToRole(row), nil
}

type CreateRoleData struct {
	Name        string
	Description string
}

func CreateRole(
	ctx context.Context,
	dbtx db.DBTX,
	data CreateRoleData,
) (Role, error) {
	if err := validate.Struct(data); err != nil {
		return Role{}, errors.Join(ErrDomainValidation, err)
	}

	params := db.NewInsertRoleParams(
		data.Name,
		pgtype.Text{String: data.Description, Valid: true},
	)
	row, err := db.New().InsertRole(ctx, dbtx, params)
	if err != nil {
		return Role{}, err
	}

	return rowToRole(row), nil
}

type UpdateRoleData struct {
	ID          uuid.UUID
	Name        string
	Description string
	UpdatedAt   time.Time
}

func UpdateRole(
	ctx context.Context,
	dbtx db.DBTX,
	data UpdateRoleData,
) (Role, error) {
	if err := validate.Struct(data); err != nil {
		return Role{}, errors.Join(ErrDomainValidation, err)
	}

	currentRow, err := db.New().QueryRoleByID(ctx, dbtx, data.ID)
	if err != nil {
		return Role{}, err
	}

	params := db.NewUpdateRoleParams(
		data.ID,
		func() string {
			if true {
				return data.Name
			}
			return currentRow.Name
		}(),
		func() pgtype.Text {
			if true {
				return pgtype.Text{String: data.Description, Valid: true}
			}
			return currentRow.Description
		}(),
	)

	row, err := db.New().UpdateRole(ctx, dbtx, params)
	if err != nil {
		return Role{}, err
	}

	return rowToRole(row), nil
}

func DestroyRole(
	ctx context.Context,
	dbtx db.DBTX,
	id uuid.UUID,
) error {
	return db.New().DeleteRole(ctx, dbtx, id)
}

func AllRoles(
	ctx context.Context,
	dbtx db.DBTX,
) ([]Role, error) {
	rows, err := db.New().QueryAllRoles(ctx, dbtx)
	if err != nil {
		return nil, err
	}

	roles := make([]Role, len(rows))
	for i, row := range rows {
		roles[i] = rowToRole(row)
	}

	return roles, nil
}

type PaginatedRoles struct {
	Roles      []Role
	TotalCount int64
	Page       int64
	PageSize   int64
	TotalPages int64
}

func PaginateRoles(
	ctx context.Context,
	dbtx db.DBTX,
	page int64,
	pageSize int64,
) (PaginatedRoles, error) {
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

	totalCount, err := db.New().CountRoles(ctx, dbtx)
	if err != nil {
		return PaginatedRoles{}, err
	}

	rows, err := db.New().QueryPaginatedRoles(
		ctx,
		dbtx,
		db.NewQueryPaginatedRolesParams(pageSize, offset),
	)
	if err != nil {
		return PaginatedRoles{}, err
	}

	roles := make([]Role, len(rows))
	for i, row := range rows {
		roles[i] = rowToRole(row)
	}

	totalPages := (totalCount + int64(pageSize) - 1) / int64(pageSize)

	return PaginatedRoles{
		Roles:      roles,
		TotalCount: totalCount,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	}, nil
}

func rowToRole(row db.Role) Role {
	return Role{
		ID:          row.ID,
		Name:        row.Name,
		Description: row.Description.String,
		CreatedAt:   row.CreatedAt.Time,
		UpdatedAt:   row.UpdatedAt.Time,
	}
}
