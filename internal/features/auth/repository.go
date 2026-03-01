package auth

import (
	"context"
	"database/sql"

	"github.com/PauloHFS/goth/internal/db"
)

type repository struct {
	db *sql.DB
	q  *db.Queries
}

func NewRepository(dbConn *sql.DB) UserRepository {
	return &repository{
		db: dbConn,
		q:  db.New(dbConn),
	}
}

func (r *repository) Create(ctx context.Context, params CreateUserParams) (db.User, error) {
	return r.q.CreateUser(ctx, db.CreateUserParams{
		TenantID:     params.TenantID,
		Email:        params.Email,
		PasswordHash: params.PasswordHash,
		RoleID:       params.RoleID,
	})
}

func (r *repository) GetByEmail(ctx context.Context, tenantID, email string) (db.User, error) {
	return r.q.GetUserByEmail(ctx, db.GetUserByEmailParams{
		TenantID: tenantID,
		Email:    email,
	})
}

func (r *repository) GetByID(ctx context.Context, id int64) (db.User, error) {
	return r.q.GetUserByID(ctx, id)
}

func (r *repository) UpdatePassword(ctx context.Context, email, hash string) error {
	return r.q.UpdateUserPassword(ctx, db.UpdateUserPasswordParams{
		PasswordHash: hash,
		Email:        email,
	})
}

func (r *repository) UpdateAvatar(ctx context.Context, id int64, url string) error {
	return r.q.UpdateUserAvatar(ctx, db.UpdateUserAvatarParams{
		AvatarUrl: sql.NullString{String: url, Valid: true},
		ID:        id,
	})
}

func (r *repository) Verify(ctx context.Context, email string) error {
	return r.q.VerifyUser(ctx, email)
}

func (r *repository) GetByProvider(ctx context.Context, tenantID, provider, providerID string) (db.User, error) {
	return r.q.GetUserByProvider(ctx, db.GetUserByProviderParams{
		TenantID:   tenantID,
		Provider:   sql.NullString{String: provider, Valid: true},
		ProviderID: sql.NullString{String: providerID, Valid: true},
	})
}

func (r *repository) UpdateWithOAuth(ctx context.Context, params UpdateWithOAuthParams) (db.User, error) {
	return r.q.UpdateUserWithOAuth(ctx, db.UpdateUserWithOAuthParams{
		Provider:   sql.NullString{String: params.Provider, Valid: true},
		ProviderID: sql.NullString{String: params.ProviderID, Valid: true},
		AvatarUrl:  sql.NullString{String: params.AvatarURL, Valid: true},
		IsVerified: params.IsVerified,
		TenantID:   params.TenantID,
		Email:      params.Email,
	})
}

func (r *repository) CreateWithOAuth(ctx context.Context, params CreateWithOAuthParams) (db.User, error) {
	return r.q.CreateUserWithOAuth(ctx, db.CreateUserWithOAuthParams{
		TenantID:     params.TenantID,
		Email:        params.Email,
		Provider:     sql.NullString{String: params.Provider, Valid: true},
		ProviderID:   sql.NullString{String: params.ProviderID, Valid: true},
		PasswordHash: params.PasswordHash,
		RoleID:       params.RoleID,
		IsVerified:   params.IsVerified,
		AvatarUrl:    sql.NullString{String: params.AvatarURL, Valid: true},
	})
}

type emailVerificationRepository struct {
	db *sql.DB
	q  *db.Queries
}

func NewEmailVerificationRepository(dbConn *sql.DB) EmailVerificationRepository {
	return &emailVerificationRepository{
		db: dbConn,
		q:  db.New(dbConn),
	}
}

func (r *emailVerificationRepository) Upsert(ctx context.Context, params UpsertVerificationParams) error {
	return r.q.UpsertEmailVerification(ctx, db.UpsertEmailVerificationParams{
		Email:     params.Email,
		Token:     params.Token,
		ExpiresAt: params.ExpiresAt,
	})
}

func (r *emailVerificationRepository) GetByToken(ctx context.Context, token string) (db.EmailVerification, error) {
	return r.q.GetEmailVerificationByToken(ctx, token)
}

func (r *emailVerificationRepository) Delete(ctx context.Context, email string) error {
	return r.q.DeleteEmailVerification(ctx, email)
}

type passwordResetRepository struct {
	db *sql.DB
	q  *db.Queries
}

func NewPasswordResetRepository(dbConn *sql.DB) PasswordResetRepository {
	return &passwordResetRepository{
		db: dbConn,
		q:  db.New(dbConn),
	}
}

func (r *passwordResetRepository) Upsert(ctx context.Context, params UpsertResetParams) error {
	return r.q.UpsertPasswordReset(ctx, db.UpsertPasswordResetParams{
		Email:     params.Email,
		TokenHash: params.TokenHash,
		ExpiresAt: params.ExpiresAt,
	})
}

func (r *passwordResetRepository) GetByToken(ctx context.Context, tokenHash string) (db.PasswordReset, error) {
	return r.q.GetPasswordResetByToken(ctx, tokenHash)
}

func (r *passwordResetRepository) Delete(ctx context.Context, email string) error {
	return r.q.DeletePasswordReset(ctx, email)
}
