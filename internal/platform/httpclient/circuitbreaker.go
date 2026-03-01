package httpclient

import (
	"errors"
	"sync"
	"time"
)

// CircuitState representa o estado do circuit breaker
type CircuitState int

const (
	StateClosed   CircuitState = iota // Normal operation
	StateOpen                         // Failing, reject requests
	StateHalfOpen                     // Testing if service recovered
)

func (s CircuitState) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// CircuitBreakerConfig configura o circuit breaker
type CircuitBreakerConfig struct {
	// MaxFailures antes de abrir o circuit
	MaxFailures int
	// Timeout para abrir o circuit (quando em estado open)
	Timeout time.Duration
	// HalfOpenMaxRequests permite testar alguns requests em half-open
	HalfOpenMaxRequests int
	// SuccessThreshold para fechar o circuit (requests success em half-open)
	SuccessThreshold int
}

// DefaultCircuitBreakerConfig retorna configuração padrão
func DefaultCircuitBreakerConfig() CircuitBreakerConfig {
	return CircuitBreakerConfig{
		MaxFailures:         5,
		Timeout:             30 * time.Second,
		HalfOpenMaxRequests: 3,
		SuccessThreshold:    2,
	}
}

// CircuitBreaker implementa o pattern circuit breaker
type CircuitBreaker struct {
	mu               sync.RWMutex
	state            CircuitState
	failures         int
	successes        int
	lastFailure      time.Time
	halfOpenReqs     int
	successThreshold int
	config           CircuitBreakerConfig
	name             string
}

// NewCircuitBreaker cria um novo circuit breaker
func NewCircuitBreaker(name string, config CircuitBreakerConfig) *CircuitBreaker {
	return &CircuitBreaker{
		state:            StateClosed,
		config:           config,
		name:             name,
		successThreshold: config.SuccessThreshold,
	}
}

// Allow verifica se o request pode prosseguir
func (cb *CircuitBreaker) Allow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case StateClosed:
		return true

	case StateOpen:
		// Check se timeout passou
		if time.Since(cb.lastFailure) > cb.config.Timeout {
			cb.state = StateHalfOpen
			cb.halfOpenReqs = 0
			cb.successes = 0
			return true
		}
		return false

	case StateHalfOpen:
		// Allow limited requests in half-open state
		if cb.halfOpenReqs < cb.config.HalfOpenMaxRequests {
			cb.halfOpenReqs++
			return true
		}
		return false
	}

	return false
}

// RecordSuccess registra um request bem-sucedido
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case StateHalfOpen:
		cb.successes++
		if cb.successes >= cb.successThreshold {
			cb.state = StateClosed
			cb.failures = 0
			cb.successes = 0
			cb.halfOpenReqs = 0
		}

	case StateClosed:
		// Reset failures on success
		cb.failures = 0
	}
}

// RecordFailure registra um request falho
func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failures++
	cb.lastFailure = time.Now()

	switch cb.state {
	case StateClosed:
		if cb.failures >= cb.config.MaxFailures {
			cb.state = StateOpen
		}

	case StateHalfOpen:
		// Immediately go back to open on failure
		cb.state = StateOpen
		cb.halfOpenReqs = 0
		cb.successes = 0
	}
}

// State retorna o estado atual
func (cb *CircuitBreaker) State() CircuitState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// Stats retorna estatísticas do circuit breaker
func (cb *CircuitBreaker) Stats() map[string]interface{} {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	return map[string]interface{}{
		"state":           cb.state.String(),
		"failures":        cb.failures,
		"successes":       cb.successes,
		"half_open_reqs":  cb.halfOpenReqs,
		"last_failure":    cb.lastFailure,
		"max_failures":    cb.config.MaxFailures,
		"timeout_seconds": cb.config.Timeout.Seconds(),
	}
}

// ErrCircuitOpen é retornado quando o circuit está aberto
var ErrCircuitOpen = errors.New("circuit breaker is open")

// Execute executa uma função com circuit breaker
func (cb *CircuitBreaker) Execute(fn func() error) error {
	if !cb.Allow() {
		return ErrCircuitOpen
	}

	err := fn()

	if err != nil {
		cb.RecordFailure()
	} else {
		cb.RecordSuccess()
	}

	return err
}

// ExecuteWithFallback executa fn ou fallback se circuit aberto
func (cb *CircuitBreaker) ExecuteWithFallback(fn func() error, fallback func() error) error {
	if !cb.Allow() {
		if fallback != nil {
			return fallback()
		}
		return ErrCircuitOpen
	}

	err := fn()

	if err != nil {
		cb.RecordFailure()
	} else {
		cb.RecordSuccess()
	}

	return err
}
