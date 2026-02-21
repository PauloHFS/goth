package llm

import (
	"net/http"
	"time"
)

type StreamingFormat string

const (
	StreamingFormatSSE    StreamingFormat = "sse"
	StreamingFormatNDJSON StreamingFormat = "ndjson"
)

const (
	URLOpenAI     = "https://api.openai.com"
	URLOpenRouter = "https://openrouter.ai/api"
	URLOllama     = "http://localhost:11434/v1"
)

type ClientOption func(*Client) error

type OptionFunc func(*Client) error

func WithBaseURL(url string) ClientOption {
	return func(c *Client) error {
		if url == "" {
			return ErrInvalidBaseURL
		}
		c.baseURL = url
		return nil
	}
}

func WithAPIKey(key string) ClientOption {
	return func(c *Client) error {
		if key == "" {
			return ErrNoAPIKey
		}
		c.apiKey = key
		return nil
	}
}

func WithModel(model string) ClientOption {
	return func(c *Client) error {
		c.model = model
		return nil
	}
}

func WithHTTPClient(client *http.Client) ClientOption {
	return func(c *Client) error {
		if client == nil {
			c.httpClient = http.DefaultClient
			return nil
		}
		c.httpClient = client
		return nil
	}
}

func WithDefaultHeaders(headers map[string]string) ClientOption {
	return func(c *Client) error {
		c.defaultHeaders = headers
		return nil
	}
}

func WithTimeout(timeout time.Duration) ClientOption {
	return func(c *Client) error {
		c.timeout = timeout
		return nil
	}
}

func WithMaxRetries(retries int) ClientOption {
	return func(c *Client) error {
		if retries < 0 {
			retries = 0
		}
		c.maxRetries = retries
		return nil
	}
}

func WithRetryWaitRange(min, max time.Duration) ClientOption {
	return func(c *Client) error {
		if min <= 0 {
			min = 500 * time.Millisecond
		}
		if max <= 0 {
			max = 30 * time.Second
		}
		if min > max {
			min, max = max, min
		}
		c.retryWaitMin = min
		c.retryWaitMax = max
		return nil
	}
}

func WithStreamingFormat(format StreamingFormat) ClientOption {
	return func(c *Client) error {
		switch format {
		case StreamingFormatSSE, StreamingFormatNDJSON:
			c.streamingFormat = format
		default:
			c.streamingFormat = StreamingFormatSSE
		}
		return nil
	}
}

func WithOrganization(org string) ClientOption {
	return func(c *Client) error {
		c.organization = org
		return nil
	}
}

func WithBetaHeader(version string) ClientOption {
	return func(c *Client) error {
		c.betaHeader = version
		return nil
	}
}
