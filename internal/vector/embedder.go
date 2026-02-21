package vector

import (
	"context"

	"github.com/PauloHFS/goth/internal/llm"
)

type Embedder struct {
	llmClient *llm.Client
	model     string
}

func NewEmbedder(llmClient *llm.Client, model string) *Embedder {
	return &Embedder{
		llmClient: llmClient,
		model:     model,
	}
}

func (e *Embedder) Embed(ctx context.Context, text string) ([]float64, error) {
	resp, err := e.llmClient.Embed(ctx, llm.EmbeddingRequest{
		Model: e.model,
		Input: text,
	})
	if err != nil {
		return nil, err
	}

	if len(resp.Data) == 0 {
		return nil, ErrNoEmbedding
	}

	return resp.Data[0].Embedding, nil
}

func (e *Embedder) EmbedBatch(ctx context.Context, texts []string) ([][]float64, error) {
	resp, err := e.llmClient.Embed(ctx, llm.EmbeddingRequest{
		Model: e.model,
		Input: texts,
	})
	if err != nil {
		return nil, err
	}

	embeddings := make([][]float64, len(resp.Data))
	for i, r := range resp.Data {
		embeddings[i] = r.Embedding
	}

	return embeddings, nil
}

var ErrNoEmbedding = &EmbeddingError{Message: "no embedding returned"}

type EmbeddingError struct {
	Message string
}

func (e *EmbeddingError) Error() string {
	return e.Message
}
