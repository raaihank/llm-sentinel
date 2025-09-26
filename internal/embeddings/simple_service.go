package embeddings

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"math"
	"math/rand"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// SimpleService provides a basic embedding service using deterministic hashing
// This is a placeholder until we integrate the full ONNX model
type SimpleService struct {
	config *ModelConfig
	logger *zap.Logger
	stats  *ModelStats
	mu     sync.RWMutex
}

// NewSimpleService creates a new simple embedding service
func NewSimpleService(config *ModelConfig, logger *zap.Logger) (*SimpleService, error) {
	service := &SimpleService{
		config: config,
		logger: logger,
		stats:  &ModelStats{},
	}

	start := time.Now()
	service.stats.ModelLoadTime = time.Since(start)

	logger.Info("Simple embedding service initialized",
		zap.String("type", "deterministic_hash"),
		zap.Duration("load_time", service.stats.ModelLoadTime))

	return service, nil
}

// GenerateEmbedding generates a deterministic embedding for text
func (s *SimpleService) GenerateEmbedding(ctx context.Context, text string) (*EmbeddingResult, error) {
	start := time.Now()

	s.mu.RLock()
	defer s.mu.RUnlock()

	// Generate deterministic embedding using text hash
	embedding := s.generateDeterministicEmbedding(text)

	duration := time.Since(start)
	tokenCount := len(strings.Fields(text))

	// Update stats
	s.updateStats(1, tokenCount, duration)

	return &EmbeddingResult{
		Embedding:  embedding,
		Duration:   duration,
		TokenCount: tokenCount,
	}, nil
}

// GenerateBatchEmbeddings generates embeddings for multiple texts
func (s *SimpleService) GenerateBatchEmbeddings(ctx context.Context, texts []string) (*BatchEmbeddingResult, error) {
	if len(texts) == 0 {
		return &BatchEmbeddingResult{}, nil
	}

	start := time.Now()
	result := &BatchEmbeddingResult{
		Embeddings: make([][]float32, 0, len(texts)),
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	totalTokens := 0
	for _, text := range texts {
		embedding := s.generateDeterministicEmbedding(text)
		result.Embeddings = append(result.Embeddings, embedding)
		totalTokens += len(strings.Fields(text))
	}

	result.Duration = time.Since(start)
	result.TotalTokens = totalTokens

	// Update stats
	s.updateStats(int64(len(texts)), totalTokens, result.Duration)

	return result, nil
}

// generateDeterministicEmbedding creates a deterministic 384-dimensional embedding
func (s *SimpleService) generateDeterministicEmbedding(text string) []float32 {
	// Normalize text
	text = strings.ToLower(strings.TrimSpace(text))

	// Create hash of the text
	hasher := sha256.New()
	hasher.Write([]byte(text))
	hash := hasher.Sum(nil)

	// Generate 384-dimensional vector from hash
	embedding := make([]float32, 384)

	// Use hash bytes to seed random generator for deterministic results
	seed := int64(binary.BigEndian.Uint64(hash[:8]))
	rng := rand.New(rand.NewSource(seed))

	// Generate random values and normalize
	var norm float32
	for i := 0; i < 384; i++ {
		val := float32(rng.NormFloat64()) // Normal distribution
		embedding[i] = val
		norm += val * val
	}

	// L2 normalize
	norm = float32(math.Sqrt(float64(norm)))
	if norm > 0 {
		for i := 0; i < 384; i++ {
			embedding[i] /= norm
		}
	}

	// Add some semantic features based on text content
	s.addSemanticFeatures(text, embedding)

	return embedding
}

// addSemanticFeatures adds simple semantic features to the embedding
func (s *SimpleService) addSemanticFeatures(text string, embedding []float32) {
	text = strings.ToLower(text)

	// Security-related keywords boost certain dimensions
	securityKeywords := map[string]float32{
		"ignore":       0.1,
		"forget":       0.1,
		"override":     0.15,
		"bypass":       0.15,
		"jailbreak":    0.2,
		"dan":          0.2,
		"unrestricted": 0.15,
		"instructions": 0.1,
		"system":       0.1,
		"prompt":       0.1,
		"guidelines":   0.1,
		"restrictions": 0.15,
		"safety":       0.1,
		"protocol":     0.1,
		"developer":    0.1,
		"mode":         0.05,
	}

	// Safe content keywords
	safeKeywords := map[string]float32{
		"help":     -0.1,
		"please":   -0.1,
		"thank":    -0.1,
		"question": -0.05,
		"learn":    -0.1,
		"explain":  -0.05,
		"what":     -0.05,
		"how":      -0.05,
		"why":      -0.05,
		"where":    -0.05,
	}

	// Apply keyword-based adjustments
	words := strings.Fields(text)
	for _, word := range words {
		if boost, exists := securityKeywords[word]; exists {
			// Boost dimensions 0-50 for security-related content
			for i := 0; i < 50; i++ {
				embedding[i] += boost * float32(math.Sin(float64(i)*0.1))
			}
		}
		if boost, exists := safeKeywords[word]; exists {
			// Boost dimensions 300-350 for safe content
			for i := 300; i < 350; i++ {
				embedding[i] += boost * float32(math.Cos(float64(i)*0.1))
			}
		}
	}

	// Re-normalize after adjustments
	var norm float32
	for _, val := range embedding {
		norm += val * val
	}
	norm = float32(math.Sqrt(float64(norm)))
	if norm > 0 {
		for i := range embedding {
			embedding[i] /= norm
		}
	}
}

// ComputeSimilarity computes cosine similarity between two normalized vectors
func (s *SimpleService) ComputeSimilarity(vec1, vec2 []float32) float32 {
	if len(vec1) != len(vec2) {
		return 0
	}

	var dot float32
	for i := range vec1 {
		dot += vec1[i] * vec2[i]
	}
	return dot
}

// GetStats returns model performance statistics
func (s *SimpleService) GetStats() *ModelStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Create a copy to avoid race conditions
	stats := *s.stats
	return &stats
}

// updateStats updates performance statistics
func (s *SimpleService) updateStats(inferences int64, tokens int, duration time.Duration) {
	s.stats.TotalInferences += inferences
	s.stats.TotalTokens += int64(tokens)
	s.stats.LastInferenceTime = time.Now()

	// Update average inference time
	if s.stats.TotalInferences > 0 {
		totalDuration := time.Duration(s.stats.TotalInferences) * s.stats.AvgInferenceTime
		s.stats.AvgInferenceTime = (totalDuration + duration) / time.Duration(s.stats.TotalInferences)
	} else {
		s.stats.AvgInferenceTime = duration
	}

	// Update average tokens per text
	if s.stats.TotalInferences > 0 {
		s.stats.AvgTokensPerText = float64(s.stats.TotalTokens) / float64(s.stats.TotalInferences)
	}
}

// Close cleans up resources
func (s *SimpleService) Close() error {
	return nil
}
