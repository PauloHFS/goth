package llm

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	llmRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "llm_request_duration_seconds",
		Help:    "LLM request duration in seconds",
		Buckets: []float64{0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
	}, []string{"method", "model", "status"})

	llmRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "llm_requests_total",
		Help: "Total number of LLM requests",
	}, []string{"method", "model"})

	llmErrorsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "llm_errors_total",
		Help: "Total number of LLM errors",
	}, []string{"method", "model", "error_type"})

	llmTokensTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "llm_tokens_total",
		Help: "Total number of tokens used",
	}, []string{"method", "model", "token_type"})
)

type MetricsCollector struct{}

func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{}
}

func recordRequest(method, model, status string, duration time.Duration) {
	llmRequestDuration.WithLabelValues(method, model, status).Observe(duration.Seconds())
	llmRequestsTotal.WithLabelValues(method, model).Inc()
}

func recordError(method, model, errorType string) {
	llmErrorsTotal.WithLabelValues(method, model, errorType).Inc()
}

func recordTokens(method, model string, usage Usage) {
	if usage.PromptTokens > 0 {
		llmTokensTotal.WithLabelValues(method, model, "prompt").Add(float64(usage.PromptTokens))
	}
	if usage.CompletionTokens > 0 {
		llmTokensTotal.WithLabelValues(method, model, "completion").Add(float64(usage.CompletionTokens))
	}
	if usage.TotalTokens > 0 {
		llmTokensTotal.WithLabelValues(method, model, "total").Add(float64(usage.TotalTokens))
	}
}

type TracedClient struct {
	client *Client
}

func NewTracedClient(client *Client) *TracedClient {
	return &TracedClient{client: client}
}

func (t *TracedClient) Generate(ctx context.Context, req CompletionRequest) (*CompletionResponse, error) {
	start := time.Now()
	status := "success"
	model := req.Model

	defer func() {
		recordRequest("generate", model, status, time.Since(start))
	}()

	resp, err := t.client.Generate(ctx, req)
	if err != nil {
		status = "error"
		recordError("generate", model, classifyError(err))
		return nil, err
	}

	if resp != nil {
		recordTokens("generate", model, resp.Usage)
	}

	return resp, nil
}

func (t *TracedClient) Stream(ctx context.Context, req CompletionRequest) (<-chan StreamChunk, error) {
	model := req.Model

	llmRequestsTotal.WithLabelValues("stream", model).Inc()

	ch, err := t.client.Stream(ctx, req)
	if err != nil {
		recordError("stream", model, classifyError(err))
		return nil, err
	}

	wrappedCh := make(chan StreamChunk)

	go func() {
		defer close(wrappedCh)
		start := time.Now()
		var lastUsage Usage

		for chunk := range ch {
			wrappedCh <- chunk
			if chunk.Usage != nil {
				lastUsage = *chunk.Usage
			}
			if len(chunk.Choices) > 0 && chunk.Choices[0].FinishReason != "" {
				recordRequest("stream", model, "success", time.Since(start))
				recordTokens("stream", model, lastUsage)
			}
		}
	}()

	return wrappedCh, nil
}

func (t *TracedClient) Embed(ctx context.Context, req EmbeddingRequest) (*EmbeddingResponse, error) {
	start := time.Now()
	status := "success"
	model := req.Model

	defer func() {
		recordRequest("embed", model, status, time.Since(start))
	}()

	resp, err := t.client.Embed(ctx, req)
	if err != nil {
		status = "error"
		recordError("embed", model, classifyError(err))
		return nil, err
	}

	if resp != nil {
		recordTokens("embed", model, resp.Usage)
	}

	return resp, nil
}

func (c *Client) WithMetrics() *TracedClient {
	return NewTracedClient(c)
}

func classifyError(err error) string {
	if err == nil {
		return "none"
	}

	if IsAuthError(err) {
		return "auth"
	}
	if IsRateLimitError(err) {
		return "rate_limit"
	}
	if IsTimeoutError(err) {
		return "timeout"
	}

	var apiErr *APIError
	if errors.As(err, &apiErr) {
		if apiErr.StatusCode >= 400 && apiErr.StatusCode < 500 {
			return "client_error"
		}
		if apiErr.StatusCode >= 500 {
			return "server_error"
		}
	}

	return "unknown"
}

type MetricsMiddleware struct {
	client LLMClient
}

func NewMetricsMiddleware(client LLMClient) *MetricsMiddleware {
	return &MetricsMiddleware{client: client}
}

func (m *MetricsMiddleware) Generate(ctx context.Context, req CompletionRequest) (*CompletionResponse, error) {
	start := time.Now()
	model := req.Model
	status := "success"

	resp, err := m.client.Generate(ctx, req)

	duration := time.Since(start)
	if err != nil {
		status = "error"
	}

	recordRequest("generate", model, status, duration)

	if err != nil {
		recordError("generate", model, classifyError(err))
	} else if resp != nil {
		recordTokens("generate", model, resp.Usage)
	}

	return resp, err
}

func (m *MetricsMiddleware) Stream(ctx context.Context, req CompletionRequest) (<-chan StreamChunk, error) {
	return m.client.Stream(ctx, req)
}

func (m *MetricsMiddleware) Embed(ctx context.Context, req EmbeddingRequest) (*EmbeddingResponse, error) {
	start := time.Now()
	model := req.Model
	status := "success"

	resp, err := m.client.Embed(ctx, req)

	duration := time.Since(start)
	if err != nil {
		status = "error"
	}

	recordRequest("embed", model, status, duration)

	if err != nil {
		recordError("embed", model, classifyError(err))
	} else if resp != nil {
		recordTokens("embed", model, resp.Usage)
	}

	return resp, err
}

type RequestMetadata struct {
	Model            string
	RequestID        string
	DurationMs       int64
	StatusCode       int
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
	Error            string
}

func (m *MetricsCollector) RecordRequest(metadata RequestMetadata) {
	status := strconv.Itoa(metadata.StatusCode)
	if metadata.Error != "" {
		status = "error"
	}
	llmRequestDuration.WithLabelValues("generate", metadata.Model, status).Observe(float64(metadata.DurationMs) / 1000)
	llmRequestsTotal.WithLabelValues("generate", metadata.Model).Inc()
}
