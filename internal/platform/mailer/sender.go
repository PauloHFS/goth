package mailer

import (
	"context"
	"fmt"
	"net/smtp"
	"time"

	"github.com/PauloHFS/goth/internal/platform/config"
	"github.com/PauloHFS/goth/internal/platform/httpclient"
)

type Mailer struct {
	addr           string
	auth           smtp.Auth
	from           string
	circuitBreaker *httpclient.CircuitBreaker
}

func New(cfg *config.Config) *Mailer {
	addr := fmt.Sprintf("%s:%s", cfg.SMTPHost, cfg.SMTPPort)
	var auth smtp.Auth
	if cfg.SMTPUser != "" {
		auth = smtp.PlainAuth("", cfg.SMTPUser, cfg.SMTPPass, cfg.SMTPHost)
	}

	// Circuit breaker para SMTP
	cbConfig := httpclient.DefaultCircuitBreakerConfig()
	cbConfig.MaxFailures = 5
	cbConfig.Timeout = 60 * time.Second

	return &Mailer{
		addr:           addr,
		auth:           auth,
		from:           cfg.SMTPFrom,
		circuitBreaker: httpclient.NewCircuitBreaker("smtp", cbConfig),
	}
}

// Send envia um email com suporte a context para timeout/cancelamento
func (m *Mailer) Send(ctx context.Context, to, subject, body string) error {
	// Check circuit breaker
	if !m.circuitBreaker.Allow() {
		return fmt.Errorf("smtp circuit breaker is open")
	}

	header := fmt.Sprintf("To: %s\r\nSubject: %s\r\nContent-Type: text/html; charset=UTF-8\r\n\r\n", to, subject)
	msg := []byte(header + body)

	// Create channels for synchronization
	errChan := make(chan error, 1)
	done := make(chan struct{})

	go func() {
		defer close(done)
		errChan <- smtp.SendMail(m.addr, m.auth, m.from, []string{to}, msg)
	}()

	// Wait for either completion or context cancellation
	select {
	case <-ctx.Done():
		<-done // Wait for goroutine to complete to prevent leak
		m.circuitBreaker.RecordFailure()
		return fmt.Errorf("context cancelled: %w", ctx.Err())
	case err := <-errChan:
		<-done // Ensure goroutine completed
		if err != nil {
			m.circuitBreaker.RecordFailure()
			return fmt.Errorf("failed to send email: %w", err)
		}
		m.circuitBreaker.RecordSuccess()
		return nil
	}
}

// SendWithTimeout envia um email com timeout específico
func (m *Mailer) SendWithTimeout(to, subject, body string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return m.Send(ctx, to, subject, body)
}

// SendLegacy é uma versão legacy sem context para compatibilidade
// Deprecated: Use Send(ctx, to, subject, body) instead
func (m *Mailer) SendLegacy(to, subject, body string) error {
	return m.Send(context.Background(), to, subject, body)
}
