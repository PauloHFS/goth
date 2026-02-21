package mailer

import (
	"errors"
	"fmt"
	"net/smtp"
	"strings"
	"sync"

	"github.com/PauloHFS/goth/internal/config"
)

var ErrSimulatedFailure = errors.New("simulated failure")

type Sender interface {
	Send(to, subject, body string) error
}

type Mailer struct {
	addr string
	auth smtp.Auth
	from string
}

func New(cfg *config.Config) *Mailer {
	addr := fmt.Sprintf("%s:%s", cfg.SMTPHost, cfg.SMTPPort)
	var auth smtp.Auth
	if cfg.SMTPUser != "" {
		auth = smtp.PlainAuth("", cfg.SMTPUser, cfg.SMTPPass, cfg.SMTPHost)
	}

	return &Mailer{
		addr: addr,
		auth: auth,
		from: cfg.SMTPFrom,
	}
}

func (m *Mailer) Send(to, subject, body string) error {
	header := fmt.Sprintf("To: %s\r\nSubject: %s\r\nContent-Type: text/html; charset=UTF-8\r\n\r\n", to, subject)
	msg := []byte(header + body)

	return smtp.SendMail(m.addr, m.auth, m.from, []string{to}, msg)
}

type Email struct {
	To      string
	Subject string
	Body    string
}

type MockMailer struct {
	mu        sync.Mutex
	emails    []Email
	ShouldErr bool
}

func NewMock() *MockMailer {
	return &MockMailer{}
}

func (m *MockMailer) Send(to, subject, body string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.ShouldErr {
		return ErrSimulatedFailure
	}

	m.emails = append(m.emails, Email{
		To:      to,
		Subject: subject,
		Body:    body,
	})
	return nil
}

func (m *MockMailer) GetEmailCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.emails)
}

func (m *MockMailer) GetLastEmail() *Email {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.emails) == 0 {
		return nil
	}
	return &m.emails[len(m.emails)-1]
}

func (m *MockMailer) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.emails = nil
	m.ShouldErr = false
}

func ValidateEmail(email string) bool {
	return strings.Contains(email, "@")
}
