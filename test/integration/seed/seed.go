package seed

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"time"

	"github.com/PauloHFS/goth/internal/db"
	"golang.org/x/crypto/bcrypt"
)

const (
	DefaultTenantID = "default"
	TestPassword    = "test123456"
)

type SeedUser struct {
	Email    string
	Password string
	Role     string
	Name     string
}

var (
	AdminUser  = SeedUser{Email: "admin@goth.local", Password: TestPassword, Role: "admin", Name: "Admin User"}
	NormalUser = SeedUser{Email: "user@goth.local", Password: TestPassword, Role: "user", Name: "Normal User"}
	TestUser   = SeedUser{Email: "test@goth.local", Password: TestPassword, Role: "user", Name: "Test User"}
)

func HashPassword(password string) string {
	hash, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(hash)
}

func SeedTestData(ctx context.Context, queries *db.Queries) error {
	hash := HashPassword(TestPassword)

	_, err := queries.CreateUser(ctx, db.CreateUserParams{
		TenantID:     DefaultTenantID,
		Email:        AdminUser.Email,
		PasswordHash: hash,
		RoleID:       "admin",
	})
	if err != nil {
		return err
	}

	_, err = queries.CreateUser(ctx, db.CreateUserParams{
		TenantID:     DefaultTenantID,
		Email:        NormalUser.Email,
		PasswordHash: hash,
		RoleID:       "user",
	})
	if err != nil {
		return err
	}

	_, err = queries.CreateUser(ctx, db.CreateUserParams{
		TenantID:     DefaultTenantID,
		Email:        TestUser.Email,
		PasswordHash: hash,
		RoleID:       "user",
	})
	return err
}

func CreateEmailVerification(ctx context.Context, queries *db.Queries, email string) (string, error) {
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", err
	}
	token := hex.EncodeToString(tokenBytes)

	err := queries.UpsertEmailVerification(ctx, db.UpsertEmailVerificationParams{
		Email:     email,
		Token:     token,
		ExpiresAt: time.Now().Add(24 * time.Hour),
	})
	return token, err
}

func CreatePasswordReset(ctx context.Context, queries *db.Queries, email string) (string, error) {
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", err
	}
	token := hex.EncodeToString(tokenBytes)

	tokenHash := sha256.Sum256([]byte(token))
	tokenHashStr := hex.EncodeToString(tokenHash[:])

	err := queries.UpsertPasswordReset(ctx, db.UpsertPasswordResetParams{
		Email:     email,
		TokenHash: tokenHashStr,
		ExpiresAt: time.Now().Add(1 * time.Hour),
	})
	return token, err
}

func GetPasswordResetTokenHash(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}
