package models

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/mbvlabs/andurel/pkg/storage"
	"github.com/mbvlabs/andurel/pkg/validation"

	"github.com/google/uuid"
	"github.com/uptrace/bun"

	"golang.org/x/crypto/argon2"
)

type Users struct {
	db queryDB
}

func NewUsers(db storage.Connection) Users {
	return Users{db: db}
}

// WithTx returns a copy that runs queries inside tx.
func (u Users) WithTx(tx storage.Transaction) Users {
	return Users{db: tx}
}

type User struct {
	bun.BaseModel    `bun:"table:users,alias:user"`
	ID               uuid.UUID    `bun:"id,pk,type:uuid"`
	CreatedAt        time.Time    `bun:"created_at"`
	UpdatedAt        time.Time    `bun:"updated_at"`
	Email            string       `bun:"email"`
	EmailValidatedAt sql.NullTime `bun:"email_validated_at"`
	Password         []byte       `bun:"password"`
	IsAdmin          bool         `bun:"is_admin"`
}

func (u *User) Validate() error {
	b := validation.NewBuilder()
	b.Required("email", u.Email)
	b.MaxLen("email", u.Email, 255)

	return b.Err()
}

func (u *User) HasValidatedEmail() bool {
	return u.EmailValidatedAt.Valid
}

func (u *User) ValidPassword(providedPassword, pepper string) (bool, error) {
	parts := strings.Split(string(u.Password), ":")
	if len(parts) != 2 {
		return false, fmt.Errorf("invalid stored password format")
	}

	expectedHash, err := base64.RawStdEncoding.DecodeString(parts[0])
	if err != nil {
		return false, fmt.Errorf("failed to decode hash: %w", err)
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[1])
	if err != nil {
		return false, fmt.Errorf("failed to decode pepper: %w", err)
	}

	newHash := argon2.IDKey(
		[]byte(providedPassword+pepper),
		salt,
		2,
		19*1024,
		1,
		uint32(len(expectedHash)),
	)

	return subtle.ConstantTimeCompare(newHash, expectedHash) == 1, nil
}

func (u Users) Find(ctx context.Context, id uuid.UUID) (User, error) {
	var entity User
	err := u.db.Executor().NewSelect().
		Model(&entity).
		Where("id = ?", id).
		Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return User{}, ErrNotFound
		}

		return User{}, err
	}

	return entity, nil
}

func (u Users) FindByEmail(
	ctx context.Context,
	email string,
) (User, error) {
	var entity User
	err := u.db.Executor().NewSelect().
		Model(&entity).
		Where("email = ?", strings.ToLower(email)).
		Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return User{}, ErrNotFound
		}

		return User{}, err
	}

	return entity, nil
}

type PasswordPair struct {
	Password        string
	ConfirmPassword string
}

type CreateUserData struct {
	Email        string
	PasswordPair PasswordPair
}

func (u Users) Create(
	ctx context.Context,
	pepper string,
	data CreateUserData,
) (User, error) {
	hashedPassword, err := HashPassword(data.PasswordPair.Password, pepper)
	if err != nil {
		return User{}, err
	}

	entity := User{
		ID:               uuid.New(),
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
		Email:            strings.ToLower(data.Email),
		EmailValidatedAt: sql.NullTime{},
		Password:         []byte(hashedPassword),
		IsAdmin:          false,
	}

	if err := validation.Validate(&entity); err != nil {
		return User{}, errors.Join(ErrDomainValidation, err)
	}

	_, err = u.db.Executor().NewInsert().Model(&entity).Exec(ctx)
	if err != nil {
		return User{}, err
	}

	return entity, nil
}

type UpdateUserData struct {
	ID               uuid.UUID
	Email            string
	EmailValidatedAt sql.NullTime
	Password         []byte
	IsAdmin          bool
}

func (u Users) Update(
	ctx context.Context,
	data UpdateUserData,
) (User, error) {
	var current User
	err := u.db.Executor().NewSelect().
		Model(&current).
		Where("id = ?", data.ID).
		Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return User{}, ErrNotFound
		}

		return User{}, err
	}

	email := strings.ToLower(data.Email)
	if email == "" {
		email = current.Email
	}

	emailValidatedAt := data.EmailValidatedAt
	if !emailValidatedAt.Valid && current.EmailValidatedAt.Valid {
		emailValidatedAt = current.EmailValidatedAt
	}

	password := data.Password
	if len(password) == 0 {
		password = current.Password
	}

	entity := User{
		ID:               data.ID,
		CreatedAt:        current.CreatedAt,
		UpdatedAt:        time.Now(),
		Email:            email,
		EmailValidatedAt: emailValidatedAt,
		Password:         password,
		IsAdmin:          data.IsAdmin,
	}

	if err := validation.Validate(&entity); err != nil {
		return User{}, errors.Join(ErrDomainValidation, err)
	}

	err = u.db.Executor().NewUpdate().
		Model(&entity).
		Column("email").
		Column("email_validated_at").
		Column("password").
		Column("is_admin").
		Column("updated_at").
		WherePK().
		Returning("*").
		Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return User{}, ErrNotFound
		}

		return User{}, err
	}

	return entity, nil
}

func (u Users) Destroy(ctx context.Context, id uuid.UUID) error {
	_, err := u.db.Executor().NewDelete().
		Model((*User)(nil)).
		Where("id = ?", id).
		Exec(ctx)

	return err
}

func (u Users) All(ctx context.Context) ([]User, error) {
	var entities []User
	err := u.db.Executor().NewSelect().
		Model(&entities).
		Scan(ctx)
	if err != nil {
		return nil, err
	}

	return entities, nil
}

type PaginatedUsers struct {
	Users      []User
	TotalCount int64
	Page       int64
	PageSize   int64
	TotalPages int64
}

func (u Users) Paginate(
	ctx context.Context,
	page, pageSize int64,
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

	totalCount, err := u.db.Executor().NewSelect().
		Model(&User{}).Count(ctx)
	if err != nil {
		return PaginatedUsers{}, err
	}

	entities := make([]User, 0, int(pageSize))
	err = u.db.Executor().NewSelect().
		Model(&entities).
		Limit(int(pageSize)).
		Offset(int(offset)).
		Scan(ctx)
	if err != nil {
		return PaginatedUsers{}, err
	}

	totalPages := (int64(totalCount) + pageSize - 1) / pageSize

	return PaginatedUsers{
		Users:      entities,
		TotalCount: int64(totalCount),
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	}, nil
}

func generateSalt(size int) ([]byte, error) {
	salt := make([]byte, size)
	_, err := rand.Read(salt)
	if err != nil {
		return nil, err
	}

	return salt, nil
}

func HashPassword(password, pepper string) (string, error) {
	salt, err := generateSalt(16)
	if err != nil {
		return "", err
	}

	hash := argon2.IDKey(
		[]byte(password+pepper),
		[]byte(salt),
		2,
		19*1024,
		1,
		32,
	)

	encodedHash := fmt.Sprintf("%s:%s",
		base64.RawStdEncoding.EncodeToString(hash),
		base64.RawStdEncoding.EncodeToString(salt))

	return encodedHash, nil
}
