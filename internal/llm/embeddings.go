package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

func (c *Client) Embed(ctx context.Context, req EmbeddingRequest) (*EmbeddingResponse, error) {
	if ctx == nil {
		return nil, ErrNilContext
	}

	if req.Model == "" && c.model != "" {
		req.Model = c.model
	}

	if req.Model == "" {
		return nil, &APIError{Message: "model is required"}
	}

	if req.Input == nil {
		return nil, &APIError{Message: "input is required"}
	}

	if req.EncodingFormat == "" {
		req.EncodingFormat = "float"
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := c.doRequestWithRetry(ctx, http.MethodPost, "/v1/embeddings", body)
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

	var embeddingResp EmbeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&embeddingResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &embeddingResp, nil
}
