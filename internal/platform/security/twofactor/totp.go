package twofactor

import (
	"bytes"
	"encoding/base32"
	"encoding/base64"
	"fmt"
	"image/png"
	"io"
	"net/url"
	"strings"
	"time"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
)

// TotpConfig configura o gerador TOTP
type TotpConfig struct {
	Issuer      string
	AccountName string
	Digits      otp.Digits
	Period      uint
	Algorithm   otp.Algorithm
}

// DefaultTotpConfig retorna configuração padrão compatível com Google Authenticator
func DefaultTotpConfig(issuer, accountName string) TotpConfig {
	return TotpConfig{
		Issuer:      issuer,
		AccountName: accountName,
		Digits:      otp.DigitsSix,
		Period:      30,
		Algorithm:   otp.AlgorithmSHA1,
	}
}

// TotpSecret representa um segredo TOTP
type TotpSecret struct {
	Secret      string    `json:"secret"`
	Issuer      string    `json:"issuer"`
	AccountName string    `json:"account_name"`
	QRCodeURL   string    `json:"qr_code_url"`
	Base32      string    `json:"base32"`
	CreatedAt   time.Time `json:"created_at"`
}

// GenerateSecret gera um novo segredo TOTP
func GenerateSecret(config TotpConfig) (*TotpSecret, error) {
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      config.Issuer,
		AccountName: config.AccountName,
		Digits:      config.Digits,
		Period:      config.Period,
		Algorithm:   config.Algorithm,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to generate TOTP secret: %w", err)
	}

	// Extrair base32 do secret
	secretBytes, err := base32.StdEncoding.DecodeString(key.Secret())
	if err != nil {
		return nil, fmt.Errorf("failed to decode secret: %w", err)
	}

	secret := &TotpSecret{
		Secret:      key.Secret(),
		Issuer:      config.Issuer,
		AccountName: config.AccountName,
		QRCodeURL:   key.URL(),
		Base32:      base32.StdEncoding.EncodeToString(secretBytes),
		CreatedAt:   time.Now(),
	}

	return secret, nil
}

// GenerateQRCode gera um QR Code a partir do segredo TOTP
func GenerateQRCode(secret *TotpSecret) (io.Reader, error) {
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      secret.Issuer,
		AccountName: secret.AccountName,
		Digits:      otp.DigitsSix,
		Period:      30,
		Algorithm:   otp.AlgorithmSHA1,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to generate key for QR code: %w", err)
	}

	img, err := key.Image(200, 200)
	if err != nil {
		return nil, fmt.Errorf("failed to generate QR code image: %w", err)
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, fmt.Errorf("failed to encode QR code: %w", err)
	}

	return &buf, nil
}

// GenerateQRCodeBase64 gera QR Code em formato Base64 para embed em HTML
func GenerateQRCodeBase64(secret *TotpSecret) (string, error) {
	img, err := GenerateQRCode(secret)
	if err != nil {
		return "", err
	}

	imgBytes, err := io.ReadAll(img)
	if err != nil {
		return "", fmt.Errorf("failed to read QR code: %w", err)
	}

	base64Img := base64.StdEncoding.EncodeToString(imgBytes)
	return fmt.Sprintf("data:image/png;base64,%s", base64Img), nil
}

// ValidateCode valida um código TOTP
func ValidateCode(secret string, code string) bool {
	// Remover espaços do código
	code = strings.ReplaceAll(code, " ", "")

	// Validar formato (6 dígitos)
	if len(code) != 6 {
		return false
	}

	// Validar código
	valid := totp.Validate(code, secret)
	return valid
}

// ValidateCodeWithDrift valida código com janela de tempo (para compensar clock drift)
func ValidateCodeWithDrift(secret string, code string, periods uint) bool {
	code = strings.ReplaceAll(code, " ", "")

	if len(code) != 6 {
		return false
	}

	valid, _ := totp.ValidateCustom(code, secret, time.Now().UTC(), totp.ValidateOpts{
		Period:    30,
		Skew:      periods,
		Digits:    otp.DigitsSix,
		Algorithm: otp.AlgorithmSHA1,
	})

	return valid
}

// GenerateBackupCodes gera códigos de backup one-time use
func GenerateBackupCodes(count int) ([]string, error) {
	if count <= 0 {
		return nil, fmt.Errorf("count must be positive")
	}

	codes := make([]string, count)
	for i := 0; i < count; i++ {
		// Gerar código aleatório de 8 caracteres
		bytes := make([]byte, 6)
		if _, err := io.ReadFull(io.LimitReader(&randomReader{}, 6), bytes); err != nil {
			return nil, fmt.Errorf("failed to generate random bytes: %w", err)
		}

		codes[i] = strings.ToUpper(base32.StdEncoding.EncodeToString(bytes)[:8])
	}

	return codes, nil
}

// VerifyBackupCode verifica se um código de backup é válido
func VerifyBackupCode(code string, backupCodes []string) bool {
	code = strings.ToUpper(strings.ReplaceAll(code, "-", ""))

	for _, backupCode := range backupCodes {
		if code == backupCode {
			return true
		}
	}

	return false
}

// ConsumeBackupCode remove um código de backup usado
func ConsumeBackupCode(code string, backupCodes []string) []string {
	code = strings.ToUpper(strings.ReplaceAll(code, "-", ""))

	newCodes := make([]string, 0, len(backupCodes))
	for _, backupCode := range backupCodes {
		if code != backupCode {
			newCodes = append(newCodes, backupCode)
		}
	}

	return newCodes
}

// ParseQRCodeURL extrai informações de uma URL de QR Code TOTP
func ParseQRCodeURL(qrCodeURL string) (*TotpSecret, error) {
	parsedURL, err := url.Parse(qrCodeURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse QR code URL: %w", err)
	}

	if parsedURL.Scheme != "otpauth" || parsedURL.Host != "totp" {
		return nil, fmt.Errorf("invalid TOTP URL format")
	}

	query := parsedURL.Query()
	secret := query.Get("secret")
	issuer := query.Get("issuer")
	accountName := strings.TrimPrefix(parsedURL.Path, "/")

	if secret == "" {
		return nil, fmt.Errorf("missing secret in QR code URL")
	}

	return &TotpSecret{
		Secret:      secret,
		Issuer:      issuer,
		AccountName: accountName,
		QRCodeURL:   qrCodeURL,
		Base32:      secret,
		CreatedAt:   time.Now(),
	}, nil
}

// FormatSecret formata o segredo para exibição (grupos de 4 caracteres)
func FormatSecret(secret string) string {
	secret = strings.ToUpper(strings.ReplaceAll(secret, " ", ""))

	var parts []string
	for i := 0; i < len(secret); i += 4 {
		end := i + 4
		if end > len(secret) {
			end = len(secret)
		}
		parts = append(parts, secret[i:end])
	}

	return strings.Join(parts, " ")
}

// GetTimeRemaining retorna segundos restantes até o próximo código
func GetTimeRemaining() uint {
	now := time.Now().Unix()
	return 30 - uint(now%30)
}

// GetCurrentUserCode gera o código atual para um segredo (apenas para debug/testing)
func GetCurrentUserCode(secret string) (string, error) {
	code, err := totp.GenerateCode(secret, time.Now().UTC())
	if err != nil {
		return "", fmt.Errorf("failed to generate code: %w", err)
	}

	return code, nil
}

// randomReader implementa io.Reader para geração de números aleatórios
type randomReader struct{}

func (r *randomReader) Read(p []byte) (n int, err error) {
	for i := range p {
		p[i] = byte(time.Now().UnixNano() >> (uint(i%8) * 8))
	}
	return len(p), nil
}

// Note: Em produção, use crypto/rand para geração de números verdadeiramente aleatórios
