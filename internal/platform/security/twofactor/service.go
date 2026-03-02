package twofactor

import (
	"crypto/rand"
	"encoding/base32"
	"fmt"
	"io"
	"strings"
)

// Service gerencia 2FA/TOTP
type Service struct {
	issuer string
	repo   Repository
}

// Repository define a interface para persistência de 2FA
type Repository interface {
	GetTOTPSecret(userID string) (*TotpSecret, error)
	SaveTOTPSecret(userID string, secret *TotpSecret) error
	GetBackupCodes(userID string) ([]string, error)
	SaveBackupCodes(userID string, codes []string) error
	Is2FAEnabled(userID string) (bool, error)
	Enable2FA(userID string) error
	Disable2FA(userID string) error
}

// NewService cria um novo serviço 2FA
func NewService(issuer string, repo Repository) *Service {
	return &Service{
		issuer: issuer,
		repo:   repo,
	}
}

// Setup2FA inicia o processo de configuração de 2FA
func (s *Service) Setup2FA(userID, userEmail string) (*TotpSecret, error) {
	// Verificar se já está configurado
	enabled, err := s.repo.Is2FAEnabled(userID)
	if err != nil {
		return nil, fmt.Errorf("failed to check 2FA status: %w", err)
	}
	if enabled {
		return nil, fmt.Errorf("2FA already enabled for this user")
	}

	// Gerar segredo TOTP
	config := DefaultTotpConfig(s.issuer, userEmail)
	secret, err := GenerateSecret(config)
	if err != nil {
		return nil, fmt.Errorf("failed to generate TOTP secret: %w", err)
	}

	// Salvar segredo (ainda não ativado)
	if err := s.repo.SaveTOTPSecret(userID, secret); err != nil {
		return nil, fmt.Errorf("failed to save TOTP secret: %w", err)
	}

	// Gerar códigos de backup
	backupCodes, err := GenerateBackupCodes(10)
	if err != nil {
		return nil, fmt.Errorf("failed to generate backup codes: %w", err)
	}

	// Salvar códigos de backup
	if err := s.repo.SaveBackupCodes(userID, backupCodes); err != nil {
		return nil, fmt.Errorf("failed to save backup codes: %w", err)
	}

	return secret, nil
}

// Enable2FA ativa o 2FA após validação do primeiro código
func (s *Service) Enable2FA(userID, code string) error {
	// Pegar segredo
	secret, err := s.repo.GetTOTPSecret(userID)
	if err != nil {
		return fmt.Errorf("failed to get TOTP secret: %w", err)
	}
	if secret == nil {
		return fmt.Errorf("TOTP secret not found. Please setup 2FA first")
	}

	// Validar código
	if !ValidateCode(secret.Secret, code) {
		return fmt.Errorf("invalid TOTP code")
	}

	// Ativar 2FA
	if err := s.repo.Enable2FA(userID); err != nil {
		return fmt.Errorf("failed to enable 2FA: %w", err)
	}

	return nil
}

// Disable2FA desativa o 2FA
func (s *Service) Disable2FA(userID, code string, useBackupCode bool) error {
	// Verificar se 2FA está ativo
	enabled, err := s.repo.Is2FAEnabled(userID)
	if err != nil {
		return fmt.Errorf("failed to check 2FA status: %w", err)
	}
	if !enabled {
		return fmt.Errorf("2FA is not enabled")
	}

	// Pegar segredo
	secret, err := s.repo.GetTOTPSecret(userID)
	if err != nil {
		return fmt.Errorf("failed to get TOTP secret: %w", err)
	}

	// Validar código TOTP ou backup
	valid := false
	if useBackupCode {
		backupCodes, err := s.repo.GetBackupCodes(userID)
		if err != nil {
			return fmt.Errorf("failed to get backup codes: %w", err)
		}

		if VerifyBackupCode(code, backupCodes) {
			// Consumir código de backup
			newCodes := ConsumeBackupCode(code, backupCodes)
			if err := s.repo.SaveBackupCodes(userID, newCodes); err != nil {
				return fmt.Errorf("failed to update backup codes: %w", err)
			}
			valid = true
		}
	} else {
		valid = ValidateCode(secret.Secret, code)
	}

	if !valid {
		return fmt.Errorf("invalid code")
	}

	// Desativar 2FA
	if err := s.repo.Disable2FA(userID); err != nil {
		return fmt.Errorf("failed to disable 2FA: %w", err)
	}

	return nil
}

// Verify2FA verifica código 2FA durante login
func (s *Service) Verify2FA(userID, code string) error {
	// Verificar se 2FA está ativo
	enabled, err := s.repo.Is2FAEnabled(userID)
	if err != nil {
		return fmt.Errorf("failed to check 2FA status: %w", err)
	}
	if !enabled {
		return nil // 2FA não está ativo, não precisa validar
	}

	// Pegar segredo
	secret, err := s.repo.GetTOTPSecret(userID)
	if err != nil {
		return fmt.Errorf("failed to get TOTP secret: %w", err)
	}
	if secret == nil {
		return fmt.Errorf("TOTP secret not found")
	}

	// Validar código com drift (janela de 1 período para frente/trás)
	if !ValidateCodeWithDrift(secret.Secret, code, 1) {
		// Tentar códigos de backup
		backupCodes, err := s.repo.GetBackupCodes(userID)
		if err != nil {
			return fmt.Errorf("invalid TOTP code")
		}

		if !VerifyBackupCode(code, backupCodes) {
			return fmt.Errorf("invalid code")
		}

		// Consumir código de backup
		newCodes := ConsumeBackupCode(code, backupCodes)
		if err := s.repo.SaveBackupCodes(userID, newCodes); err != nil {
			return fmt.Errorf("failed to update backup codes: %w", err)
		}
	}

	return nil
}

// RegenerateBackupCodes gera novos códigos de backup
func (s *Service) RegenerateBackupCodes(userID string) ([]string, error) {
	// Verificar se 2FA está ativo
	enabled, err := s.repo.Is2FAEnabled(userID)
	if err != nil {
		return nil, fmt.Errorf("failed to check 2FA status: %w", err)
	}
	if !enabled {
		return nil, fmt.Errorf("2FA is not enabled")
	}

	// Gerar novos códigos
	backupCodes, err := GenerateBackupCodes(10)
	if err != nil {
		return nil, fmt.Errorf("failed to generate backup codes: %w", err)
	}

	// Salvar códigos
	if err := s.repo.SaveBackupCodes(userID, backupCodes); err != nil {
		return nil, fmt.Errorf("failed to save backup codes: %w", err)
	}

	return backupCodes, nil
}

// Get2FAStatus retorna o status do 2FA do usuário
func (s *Service) Get2FAStatus(userID string) (map[string]interface{}, error) {
	enabled, err := s.repo.Is2FAEnabled(userID)
	if err != nil {
		return nil, fmt.Errorf("failed to check 2FA status: %w", err)
	}

	status := map[string]interface{}{
		"enabled": enabled,
	}

	if enabled {
		// Pegar backup codes restantes
		backupCodes, err := s.repo.GetBackupCodes(userID)
		if err == nil {
			status["backup_codes_remaining"] = len(backupCodes)
		}
	}

	return status, nil
}

// GenerateSecureSecret gera um segredo aleatório seguro
func GenerateSecureSecret() (string, error) {
	bytes := make([]byte, 20) // 160 bits
	if _, err := io.ReadFull(rand.Reader, bytes); err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}

	secret := base32.StdEncoding.EncodeToString(bytes)
	return strings.ToUpper(secret), nil
}

// GetQRCodeData retorna dados do QR Code para frontend
func (s *Service) GetQRCodeData(userID string) (map[string]string, error) {
	secret, err := s.repo.GetTOTPSecret(userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get TOTP secret: %w", err)
	}
	if secret == nil {
		return nil, fmt.Errorf("TOTP secret not found")
	}

	qrCodeBase64, err := GenerateQRCodeBase64(secret)
	if err != nil {
		return nil, fmt.Errorf("failed to generate QR code: %w", err)
	}

	return map[string]string{
		"secret":         FormatSecret(secret.Secret),
		"qr_code_base64": qrCodeBase64,
		"issuer":         secret.Issuer,
		"account_name":   secret.AccountName,
	}, nil
}
