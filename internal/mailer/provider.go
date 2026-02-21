package mailer

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/smtp"
	"strings"
	"sync"
	"time"

	"github.com/PauloHFS/goth/internal/config"
)

var (
	ErrRateLimitExceeded = errors.New("rate limit exceeded")
	ErrQuotaExceeded     = errors.New("quota exceeded")
	ErrInvalidAPIKey     = errors.New("invalid api key")
	ErrProviderNotActive = errors.New("provider not active")
)

type ProviderType string

const (
	ProviderSMTP     ProviderType = "smtp"
	ProviderResend   ProviderType = "resend"
	ProviderAWSES    ProviderType = "aws_ses"
	ProviderSendGrid ProviderType = "sendgrid"
)

type ProviderConfig struct {
	Type         ProviderType
	APIKey       string
	SecretKey    string
	Region       string
	FromEmail    string
	FromName     string
	DailyLimit   int
	MonthlyLimit int
}

type EmailProvider interface {
	Send(ctx context.Context, to, subject, body string) error
	SendBatch(ctx context.Context, emails []Email) error
	GetType() ProviderType
	IsAvailable() bool
}

type MultiProvider struct {
	providers []EmailProvider
	current   int
	mu        sync.Mutex
}

func NewMultiProvider(providers ...EmailProvider) *MultiProvider {
	return &MultiProvider{
		providers: providers,
		current:   0,
	}
}

func (mp *MultiProvider) Send(ctx context.Context, to, subject, body string) error {
	mp.mu.Lock()
	defer mp.mu.Unlock()

	var lastErr error
	for i := 0; i < len(mp.providers); i++ {
		idx := (mp.current + i) % len(mp.providers)
		provider := mp.providers[idx]

		if !provider.IsAvailable() {
			continue
		}

		if err := provider.Send(ctx, to, subject, body); err != nil {
			lastErr = err
			if isRateLimitError(err) {
				continue
			}
			return err
		}

		mp.current = idx
		return nil
	}

	if lastErr != nil {
		return lastErr
	}
	return ErrProviderNotActive
}

func (mp *MultiProvider) SendBatch(ctx context.Context, emails []Email) error {
	for _, email := range emails {
		if err := mp.Send(ctx, email.To, email.Subject, email.Body); err != nil {
			return err
		}
	}
	return nil
}

func isRateLimitError(err error) bool {
	if err == nil {
		return false
	}
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "rate limit") ||
		strings.Contains(errStr, "429") ||
		strings.Contains(errStr, "too many requests")
}

type SMTPProvider struct {
	addr     string
	auth     smtp.Auth
	from     string
	fromName string
}

func NewSMTPProvider(cfg *config.Config) *SMTPProvider {
	addr := fmt.Sprintf("%s:%s", cfg.SMTPHost, cfg.SMTPPort)
	var auth smtp.Auth
	if cfg.SMTPUser != "" {
		auth = smtp.PlainAuth("", cfg.SMTPUser, cfg.SMTPPass, cfg.SMTPHost)
	}

	return &SMTPProvider{
		addr:     addr,
		auth:     auth,
		from:     cfg.SMTPFrom,
		fromName: "",
	}
}

func (s *SMTPProvider) Send(ctx context.Context, to, subject, body string) error {
	header := fmt.Sprintf("To: %s\r\nSubject: %s\r\nContent-Type: text/html; charset=UTF-8\r\n\r\n", to, subject)
	msg := []byte(header + body)
	return smtp.SendMail(s.addr, s.auth, s.from, []string{to}, msg)
}

func (s *SMTPProvider) SendBatch(ctx context.Context, emails []Email) error {
	for _, email := range emails {
		if err := s.Send(ctx, email.To, email.Subject, email.Body); err != nil {
			return err
		}
	}
	return nil
}

func (s *SMTPProvider) GetType() ProviderType { return ProviderSMTP }
func (s *SMTPProvider) IsAvailable() bool     { return true }

type ResendProvider struct {
	apiKey    string
	fromEmail string
	fromName  string
	client    *http.Client
	baseURL   string
}

func NewResendProvider(apiKey, fromEmail, fromName string) *ResendProvider {
	return &ResendProvider{
		apiKey:    apiKey,
		fromEmail: fromEmail,
		fromName:  fromName,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		baseURL: "https://api.resend.com",
	}
}

type resendRequest struct {
	From    string `json:"from"`
	To      string `json:"to"`
	Subject string `json:"subject"`
	Html    string `json:"html"`
}

type resendError struct {
	Name    string `json:"name"`
	Message string `json:"message"`
}

func (r *ResendProvider) Send(ctx context.Context, to, subject, body string) error {
	from := r.fromEmail
	if r.fromName != "" {
		from = fmt.Sprintf("%s <%s>", r.fromName, r.fromEmail)
	}

	reqBody := resendRequest{
		From:    from,
		To:      to,
		Subject: subject,
		Html:    body,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.baseURL+"/emails", bytes.NewReader(bodyBytes))
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+r.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := r.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests {
		return ErrRateLimitExceeded
	}

	if resp.StatusCode >= 400 {
		var apiErr resendError
		if err := json.NewDecoder(resp.Body).Decode(&apiErr); err == nil {
			return fmt.Errorf("resend error: %s - %s", apiErr.Name, apiErr.Message)
		}
		return fmt.Errorf("resend error: status %d", resp.StatusCode)
	}

	return nil
}

func (r *ResendProvider) SendBatch(ctx context.Context, emails []Email) error {
	for _, email := range emails {
		if err := r.Send(ctx, email.To, email.Subject, email.Body); err != nil {
			return err
		}
	}
	return nil
}

func (r *ResendProvider) GetType() ProviderType { return ProviderResend }
func (r *ResendProvider) IsAvailable() bool     { return r.apiKey != "" }

type AWSESProvider struct {
	accessKey string
	secretKey string
	region    string
	fromEmail string
	fromName  string
	client    *http.Client
	endpoint  string
}

func NewAWSESProvider(accessKey, secretKey, region, fromEmail, fromName string) *AWSESProvider {
	endpoint := fmt.Sprintf("https://email.%s.amazonaws.com", region)
	return &AWSESProvider{
		accessKey: accessKey,
		secretKey: secretKey,
		region:    region,
		fromEmail: fromEmail,
		fromName:  fromName,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		endpoint: endpoint,
	}
}

func (a *AWSESProvider) Send(ctx context.Context, to, subject, body string) error {
	return errors.New("aws ses: not implemented - use aws-sdk-go-v2")
}

func (a *AWSESProvider) SendBatch(ctx context.Context, emails []Email) error {
	for _, email := range emails {
		if err := a.Send(ctx, email.To, email.Subject, email.Body); err != nil {
			return err
		}
	}
	return nil
}

func (a *AWSESProvider) GetType() ProviderType { return ProviderAWSES }
func (a *AWSESProvider) IsAvailable() bool {
	return a.accessKey != "" && a.secretKey != ""
}

func EncryptAPIKey(key string, secret []byte) (string, error) {
	block, err := aes.NewCipher(secret)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(key), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

func DecryptAPIKey(encrypted string, secret []byte) (string, error) {
	ciphertext, err := base64.StdEncoding.DecodeString(encrypted)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(secret)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return "", errors.New("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}

func ValidateEmailAddress(email string) bool {
	return strings.Contains(email, "@")
}
