package validator

import (
	"sync"

	"github.com/go-playground/validator/v10"
)

// Validator encapsula o validador para evitar variáveis globais
type Validator struct {
	validate *validator.Validate
}

// New cria um novo Validator
func New() *Validator {
	v := &Validator{
		validate: validator.New(),
	}
	v.registerCustomValidations()
	return v
}

// registerCustomValidations registra validações customizadas
func (v *Validator) registerCustomValidations() {
	// Registrar validações customizadas aqui
	// Ex: v.validate.RegisterValidation("email", validateEmail)
}

// Validate verifica se a struct segue as regras das tags `validate`
func (v *Validator) Validate(s interface{}) error {
	return v.validate.Struct(s)
}

// ValidateStruct é um alias para Validate
func (v *Validator) ValidateStruct(s interface{}) error {
	return v.Validate(s)
}

// ValidateField valida um campo específico
func (v *Validator) ValidateField(field interface{}, tag string) error {
	return v.validate.Var(field, tag)
}

// Singleton com lazy initialization para uso em DI
var (
	instance *Validator
	once     sync.Once
)

// GetInstance retorna a instância singleton do Validator
// Use isso para injeção de dependência em vez de variável global
func GetInstance() *Validator {
	once.Do(func() {
		instance = New()
	})
	return instance
}

// ResetInstance reseta a instância singleton (útil para testes)
func ResetInstance() {
	once = sync.Once{}
	instance = nil
}
