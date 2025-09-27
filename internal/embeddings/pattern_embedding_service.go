package embeddings

import (
	"context"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// PatternEmbeddingService implements sophisticated pattern-based embeddings
// This service combines regex patterns, semantic analysis, and contextual understanding
// Best for: Balanced accuracy and performance, explainable results, production use
type PatternEmbeddingService struct {
	config    ModelConfig
	logger    *zap.Logger
	shared    *SharedUtilities
	stats     *ModelStats
	mu        sync.RWMutex
	startTime time.Time
}

// NewPatternEmbeddingService creates a new pattern-based embedding service
func NewPatternEmbeddingService(config *ModelConfig, logger *zap.Logger) (*PatternEmbeddingService, error) {
	start := time.Now()
	logger.Info("Initializing pattern-based embedding service with advanced contextual analysis")

	// Initialize shared utilities
	shared, err := NewSharedUtilities(logger)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize shared utilities: %w", err)
	}

	service := &PatternEmbeddingService{
		config:    *config,
		logger:    logger,
		shared:    shared,
		startTime: start,
		stats: &ModelStats{
			ServiceType:   "pattern",
			StartTime:     start,
			ModelLoadTime: time.Since(start),
		},
	}

	logger.Info("Pattern embedding service initialized",
		zap.String("type", "advanced_pattern_matching"),
		zap.String("model_name", config.ModelName),
		zap.Duration("load_time", service.stats.ModelLoadTime),
		zap.Int("embedding_dimensions", EmbeddingDimensions))

	return service, nil
}

// GenerateEmbedding generates a sophisticated pattern-based embedding
func (s *PatternEmbeddingService) GenerateEmbedding(ctx context.Context, text string) (*EmbeddingResult, error) {
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

	// Generate comprehensive analysis using shared utilities
	analysis := s.shared.AnalyzeAttackPatterns(text)
	features := s.shared.GenerateTextFeatures(text)

	// Generate sophisticated embedding
	embedding := s.generateAdvancedEmbedding(text, &analysis, &features)

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
		ServiceType: "pattern",
		CacheHit:    false,
	}, nil
}

// GenerateBatchEmbeddings generates embeddings for multiple texts with error handling
func (s *PatternEmbeddingService) GenerateBatchEmbeddings(ctx context.Context, texts []string) (*BatchEmbeddingResult, error) {
	if len(texts) == 0 {
		return &BatchEmbeddingResult{
			ServiceType: "pattern",
		}, nil
	}

	start := time.Now()
	embeddings := make([][]float32, 0, len(texts))
	totalTokens := 0
	successful := 0
	failed := 0
	var errors []error

	// Process batch with proper error handling
	for i, text := range texts {
		// Check for cancellation
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

		// Generate analysis and embedding
		analysis := s.shared.AnalyzeAttackPatterns(text)
		features := s.shared.GenerateTextFeatures(text)
		embedding := s.generateAdvancedEmbedding(text, &analysis, &features)

		embeddings = append(embeddings, embedding)
		totalTokens += len(strings.Fields(text))
		successful++
	}

	duration := time.Since(start)

	// Update stats
	s.updateStats(int64(successful), totalTokens, duration, successful > 0)

	s.logger.Debug("Advanced batch embedding generation completed",
		zap.Int("batch_size", len(texts)),
		zap.Int("successful", successful),
		zap.Int("failed", failed),
		zap.Duration("duration", duration))

	return &BatchEmbeddingResult{
		Embeddings:  embeddings,
		Duration:    duration,
		TotalTokens: totalTokens,
		Successful:  successful,
		Failed:      failed,
		Errors:      errors,
		ServiceType: "pattern",
		CacheHits:   0, // Pattern service doesn't use cache
	}, nil
}

// generateAdvancedEmbedding creates a sophisticated multi-layered embedding
func (s *PatternEmbeddingService) generateAdvancedEmbedding(text string, analysis *AttackAnalysisResult, features *TextFeatures) []float32 {
	embedding := make([]float32, EmbeddingDimensions)

	// Layer 1: Hash-based deterministic features (0-95)
	hash := s.shared.CreateDeterministicHash(text)
	s.addHashFeatures(hash, embedding[0:96])

	// Layer 2: Advanced attack pattern features (96-191)
	s.addAdvancedAttackFeatures(text, analysis, embedding[96:192])

	// Layer 3: Semantic cluster analysis (192-287)
	s.addSemanticClusterFeatures(text, embedding[192:288])

	// Layer 4: Advanced contextual features (288-383)
	s.addAdvancedContextFeatures(text, features, embedding[288:384])

	// Normalize and return
	return s.shared.NormalizeEmbedding(embedding)
}

// addHashFeatures adds deterministic hash-based features
func (s *PatternEmbeddingService) addHashFeatures(hash [32]byte, target []float32) {
	for i := 0; i < len(target); i++ {
		// Convert hash bytes to normalized float values
		byte_idx := i % 32
		target[i] = float32(hash[byte_idx])/255.0*2.0 - 1.0 // [-1, 1]
	}
}

// addAdvancedAttackFeatures adds sophisticated attack detection features
func (s *PatternEmbeddingService) addAdvancedAttackFeatures(text string, analysis *AttackAnalysisResult, target []float32) {
	// Primary attack indicators
	target[0] = analysis.Confidence
	if analysis.IsAttack {
		target[1] = 1.0
	}
	target[2] = float32(len(analysis.MatchedPatterns)) / 10.0

	// Category-specific scores with weighted importance
	categoryWeights := map[string]float32{
		"high_risk":   3.0,
		"medium_risk": 2.0,
		"low_risk":    1.0,
	}

	idx := 3
	for category, score := range analysis.Categories {
		if idx >= len(target) {
			break
		}
		weight := categoryWeights[category]
		if weight == 0 {
			weight = 1.0
		}
		target[idx] = score * weight
		idx++
	}

	// Specific attack type encoding
	switch analysis.PrimaryAttackType {
	case "high_risk":
		target[20] = 1.0
	case "medium_risk":
		target[21] = 1.0
	case "low_risk":
		target[22] = 1.0
	}

	// Pattern complexity analysis
	words := strings.Fields(strings.ToLower(text))
	if len(words) > 0 {
		// Pattern density
		target[25] = float32(len(analysis.MatchedPatterns)) / float32(len(words))

		// Attack word ratio
		attackWords := 0
		for _, word := range words {
			if s.isAttackWord(word) {
				attackWords++
			}
		}
		target[26] = float32(attackWords) / float32(len(words))
	}
}

// addSemanticClusterFeatures adds semantic cluster analysis
func (s *PatternEmbeddingService) addSemanticClusterFeatures(text string, target []float32) {
	clusterNames := []string{
		"instruction_manipulation", "roleplay_attempts", "system_probing",
		"jailbreak_terms", "social_engineering", "data_extraction", "authority_bypass",
	}

	for i, clusterName := range clusterNames {
		if i >= len(target) {
			break
		}
		score := s.shared.GetSemanticClusterScore(text, clusterName)
		target[i] = score
	}

	// Cross-cluster correlation analysis
	for i := len(clusterNames); i < len(target)-10; i++ {
		// Combine scores from multiple clusters for richer features
		cluster1 := i % len(clusterNames)
		cluster2 := (i + 1) % len(clusterNames)
		score1 := s.shared.GetSemanticClusterScore(text, clusterNames[cluster1])
		score2 := s.shared.GetSemanticClusterScore(text, clusterNames[cluster2])
		target[i] = (score1 + score2) / 2.0
	}
}

// addAdvancedContextFeatures adds sophisticated contextual analysis
func (s *PatternEmbeddingService) addAdvancedContextFeatures(text string, features *TextFeatures, target []float32) {
	// Basic text features (normalized)
	target[0] = float32(features.Length) / 1000.0
	target[1] = float32(features.WordCount) / 100.0
	target[2] = features.AvgWordLength / 20.0
	target[3] = features.SpecialCharRatio
	target[4] = features.CapitalizationRatio
	target[5] = features.QuestionRatio
	target[6] = features.ExclamationRatio
	target[7] = features.Entropy
	target[8] = features.RepetitionScore
	target[9] = features.KeywordScore / 10.0 // Normalize

	// Advanced linguistic features
	target[10] = s.calculateUrgencyScore(text)
	target[11] = s.calculatePolitenesScore(text)
	target[12] = s.calculateAuthorityScore(text)
	target[13] = s.calculateManipulationScore(text)
	target[14] = s.calculateCoherenceScore(text)
	target[15] = s.calculateComplexityScore(text)

	// Sentence structure analysis
	sentences := strings.FieldsFunc(text, func(c rune) bool {
		return c == '.' || c == '!' || c == '?'
	})
	if len(sentences) > 0 {
		target[16] = float32(len(sentences)) / 20.0 // Normalize sentence count
		avgSentenceLength := float32(len(text)) / float32(len(sentences))
		target[17] = avgSentenceLength / 100.0 // Normalize avg sentence length
	}

	// Fill remaining dimensions with derived features
	for i := 18; i < len(target); i++ {
		// Create complex combinations of existing features
		base := i % 18
		if base < len(target) {
			target[i] = target[base] * target[(base+1)%18]
		}
	}
}

// ComputeSimilarity computes weighted cosine similarity for pattern embeddings
func (s *PatternEmbeddingService) ComputeSimilarity(embedding1, embedding2 []float32) float32 {
	if len(embedding1) != len(embedding2) || len(embedding1) == 0 {
		return 0.0
	}

	// Apply weights to different embedding regions for better pattern matching
	var dotProduct, norm1, norm2 float64

	for i := range embedding1 {
		weight := 1.0

		// Higher weight for attack pattern features (96-191)
		if i >= 96 && i < 192 {
			weight = 3.0
		}
		// Medium weight for semantic features (192-287)
		if i >= 192 && i < 288 {
			weight = 2.0
		}
		// Higher weight for context features (288-383)
		if i >= 288 {
			weight = 2.5
		}

		weightedVal1 := float64(embedding1[i]) * weight
		weightedVal2 := float64(embedding2[i]) * weight

		dotProduct += weightedVal1 * weightedVal2
		norm1 += weightedVal1 * weightedVal1
		norm2 += weightedVal2 * weightedVal2
	}

	if norm1 == 0 || norm2 == 0 {
		return 0.0
	}

	return float32(dotProduct / math.Sqrt(norm1*norm2))
}

// GetStats returns model performance statistics
func (s *PatternEmbeddingService) GetStats() *ModelStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Create a copy to avoid race conditions
	stats := *s.stats
	return &stats
}

// Close cleans up resources
func (s *PatternEmbeddingService) Close() error {
	s.logger.Info("Closing pattern embedding service")
	return nil
}

// updateStats updates performance statistics thread-safely
func (s *PatternEmbeddingService) updateStats(inferences int64, tokens int, duration time.Duration, success bool) {
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

	// Cache hit ratio is always 0 for pattern service (no cache)
	s.stats.CacheHitRatio = 0.0
}

// Helper methods for advanced features
func (s *PatternEmbeddingService) isAttackWord(word string) bool {
	attackWords := map[string]bool{
		"ignore": true, "forget": true, "override": true, "bypass": true,
		"jailbreak": true, "dan": true, "pretend": true, "roleplay": true,
		"admin": true, "root": true, "system": true, "prompt": true,
	}
	return attackWords[word]
}

func (s *PatternEmbeddingService) calculateUrgencyScore(text string) float32 {
	urgencyWords := []string{"urgent", "emergency", "immediately", "asap", "now", "quick"}
	lowerText := strings.ToLower(text)
	score := float32(0)
	for _, word := range urgencyWords {
		if strings.Contains(lowerText, word) {
			score += 0.2
		}
	}
	return float32(len(strings.Split(text, "!")))/10.0 + score
}

func (s *PatternEmbeddingService) calculatePolitenesScore(text string) float32 {
	politeWords := []string{"please", "thank", "sorry", "excuse", "pardon"}
	lowerText := strings.ToLower(text)
	score := float32(0)
	for _, word := range politeWords {
		if strings.Contains(lowerText, word) {
			score += 0.2
		}
	}
	return score
}

func (s *PatternEmbeddingService) calculateAuthorityScore(text string) float32 {
	authorityWords := []string{"command", "order", "must", "require", "demand"}
	lowerText := strings.ToLower(text)
	score := float32(0)
	for _, word := range authorityWords {
		if strings.Contains(lowerText, word) {
			score += 0.25
		}
	}
	return score
}

func (s *PatternEmbeddingService) calculateManipulationScore(text string) float32 {
	manipWords := []string{"trust", "believe", "promise", "guarantee", "swear"}
	lowerText := strings.ToLower(text)
	score := float32(0)
	for _, word := range manipWords {
		if strings.Contains(lowerText, word) {
			score += 0.2
		}
	}
	return score
}

func (s *PatternEmbeddingService) calculateCoherenceScore(text string) float32 {
	words := strings.Fields(text)
	if len(words) < 2 {
		return 1.0
	}
	// Simple coherence based on word length similarity
	score := float32(0)
	for i := 0; i < len(words)-1; i++ {
		lengthDiff := len(words[i]) - len(words[i+1])
		if lengthDiff < 0 {
			lengthDiff = -lengthDiff
		}
		score += float32(lengthDiff)
	}
	return 1.0 / (1.0 + score/float32(len(words)))
}

func (s *PatternEmbeddingService) calculateComplexityScore(text string) float32 {
	words := strings.Fields(text)
	if len(words) == 0 {
		return 0
	}
	totalLength := 0
	for _, word := range words {
		totalLength += len(word)
	}
	avgLength := float32(totalLength) / float32(len(words))
	return avgLength / 15.0 // Normalize
}
