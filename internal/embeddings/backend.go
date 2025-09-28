package embeddings

import (
	"context"
)

// TransformerBackend defines a pluggable backend for transformer inference.
// Implementations may use ONNX Runtime, TensorRT, or other engines.
type TransformerBackend interface {
	// EmbedBatch runs a single inference for a batch of tokenized inputs and
	// returns one embedding per input with length == EmbeddingDimensions.
	EmbedBatch(ctx context.Context, tokensBatch []*TokenizedInput) ([][]float32, error)
	// IsReady returns whether the backend is initialized and ready.
	IsReady() bool
	// Close releases any native resources.
	Close() error
}

// NewTransformerBackend creates a backend if supported by the current build.
// The default (no build tags) returns nil to avoid CGO dependencies.
// Note: Implementations are provided in build-tagged files, e.g., backend_onnx.go and backend_stub.go
