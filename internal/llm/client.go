package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"strings"
	"time"
)

type LLMClient interface {
	Generate(ctx context.Context, req CompletionRequest) (*CompletionResponse, error)
	Stream(ctx context.Context, req CompletionRequest) (<-chan StreamChunk, error)
	Embed(ctx context.Context, req EmbeddingRequest) (*EmbeddingResponse, error)
}

type Client struct {
	baseURL         string
	apiKey          string
	model           string
	httpClient      *http.Client
	defaultHeaders  map[string]string
	timeout         time.Duration
	maxRetries      int
	retryWaitMin    time.Duration
	retryWaitMax    time.Duration
	streamingFormat StreamingFormat
	organization    string
	betaHeader      string
}

func NewClient(opts ...ClientOption) (*Client, error) {
	c := &Client{
		baseURL:         URLOpenAI,
		httpClient:      http.DefaultClient,
		timeout:         60 * time.Second,
		maxRetries:      3,
		retryWaitMin:    500 * time.Millisecond,
		retryWaitMax:    30 * time.Second,
		streamingFormat: StreamingFormatSSE,
	}

	for _, opt := range opts {
		if err := opt(c); err != nil {
			return nil, fmt.Errorf("failed to apply option: %w", err)
		}
	}

	return c, nil
}

func (c *Client) buildURL(path string) string {
	base := strings.TrimRight(c.baseURL, "/")
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return base + path
}

func (c *Client) newRequest(ctx context.Context, method, path string, body any) (*http.Request, error) {
	url := c.buildURL(path)

	var bodyData []byte
	if body != nil {
		var err error
		bodyData, err = json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(bodyData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	if c.organization != "" {
		req.Header.Set("OpenAI-Organization", c.organization)
	}

	if c.betaHeader != "" {
		req.Header.Set("OpenAI-Beta", c.betaHeader)
	}

	for key, value := range c.defaultHeaders {
		req.Header.Set(key, value)
	}

	return req, nil
}

func (c *Client) doRequest(req *http.Request) (*http.Response, error) {
	client := c.httpClient
	if c.timeout > 0 {
		client.Timeout = c.timeout
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (c *Client) doRequestWithRetry(ctx context.Context, method, path string, body any) (*http.Response, error) {
	var lastErr error

	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		req, err := c.newRequest(ctx, method, path, body)
		if err != nil {
			return nil, err
		}

		resp, err := c.doRequest(req)
		if err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			lastErr = err
			continue
		}

		if !IsRetryableError(&APIError{StatusCode: resp.StatusCode}) {
			return resp, nil
		}

		resp.Body.Close()

		retryAfter := c.calculateRetryAfter(attempt)
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(retryAfter):
		}
		lastErr = &APIError{
			StatusCode: resp.StatusCode,
			Message:    "max retries exceeded",
		}
	}

	return nil, lastErr
}

func (c *Client) calculateRetryAfter(attempt int) time.Duration {
	waitTime := c.retryWaitMin * time.Duration(1<<attempt)
	if waitTime > c.retryWaitMax {
		waitTime = c.retryWaitMax
	}

	jitter := float64(waitTime) * 0.1
	waitTime = waitTime + time.Duration(float64(waitTime)*(rand.Float64()*2-1)*jitter/float64(waitTime))

	return waitTime
}
