package httpclient

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/PauloHFS/goth/internal/logging"
)

type Client struct {
	*http.Client
	name string
}

type Config struct {
	Name         string
	Timeout      time.Duration
	MaxRetries   int
	RetryWaitMin time.Duration
	RetryWaitMax time.Duration
}

func New(cfg Config) *Client {
	transport := &loggingTransport{
		RoundTripper: http.DefaultTransport,
		name:         cfg.Name,
	}

	return &Client{
		Client: &http.Client{
			Timeout:   cfg.Timeout,
			Transport: transport,
		},
		name: cfg.Name,
	}
}

func Default() *Client {
	return New(Config{
		Name:         "default",
		Timeout:      30 * time.Second,
		MaxRetries:   3,
		RetryWaitMin: 500 * time.Millisecond,
		RetryWaitMax: 30 * time.Second,
	})
}

type loggingTransport struct {
	http.RoundTripper
	name string
}

func (t *loggingTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	start := time.Now()

	ctx, event := logging.NewEventContext(r.Context())
	event.Add(
		slog.String("http_client", t.name),
		slog.String("method", r.Method),
		slog.String("url", r.URL.String()),
	)

	resp, err := t.RoundTripper.RoundTrip(r.WithContext(ctx))

	duration := time.Since(start)

	if err != nil {
		event.Add(
			slog.String("outcome", "error"),
			slog.String("error", err.Error()),
			slog.Float64("duration_ms", float64(duration.Milliseconds())),
		)
		logging.Get().Log(ctx, slog.LevelError, "http request failed", event.Attrs()...)
		return nil, err
	}

	event.Add(
		slog.Int("status", resp.StatusCode),
		slog.Float64("duration_ms", float64(duration.Milliseconds())),
	)

	level := slog.LevelInfo
	if resp.StatusCode >= 400 {
		level = slog.LevelWarn
	}

	logging.Get().Log(ctx, level, "http request completed", event.Attrs()...)
	return resp, nil
}

func WithAuth(authFunc func(*http.Request)) func(*Client) {
	return func(c *Client) {
		transport := &authTransport{
			RoundTripper: c.Transport,
			authFunc:     authFunc,
		}
		c.Transport = transport
	}
}

type authTransport struct {
	http.RoundTripper
	authFunc func(*http.Request)
}

func (t *authTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	t.authFunc(r)
	return t.RoundTripper.RoundTrip(r)
}
