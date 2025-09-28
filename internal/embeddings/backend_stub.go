//go:build !onnx
// +build !onnx

package embeddings

import (
	"go.uber.org/zap"
)

// Stub implementation used when the 'onnx' build tag is not set.
func NewTransformerBackend(logger *zap.Logger, modelPath string, maxLength int) TransformerBackend {
	return nil
}
