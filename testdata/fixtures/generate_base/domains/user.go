package domains

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"
	"uuid"

	"example.com/app/domains/internal/queries"

	"github.com/mbvlabs/andurel/pkg/storage"
	"github.com/mbvlabs/andurel/pkg/validation"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"golang.org/x/crypto/argon2"
)

const minPasswordLength = 8

type Users struct {
	queries *queries.Queries
}

func NewUsers(db storage.Connection) Users {
	return Users{queries: queries.New(db)}
}

// InTx binds this receiver's queries to the caller's transaction.
func (u Users) InTx(tx storage.Tx) Users {
	return Users{queries: queries.New(storage.Unwrap(tx))}
}

// normaliseEmail is shared by lookups and writes so they agree on identity.
func normaliseEmail(raw string) string {
	return strings.ToLower(strings.TrimSpace(raw))
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

type User struct {
	ID               uuid.UUID          `andurel:"id"`
	CreatedAt        pgtype.Timestamptz `andurel:"created_at"`
	UpdatedAt        pgtype.Timestamptz `andurel:"updated_at"`
	Email            string             `andurel:"email"`
	EmailValidatedAt pgtype.Timestamptz `andurel:"email_validated_at"`
	Password         []byte             `andurel:"password"`
	IsAdmin          bool               `andurel:"is_admin"`
}

func (u *User) Validate() error {
	b := validation.NewBuilder()
	u.validate(b)

	return b.Err()
}

func (u *User) validate(b *validation.ValidationBuilder) {
	b.Required("email", u.Email)
	b.MaxLen("email", u.Email, 255)
	if u.Email != "" && !validEmailAddress(u.Email) {
		b.Add("email", "email", "Email must be a valid email address")
	}
}

func validEmailAddress(value string) bool {
	addr, err := mail.ParseAddress(value)

	return err == nil && addr.Address == value
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
		return false, fmt.Errorf("failed to decode salt: %w", err)
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
	entity, err := u.queries.GetUser[User](ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
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
	entity, err := u.queries.GetUserByEmail[User](ctx, normaliseEmail(email))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
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

func (p PasswordPair) validate(b *validation.ValidationBuilder) {
	b.Required("password", p.Password)
	b.MinLen("password", p.Password, minPasswordLength)
	b.Required("confirmPassword", p.ConfirmPassword)
	if p.Password != p.ConfirmPassword {
		b.Add("confirmPassword", "mismatch", "Passwords do not match")
	}
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
	now := time.Now()
	entity := User{
		ID:               uuid.New(),
		CreatedAt:        pgtype.Timestamptz{Time: now, Valid: true},
		UpdatedAt:        pgtype.Timestamptz{Time: now, Valid: true},
		Email:            normaliseEmail(data.Email),
		EmailValidatedAt: pgtype.Timestamptz{},
		IsAdmin:          false,
	}

	b := validation.NewBuilder()
	entity.validate(b)
	data.PasswordPair.validate(b)
	if err := b.Err(); err != nil {
		return User{}, err
	}

	hashedPassword, err := HashPassword(data.PasswordPair.Password, pepper)
	if err != nil {
		return User{}, err
	}
	entity.Password = []byte(hashedPassword)

	created, err := u.queries.CreateUser[User](ctx, queries.CreateUserParams{
		ID:               entity.ID,
		CreatedAt:        entity.CreatedAt,
		UpdatedAt:        entity.UpdatedAt,
		Email:            entity.Email,
		EmailValidatedAt: entity.EmailValidatedAt,
		Password:         entity.Password,
		IsAdmin:          entity.IsAdmin,
	})
	if err != nil {
		return User{}, userWriteError(err)
	}

	return created, nil
}

// userWriteError translates the expected email conflict without classifying
// unrelated database failures as user errors. Keep the constraint in sync with
// the users migration.
func userWriteError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" &&
		pgErr.TableName == "users" && pgErr.ConstraintName == "users_email_key" {
		return errors.Join(ErrConflict, err)
	}

	return err
}

// UpdateUserData changes the fields that are set; a nil field keeps its current
// value. Passwords change through ChangePassword or RehashPassword.
type UpdateUserData struct {
	ID               uuid.UUID
	Email            *string
	EmailValidatedAt *pgtype.Timestamptz
	IsAdmin          *bool
}

func (u Users) Update(
	ctx context.Context,
	data UpdateUserData,
) (User, error) {
	entity, err := u.Find(ctx, data.ID)
	if err != nil {
		return User{}, err
	}

	if data.Email != nil {
		entity.Email = normaliseEmail(*data.Email)
	}
	if data.EmailValidatedAt != nil {
		entity.EmailValidatedAt = *data.EmailValidatedAt
	}
	if data.IsAdmin != nil {
		entity.IsAdmin = *data.IsAdmin
	}

	return u.save(ctx, entity)
}

// save validates entity and writes it in one statement. Callers load the user
// first, so every update reads the row once.
func (u Users) save(ctx context.Context, entity User) (User, error) {
	entity.UpdatedAt = pgtype.Timestamptz{Time: time.Now(), Valid: true}

	if err := validation.Validate(&entity); err != nil {
		return User{}, err
	}

	row, err := u.queries.UpdateUser[User](ctx, queries.UpdateUserParams{
		ID:               entity.ID,
		Email:            entity.Email,
		EmailValidatedAt: entity.EmailValidatedAt,
		Password:         entity.Password,
		IsAdmin:          entity.IsAdmin,
		UpdatedAt:        entity.UpdatedAt,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return User{}, ErrNotFound
		}

		return User{}, userWriteError(err)
	}

	return row, nil
}

func (u Users) ChangePassword(
	ctx context.Context,
	pepper string,
	id uuid.UUID,
	pair PasswordPair,
) (User, error) {
	b := validation.NewBuilder()
	pair.validate(b)
	if err := b.Err(); err != nil {
		return User{}, err
	}

	entity, err := u.Find(ctx, id)
	if err != nil {
		return User{}, err
	}

	hashedPassword, err := HashPassword(pair.Password, pepper)
	if err != nil {
		return User{}, err
	}
	entity.Password = []byte(hashedPassword)

	return u.save(ctx, entity)
}

// RehashPassword stores password hashed with the current pepper. Login calls it
// with the user it already loaded after the password matched a previous pepper.
func (u Users) RehashPassword(
	ctx context.Context,
	pepper string,
	user User,
	password string,
) (User, error) {
	hashedPassword, err := HashPassword(password, pepper)
	if err != nil {
		return User{}, err
	}
	user.Password = []byte(hashedPassword)

	return u.save(ctx, user)
}

func (u Users) Destroy(ctx context.Context, id uuid.UUID) error {
	return u.queries.DeleteUser(ctx, id)
}

func (u Users) All(ctx context.Context) ([]User, error) {
	return u.queries.ListUsers[User](ctx).All()
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

	totalCount, err := u.queries.CountUsers(ctx)
	if err != nil {
		return PaginatedUsers{}, err
	}

	entities, err := u.queries.ListUsers[User](ctx).
		Limit(int32(pageSize)).
		Offset(int32(offset)).
		All()
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
