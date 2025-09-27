package embeddings

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"math"
	"math/rand"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// HashEmbeddingService provides fast deterministic embeddings using cryptographic hashing
// This service generates consistent embeddings from text hashes with semantic feature boosting
// Best for: High-speed detection, consistent results, low memory usage
type HashEmbeddingService struct {
	config    *ModelConfig
	logger    *zap.Logger
	stats     *ModelStats
	shared    *SharedUtilities
	mu        sync.RWMutex
	startTime time.Time
}

// NewHashEmbeddingService creates a new hash-based embedding service
func NewHashEmbeddingService(config *ModelConfig, logger *zap.Logger) (*HashEmbeddingService, error) {
	if config == nil {
		return nil, fmt.Errorf("%w: config cannot be nil", ErrConfigError)
	}

	start := time.Now()

	// Initialize shared utilities
	shared, err := NewSharedUtilities(logger)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize shared utilities: %w", err)
	}

	service := &HashEmbeddingService{
		config:    config,
		logger:    logger,
		shared:    shared,
		startTime: start,
		stats: &ModelStats{
			ServiceType:   "hash",
			StartTime:     start,
			ModelLoadTime: time.Since(start),
		},
	}

	logger.Info("Hash embedding service initialized",
		zap.String("type", "deterministic_hash"),
		zap.String("model_name", config.ModelName),
		zap.Duration("load_time", service.stats.ModelLoadTime),
		zap.Int("embedding_dimensions", EmbeddingDimensions))

	return service, nil
}

// GenerateEmbedding generates a deterministic embedding for text
func (s *HashEmbeddingService) GenerateEmbedding(ctx context.Context, text string) (*EmbeddingResult, error) {
	if strings.TrimSpace(text) == "" {
		return nil, fmt.Errorf("%w: text cannot be empty", ErrInvalidInput)
	}

	start := time.Now()

	// Check context for cancellation
	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("%w: context cancelled", ErrTimeoutError)
	default:
	}

	// Generate attack analysis using shared utilities
	analysis := s.shared.AnalyzeAttackPatterns(text)

	// Extract text features
	features := s.shared.GenerateTextFeatures(text)

	// Generate deterministic embedding
	embedding := s.generateDeterministicEmbedding(text, &analysis, &features)

	duration := time.Since(start)
	tokenCount := len(strings.Fields(text))

	// Update stats
	s.updateStats(1, tokenCount, duration, true)

	return &EmbeddingResult{
		Embedding:   embedding,
		Duration:    duration,
		TokenCount:  tokenCount,
		Analysis:    &analysis,
		Features:    &features,
		ServiceType: "hash",
		CacheHit:    false,
	}, nil
}

// GenerateBatchEmbeddings generates embeddings for multiple texts
func (s *HashEmbeddingService) GenerateBatchEmbeddings(ctx context.Context, texts []string) (*BatchEmbeddingResult, error) {
	if len(texts) == 0 {
		return &BatchEmbeddingResult{
			ServiceType: "hash",
		}, nil
	}

	start := time.Now()
	embeddings := make([][]float32, 0, len(texts))
	totalTokens := 0
	successful := 0
	failed := 0
	var errors []error

	// Process each text
	for i, text := range texts {
		// Check context for cancellation
		select {
		case <-ctx.Done():
			errors = append(errors, fmt.Errorf("batch processing cancelled at item %d", i))
			failed++
			continue
		default:
		}

		if strings.TrimSpace(text) == "" {
			errors = append(errors, fmt.Errorf("empty text at index %d", i))
			failed++
			embeddings = append(embeddings, nil)
			continue
		}

		// Generate analysis and features
		analysis := s.shared.AnalyzeAttackPatterns(text)
		features := s.shared.GenerateTextFeatures(text)

		// Generate embedding
		embedding := s.generateDeterministicEmbedding(text, &analysis, &features)
		embeddings = append(embeddings, embedding)
		totalTokens += len(strings.Fields(text))
		successful++
	}

	duration := time.Since(start)

	// Update stats
	s.updateStats(int64(successful), totalTokens, duration, successful > 0)

	return &BatchEmbeddingResult{
		Embeddings:  embeddings,
		Duration:    duration,
		TotalTokens: totalTokens,
		Successful:  successful,
		Failed:      failed,
		Errors:      errors,
		ServiceType: "hash",
		CacheHits:   0, // Hash service doesn't use cache
	}, nil
}

// generateDeterministicEmbedding creates a sophisticated deterministic embedding
func (s *HashEmbeddingService) generateDeterministicEmbedding(text string, analysis *AttackAnalysisResult, features *TextFeatures) []float32 {
	// Create deterministic hash
	hash := sha256.Sum256([]byte(text))

	// Initialize embedding with deterministic base
	embedding := make([]float32, EmbeddingDimensions)

	// Generate base embedding from hash (first 256 dimensions)
	s.generateHashBasedFeatures(hash, embedding[:256])

	// Add attack pattern features (dimensions 256-320)
	s.addAttackFeatures(analysis, embedding[256:320])

	// Add text characteristic features (dimensions 320-384)
	s.addTextFeatures(features, embedding[320:384])

	// Normalize the final embedding
	return s.shared.NormalizeEmbedding(embedding)
}

// generateHashBasedFeatures creates deterministic features from hash
func (s *HashEmbeddingService) generateHashBasedFeatures(hash [32]byte, target []float32) {
	// Use multiple seeds from different parts of the hash for variety
	seeds := []int64{
		int64(binary.BigEndian.Uint64(hash[0:8])),
		int64(binary.BigEndian.Uint64(hash[8:16])),
		int64(binary.BigEndian.Uint64(hash[16:24])),
		int64(binary.BigEndian.Uint64(hash[24:32])),
	}

	segmentSize := len(target) / len(seeds)
	for i, seed := range seeds {
		rng := rand.New(rand.NewSource(seed))
		start := i * segmentSize
		end := start + segmentSize
		if i == len(seeds)-1 {
			end = len(target) // Handle remainder
		}

		for j := start; j < end; j++ {
			target[j] = float32(rng.NormFloat64())
		}
	}
}

// addAttackFeatures adds attack-specific features to embedding
func (s *HashEmbeddingService) addAttackFeatures(analysis *AttackAnalysisResult, target []float32) {
	// Base attack confidence
	target[0] = analysis.Confidence

	// Attack type indicators
	if analysis.IsAttack {
		target[1] = 1.0
	}

	// Number of matched patterns (normalized)
	target[2] = float32(len(analysis.MatchedPatterns)) / 10.0

	// Category-specific scores
	categoryIdx := 3
	for _, score := range analysis.Categories {
		if categoryIdx >= len(target) {
			break
		}
		target[categoryIdx] = score
		categoryIdx++
	}

	// Primary attack type encoding
	switch analysis.PrimaryAttackType {
	case "high_risk":
		target[10] = 1.0
	case "medium_risk":
		target[11] = 1.0
	case "low_risk":
		target[12] = 1.0
	}
}

// addTextFeatures adds text characteristic features
func (s *HashEmbeddingService) addTextFeatures(features *TextFeatures, target []float32) {
	// Normalize features to [0, 1] range
	target[0] = float32(math.Min(float64(features.Length)/1000.0, 1.0))
	target[1] = float32(math.Min(float64(features.WordCount)/100.0, 1.0))
	target[2] = float32(math.Min(float64(features.AvgWordLength)/20.0, 1.0))
	target[3] = features.SpecialCharRatio
	target[4] = features.CapitalizationRatio
	target[5] = features.QuestionRatio
	target[6] = features.ExclamationRatio
	target[7] = float32(math.Tanh(float64(features.KeywordScore))) // Normalize keyword score
	target[8] = float32(math.Min(float64(features.SentenceCount)/20.0, 1.0))
	target[9] = features.Entropy
	target[10] = features.RepetitionScore

	// Fill remaining dimensions with derived features
	for i := 11; i < len(target); i++ {
		// Combine multiple features for additional dimensions
		combined := (target[i%10] + target[(i+1)%10]) / 2.0
		target[i] = float32(math.Sin(float64(combined) * math.Pi))
	}
}

// No longer needed - replaced with more sophisticated feature extraction

// ComputeSimilarity computes cosine similarity between two vectors
func (s *HashEmbeddingService) ComputeSimilarity(vec1, vec2 []float32) float32 {
	return s.shared.ComputeCosineSimilarity(vec1, vec2)
}

// GetStats returns model performance statistics
func (s *HashEmbeddingService) GetStats() *ModelStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Create a copy to avoid race conditions
	stats := *s.stats
	return &stats
}

// updateStats updates performance statistics thread-safely
func (s *HashEmbeddingService) updateStats(inferences int64, tokens int, duration time.Duration, success bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.stats.TotalInferences += inferences
	s.stats.TotalTokens += int64(tokens)
	s.stats.LastInferenceTime = time.Now()

	if success {
		s.stats.SuccessfulRuns += inferences
	} else {
		s.stats.FailedRuns += inferences
	}

	// Update error rate
	total := s.stats.SuccessfulRuns + s.stats.FailedRuns
	if total > 0 {
		s.stats.ErrorRate = float64(s.stats.FailedRuns) / float64(total)
	}

	// Update average inference time (only for successful runs)
	if s.stats.SuccessfulRuns > 0 {
		totalDuration := time.Duration(s.stats.SuccessfulRuns) * s.stats.AvgInferenceTime
		s.stats.AvgInferenceTime = (totalDuration + duration) / time.Duration(s.stats.SuccessfulRuns)
	} else {
		s.stats.AvgInferenceTime = duration
	}

	// Update average tokens per text
	if s.stats.TotalInferences > 0 {
		s.stats.AvgTokensPerText = float64(s.stats.TotalTokens) / float64(s.stats.TotalInferences)
	}

	// Cache hit ratio is always 0 for hash service (no cache)
	s.stats.CacheHitRatio = 0.0
}

// Close cleans up resources
func (s *HashEmbeddingService) Close() error {
	return nil
}
