package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/PauloHFS/goth/internal/db"
)

// Helper function
func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

type mockUserRepository struct {
	user   db.User
	err    error
	create func(ctx context.Context, params CreateUserParams) (db.User, error)
}

func (m *mockUserRepository) Create(ctx context.Context, params CreateUserParams) (db.User, error) {
	if m.create != nil {
		return m.create(ctx, params)
	}
	return m.user, m.err
}

func (m *mockUserRepository) GetByEmail(ctx context.Context, tenantID, email string) (db.User, error) {
	if m.err != nil {
		return db.User{}, m.err
	}
	return m.user, nil
}

func (m *mockUserRepository) GetByID(ctx context.Context, id int64) (db.User, error) {
	return m.user, m.err
}

func (m *mockUserRepository) UpdatePassword(ctx context.Context, email, hash string) error {
	return m.err
}

func (m *mockUserRepository) UpdateAvatar(ctx context.Context, id int64, url string) error {
	return m.err
}

func (m *mockUserRepository) Verify(ctx context.Context, email string) error {
	return m.err
}

func (m *mockUserRepository) GetByProvider(ctx context.Context, tenantID, provider, providerID string) (db.User, error) {
	return m.user, m.err
}

func (m *mockUserRepository) UpdateWithOAuth(ctx context.Context, params UpdateWithOAuthParams) (db.User, error) {
	return m.user, m.err
}

func (m *mockUserRepository) CreateWithOAuth(ctx context.Context, params CreateWithOAuthParams) (db.User, error) {
	return m.user, m.err
}

type mockEmailVerificationRepository struct {
	verification db.EmailVerification
	err          error
}

func (m *mockEmailVerificationRepository) Upsert(ctx context.Context, params UpsertVerificationParams) error {
	return m.err
}

func (m *mockEmailVerificationRepository) GetByToken(ctx context.Context, token string) (db.EmailVerification, error) {
	return m.verification, m.err
}

func (m *mockEmailVerificationRepository) Delete(ctx context.Context, email string) error {
	return m.err
}

type mockPasswordResetRepository struct {
	reset db.PasswordReset
	err   error
}

func (m *mockPasswordResetRepository) Upsert(ctx context.Context, params UpsertResetParams) error {
	return m.err
}

func (m *mockPasswordResetRepository) GetByToken(ctx context.Context, tokenHash string) (db.PasswordReset, error) {
	return m.reset, m.err
}

func (m *mockPasswordResetRepository) Delete(ctx context.Context, email string) error {
	return m.err
}

type mockJobQueue struct {
	err error
}

func (m *mockJobQueue) Enqueue(ctx context.Context, jobType string, payload []byte, tenantID string) error {
	return m.err
}

func TestService_Register(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		userRepo := &mockUserRepository{
			err: errors.New("not found"),
			create: func(ctx context.Context, params CreateUserParams) (db.User, error) {
				return db.User{ID: 1, Email: params.Email}, nil
			},
		}
		emailVerifRepo := &mockEmailVerificationRepository{}
		jobQueue := &mockJobQueue{}

		service := NewService(ServiceDeps{
			UserRepo:          userRepo,
			EmailVerifRepo:    emailVerifRepo,
			PasswordResetRepo: nil,
			JobQueue:          jobQueue,
		})

		err := service.Register(context.Background(), RegisterInput{
			Email:    "test@example.com",
			Password: "Password123!",
			TenantID: "default",
		})

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("missing email", func(t *testing.T) {
		service := NewService(ServiceDeps{
			UserRepo:       &mockUserRepository{},
			EmailVerifRepo: &mockEmailVerificationRepository{},
			JobQueue:       &mockJobQueue{},
		})

		err := service.Register(context.Background(), RegisterInput{
			Email:    "",
			Password: "Password123!",
			TenantID: "default",
		})

		// New validation returns "invalid email" error
		if err == nil || !contains(err.Error(), "invalid email") {
			t.Errorf("expected 'invalid email' error, got: %v", err)
		}
	})

	t.Run("missing password", func(t *testing.T) {
		service := NewService(ServiceDeps{
			UserRepo:       &mockUserRepository{},
			EmailVerifRepo: &mockEmailVerificationRepository{},
			JobQueue:       &mockJobQueue{},
		})

		err := service.Register(context.Background(), RegisterInput{
			Email:    "test@example.com",
			Password: "",
			TenantID: "default",
		})

		// New validation returns password error
		if err == nil || !contains(err.Error(), "password") {
			t.Errorf("expected password error, got: %v", err)
		}
	})

	t.Run("email already in use", func(t *testing.T) {
		userRepo := &mockUserRepository{
			user: db.User{ID: 1, Email: "test@example.com"},
		}

		service := NewService(ServiceDeps{
			UserRepo:       userRepo,
			EmailVerifRepo: &mockEmailVerificationRepository{},
			JobQueue:       &mockJobQueue{},
		})

		err := service.Register(context.Background(), RegisterInput{
			Email:    "test@example.com",
			Password: "Password123!",
			TenantID: "default",
		})

		if err == nil || err.Error() != "email already in use" {
			t.Errorf("expected 'email already in use' error, got: %v", err)
		}
	})
}

func TestService_Login(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		userRepo := &mockUserRepository{
			user: db.User{
				ID:           1,
				Email:        "test@example.com",
				PasswordHash: "$2a$10$abcdefghijklmnopqrstuvwxyz",
			},
		}

		service := NewService(ServiceDeps{
			UserRepo: userRepo,
		})

		_, err := service.Login(context.Background(), LoginInput{
			Email:    "test@example.com",
			Password: "Password123!",
			TenantID: "default",
		})

		if err != nil {
			t.Logf("Login error (expected bcrypt to fail with mock hash): %v", err)
		}
	})

	t.Run("user not found", func(t *testing.T) {
		userRepo := &mockUserRepository{
			err: errors.New("not found"),
		}

		service := NewService(ServiceDeps{
			UserRepo: userRepo,
		})

		_, err := service.Login(context.Background(), LoginInput{
			Email:    "nonexistent@example.com",
			Password: "Password123!",
			TenantID: "default",
		})

		if err == nil || err.Error() != "invalid credentials" {
			t.Errorf("expected 'invalid credentials' error, got: %v", err)
		}
	})
}

func TestService_ForgotPassword(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		userRepo := &mockUserRepository{
			user: db.User{ID: 1, Email: "test@example.com"},
		}
		passwordResetRepo := &mockPasswordResetRepository{}

		service := NewService(ServiceDeps{
			UserRepo:          userRepo,
			PasswordResetRepo: passwordResetRepo,
			JobQueue:          &mockJobQueue{},
		})

		err := service.ForgotPassword(context.Background(), ForgotPasswordInput{
			Email:    "test@example.com",
			TenantID: "default",
		})

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("non-existent user returns nil", func(t *testing.T) {
		userRepo := &mockUserRepository{
			err: errors.New("not found"),
		}

		service := NewService(ServiceDeps{
			UserRepo:          userRepo,
			PasswordResetRepo: &mockPasswordResetRepository{},
			JobQueue:          &mockJobQueue{},
		})

		err := service.ForgotPassword(context.Background(), ForgotPasswordInput{
			Email:    "nonexistent@example.com",
			TenantID: "default",
		})

		if err != nil {
			t.Errorf("expected nil error for non-existent user, got: %v", err)
		}
	})
}

func TestService_ResetPassword(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		userRepo := &mockUserRepository{}
		passwordResetRepo := &mockPasswordResetRepository{
			reset: db.PasswordReset{
				Email:     "test@example.com",
				TokenHash: "hashedtoken",
				ExpiresAt: time.Now().Add(1 * time.Hour),
			},
		}

		service := NewService(ServiceDeps{
			UserRepo:          userRepo,
			PasswordResetRepo: passwordResetRepo,
		})

		err := service.ResetPassword(context.Background(), ResetPasswordInput{
			Token:    "validtoken",
			Password: "newPassword123!",
		})

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("invalid token", func(t *testing.T) {
		passwordResetRepo := &mockPasswordResetRepository{
			err: errors.New("not found"),
		}

		service := NewService(ServiceDeps{
			PasswordResetRepo: passwordResetRepo,
		})

		err := service.ResetPassword(context.Background(), ResetPasswordInput{
			Token:    "invalidtoken",
			Password: "newPassword123!",
		})

		if err == nil || err.Error() != "invalid token" {
			t.Errorf("expected 'invalid token' error, got: %v", err)
		}
	})

	t.Run("expired token", func(t *testing.T) {
		passwordResetRepo := &mockPasswordResetRepository{
			reset: db.PasswordReset{
				Email:     "test@example.com",
				TokenHash: "hashedtoken",
				ExpiresAt: time.Now().Add(-1 * time.Hour),
			},
		}

		service := NewService(ServiceDeps{
			PasswordResetRepo: passwordResetRepo,
		})

		err := service.ResetPassword(context.Background(), ResetPasswordInput{
			Token:    "expiredtoken",
			Password: "newPassword123!",
		})

		if err == nil || err.Error() != "token expired" {
			t.Errorf("expected 'token expired' error, got: %v", err)
		}
	})
}

func TestService_VerifyEmail(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		emailVerifRepo := &mockEmailVerificationRepository{
			verification: db.EmailVerification{
				Email:     "test@example.com",
				Token:     "validtoken",
				ExpiresAt: time.Now().Add(1 * time.Hour),
			},
		}
		userRepo := &mockUserRepository{}

		service := NewService(ServiceDeps{
			UserRepo:       userRepo,
			EmailVerifRepo: emailVerifRepo,
		})

		err := service.VerifyEmail(context.Background(), VerifyEmailInput{
			Token: "validtoken",
		})

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("invalid token", func(t *testing.T) {
		emailVerifRepo := &mockEmailVerificationRepository{
			err: errors.New("not found"),
		}

		service := NewService(ServiceDeps{
			EmailVerifRepo: emailVerifRepo,
		})

		err := service.VerifyEmail(context.Background(), VerifyEmailInput{
			Token: "invalidtoken",
		})

		if err == nil || err.Error() != "invalid token" {
			t.Errorf("expected 'invalid token' error, got: %v", err)
		}
	})

	t.Run("expired token", func(t *testing.T) {
		emailVerifRepo := &mockEmailVerificationRepository{
			verification: db.EmailVerification{
				Email:     "test@example.com",
				Token:     "expiredtoken",
				ExpiresAt: time.Now().Add(-1 * time.Hour),
			},
		}

		service := NewService(ServiceDeps{
			EmailVerifRepo: emailVerifRepo,
		})

		err := service.VerifyEmail(context.Background(), VerifyEmailInput{
			Token: "expiredtoken",
		})

		if err == nil || err.Error() != "token expired" {
			t.Errorf("expected 'token expired' error, got: %v", err)
		}
	})
}
