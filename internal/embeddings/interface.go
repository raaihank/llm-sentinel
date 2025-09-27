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

// Ensure all embedding services implement the interface
var _ EmbeddingService = (*HashEmbeddingService)(nil)
var _ EmbeddingService = (*PatternEmbeddingService)(nil)
var _ EmbeddingService = (*MLEmbeddingService)(nil)
