package llm

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

var (
	ErrNoAPIKey        = errors.New("API key is required")
	ErrInvalidBaseURL  = errors.New("invalid base URL")
	ErrNilContext      = errors.New("context cannot be nil")
	ErrRequestFailed   = errors.New("request failed")
	ErrStreamClosed    = errors.New("stream closed")
	ErrMaxRetries      = errors.New("max retries exceeded")
	ErrStreamingFormat = errors.New("invalid streaming format")
)

type APIErrorResponse struct {
	Error APIError `json:"error"`
}

type APIError struct {
	StatusCode int
	Message    string
	Type       string
	Param      string
	Code       any
}

func (e *APIError) Error() string {
	return fmt.Sprintf("API error (status %d): %s", e.StatusCode, e.Message)
}

func (e *APIError) Unwrap() error {
	return errors.New("request failed")
}

type RateLimitError struct {
	APIError
	RetryAfter time.Duration
}

func (e *RateLimitError) Error() string {
	return fmt.Sprintf("rate limited (status %d): %s", e.StatusCode, e.Message)
}

type AuthenticationError struct {
	APIError
}

func (e *AuthenticationError) Error() string {
	return fmt.Sprintf("authentication failed (status %d): %s", e.StatusCode, e.Message)
}

type TimeoutError struct {
	APIError
}

func (e *TimeoutError) Error() string {
	return fmt.Sprintf("request timeout: %s", e.Message)
}

type InvalidRequestError struct {
	APIError
}

func (e *InvalidRequestError) Error() string {
	return fmt.Sprintf("invalid request (status %d): %s", e.StatusCode, e.Message)
}

func parseAPIError(statusCode int, body []byte) error {
	var resp APIErrorResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return &APIError{
			StatusCode: statusCode,
			Message:    string(body),
		}
	}

	apiErr := &APIError{
		StatusCode: statusCode,
		Message:    resp.Error.Message,
		Type:       resp.Error.Type,
		Param:      resp.Error.Param,
		Code:       resp.Error.Code,
	}

	switch statusCode {
	case http.StatusUnauthorized:
		return &AuthenticationError{APIError: *apiErr}
	case http.StatusTooManyRequests:
		retryAfter := parseRetryAfter(string(body))
		return &RateLimitError{
			APIError:   *apiErr,
			RetryAfter: retryAfter,
		}
	case http.StatusBadRequest:
		return &InvalidRequestError{APIError: *apiErr}
	case http.StatusGatewayTimeout, http.StatusRequestTimeout:
		return &TimeoutError{APIError: *apiErr}
	default:
		return apiErr
	}
}

func parseRetryAfter(body string) time.Duration {
	lowerBody := strings.ToLower(body)
	idx := strings.Index(lowerBody, "retry-after")
	if idx == -1 {
		return 0
	}

	rest := strings.TrimSpace(body[idx+len("retry-after"):])
	parts := strings.Fields(rest)
	if len(parts) == 0 {
		return 0
	}

	// Try parsing as seconds
	var n int
	if _, err := fmt.Sscanf(parts[0], "%d", &n); err == nil {
		return time.Duration(n) * time.Second
	}

	// Try parsing as Unix timestamp
	if ts, err := strconv.ParseInt(parts[0], 10, 64); err == nil {
		now := time.Now()
		retryTime := time.Unix(ts, 0)
		if retryTime.After(now) {
			return retryTime.Sub(now)
		}
	}

	return 0
}

func IsRateLimitError(err error) bool {
	var rateLimitErr *RateLimitError
	return errors.As(err, &rateLimitErr)
}

func IsAuthError(err error) bool {
	var authErr *AuthenticationError
	return errors.As(err, &authErr)
}

func IsTimeoutError(err error) bool {
	var timeoutErr *TimeoutError
	return errors.As(err, &timeoutErr)
}

func IsRetryableError(err error) bool {
	if IsRateLimitError(err) {
		return true
	}
	if IsTimeoutError(err) {
		return true
	}

	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.StatusCode >= 500 && apiErr.StatusCode < 600
	}

	return false
}
