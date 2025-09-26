package security

import (
	"context"
)

// VectorSecurityAnalyzer defines the interface for vector-based security analysis
type VectorSecurityAnalyzer interface {
	AnalyzePrompt(ctx context.Context, prompt string) (*SecurityResult, error)
	IsEnabled() bool
	GetBlockThreshold() float32
}
