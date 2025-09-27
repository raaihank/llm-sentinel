package vector

import (
	"time"
)

// SecurityVector represents a security pattern with its embedding
type SecurityVector struct {
	ID            int64     `db:"id" json:"id"`
	Text          string    `db:"text" json:"text"`
	EmbeddingType string    `db:"embedding_type" json:"embedding_type"`
	TextHash      string    `db:"text_hash" json:"text_hash"`
	LabelText     string    `db:"label_text" json:"label_text"`
	Label         int       `db:"label" json:"label"`
	Embedding     []float32 `db:"embedding" json:"embedding"`
	CreatedAt     time.Time `db:"created_at" json:"created_at"`
	UpdatedAt     time.Time `db:"updated_at" json:"updated_at"`
}

// SimilarityResult represents a vector similarity search result
type SimilarityResult struct {
	Vector     *SecurityVector `json:"vector"`
	Similarity float32         `json:"similarity"`
	Distance   float32         `json:"distance"`
}

// SearchOptions contains options for vector similarity search
type SearchOptions struct {
	Limit           int     `json:"limit"`
	MinSimilarity   float32 `json:"min_similarity"`
	LabelFilter     *int    `json:"label_filter,omitempty"`
	LabelTextFilter string  `json:"label_text_filter,omitempty"`
}

// VectorStats represents database statistics
type VectorStats struct {
	TotalVectors    int64   `json:"total_vectors"`
	MaliciousCount  int64   `json:"malicious_count"`
	SafeCount       int64   `json:"safe_count"`
	AvgSearchTimeMs float64 `json:"avg_search_time_ms"`
	CacheHitRate    float64 `json:"cache_hit_rate"`
}

// BatchInsertResult represents the result of a batch insert operation
type BatchInsertResult struct {
	Inserted int64         `json:"inserted"`
	Failed   int64         `json:"failed"`
	Duration time.Duration `json:"duration"`
	Errors   []error       `json:"errors,omitempty"`
}
