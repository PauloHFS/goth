package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/PauloHFS/goth/internal/db"
	"golang.org/x/crypto/bcrypt"
)

type ServiceDeps struct {
	UserRepo          UserRepository
	EmailVerifRepo    EmailVerificationRepository
	PasswordResetRepo PasswordResetRepository
	JobQueue          JobQueue
	PasswordPepper    string
}

type Service struct {
	deps ServiceDeps
}

func NewService(deps ServiceDeps) *Service {
	return &Service{deps: deps}
}

// RegisterInput representa input para registro de usuário
// @Summary User registration input
// @Description Input data for user registration
// @Tags Authentication
type RegisterInput struct {
	// Email do usuário
	Email string `json:"email" example:"user@example.com"`
	// Senha do usuário (mínimo 8 caracteres)
	Password string `json:"password" example:"securepassword123"`
	// Tenant ID (multi-tenancy)
	TenantID string `json:"tenant_id" example:"default"`
}

// Validate implementa a interface Validatable para validação centralizada
func (i RegisterInput) Validate() error {
	i.Email = SanitizeEmail(i.Email)
	if err := ValidateEmail(i.Email); err != nil {
		return err
	}
	if err := ValidatePassword(i.Password, i.Email); err != nil {
		return err
	}
	return nil
}

func (s *Service) Register(ctx context.Context, input RegisterInput) error {
	// Sanitize e validate email
	input.Email = SanitizeEmail(input.Email)
	if err := ValidateEmail(input.Email); err != nil {
		return fmt.Errorf("invalid email: %w", err)
	}

	// Validate password
	if err := ValidatePassword(input.Password, input.Email); err != nil {
		return fmt.Errorf("weak password: %w", err)
	}

	if input.TenantID == "" {
		input.TenantID = "default"
	}

	_, err := s.deps.UserRepo.GetByEmail(ctx, input.TenantID, input.Email)
	if err == nil {
		return errors.New("email already in use")
	}

	_, err = s.deps.UserRepo.Create(ctx, CreateUserParams{
		TenantID:     input.TenantID,
		Email:        input.Email,
		PasswordHash: string(hashWithPepper(input.Password, s.deps.PasswordPepper)),
		RoleID:       "user",
	})
	if err != nil {
		return err
	}

	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return err
	}
	token := hex.EncodeToString(tokenBytes)

	if err := s.deps.EmailVerifRepo.Upsert(ctx, UpsertVerificationParams{
		Email:     input.Email,
		Token:     token,
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}); err != nil {
		return err
	}

	_ = s.deps.JobQueue.Enqueue(ctx, "send_verification_email", []byte(input.Email+"|"+token), input.TenantID)

	return nil
}

// LoginInput representa input para login de usuário
// @Summary User login input
// @Description Input data for user login
// @Tags Authentication
type LoginInput struct {
	// Email do usuário
	Email string `json:"email" example:"user@example.com"`
	// Senha do usuário
	Password string `json:"password" example:"securepassword123"`
	// Tenant ID (multi-tenancy)
	TenantID string `json:"tenant_id" example:"default"`
}

// Validate implementa a interface Validatable para validação centralizada
func (i LoginInput) Validate() error {
	i.Email = SanitizeEmail(i.Email)
	if i.Email == "" {
		return errors.New("email is required")
	}
	if i.Password == "" {
		return errors.New("password is required")
	}
	return nil
}

func (s *Service) Login(ctx context.Context, input LoginInput) (db.User, error) {
	// Sanitize email
	input.Email = SanitizeEmail(input.Email)

	if input.TenantID == "" {
		input.TenantID = "default"
	}

	user, err := s.deps.UserRepo.GetByEmail(ctx, input.TenantID, input.Email)
	if err != nil {
		return db.User{}, errors.New("invalid credentials")
	}

	if err := compareHashWithPepper([]byte(user.PasswordHash), []byte(input.Password), s.deps.PasswordPepper); err != nil {
		return db.User{}, errors.New("invalid credentials")
	}

	return user, nil
}

// ForgotPasswordInput representa input para recuperação de senha
// @Summary Password reset request input
// @Description Input data for password reset request
// @Tags Authentication
type ForgotPasswordInput struct {
	// Email do usuário
	Email string `json:"email" example:"user@example.com"`
	// Tenant ID (multi-tenancy)
	TenantID string `json:"tenant_id" example:"default"`
}

// Validate implementa a interface Validatable para validação centralizada
func (i ForgotPasswordInput) Validate() error {
	i.Email = SanitizeEmail(i.Email)
	if i.Email == "" {
		return errors.New("email is required")
	}
	return nil
}

func (s *Service) ForgotPassword(ctx context.Context, input ForgotPasswordInput) error {
	// Sanitize email
	input.Email = SanitizeEmail(input.Email)

	if input.TenantID == "" {
		input.TenantID = "default"
	}

	_, err := s.deps.UserRepo.GetByEmail(ctx, input.TenantID, input.Email)
	if err != nil {
		return nil
	}

	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return err
	}
	token := hex.EncodeToString(tokenBytes)

	hash := sha256.Sum256([]byte(token))
	tokenHash := hex.EncodeToString(hash[:])

	if err := s.deps.PasswordResetRepo.Upsert(ctx, UpsertResetParams{
		Email:     input.Email,
		TokenHash: tokenHash,
		ExpiresAt: time.Now().Add(1 * time.Hour),
	}); err != nil {
		return err
	}

	payload := input.Email + "|" + token
	_ = s.deps.JobQueue.Enqueue(ctx, "send_password_reset_email", []byte(payload), input.TenantID)

	return nil
}

// ResetPasswordInput representa input para reset de senha
// @Summary Password reset input
// @Description Input data for password reset with token
// @Tags Authentication
type ResetPasswordInput struct {
	// Token de reset de senha (recebido por email)
	Token string `json:"token" example:"abc123def456"`
	// Nova senha do usuário
	Password string `json:"password" example:"newSecurePassword123"`
}

// Validate implementa a interface Validatable para validação centralizada
func (i ResetPasswordInput) Validate() error {
	if i.Token == "" {
		return errors.New("token is required")
	}
	if err := ValidatePassword(i.Password, ""); err != nil {
		return err
	}
	return nil
}

func (s *Service) ResetPassword(ctx context.Context, input ResetPasswordInput) error {
	hash := sha256.Sum256([]byte(input.Token))
	tokenHash := hex.EncodeToString(hash[:])

	reset, err := s.deps.PasswordResetRepo.GetByToken(ctx, tokenHash)
	if err != nil {
		return errors.New("invalid token")
	}

	if reset.ExpiresAt.Before(time.Now()) {
		return errors.New("token expired")
	}

	if err := s.deps.UserRepo.UpdatePassword(ctx, reset.Email, string(hashWithPepper(input.Password, s.deps.PasswordPepper))); err != nil {
		return err
	}

	_ = s.deps.PasswordResetRepo.Delete(ctx, reset.Email)

	return nil
}

type VerifyEmailInput struct {
	Token string
}

func (s *Service) VerifyEmail(ctx context.Context, input VerifyEmailInput) error {
	verification, err := s.deps.EmailVerifRepo.GetByToken(ctx, input.Token)
	if err != nil {
		return errors.New("invalid token")
	}

	if verification.ExpiresAt.Before(time.Now()) {
		return errors.New("token expired")
	}

	if err := s.deps.UserRepo.Verify(ctx, verification.Email); err != nil {
		return err
	}

	_ = s.deps.EmailVerifRepo.Delete(ctx, verification.Email)

	return nil
}

// hashWithPepper gera hash bcrypt com pepper
// pepper é um segredo adicional que não é armazenado no DB
func hashWithPepper(password, pepper string) []byte {
	// Combina password + pepper antes de hash
	peppered := password + pepper
	hash, _ := bcrypt.GenerateFromPassword([]byte(peppered), bcrypt.DefaultCost)
	return hash
}

// compareHashWithPepper compara hash com pepper
func compareHashWithPepper(hash, password []byte, pepper string) error {
	peppered := append(password, []byte(pepper)...)
	return bcrypt.CompareHashAndPassword(hash, peppered)
}
