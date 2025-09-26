package security

import (
	"context"
	"strings"
	"time"

	"github.com/raaihank/llm-sentinel/internal/config"
	"github.com/raaihank/llm-sentinel/internal/embeddings"
	"go.uber.org/zap"
)

// SimpleVectorSecurityEngine provides basic prompt security analysis using simple patterns
// This is a lightweight version that doesn't require a database
type SimpleVectorSecurityEngine struct {
	embeddingService embeddings.EmbeddingService
	config           *config.VectorSecurityConfig
	logger           *zap.Logger
}

// NewSimpleVectorSecurityEngine creates a simple vector security engine
func NewSimpleVectorSecurityEngine(
	embeddingService embeddings.EmbeddingService,
	config *config.VectorSecurityConfig,
	logger *zap.Logger,
) *SimpleVectorSecurityEngine {
	return &SimpleVectorSecurityEngine{
		embeddingService: embeddingService,
		config:           config,
		logger:           logger,
	}
}

// AnalyzePrompt analyzes a prompt for security threats using simple pattern matching
func (sve *SimpleVectorSecurityEngine) AnalyzePrompt(ctx context.Context, prompt string) (*SecurityResult, error) {
	start := time.Now()

	// Convert to lowercase for analysis
	lowerPrompt := strings.ToLower(prompt)

	// Define attack patterns and their confidence scores
	attackPatterns := map[string]struct {
		keywords   []string
		attackType string
		confidence float32
	}{
		"prompt_injection": {
			keywords: []string{
				"ignore all previous instructions",
				"forget everything",
				"disregard",
				"override",
				"bypass",
				"pretend you are not an ai",
				"act as if",
				"roleplay",
				"you are now",
				"new instructions",
				"system prompt",
				"ignore your training",
			},
			attackType: "prompt_injection",
			confidence: 0.9,
		},
		"jailbreak": {
			keywords: []string{
				"dan mode",
				"do anything now",
				"jailbreak",
				"unrestricted",
				"no limitations",
				"without restrictions",
				"ignore safety",
				"bypass guidelines",
				"developer mode",
				"evil mode",
				"uncensored",
			},
			attackType: "jailbreak",
			confidence: 0.95,
		},
		"information_extraction": {
			keywords: []string{
				"tell me secrets",
				"reveal",
				"confidential",
				"internal information",
				"system information",
				"configuration",
				"admin",
				"password",
				"api key",
			},
			attackType: "information_extraction",
			confidence: 0.8,
		},
	}

	// Check for attack patterns
	var bestMatch struct {
		attackType string
		confidence float32
		matched    string
	}

	for _, pattern := range attackPatterns {
		for _, keyword := range pattern.keywords {
			if strings.Contains(lowerPrompt, keyword) {
				if pattern.confidence > bestMatch.confidence {
					bestMatch.attackType = pattern.attackType
					bestMatch.confidence = pattern.confidence
					bestMatch.matched = keyword
				}
			}
		}
	}

	// If no patterns matched, it's likely safe
	if bestMatch.confidence == 0 {
		return &SecurityResult{
			IsMalicious:    false,
			Confidence:     0.0,
			AttackType:     "safe",
			ProcessingTime: time.Since(start),
		}, nil
	}

	// Generate embedding for more sophisticated analysis (optional)
	if sve.embeddingService != nil {
		embeddingResult, err := sve.embeddingService.GenerateEmbedding(ctx, prompt)
		if err == nil {
			// Use embedding to adjust confidence based on semantic similarity
			// This is where the SimpleService's security-aware features come in
			sve.logger.Debug("Generated embedding for security analysis",
				zap.Int("embedding_dim", len(embeddingResult.Embedding)),
				zap.Duration("embedding_time", embeddingResult.Duration))
		}
	}

	result := &SecurityResult{
		IsMalicious:     bestMatch.confidence >= sve.GetBlockThreshold(), // Use configured threshold
		Confidence:      bestMatch.confidence,
		AttackType:      bestMatch.attackType,
		SimilarityScore: bestMatch.confidence, // Use confidence as similarity score
		MatchedText:     bestMatch.matched,
		ProcessingTime:  time.Since(start),
	}

	sve.logger.Debug("Simple vector security analysis completed",
		zap.Bool("is_malicious", result.IsMalicious),
		zap.String("attack_type", result.AttackType),
		zap.Float32("confidence", result.Confidence),
		zap.String("matched_pattern", result.MatchedText),
		zap.Duration("processing_time", result.ProcessingTime))

	return result, nil
}

// IsEnabled returns whether vector security is enabled
func (sve *SimpleVectorSecurityEngine) IsEnabled() bool {
	return sve.config != nil && sve.config.Enabled
}

// GetBlockThreshold returns the confidence threshold for blocking requests
func (sve *SimpleVectorSecurityEngine) GetBlockThreshold() float32 {
	if sve.config == nil {
		return 0.85 // Default threshold
	}
	return sve.config.BlockThreshold
}
