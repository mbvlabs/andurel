package models

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base32"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"

	"github.com/mbvlabs/andurel/pkg/storage"
	"github.com/mbvlabs/andurel/pkg/validation"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

type Tokens struct {
	db queryDB
}

func NewTokens(db storage.Connection) Tokens {
	return Tokens{db: db}
}

// WithTx returns a copy that runs queries inside tx.
func (t Tokens) WithTx(tx storage.Transaction) Tokens {
	return Tokens{db: tx}
}

type Token struct {
	bun.BaseModel `bun:"table:tokens,alias:tokens"`
	ID            uuid.UUID       `bun:"id,pk,type:uuid"`
	CreatedAt     time.Time       `bun:"created_at"`
	UpdatedAt     time.Time       `bun:"updated_at"`
	Scope         string          `bun:"scope"`
	ExpiresAt     time.Time       `bun:"expires_at"`
	Hash          string          `bun:"hash"`
	MetaData      json.RawMessage `bun:"meta_data,type:jsonb"`
}

func (t Token) IsValid(token, secret string) bool {
	expected := HashForStorage(token, secret)

	isEqual := hmac.Equal([]byte(expected), []byte(t.Hash))
	isNotExpired := time.Now().Before(t.ExpiresAt)

	return isEqual && isNotExpired
}

const codeAlphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

func GenerateCode(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	for i := range b {
		b[i] = codeAlphabet[int(b[i])%len(codeAlphabet)]
	}

	return string(b), nil
}

func GenerateSecureToken() (string, error) {
	b := make([]byte, 15) // 120 bits
	if _, err := rand.Read(b); err != nil {
		return "", err
	}

	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(b), nil
}

func HashForStorage(plain, secret string) string {
	m := hmac.New(sha256.New, []byte(secret))
	m.Write([]byte(plain))

	return hex.EncodeToString(m.Sum(nil))
}

func (t Tokens) Find(ctx context.Context, id uuid.UUID) (Token, error) {
	var entity Token
	err := t.db.Executor().NewSelect().
		Model(&entity).
		Where("id = ?", id).
		Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Token{}, ErrNotFound
		}

		return Token{}, err
	}

	return entity, nil
}

func (t Tokens) FindByScopeAndHash(
	ctx context.Context,
	secret string,
	scope string,
	plainToken string,
) (Token, error) {
	hash := HashForStorage(plainToken, secret)

	var entity Token
	err := t.db.Executor().NewSelect().
		Model(&entity).
		Where("scope = ?", scope).
		Where("hash = ?", hash).
		Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Token{}, ErrNotFound
		}

		return Token{}, err
	}

	return entity, nil
}

type createTokenData struct {
	Scope     string
	ExpiresAt time.Time
	Hash      string
	MetaData  json.RawMessage
}

func (t *Token) Validate() error {
	b := validation.NewBuilder()
	b.Required("scope", t.Scope)
	b.Required("expires_at", t.ExpiresAt)
	b.Required("hash", t.Hash)
	b.Required("meta_data", t.MetaData)

	return b.Err()
}

func (t Tokens) createToken(
	ctx context.Context,
	data createTokenData,
) (Token, error) {
	entity := Token{
		ID:        uuid.New(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Scope:     data.Scope,
		ExpiresAt: data.ExpiresAt,
		Hash:      data.Hash,
		MetaData:  data.MetaData,
	}

	if err := validation.Validate(&entity); err != nil {
		return Token{}, errors.Join(ErrDomainValidation, err)
	}

	if _, err := t.db.Executor().NewInsert().Model(&entity).Exec(ctx); err != nil {
		return Token{}, err
	}

	return entity, nil
}

func (t Tokens) CreateCode(
	ctx context.Context,
	secret string,
	scope string,
	expiresAt time.Time,
	metaData json.RawMessage,
) (string, error) {
	tkn, err := GenerateCode(6)
	if err != nil {
		return "", err
	}

	if _, err := t.createToken(ctx, createTokenData{
		Scope:     scope,
		ExpiresAt: expiresAt,
		Hash:      HashForStorage(tkn, secret),
		MetaData:  metaData,
	}); err != nil {
		return "", err
	}

	return tkn, nil
}

func (t Tokens) Create(
	ctx context.Context,
	secret string,
	scope string,
	expiresAt time.Time,
	metaData json.RawMessage,
) (string, error) {
	tkn, err := GenerateSecureToken()
	if err != nil {
		return "", err
	}

	if _, err := t.createToken(ctx, createTokenData{
		Scope:     scope,
		ExpiresAt: expiresAt,
		Hash:      HashForStorage(tkn, secret),
		MetaData:  metaData,
	}); err != nil {
		return "", err
	}

	return tkn, nil
}

func (t Tokens) Destroy(ctx context.Context, id uuid.UUID) error {
	_, err := t.db.Executor().NewDelete().
		Model((*Token)(nil)).
		Where("id = ?", id).
		Exec(ctx)
	return err
}

func (t Tokens) All(ctx context.Context) ([]Token, error) {
	var entities []Token
	err := t.db.Executor().NewSelect().
		Model(&entities).
		Scan(ctx)
	if err != nil {
		return nil, err
	}

	return entities, nil
}

type PaginatedTokens struct {
	Tokens     []Token
	TotalCount int64
	Page       int64
	PageSize   int64
	TotalPages int64
}

func (t Tokens) Paginate(
	ctx context.Context,
	page, pageSize int64,
) (PaginatedTokens, error) {
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

	totalCount, err := t.db.Executor().NewSelect().
		Model(&Token{}).Count(ctx)
	if err != nil {
		return PaginatedTokens{}, err
	}

	entities := make([]Token, 0, int(pageSize))
	if err := t.db.Executor().NewSelect().
		Model(&entities).
		Limit(int(pageSize)).
		Offset(int(offset)).
		Scan(ctx); err != nil {
		return PaginatedTokens{}, err
	}

	totalPages := (int64(totalCount) + pageSize - 1) / pageSize

	return PaginatedTokens{
		Tokens:     entities,
		TotalCount: int64(totalCount),
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	}, nil
}
