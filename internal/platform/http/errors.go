package http

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"runtime"

	"github.com/PauloHFS/goth/internal/platform/http/middleware"
)

// Error types
type ErrorType string

const (
	ErrorTypeValidation   ErrorType = "validation_error"
	ErrorTypeNotFound     ErrorType = "not_found"
	ErrorTypeUnauthorized ErrorType = "unauthorized"
	ErrorTypeForbidden    ErrorType = "forbidden"
	ErrorTypeConflict     ErrorType = "conflict"
	ErrorTypeInternal     ErrorType = "internal_error"
	ErrorTypeTimeout      ErrorType = "timeout"
	ErrorTypeService      ErrorType = "service_error"
)

// AppError representa um erro de aplicação
type AppError struct {
	Type       ErrorType      `json:"type"`
	Message    string         `json:"message"`
	Code       string         `json:"code"`
	HTTPStatus int            `json:"-"`
	Err        error          `json:"-"`
	Metadata   map[string]any `json:"metadata,omitempty"`
}

func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	return e.Message
}

func (e *AppError) Unwrap() error {
	return e.Err
}

// Response structure para erros HTTP
type ErrorResponse struct {
	Error   string         `json:"error"`
	Message string         `json:"message"`
	Code    string         `json:"code,omitempty"`
	Details map[string]any `json:"details,omitempty"`
}

// HandleError trata um erro e escreve resposta HTTP
func HandleError(w http.ResponseWriter, r *http.Request, err error, operation string) {
	var appErr *AppError

	if errors.As(err, &appErr) {
		logAppError(appErr, operation, r)
		writeAppError(w, r, appErr)
		return
	}

	// Erro genérico = internal server error
	appErr = &AppError{
		Type:       ErrorTypeInternal,
		Message:    "Internal server error",
		Code:       "INTERNAL_ERROR",
		HTTPStatus: http.StatusInternalServerError,
		Err:        err,
	}

	logAppError(appErr, operation, r)
	writeAppError(w, r, appErr)
}

func logAppError(err *AppError, operation string, r *http.Request) {
	pc, file, line, ok := runtime.Caller(2)
	funcName := "unknown"
	if ok {
		funcName = runtime.FuncForPC(pc).Name()
	}

	slog.Error("app_error",
		"operation", operation,
		"error_type", err.Type,
		"error_code", err.Code,
		"message", err.Message,
		"error", err.Err,
		"path", r.URL.Path,
		"method", r.Method,
		"function", funcName,
		"file", file,
		"line", line,
	)
}

func writeAppError(w http.ResponseWriter, r *http.Request, err *AppError) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Request-ID", middleware.GetRequestID(r.Context()))
	w.WriteHeader(err.HTTPStatus)

	resp := ErrorResponse{
		Error:   string(err.Type),
		Message: err.Message,
		Code:    err.Code,
		Details: err.Metadata,
	}

	// Ignora erro de encode
	_ = json.NewEncoder(w).Encode(resp)
}

// Factory functions para criar erros

// NewValidationError cria erro de validação (400)
func NewValidationError(message string, metadata map[string]any) *AppError {
	return &AppError{
		Type:       ErrorTypeValidation,
		Message:    message,
		Code:       "VALIDATION_ERROR",
		HTTPStatus: http.StatusBadRequest,
		Metadata:   metadata,
	}
}

// NewNotFoundError cria erro de não encontrado (404)
func NewNotFoundError(resource string) *AppError {
	return &AppError{
		Type:       ErrorTypeNotFound,
		Message:    fmt.Sprintf("%s not found", resource),
		Code:       "NOT_FOUND",
		HTTPStatus: http.StatusNotFound,
	}
}

// NewUnauthorizedError cria erro de não autorizado (401)
func NewUnauthorizedError(message string) *AppError {
	if message == "" {
		message = "Unauthorized"
	}
	return &AppError{
		Type:       ErrorTypeUnauthorized,
		Message:    message,
		Code:       "UNAUTHORIZED",
		HTTPStatus: http.StatusUnauthorized,
	}
}

// NewForbiddenError cria erro de proibido (403)
func NewForbiddenError(message string) *AppError {
	if message == "" {
		message = "Forbidden"
	}
	return &AppError{
		Type:       ErrorTypeForbidden,
		Message:    message,
		Code:       "FORBIDDEN",
		HTTPStatus: http.StatusForbidden,
	}
}

// NewConflictError cria erro de conflito (409)
func NewConflictError(resource string, message string) *AppError {
	if message == "" {
		message = fmt.Sprintf("%s already exists", resource)
	}
	return &AppError{
		Type:       ErrorTypeConflict,
		Message:    message,
		Code:       "CONFLICT",
		HTTPStatus: http.StatusConflict,
	}
}

// NewInternalError cria erro interno (500)
func NewInternalError(message string, err error) *AppError {
	if message == "" {
		message = "Internal server error"
	}
	return &AppError{
		Type:       ErrorTypeInternal,
		Message:    message,
		Code:       "INTERNAL_ERROR",
		HTTPStatus: http.StatusInternalServerError,
		Err:        err,
	}
}

// NewTimeoutError cria erro de timeout (504)
func NewTimeoutError(message string) *AppError {
	if message == "" {
		message = "Request timeout"
	}
	return &AppError{
		Type:       ErrorTypeTimeout,
		Message:    message,
		Code:       "TIMEOUT",
		HTTPStatus: http.StatusGatewayTimeout,
	}
}

// NewServiceError cria erro de serviço externo (503)
func NewServiceError(service, message string, err error) *AppError {
	if message == "" {
		message = fmt.Sprintf("Service %s unavailable", service)
	}
	return &AppError{
		Type:       ErrorTypeService,
		Message:    message,
		Code:       "SERVICE_ERROR",
		HTTPStatus: http.StatusServiceUnavailable,
		Err:        err,
	}
}

// WithError adiciona erro wrapped
func (e *AppError) WithError(err error) *AppError {
	e.Err = err
	return e
}

// WithMetadata adiciona metadata
func (e *AppError) WithMetadata(key string, value any) *AppError {
	if e.Metadata == nil {
		e.Metadata = make(map[string]any)
	}
	e.Metadata[key] = value
	return e
}
