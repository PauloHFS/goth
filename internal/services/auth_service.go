package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/PauloHFS/goth/internal/config"
	"github.com/PauloHFS/goth/internal/db"
	"github.com/PauloHFS/goth/internal/validator"
	"golang.org/x/crypto/bcrypt"
)

type AuthService struct {
	queries *db.Queries
	db      *sql.DB
	config  *config.Config
}

func NewAuthService(queries *db.Queries, db *sql.DB, cfg *config.Config) *AuthService {
	return &AuthService{
		queries: queries,
		db:      db,
		config:  cfg,
	}
}

type RegisterInput struct {
	Email    string
	Password string
}

type RegisterOutput struct {
	Success bool
	Error   string
}

func (s *AuthService) Register(ctx context.Context, input RegisterInput) RegisterOutput {
	validation := validator.ValidateRegistration(input.Email, input.Password)
	if !validation.Valid {
		errMsg := ""
		for _, e := range validation.Errors {
			errMsg += e.Message + " "
		}
		return RegisterOutput{Success: false, Error: errMsg}
	}

	_, err := s.queries.GetUserByEmail(ctx, db.GetUserByEmailParams{
		TenantID: "default",
		Email:    input.Email,
	})
	if err == nil {
		return RegisterOutput{Success: false, Error: "Este e-mail já está em uso"}
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		return RegisterOutput{Success: false, Error: "Erro ao processar senha"}
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return RegisterOutput{Success: false, Error: "Erro interno"}
	}
	defer func() { _ = tx.Rollback() }()

	qtx := s.queries.WithTx(tx)

	_, err = qtx.CreateUser(ctx, db.CreateUserParams{
		TenantID:     "default",
		Email:        input.Email,
		PasswordHash: string(hash),
		RoleID:       "user",
	})
	if err != nil {
		return RegisterOutput{Success: false, Error: "Erro ao criar usuário"}
	}

	tokenBytes := make([]byte, 32)
	if _, err := fmt.Scanln(tokenBytes); err != nil {
		return RegisterOutput{Success: false, Error: "Erro interno"}
	}
	token := fmt.Sprintf("%x", tokenBytes)

	if err := qtx.UpsertEmailVerification(ctx, db.UpsertEmailVerificationParams{
		Email:     input.Email,
		Token:     token,
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}); err != nil {
		return RegisterOutput{Success: false, Error: "Erro interno"}
	}

	jobPayload, _ := json.Marshal(map[string]string{
		"email": input.Email,
		"token": token,
	})

	if _, err := qtx.CreateJob(ctx, db.CreateJobParams{
		TenantID: sql.NullString{String: "default", Valid: true},
		Type:     "send_verification_email",
		Payload:  jobPayload,
		RunAt:    sql.NullTime{Time: time.Now(), Valid: true},
	}); err != nil {
		return RegisterOutput{Success: false, Error: "Erro interno"}
	}

	if err := tx.Commit(); err != nil {
		return RegisterOutput{Success: false, Error: "Erro interno"}
	}

	return RegisterOutput{Success: true}
}

type LoginInput struct {
	Email    string
	Password string
}

type LoginOutput struct {
	Success bool
	Error   string
	User    *db.User
}

func (s *AuthService) Login(ctx context.Context, input LoginInput) LoginOutput {
	if input.Email == "" || input.Password == "" {
		return LoginOutput{Success: false, Error: "Email e senha são obrigatórios"}
	}

	user, err := s.queries.GetUserByEmail(ctx, db.GetUserByEmailParams{
		TenantID: "default",
		Email:    input.Email,
	})

	if err != nil {
		return LoginOutput{Success: false, Error: "Usuário ou senha inválidos"}
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(input.Password)); err != nil {
		return LoginOutput{Success: false, Error: "Usuário ou senha inválidos"}
	}

	return LoginOutput{Success: true, User: &user}
}

type ForgotPasswordInput struct {
	Email string
}

type ForgotPasswordOutput struct {
	Success bool
	Message string
}

func (s *AuthService) ForgotPassword(ctx context.Context, input ForgotPasswordInput) ForgotPasswordOutput {
	if err := validator.ValidateEmail(input.Email); err != nil {
		return ForgotPasswordOutput{Success: false, Message: err.Error()}
	}

	_, err := s.queries.GetUserByEmail(ctx, db.GetUserByEmailParams{
		TenantID: "default",
		Email:    input.Email,
	})
	if err != nil {
		return ForgotPasswordOutput{Success: true, Message: "Se o e-mail existir, um link será enviado."}
	}

	return ForgotPasswordOutput{Success: true, Message: "Se o e-mail existir, um link será enviado."}
}

type ResetPasswordInput struct {
	Token    string
	Password string
}

type ResetPasswordOutput struct {
	Success bool
	Error   string
}

func (s *AuthService) ResetPassword(ctx context.Context, input ResetPasswordInput) ResetPasswordOutput {
	if err := validator.ValidatePassword(input.Password); err != nil {
		return ResetPasswordOutput{Success: false, Error: err.Error()}
	}

	return ResetPasswordOutput{Success: true}
}
