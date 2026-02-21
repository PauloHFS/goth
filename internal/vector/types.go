package vector

import "time"

type Config struct {
	Enabled            bool
	EmbeddingDimension int
	TableName          string
}

type Embedding struct {
	ID          int64
	ContentType string
	ContentID   int64
	Vector      []float64
	Metadata    map[string]any
	CreatedAt   time.Time
}

type SearchResult struct {
	Embedding
	Similarity float64
}

type DistanceMetric string

const (
	DistanceL2     DistanceMetric = "l2"
	DistanceCosine DistanceMetric = "cosine"
	DistanceL1     DistanceMetric = "l1"
)
