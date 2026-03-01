package worker

import (
	"context"
	"database/sql"
	"sync"
	"time"

	"github.com/PauloHFS/goth/internal/db"
)

// CircuitState representa o estado do circuit breaker
type CircuitState string

const (
	StateClosed   CircuitState = "closed"    // Normal operation
	StateOpen     CircuitState = "open"      // Failing fast
	StateHalfOpen CircuitState = "half-open" // Testing if recovered
)

// CircuitBreakerConfig configurações do circuit breaker
type CircuitBreakerConfig struct {
	FailureThreshold int           // Número de falhas para abrir
	SuccessThreshold int           // Número de sucessos para fechar
	Timeout          time.Duration // Tempo que fica aberto antes de tentar de novo
}

// DefaultCircuitBreakerConfig configurações default
var DefaultCircuitBreakerConfig = CircuitBreakerConfig{
	FailureThreshold: 5,
	SuccessThreshold: 3,
	Timeout:          30 * time.Second,
}

// CircuitBreaker gerencia estado de circuit breaker por job type
type CircuitBreaker struct {
	config  CircuitBreakerConfig
	queries *db.Queries
	states  map[string]*circuitState
	mu      sync.RWMutex
}

type circuitState struct {
	state       CircuitState
	failures    int
	successes   int
	lastFailure time.Time
	lastSuccess time.Time
	openedAt    time.Time
}

// NewCircuitBreaker cria um novo circuit breaker
func NewCircuitBreaker(queries *db.Queries, config CircuitBreakerConfig) *CircuitBreaker {
	if config.FailureThreshold == 0 {
		config = DefaultCircuitBreakerConfig
	}

	cb := &CircuitBreaker{
		config:  config,
		queries: queries,
		states:  make(map[string]*circuitState),
	}

	// Carrega estados do banco (se queries estiver disponível)
	if queries != nil {
		cb.loadStates(context.Background())
	}

	return cb
}

// loadStates carrega estados do banco de dados
func (cb *CircuitBreaker) loadStates(ctx context.Context) {
	states, err := cb.queries.ListCircuitBreakerStates(ctx)
	if err != nil {
		return
	}

	cb.mu.Lock()
	defer cb.mu.Unlock()

	for _, s := range states {
		cb.states[s.JobType] = &circuitState{
			state:       CircuitState(s.State),
			failures:    int(s.FailureCount),
			successes:   int(s.SuccessCount),
			lastFailure: s.LastFailureAt.Time,
			lastSuccess: s.LastSuccessAt.Time,
			openedAt:    s.OpenedAt.Time,
		}
	}
}

// Allow verifica se o job type pode ser processado
func (cb *CircuitBreaker) Allow(jobType string) bool {
	cb.mu.RLock()
	state, exists := cb.states[jobType]
	cb.mu.RUnlock()

	if !exists {
		// Se não existe, assume closed
		return true
	}

	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch state.state {
	case StateClosed:
		return true

	case StateOpen:
		// Verifica se timeout passou
		if time.Since(state.openedAt) > cb.config.Timeout {
			// Muda para half-open
			state.state = StateHalfOpen
			return true
		}
		return false

	case StateHalfOpen:
		// Permite tentativa limitada
		return true

	default:
		return true
	}
}

// RecordSuccess registra sucesso de um job
func (cb *CircuitBreaker) RecordSuccess(ctx context.Context, jobType string) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	state, exists := cb.states[jobType]
	if !exists {
		state = &circuitState{}
		cb.states[jobType] = state
	}

	state.successes++
	state.failures = 0
	state.lastSuccess = time.Now()

	// Se estava half-open e atingiu threshold, fecha
	if state.state == StateHalfOpen && state.successes >= cb.config.SuccessThreshold {
		state.state = StateClosed
		state.openedAt = time.Time{}
	} else if state.state == StateClosed {
		// Persiste no banco
		go cb.persistState(ctx, jobType, state)
	}
}

// RecordFailure registra falha de um job
func (cb *CircuitBreaker) RecordFailure(ctx context.Context, jobType string) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	state, exists := cb.states[jobType]
	if !exists {
		state = &circuitState{}
		cb.states[jobType] = state
	}

	state.failures++
	state.lastFailure = time.Now()
	state.successes = 0

	// Se atingiu threshold, abre o circuit
	if state.failures >= cb.config.FailureThreshold {
		state.state = StateOpen
		state.openedAt = time.Now()
	}

	// Persiste no banco
	go cb.persistState(ctx, jobType, state)
}

// persisteState persiste estado no banco
func (cb *CircuitBreaker) persistState(ctx context.Context, jobType string, state *circuitState) {
	_, err := cb.queries.UpsertCircuitBreakerState(ctx, db.UpsertCircuitBreakerStateParams{
		JobType:       jobType,
		State:         string(state.state),
		FailureCount:  int64(state.failures),
		SuccessCount:  int64(state.successes),
		LastFailureAt: sql.NullTime{Time: state.lastFailure, Valid: !state.lastFailure.IsZero()},
		LastSuccessAt: sql.NullTime{Time: state.lastSuccess, Valid: !state.lastSuccess.IsZero()},
		OpenedAt:      sql.NullTime{Time: state.openedAt, Valid: !state.openedAt.IsZero()},
	})
	if err != nil {
		// Log error but don't block
		// TODO: Add proper logging with slog
		return
	}
}

// GetState retorna o estado atual de um job type
func (cb *CircuitBreaker) GetState(jobType string) CircuitState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	if state, exists := cb.states[jobType]; exists {
		return state.state
	}
	return StateClosed
}

// GetAllStates retorna todos os estados
func (cb *CircuitBreaker) GetAllStates() map[string]CircuitState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	result := make(map[string]CircuitState)
	for jobType, state := range cb.states {
		result[jobType] = state.state
	}
	return result
}
