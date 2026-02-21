package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

func (c *Client) Generate(ctx context.Context, req CompletionRequest) (*CompletionResponse, error) {
	if ctx == nil {
		return nil, ErrNilContext
	}

	if req.Model == "" && c.model != "" {
		req.Model = c.model
	}

	if req.Model == "" {
		return nil, &APIError{Message: "model is required"}
	}

	req.Stream = false

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := c.doRequestWithRetry(ctx, http.MethodPost, "/v1/chat/completions", body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return nil, fmt.Errorf("failed to read error response: %w", readErr)
		}
		return nil, parseAPIError(resp.StatusCode, respBody)
	}

	var completion CompletionResponse
	if err := json.NewDecoder(resp.Body).Decode(&completion); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &completion, nil
}
