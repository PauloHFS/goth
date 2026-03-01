package auth

import (
	"context"
	"time"

	"github.com/PauloHFS/goth/internal/db"
)

// UserReader define interface para operações de leitura de usuários
type UserReader interface {
	GetByID(ctx context.Context, id int64) (db.User, error)
	GetByEmail(ctx context.Context, tenantID, email string) (db.User, error)
	GetByProvider(ctx context.Context, tenantID, provider, providerID string) (db.User, error)
}

// UserWriter define interface para operações de escrita de usuários
type UserWriter interface {
	Create(ctx context.Context, params CreateUserParams) (db.User, error)
	CreateWithOAuth(ctx context.Context, params CreateWithOAuthParams) (db.User, error)
	UpdateWithOAuth(ctx context.Context, params UpdateWithOAuthParams) (db.User, error)
	UpdatePassword(ctx context.Context, email, hash string) error
	UpdateAvatar(ctx context.Context, id int64, url string) error
	Verify(ctx context.Context, email string) error
}

// UserRepository combina interfaces de leitura e escrita (CQS)
type UserRepository interface {
	UserReader
	UserWriter
}

type EmailVerificationRepository interface {
	Upsert(ctx context.Context, params UpsertVerificationParams) error
	GetByToken(ctx context.Context, token string) (db.EmailVerification, error)
	Delete(ctx context.Context, email string) error
}

type PasswordResetRepository interface {
	Upsert(ctx context.Context, params UpsertResetParams) error
	GetByToken(ctx context.Context, tokenHash string) (db.PasswordReset, error)
	Delete(ctx context.Context, email string) error
}

type JobQueue interface {
	Enqueue(ctx context.Context, jobType string, payload []byte, tenantID string) error
}

type CreateUserParams struct {
	TenantID     string
	Email        string
	PasswordHash string
	RoleID       string
}

type UpsertVerificationParams struct {
	Email     string
	Token     string
	ExpiresAt time.Time
}

type UpsertResetParams struct {
	Email     string
	TokenHash string
	ExpiresAt time.Time
}

type UpdateWithOAuthParams struct {
	TenantID   string
	Email      string
	Provider   string
	ProviderID string
	AvatarURL  string
	IsVerified bool
}

type CreateWithOAuthParams struct {
	TenantID     string
	Email        string
	Provider     string
	ProviderID   string
	PasswordHash string
	RoleID       string
	IsVerified   bool
	AvatarURL    string
}
