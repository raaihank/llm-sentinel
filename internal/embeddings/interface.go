package embeddings

import (
	"context"
)

// EmbeddingService defines the interface for embedding generation services
type EmbeddingService interface {
	GenerateEmbedding(ctx context.Context, text string) (*EmbeddingResult, error)
	GenerateBatchEmbeddings(ctx context.Context, texts []string) (*BatchEmbeddingResult, error)
	ComputeSimilarity(vec1, vec2 []float32) float32
	GetStats() *ModelStats
	Close() error
}

// Ensure SimpleService implements the interface
var _ EmbeddingService = (*SimpleService)(nil)
