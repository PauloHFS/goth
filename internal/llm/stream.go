package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"unicode/utf8"
)

type StreamChunkWithError struct {
	Chunk StreamChunk
	Error error
	Done  bool
}

func (c *Client) Stream(ctx context.Context, req CompletionRequest) (<-chan StreamChunk, error) {
	if ctx == nil {
		return nil, ErrNilContext
	}

	if req.Model == "" && c.model != "" {
		req.Model = c.model
	}

	if req.Model == "" {
		return nil, &APIError{Message: "model is required"}
	}

	req.Stream = true

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	reqHTTP, err := c.newRequest(ctx, http.MethodPost, "/v1/chat/completions", body)
	if err != nil {
		return nil, err
	}

	if c.streamingFormat == StreamingFormatSSE {
		reqHTTP.Header.Set("Accept", "text/event-stream")
	} else {
		reqHTTP.Header.Set("Accept", "application/x-ndjson")
	}

	resp, err := c.doRequest(reqHTTP)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		respBody, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return nil, fmt.Errorf("failed to read error response: %w", readErr)
		}
		return nil, parseAPIError(resp.StatusCode, respBody)
	}

	ch := make(chan StreamChunk, 1)

	go c.streamReader(ctx, resp.Body, ch, c.streamingFormat)

	return ch, nil
}

func (c *Client) streamReader(ctx context.Context, body io.Reader, ch chan<- StreamChunk, format StreamingFormat) {
	defer close(ch)

	var reader *bufio.Reader
	if r, ok := body.(*bufio.Reader); ok {
		reader = r
	} else {
		reader = bufio.NewReader(body)
	}

	if format == StreamingFormatNDJSON {
		c.readNDJSONStream(ctx, reader, ch)
	} else {
		c.readSSEStream(ctx, reader, ch)
	}
}

func (c *Client) readSSEStream(ctx context.Context, reader *bufio.Reader, ch chan<- StreamChunk) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		line, err := reader.ReadString('\n')
		if err != nil {
			if err != io.EOF {
				ch <- StreamChunk{Choices: []StreamChoice{{Delta: Message{Content: ""}}}} // Signal error
			}
			return
		}

		line = strings.TrimRight(line, "\r\n")
		line = strings.TrimSpace(line)

		if line == "" {
			continue
		}

		if strings.HasPrefix(line, ":") {
			continue
		}

		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")
		data = strings.TrimSpace(data)

		if data == "[DONE]" {
			return
		}

		var chunk StreamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}

		select {
		case ch <- chunk:
		case <-ctx.Done():
			return
		}
	}
}

func (c *Client) readNDJSONStream(ctx context.Context, reader *bufio.Reader, ch chan<- StreamChunk) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		line, err := reader.ReadBytes('\n')
		if err != nil {
			if err != io.EOF {
				ch <- StreamChunk{Choices: []StreamChoice{{Delta: Message{Content: ""}}}}
			}
			return
		}

		line = bytes.TrimRight(line, "\r\n")

		if len(line) == 0 {
			continue
		}

		if !utf8.Valid(line) {
			continue
		}

		var ollamaChunk OllamaStreamChunk
		if err := json.Unmarshal(line, &ollamaChunk); err != nil {
			continue
		}

		chunk := ollamaChunk.ToStreamChunk()

		select {
		case ch <- chunk:
		case <-ctx.Done():
			return
		}
	}
}
