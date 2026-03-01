package auth

import (
	"errors"
	"regexp"
	"strings"
	"unicode"
)

var (
	ErrPasswordTooShort      = errors.New("password must be at least 8 characters")
	ErrPasswordNoUppercase   = errors.New("password must contain at least one uppercase letter")
	ErrPasswordNoLowercase   = errors.New("password must contain at least one lowercase letter")
	ErrPasswordNoNumber      = errors.New("password must contain at least one number")
	ErrPasswordNoSpecialChar = errors.New("password must contain at least one special character")
	ErrPasswordInCommonList  = errors.New("password is too common")
	ErrPasswordContainsEmail = errors.New("password cannot contain your email address")
)

// PasswordPolicy define as regras de validação de senha
type PasswordPolicy struct {
	MinLength            int
	RequireUppercase     bool
	RequireLowercase     bool
	RequireNumber        bool
	RequireSpecialChar   bool
	CheckCommonPasswords bool
	CheckEmailInPassword bool
}

// DefaultPasswordPolicy retorna política padrão segura
func DefaultPasswordPolicy() PasswordPolicy {
	return PasswordPolicy{
		MinLength:            8,
		RequireUppercase:     true,
		RequireLowercase:     true,
		RequireNumber:        true,
		RequireSpecialChar:   false, // Opcional para melhor UX
		CheckCommonPasswords: true,
		CheckEmailInPassword: true,
	}
}

// ValidatePassword valida uma senha contra a política
func ValidatePassword(password, email string) error {
	return ValidatePasswordWithPolicy(password, email, DefaultPasswordPolicy())
}

// ValidatePasswordWithPolicy valida senha com política customizada
func ValidatePasswordWithPolicy(password, email string, policy PasswordPolicy) error {
	if len(password) < policy.MinLength {
		return ErrPasswordTooShort
	}

	// Check uppercase
	if policy.RequireUppercase {
		hasUpper := false
		for _, r := range password {
			if unicode.IsUpper(r) {
				hasUpper = true
				break
			}
		}
		if !hasUpper {
			return ErrPasswordNoUppercase
		}
	}

	// Check lowercase
	if policy.RequireLowercase {
		hasLower := false
		for _, r := range password {
			if unicode.IsLower(r) {
				hasLower = true
				break
			}
		}
		if !hasLower {
			return ErrPasswordNoLowercase
		}
	}

	// Check number
	if policy.RequireNumber {
		hasNumber := false
		for _, r := range password {
			if unicode.IsNumber(r) {
				hasNumber = true
				break
			}
		}
		if !hasNumber {
			return ErrPasswordNoNumber
		}
	}

	// Check special character
	if policy.RequireSpecialChar {
		hasSpecial := false
		for _, r := range password {
			if !unicode.IsLetter(r) && !unicode.IsNumber(r) {
				hasSpecial = true
				break
			}
		}
		if !hasSpecial {
			return ErrPasswordNoSpecialChar
		}
	}

	// Check common passwords
	if policy.CheckCommonPasswords && isCommonPassword(password) {
		return ErrPasswordInCommonList
	}

	// Check if password contains email
	if policy.CheckEmailInPassword && email != "" {
		emailParts := strings.Split(strings.ToLower(email), "@")
		if len(emailParts) > 0 {
			username := emailParts[0]
			if len(username) >= 3 && strings.Contains(strings.ToLower(password), username) {
				return ErrPasswordContainsEmail
			}
		}
	}

	return nil
}

// PasswordStrength retorna a força da senha (0-4)
func PasswordStrength(password string) int {
	strength := 0

	// Length score
	if len(password) >= 8 {
		strength++
	}
	if len(password) >= 12 {
		strength++
	}
	if len(password) >= 16 {
		strength++
	}

	// Character variety score
	hasUpper := false
	hasLower := false
	hasNumber := false
	hasSpecial := false

	for _, r := range password {
		switch {
		case unicode.IsUpper(r):
			hasUpper = true
		case unicode.IsLower(r):
			hasLower = true
		case unicode.IsNumber(r):
			hasNumber = true
		case !unicode.IsLetter(r) && !unicode.IsNumber(r):
			hasSpecial = true
		}
	}

	variety := 0
	if hasUpper {
		variety++
	}
	if hasLower {
		variety++
	}
	if hasNumber {
		variety++
	}
	if hasSpecial {
		variety++
	}

	if variety >= 3 {
		strength++
	}
	if variety >= 4 {
		strength++
	}

	// Cap at 4
	if strength > 4 {
		strength = 4
	}

	return strength
}

// commonPasswords lista de senhas comuns (top 50)
var commonPasswords = map[string]bool{
	"password":     true,
	"123456":       true,
	"12345678":     true,
	"qwerty":       true,
	"abc123":       true,
	"monkey":       true,
	"1234567":      true,
	"letmein":      true,
	"trustno1":     true,
	"dragon":       true,
	"baseball":     true,
	"iloveyou":     true,
	"sunshine":     true,
	"ashley":       true,
	"bailey":       true,
	"shadow":       true,
	"123123":       true,
	"654321":       true,
	"superman":     true,
	"qazwsx":       true,
	"michael":      true,
	"football":     true,
	"password1":    true,
	"password123":  true,
	"welcome":      true,
	"jesus":        true,
	"ninja":        true,
	"mustang":      true,
	"password1234": true,
	"admin":        true,
	"admin123":     true,
	"root":         true,
	"toor":         true,
	"pass":         true,
	"test":         true,
	"guest":        true,
	"changeme":     true,
	"123456789":    true,
	"1234567890":   true,
	"000000":       true,
	"111111":       true,
	"121212":       true,
	"123321":       true,
	"666666":       true,
	"696969":       true,
	"7777777":      true,
	"888888":       true,
}

// isCommonPassword verifica se a senha está na lista de senhas comuns
func isCommonPassword(password string) bool {
	return commonPasswords[strings.ToLower(password)]
}

// Email validation
var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

// ValidateEmail valida formato de email
func ValidateEmail(email string) error {
	email = SanitizeEmail(email)

	if email == "" {
		return errors.New("email is required")
	}

	if len(email) > 254 {
		return errors.New("email must be at most 254 characters")
	}

	if !emailRegex.MatchString(email) {
		return errors.New("invalid email format")
	}

	// Check for consecutive dots
	if strings.Contains(email, "..") {
		return errors.New("email cannot contain consecutive dots")
	}

	// Check domain has valid format
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return errors.New("invalid email format")
	}

	domain := parts[1]
	if strings.HasPrefix(domain, "-") || strings.HasSuffix(domain, "-") {
		return errors.New("invalid domain format")
	}

	return nil
}

// SanitizeEmail normaliza e limpa email
func SanitizeEmail(email string) string {
	// Trim whitespace
	email = strings.TrimSpace(email)
	// Lowercase
	email = strings.ToLower(email)
	// Remove caracteres inválidos
	email = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') ||
			(r >= '0' && r <= '9') ||
			r == '@' || r == '.' || r == '-' || r == '_' || r == '+' {
			return r
		}
		return -1
	}, email)
	return email
}
