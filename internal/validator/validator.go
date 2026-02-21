package validator

import (
	"fmt"
	"mime"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/go-playground/validator/v10"
)

var validate *validator.Validate

var (
	emailRegex    = regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	allowedImages = map[string]bool{
		".jpg":  true,
		".jpeg": true,
		".png":  true,
		".gif":  true,
		".webp": true,
	}
)

const (
	maxPasswordLength = 128
	minPasswordLength = 8
	maxEmailLength    = 254
)

func init() {
	validate = validator.New()
}

type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

type ValidationResult struct {
	Valid  bool              `json:"valid"`
	Errors []ValidationError `json:"errors,omitempty"`
}

func Validate(s any) error {
	return validate.Struct(s)
}

func ValidateEmail(email string) error {
	if email == "" {
		return fmt.Errorf("email é obrigatório")
	}
	if len(email) > maxEmailLength {
		return fmt.Errorf("email muito longo (máximo %d caracteres)", maxEmailLength)
	}
	if !emailRegex.MatchString(email) {
		return fmt.Errorf("formato de email inválido")
	}
	return nil
}

func ValidatePassword(password string) error {
	if password == "" {
		return fmt.Errorf("senha é obrigatória")
	}
	if len(password) < minPasswordLength {
		return fmt.Errorf("senha deve ter pelo menos %d caracteres", minPasswordLength)
	}
	if len(password) > maxPasswordLength {
		return fmt.Errorf("senha muito longa (máximo %d caracteres)", maxPasswordLength)
	}
	return nil
}

func ValidateRegistration(email, password string) ValidationResult {
	result := ValidationResult{Valid: true, Errors: []ValidationError{}}

	if err := ValidateEmail(email); err != nil {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{Field: "email", Message: err.Error()})
	}

	if err := ValidatePassword(password); err != nil {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{Field: "password", Message: err.Error()})
	}

	return result
}

func ValidateUpload(filename string, contentType string, maxSize int64) error {
	ext := strings.ToLower(filepath.Ext(filename))

	if !allowedImages[ext] {
		return fmt.Errorf("tipo de arquivo não permitido. Use: jpg, jpeg, png, gif, webp")
	}

	allowedContentTypes := map[string]bool{
		"image/jpeg": true,
		"image/png":  true,
		"image/gif":  true,
		"image/webp": true,
	}

	if !allowedContentTypes[contentType] {
		return fmt.Errorf("tipo de conteúdo não permitido")
	}

	mimeType := mime.TypeByExtension(ext)
	if mimeType != contentType && !strings.HasPrefix(contentType, "image/") {
		return fmt.Errorf("extensão não corresponde ao tipo do arquivo")
	}

	return nil
}

func SanitizeFilename(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	name := strings.TrimSuffix(filepath.Base(filename), ext)

	name = regexp.MustCompile(`[^a-zA-Z0-9_-]`).ReplaceAllString(name, "_")

	if len(name) > 50 {
		name = name[:50]
	}

	return name + ext
}
