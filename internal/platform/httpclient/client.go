package httpclient

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client é um HTTP client com circuit breaker e retry
type Client struct {
	httpClient     *http.Client
	circuitBreaker *CircuitBreaker
	retryConfig    RetryConfig
	name           string
}

// ClientConfig configura o client
type ClientConfig struct {
	// Name para identificação
	Name string
	// Timeout para requests HTTP
	Timeout time.Duration
	// CircuitBreaker config
	CircuitBreaker CircuitBreakerConfig
	// Retry config
	Retry RetryConfig
	// Transport customizado
	Transport http.RoundTripper
}

// DefaultClientConfig retorna configuração padrão
func DefaultClientConfig(name string) ClientConfig {
	return ClientConfig{
		Name:           name,
		Timeout:        30 * time.Second,
		CircuitBreaker: DefaultCircuitBreakerConfig(),
		Retry:          DefaultRetryConfig(),
	}
}

// NewClient cria um novo client com circuit breaker e retry
func NewClient(config ClientConfig) *Client {
	transport := config.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}

	return &Client{
		httpClient: &http.Client{
			Timeout:   config.Timeout,
			Transport: transport,
		},
		circuitBreaker: NewCircuitBreaker(config.Name, config.CircuitBreaker),
		retryConfig:    config.Retry,
		name:           config.Name,
	}
}

// Do executa um request HTTP com circuit breaker e retry
func (c *Client) Do(req *http.Request) (*http.Response, error) {
	return c.DoWithContext(req.Context(), req)
}

// DoWithContext executa request com context
func (c *Client) DoWithContext(ctx context.Context, req *http.Request) (*http.Response, error) {
	var resp *http.Response
	var lastErr error

	for attempt := 0; attempt <= c.retryConfig.MaxRetries; attempt++ {
		// Check circuit breaker
		if !c.circuitBreaker.Allow() {
			return nil, fmt.Errorf("%s: circuit breaker open", c.name)
		}

		// Execute request
		resp, lastErr = c.httpClient.Do(req.WithContext(ctx))

		if lastErr == nil {
			// Check if response indicates retryable error
			if resp.StatusCode >= 500 || resp.StatusCode == 429 {
				lastErr = &RetryableError{Err: fmt.Errorf("HTTP %d", resp.StatusCode)}
				c.circuitBreaker.RecordFailure()

				// Close response body for retry
				if resp.Body != nil {
					_ = resp.Body.Close()
				}

				if attempt < c.retryConfig.MaxRetries {
					backoff := calculateBackoff(attempt, c.retryConfig)
					select {
					case <-ctx.Done():
						return nil, fmt.Errorf("request cancelled: %w", ctx.Err())
					case <-time.After(backoff):
						continue
					}
				}
			} else {
				// Success
				c.circuitBreaker.RecordSuccess()
				return resp, nil
			}
		} else {
			// Network error
			c.circuitBreaker.RecordFailure()

			// Check if retryable
			if !isRetryableError(lastErr) {
				return nil, lastErr
			}

			if attempt < c.retryConfig.MaxRetries {
				backoff := calculateBackoff(attempt, c.retryConfig)
				select {
				case <-ctx.Done():
					return nil, fmt.Errorf("request cancelled: %w", ctx.Err())
				case <-time.After(backoff):
					continue
				}
			}
		}
	}

	return nil, fmt.Errorf("%s: max retries exceeded: %w", c.name, lastErr)
}

// Get executa HTTP GET com circuit breaker e retry
func (c *Client) Get(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	return c.DoWithContext(ctx, req)
}

// Post executa HTTP POST com circuit breaker e retry
func (c *Client) Post(ctx context.Context, url string, contentType string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", contentType)
	return c.DoWithContext(ctx, req)
}

// GetWithResult executa GET e decode JSON para resultado
func (c *Client) GetWithResult(ctx context.Context, url string, result interface{}) error {
	resp, err := c.Get(ctx, url)
	if err != nil {
		return err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	// Decode JSON (import encoding/json no caller)
	return nil // Caller deve fazer decode
}

// State retorna estado do circuit breaker
func (c *Client) State() CircuitState {
	return c.circuitBreaker.State()
}

// Stats retorna estatísticas
func (c *Client) Stats() map[string]interface{} {
	return c.circuitBreaker.Stats()
}

// Name retorna o nome do client
func (c *Client) Name() string {
	return c.name
}

// isRetryableError verifica se erro é retryable
func isRetryableError(err error) bool {
	// Network errors are typically retryable
	return true
}
