package mailer

import (
	"testing"
)

func TestMockMailer_Send(t *testing.T) {
	mock := NewMock()

	err := mock.Send("to@example.com", "Test Subject", "<p>Test Body</p>")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if mock.GetEmailCount() != 1 {
		t.Errorf("expected 1 email, got %d", mock.GetEmailCount())
	}

	lastEmail := mock.GetLastEmail()
	if lastEmail.To != "to@example.com" {
		t.Errorf("expected to 'to@example.com', got %s", lastEmail.To)
	}
	if lastEmail.Subject != "Test Subject" {
		t.Errorf("expected subject 'Test Subject', got %s", lastEmail.Subject)
	}
	if lastEmail.Body != "<p>Test Body</p>" {
		t.Errorf("expected body '<p>Test Body</p>', got %s", lastEmail.Body)
	}
}

func TestMockMailer_SendMultiple(t *testing.T) {
	mock := NewMock()

	_ = mock.Send("user1@example.com", "Subject 1", "Body 1")
	_ = mock.Send("user2@example.com", "Subject 2", "Body 2")
	_ = mock.Send("user3@example.com", "Subject 3", "Body 3")

	if mock.GetEmailCount() != 3 {
		t.Errorf("expected 3 emails, got %d", mock.GetEmailCount())
	}
}

func TestMockMailer_SimulateError(t *testing.T) {
	mock := NewMock()
	mock.ShouldErr = true

	err := mock.Send("to@example.com", "Subject", "Body")
	if err == nil {
		t.Error("expected error, got nil")
	}

	if err != ErrSimulatedFailure {
		t.Errorf("expected ErrSimulatedFailure, got %v", err)
	}
}

func TestMockMailer_Reset(t *testing.T) {
	mock := NewMock()

	mock.Send("to@example.com", "Subject", "Body")
	if mock.GetEmailCount() != 1 {
		t.Fatalf("expected 1 email, got %d", mock.GetEmailCount())
	}

	mock.Reset()

	if mock.GetEmailCount() != 0 {
		t.Errorf("expected 0 emails after reset, got %d", mock.GetEmailCount())
	}
	if mock.GetLastEmail() != nil {
		t.Error("expected nil after reset")
	}
}

func TestMockMailer_GetLastEmail_Empty(t *testing.T) {
	mock := NewMock()

	if mock.GetLastEmail() != nil {
		t.Error("expected nil when no emails sent")
	}
}

func TestMockMailer_ImplementsSender(t *testing.T) {
	var _ Sender = NewMock()
	var _ Sender = (*MockMailer)(nil)
}
