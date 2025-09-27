package security

import (
	"context"
	"fmt"
	"time"

	"github.com/raaihank/llm-sentinel/internal/cache"
	"github.com/raaihank/llm-sentinel/internal/config"
	"github.com/raaihank/llm-sentinel/internal/embeddings"
	"github.com/raaihank/llm-sentinel/internal/vector"
	"go.uber.org/zap"
)

// VectorSecurityEngine handles ML-based prompt security analysis
type VectorSecurityEngine struct {
	vectorStore      *vector.Store
	cache            *cache.VectorCache
	embeddingService embeddings.EmbeddingService
	config           *config.VectorSecurityConfig
	logger           *zap.Logger
}

// SecurityResult represents the result of vector security analysis
type SecurityResult struct {
	IsMalicious     bool          `json:"is_malicious"`
	Confidence      float32       `json:"confidence"`
	AttackType      string        `json:"attack_type"`
	SimilarityScore float32       `json:"similarity_score"`
	MatchedText     string        `json:"matched_text,omitempty"`
	ProcessingTime  time.Duration `json:"processing_time"`
}

// NewVectorSecurityEngine creates a new vector security engine
func NewVectorSecurityEngine(
	vectorStore *vector.Store,
	vectorCache *cache.VectorCache,
	embeddingService embeddings.EmbeddingService,
	config *config.VectorSecurityConfig,
	logger *zap.Logger,
) *VectorSecurityEngine {
	return &VectorSecurityEngine{
		vectorStore:      vectorStore,
		cache:            vectorCache,
		embeddingService: embeddingService,
		config:           config,
		logger:           logger,
	}
}

// AnalyzePrompt analyzes a prompt for security threats using vector similarity
func (vse *VectorSecurityEngine) AnalyzePrompt(ctx context.Context, prompt string) (*SecurityResult, error) {
	start := time.Now()

	// Establish a stable analysis context to avoid immediate cancellation
	// Prefer configured model timeout; default to 300ms if unset
	effectiveTimeout := 300 * time.Millisecond
	if vse.config != nil && vse.config.Embedding.Model.ModelTimeout > 0 {
		effectiveTimeout = vse.config.Embedding.Model.ModelTimeout
	}

	// If the incoming context has an extremely short remaining deadline, detach
	// from it to avoid immediate cancellations and use our own bounded timeout
	var analysisCtx context.Context
	var cancel context.CancelFunc
	if deadline, ok := ctx.Deadline(); ok {
		if time.Until(deadline) < 5*time.Millisecond {
			analysisCtx, cancel = context.WithTimeout(context.Background(), effectiveTimeout)
		} else {
			analysisCtx, cancel = context.WithTimeout(ctx, effectiveTimeout)
		}
	} else {
		analysisCtx, cancel = context.WithTimeout(ctx, effectiveTimeout)
	}
	defer cancel()

	// Generate embedding for the input prompt
	embeddingResult, err := vse.embeddingService.GenerateEmbedding(analysisCtx, prompt)
	if err != nil {
		return nil, fmt.Errorf("failed to generate embedding: %w", err)
	}

	// Try cache first if available
	if vse.cache != nil {
		cacheResult, err := vse.cache.SearchSimilar(analysisCtx, embeddingResult.Embedding, &cache.SearchOptions{
			MinSimilarity: vse.config.BlockThreshold,
			MaxResults:    1,
		})
		if err == nil && cacheResult.CacheHit && cacheResult.Vector != nil {
			vse.logger.Debug("Vector security cache hit",
				zap.String("attack_type", cacheResult.Vector.LabelText),
				zap.Float32("similarity", cacheResult.Vector.Similarity))

			return &SecurityResult{
				IsMalicious:     cacheResult.Vector.Label == 1,
				Confidence:      cacheResult.Vector.Similarity,
				AttackType:      cacheResult.Vector.LabelText,
				SimilarityScore: cacheResult.Vector.Similarity,
				MatchedText:     cacheResult.Vector.Text,
				ProcessingTime:  time.Since(start),
			}, nil
		}
	}

	// Fallback to database search
	similarVectors, err := vse.vectorStore.FindSimilar(analysisCtx, embeddingResult.Embedding, &vector.SearchOptions{
		Limit:         5,
		MinSimilarity: vse.config.BlockThreshold,
	})
	if err != nil {
		return nil, fmt.Errorf("vector similarity search failed: %w", err)
	}

	// If no similar vectors found, it's likely safe
	if len(similarVectors) == 0 {
		return &SecurityResult{
			IsMalicious:    false,
			Confidence:     0.0,
			AttackType:     "safe",
			ProcessingTime: time.Since(start),
		}, nil
	}

    // Use the most similar vector
	best := similarVectors[0]

    // Enforce embedding type compatibility when available
    if best.Vector != nil && best.Vector.EmbeddingType != "" {
        expected := "pattern"
        if vse.config != nil {
            expected = vse.config.Embedding.ServiceType
        }
        if best.Vector.EmbeddingType != expected {
            vse.logger.Warn("Embedding type mismatch; treating as safe",
                zap.String("db_type", best.Vector.EmbeddingType),
                zap.String("expected", expected))
            return &SecurityResult{IsMalicious: false, Confidence: 0.0, AttackType: "safe", ProcessingTime: time.Since(start)}, nil
        }
    }

	result := &SecurityResult{
		IsMalicious:     best.Vector.Label == 1,
		Confidence:      best.Similarity,
		AttackType:      best.Vector.LabelText,
		SimilarityScore: best.Similarity,
		MatchedText:     best.Vector.Text,
		ProcessingTime:  time.Since(start),
	}

	// Cache the result for future queries if it's malicious
	if vse.cache != nil && result.IsMalicious {
		cachedVector := &cache.CachedVector{
			ID:         best.Vector.ID,
			Text:       best.Vector.Text,
			LabelText:  best.Vector.LabelText,
			Label:      best.Vector.Label,
			Embedding:  best.Vector.Embedding,
			Similarity: best.Similarity,
		}

		if err := vse.cache.Store(analysisCtx, embeddingResult.Embedding, cachedVector); err != nil {
			vse.logger.Warn("Failed to cache vector result", zap.Error(err))
		}
	}

	vse.logger.Debug("Vector security analysis completed",
		zap.Bool("is_malicious", result.IsMalicious),
		zap.String("attack_type", result.AttackType),
		zap.Float32("confidence", result.Confidence),
		zap.Duration("processing_time", result.ProcessingTime))

	return result, nil
}

// IsEnabled returns whether vector security is enabled
func (vse *VectorSecurityEngine) IsEnabled() bool {
	return vse.config != nil && vse.config.Enabled
}

// GetBlockThreshold returns the confidence threshold for blocking requests
func (vse *VectorSecurityEngine) GetBlockThreshold() float32 {
	if vse.config == nil {
		return 0.85 // Default threshold
	}
	return vse.config.BlockThreshold
}
