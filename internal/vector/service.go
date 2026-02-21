package vector

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type Service struct {
	store *Store
}

func NewService(store *Store) *Service {
	return &Service{store: store}
}

func (s *Service) Store(ctx context.Context, embedding Embedding) (int64, error) {
	metadataJSON, err := json.Marshal(embedding.Metadata)
	if err != nil {
		return 0, fmt.Errorf("failed to marshal metadata: %w", err)
	}

	vectorJSON, err := json.Marshal(embedding.Vector)
	if err != nil {
		return 0, fmt.Errorf("failed to marshal vector: %w", err)
	}

	query := fmt.Sprintf(`
		INSERT INTO %s (content_type, content_id, embedding, metadata, created_at)
		VALUES (?, ?, ?, ?, ?)
	`, s.store.Config().TableName)

	result, err := s.store.DB().ExecContext(ctx, query,
		embedding.ContentType,
		embedding.ContentID,
		string(vectorJSON),
		string(metadataJSON),
		time.Now(),
	)
	if err != nil {
		return 0, fmt.Errorf("failed to insert embedding: %w", err)
	}

	return result.LastInsertId()
}

func (s *Service) Upsert(ctx context.Context, embedding Embedding) (int64, error) {
	metadataJSON, err := json.Marshal(embedding.Metadata)
	if err != nil {
		return 0, fmt.Errorf("failed to marshal metadata: %w", err)
	}

	vectorJSON, err := json.Marshal(embedding.Vector)
	if err != nil {
		return 0, fmt.Errorf("failed to marshal vector: %w", err)
	}

	query := fmt.Sprintf(`
		INSERT INTO %s (content_type, content_id, embedding, metadata, created_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			content_type = excluded.content_type,
			content_id = excluded.content_id,
			embedding = excluded.embedding,
			metadata = excluded.metadata
	`, s.store.Config().TableName)

	result, err := s.store.DB().ExecContext(ctx, query,
		embedding.ContentType,
		embedding.ContentID,
		string(vectorJSON),
		string(metadataJSON),
		time.Now(),
	)
	if err != nil {
		return 0, fmt.Errorf("failed to upsert embedding: %w", err)
	}

	return result.LastInsertId()
}

func (s *Service) Search(ctx context.Context, contentType string, queryVector []float64, limit int, metric DistanceMetric) ([]SearchResult, error) {
	if limit <= 0 {
		limit = 10
	}

	queryVectorJSON, err := json.Marshal(queryVector)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal query vector: %w", err)
	}

	var distanceFunc string
	switch metric {
	case DistanceCosine:
		distanceFunc = "vec_distance_cosine"
	case DistanceL1:
		distanceFunc = "vec_distance_l1"
	case DistanceL2:
		distanceFunc = "vec_distance_l2"
	default:
		distanceFunc = "vec_distance_cosine"
	}

	query := fmt.Sprintf(`
		SELECT 
			id, content_type, content_id, embedding, metadata, created_at,
			%s(embedding, '%s') as distance
		FROM %s
		WHERE content_type = ?
		ORDER BY distance
		LIMIT ?
	`, distanceFunc, string(queryVectorJSON), s.store.Config().TableName)

	rows, err := s.store.DB().QueryContext(ctx, query, contentType, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to search embeddings: %w", err)
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var e Embedding
		var embeddingJSON string
		var metadataJSON string
		var distance float64

		err := rows.Scan(
			&e.ID,
			&e.ContentType,
			&e.ContentID,
			&embeddingJSON,
			&metadataJSON,
			&e.CreatedAt,
			&distance,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		if err := json.Unmarshal([]byte(embeddingJSON), &e.Vector); err != nil {
			return nil, fmt.Errorf("failed to unmarshal vector: %w", err)
		}

		if metadataJSON != "" {
			if err := json.Unmarshal([]byte(metadataJSON), &e.Metadata); err != nil {
				return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
			}
		}

		results = append(results, SearchResult{
			Embedding:  e,
			Similarity: distance,
		})
	}

	return results, nil
}

func (s *Service) SearchGlobal(ctx context.Context, queryVector []float64, limit int, metric DistanceMetric) ([]SearchResult, error) {
	if limit <= 0 {
		limit = 10
	}

	queryVectorJSON, err := json.Marshal(queryVector)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal query vector: %w", err)
	}

	var distanceFunc string
	switch metric {
	case DistanceCosine:
		distanceFunc = "vec_distance_cosine"
	case DistanceL1:
		distanceFunc = "vec_distance_l1"
	case DistanceL2:
		distanceFunc = "vec_distance_l2"
	default:
		distanceFunc = "vec_distance_cosine"
	}

	query := fmt.Sprintf(`
		SELECT 
			id, content_type, content_id, embedding, metadata, created_at,
			%s(embedding, '%s') as distance
		FROM %s
		ORDER BY distance
		LIMIT ?
	`, distanceFunc, string(queryVectorJSON), s.store.Config().TableName)

	rows, err := s.store.DB().QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to search embeddings: %w", err)
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var e Embedding
		var embeddingJSON string
		var metadataJSON string
		var distance float64

		err := rows.Scan(
			&e.ID,
			&e.ContentType,
			&e.ContentID,
			&embeddingJSON,
			&metadataJSON,
			&e.CreatedAt,
			&distance,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		if err := json.Unmarshal([]byte(embeddingJSON), &e.Vector); err != nil {
			return nil, fmt.Errorf("failed to unmarshal vector: %w", err)
		}

		if metadataJSON != "" {
			if err := json.Unmarshal([]byte(metadataJSON), &e.Metadata); err != nil {
				return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
			}
		}

		results = append(results, SearchResult{
			Embedding:  e,
			Similarity: distance,
		})
	}

	return results, nil
}

func (s *Service) Delete(ctx context.Context, contentType string, contentID int64) error {
	query := fmt.Sprintf(`
		DELETE FROM %s WHERE content_type = ? AND content_id = ?
	`, s.store.Config().TableName)

	_, err := s.store.DB().ExecContext(ctx, query, contentType, contentID)
	return err
}

func (s *Service) DeleteByIDs(ctx context.Context, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}

	placeholders := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}

	query := fmt.Sprintf(`
		DELETE FROM %s WHERE id IN (%s)
	`, s.store.Config().TableName, strings.Join(placeholders, ","))

	_, err := s.store.DB().ExecContext(ctx, query, args...)
	return err
}

func (s *Service) GetByContent(ctx context.Context, contentType string, contentID int64) (*Embedding, error) {
	query := fmt.Sprintf(`
		SELECT id, content_type, content_id, embedding, metadata, created_at
		FROM %s WHERE content_type = ? AND content_id = ?
	`, s.store.Config().TableName)

	var e Embedding
	var embeddingJSON string
	var metadataJSON string

	err := s.store.DB().QueryRowContext(ctx, query, contentType, contentID).Scan(
		&e.ID,
		&e.ContentType,
		&e.ContentID,
		&embeddingJSON,
		&metadataJSON,
		&e.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get embedding: %w", err)
	}

	if err := json.Unmarshal([]byte(embeddingJSON), &e.Vector); err != nil {
		return nil, fmt.Errorf("failed to unmarshal vector: %w", err)
	}

	if metadataJSON != "" {
		if err := json.Unmarshal([]byte(metadataJSON), &e.Metadata); err != nil {
			return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}
	}

	return &e, nil
}

func (s *Service) Count(ctx context.Context, contentType string) (int, error) {
	query := fmt.Sprintf(`
		SELECT COUNT(*) FROM %s WHERE content_type = ?
	`, s.store.Config().TableName)

	var count int
	err := s.store.DB().QueryRowContext(ctx, query, contentType).Scan(&count)
	return count, err
}
