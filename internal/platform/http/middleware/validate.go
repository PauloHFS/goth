package middleware

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/PauloHFS/goth/internal/contextkeys"
)

// Validatable é a interface para structs que podem ser validadas.
// Implemente esta interface nos seus input structs para validação automática.
//
// Exemplo:
//
//	type RegisterInput struct {
//	    Email    string `json:"email"`
//	    Password string `json:"password"`
//	}
//
//	func (i RegisterInput) Validate() error {
//	    if i.Email == "" {
//	        return errors.New("email is required")
//	    }
//	    if len(i.Password) < 8 {
//	        return errors.New("password must be at least 8 characters")
//	    }
//	    return nil
//	}
type Validatable interface {
	Validate() error
}

// ValidationError representa um erro de validação estruturado
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

func (e ValidationError) Error() string {
	return e.Message
}

// ValidationErrors representa múltiplos erros de validação
type ValidationErrors []ValidationError

func (e ValidationErrors) Error() string {
	if len(e) == 0 {
		return ""
	}
	return e[0].Message
}

// JSON retorna os erros em formato JSON
func (e ValidationErrors) JSON() []byte {
	data, _ := json.Marshal(e)
	return data
}

// ValidateInput é um middleware que valida inputs no contexto.
// O handler deve colocar o input validável no contexto antes deste middleware rodar.
//
// Uso recomendado: criar um wrapper handler que:
// 1. Parseia o input do request
// 2. Coloca no contexto com contextkeys.ValidatableKey
// 3. Chama o próximo handler que será validado por este middleware
func ValidateInput(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Tentar pegar input validável do contexto
		if v, ok := r.Context().Value(contextkeys.ValidatableKey).(Validatable); ok {
			if err := v.Validate(); err != nil {
				respondValidationError(w, r, err)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// respondValidationError retorna erro de validação padronizado
func respondValidationError(w http.ResponseWriter, r *http.Request, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Request-ID", GetRequestID(r.Context()))

	// Check se é ValidationErrors
	if ve, ok := err.(ValidationErrors); ok {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write(ve.JSON())
		return
	}

	// Erro genérico de validação
	w.WriteHeader(http.StatusBadRequest)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error":   "validation_error",
		"message": err.Error(),
	})
}

// WithValidatable coloca um input validável no contexto
// Use isso nos seus handlers antes de chamar o próximo middleware
func WithValidatable(ctx context.Context, v Validatable) context.Context {
	return context.WithValue(ctx, contextkeys.ValidatableKey, v)
}
