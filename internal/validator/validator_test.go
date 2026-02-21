package validator

import "testing"

func TestValidateEmail(t *testing.T) {
	tests := []struct {
		name    string
		email   string
		wantErr bool
	}{
		{"valid email", "test@example.com", false},
		{"valid email with subdomain", "user@sub.domain.com", false},
		{"invalid email no @", "testexample.com", true},
		{"invalid email no domain", "test@", true},
		{"invalid email no user", "@example.com", true},
		{"empty email", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateEmail(tt.email)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateEmail() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidatePassword(t *testing.T) {
	tests := []struct {
		name    string
		pwd     string
		wantErr bool
	}{
		{"valid password", "password123", false},
		{"valid password long", "verylongpassword1234567890", false},
		{"short password", "pass", true},
		{"empty password", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePassword(tt.pwd)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePassword() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateRegistration(t *testing.T) {
	tests := []struct {
		name      string
		email     string
		password  string
		wantValid bool
	}{
		{"valid registration", "test@example.com", "password123", true},
		{"invalid email", "invalid", "password123", false},
		{"short password", "test@example.com", "123", false},
		{"both invalid", "invalid", "123", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateRegistration(tt.email, tt.password)
			if result.Valid != tt.wantValid {
				t.Errorf("ValidateRegistration() valid = %v, wantValid %v", result.Valid, tt.wantValid)
			}
		})
	}
}

func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"normal file", "avatar.jpg", "avatar.jpg"},
		{"spaces replaced", "my avatar.png", "my_avatar.png"},
		{"special chars removed", "test@file#1.txt", "test_file_1.txt"},
		{"long name truncated", "this_is_a_very_long_filename_that_exceeds_fifty_characters.png", "this_is_a_very_long_filename_that_exceeds_fifty_ch.png"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeFilename(tt.input)
			if result != tt.expected {
				t.Errorf("SanitizeFilename() = %v, want %v", result, tt.expected)
			}
		})
	}
}
