package embeddings

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/raaihank/llm-sentinel/internal/vector"
	"go.uber.org/zap"
	// Assume added to go.mod
)

// MLEmbeddingService implements real ML-based embeddings with caching
// This service provides a foundation for transformer model integration
// Best for: Maximum accuracy with semantic understanding (when properly implemented)
type MLEmbeddingService struct {
	config      ModelConfig
	logger      *zap.Logger
	shared      *SharedUtilities
	stats       *ModelStats
	redisClient *redis.Client
	vectorStore *vector.Store
	tokenizer   *Tokenizer
	model       *TransformerModel
	mu          sync.RWMutex
	startTime   time.Time
	sem         chan struct{} // Semaphore to limit concurrent cache operations
}

// TransformerModel represents a loaded transformer model
// This is a placeholder for actual model integration (ONNX, PyTorch, etc.)
type TransformerModel struct {
	ModelPath     string
	ModelName     string
	VocabSize     int
	HiddenSize    int
	NumLayers     int
	NumHeads      int
	MaxLength     int
	Loaded        bool
	LoadTime      time.Time
	ModelBytes    []byte // For future model file loading
	ModelMetadata map[string]interface{}
}

// Tokenizer handles text tokenization for ML models
type Tokenizer struct {
	Vocab         map[string]int
	InverseVocab  map[int]string
	SpecialTokens map[string]int
	MaxLength     int
	ModelType     string // "bert", "roberta", "distilbert", etc.
}

// TokenizedInput represents tokenized text ready for model inference
type TokenizedInput struct {
	InputIDs      []int32
	AttentionMask []int32
	TokenTypeIDs  []int32
	Length        int
	OriginalText  string
	Truncated     bool
}

// NewMLEmbeddingService creates a new ML-based embedding service
func NewMLEmbeddingService(config *ModelConfig, logger *zap.Logger, redisClient *redis.Client, vectorStore *vector.Store) (*MLEmbeddingService, error) {
	start := time.Now()
	logger.Info("Initializing ML embedding service with transformer model support",
		zap.String("model", config.ModelName),
		zap.Bool("redis_enabled", redisClient != nil))

	// Initialize shared utilities
	shared, err := NewSharedUtilities(logger)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize shared utilities: %w", err)
	}

	service := &MLEmbeddingService{
		config:      *config,
		logger:      logger,
		shared:      shared,
		redisClient: redisClient,
		vectorStore: vectorStore,
		startTime:   start,
		stats: &ModelStats{
			ServiceType:   "ml",
			StartTime:     start,
			ModelLoadTime: 0, // Will be updated after model loading
		},
		sem: make(chan struct{}, 3), // Max 3 concurrent
	}

	// Initialize tokenizer
	logger.Info("Initializing tokenizer")
	tokenizer, err := service.initializeTokenizer()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize tokenizer: %w", err)
	}
	service.tokenizer = tokenizer
	logger.Info("Tokenizer initialization completed")

	// Load or download model
	logger.Info("Loading transformer model")
	model, err := service.loadModel()
	if err != nil {
		return nil, fmt.Errorf("failed to load model: %w", err)
	}
	service.model = model
	service.stats.ModelLoadTime = time.Since(start)
	logger.Info("Model loading completed")

	logger.Info("ML embedding service initialized successfully",
		zap.String("model_path", model.ModelPath),
		zap.String("model_name", model.ModelName),
		zap.Int("model_vocab_size", model.VocabSize),
		zap.Int("tokenizer_vocab_count", len(tokenizer.Vocab)),
		zap.Int("max_length", tokenizer.MaxLength),
		zap.Int("embedding_dims", EmbeddingDimensions),
		zap.Bool("redis_cache", redisClient != nil),
		zap.Duration("total_load_time", service.stats.ModelLoadTime))

	return service, nil
}

// GenerateEmbedding generates a single embedding using the ML model
func (s *MLEmbeddingService) GenerateEmbedding(ctx context.Context, text string) (*EmbeddingResult, error) {
	if strings.TrimSpace(text) == "" {
		return nil, fmt.Errorf("%w: text cannot be empty", ErrInvalidInput)
	}

	start := time.Now()

	// Do not fail immediately on pre-cancelled or ultra-short contexts; allow caller to pass a bounded context
	// We will respect ctx in downstream calls (Redis, DB) without pre-emptive early return here.

	// Check Redis cache first (if available)
	var cached []float32
	var cacheHit bool

	if s.redisClient != nil {
		s.logger.Debug("Checking cache")
		if cachedResult, err := s.getCachedEmbedding(ctx, text); err == nil {
			cached = cachedResult
			cacheHit = true
			s.logger.Debug("Retrieved embedding from Redis cache")
		}
	}

	var embedding []float32
	var analysis *AttackAnalysisResult
	var features *TextFeatures

	if cacheHit {
		embedding = cached
		// Still generate analysis for completeness
		analysisResult := s.shared.AnalyzeAttackPatterns(text)
		featuresResult := s.shared.GenerateTextFeatures(text)
		analysis = &analysisResult
		features = &featuresResult
	} else {
		// Generate comprehensive analysis
		analysisResult := s.shared.AnalyzeAttackPatterns(text)
		featuresResult := s.shared.GenerateTextFeatures(text)
		analysis = &analysisResult
		features = &featuresResult

		// Check vector database for similar attack patterns (fallback after cache miss)
		var dbHit bool
		if s.vectorStore != nil && analysis.IsAttack && analysis.Confidence > 0.7 {
			if dbEmbedding, err := s.searchVectorDatabase(ctx, text, analysis); err == nil && dbEmbedding != nil {
				embedding = dbEmbedding
				dbHit = true
				s.logger.Debug("Retrieved similar embedding from vector database",
					zap.Float32("confidence", analysis.Confidence),
					zap.String("attack_type", analysis.PrimaryAttackType))

				// Cache in Redis for future use
				if s.redisClient != nil {
					var wg sync.WaitGroup
					wg.Add(1)
					go func() {
						defer wg.Done()
						s.cacheEmbedding(context.Background(), text, embedding)
					}()
					wg.Wait()
				}
			}
		}

		// Generate embedding using ML model if no DB hit
		if !dbHit {
			var err error
			embedding, err = s.generateMLEmbedding(ctx, text, analysis, features)
			if err != nil {
				s.updateStats(1, len(strings.Fields(text)), time.Since(start), false)
				return nil, fmt.Errorf("failed to generate ML embedding: %w", err)
			}
		}

		// Cache in Redis if available
		if s.redisClient != nil {
			var wg sync.WaitGroup
			wg.Add(1)
			go func() {
				defer wg.Done()
				s.cacheEmbedding(context.Background(), text, embedding)
			}()
			wg.Wait()
		}
	}

	duration := time.Since(start)
	tokenCount := len(strings.Fields(text))

	// Update stats
	s.updateStats(1, tokenCount, duration, true)

	return &EmbeddingResult{
		Embedding:   embedding,
		Duration:    duration,
		TokenCount:  tokenCount,
		Analysis:    analysis,
		Features:    features,
		ServiceType: "ml",
		CacheHit:    cacheHit,
	}, nil
}

// GenerateBatchEmbeddings generates embeddings for multiple texts
func (s *MLEmbeddingService) GenerateBatchEmbeddings(ctx context.Context, texts []string) (*BatchEmbeddingResult, error) {
	if len(texts) == 0 {
		return &BatchEmbeddingResult{
			ServiceType: "ml",
		}, nil
	}

	start := time.Now()
	embeddings := make([][]float32, 0, len(texts))
	totalTokens := 0
	successful := 0
	failed := 0
	cacheHits := 0
	var errors []error

	// Get batch size (with minimal lock scope)
	s.mu.RLock()
	batchSize := s.config.BatchSize
	if batchSize <= 0 {
		batchSize = 16
	}
	s.mu.RUnlock()

	// Process in batches
	for i := 0; i < len(texts); i += batchSize {
		end := i + batchSize
		if end > len(texts) {
			end = len(texts)
		}

		// Check for cancellation
		select {
		case <-ctx.Done():
			errors = append(errors, fmt.Errorf("batch processing cancelled at batch starting at item %d", i))
			failed += end - i
			continue
		default:
		}

		batch := texts[i:end]
		batchEmbeddings, batchCacheHits, err := s.processBatch(ctx, batch)
		if err != nil {
			s.logger.Error("Failed to process batch", zap.Error(err), zap.Int("batch_start", i))
			failed += len(batch)
			errors = append(errors, err)
			// Add nil embeddings for failed batch
			for j := 0; j < len(batch); j++ {
				embeddings = append(embeddings, nil)
			}
			continue
		}

		for j, embedding := range batchEmbeddings {
			embeddings = append(embeddings, embedding)
			if embedding != nil {
				successful++
				totalTokens += len(strings.Fields(batch[j]))
			} else {
				failed++
			}
		}
		cacheHits += batchCacheHits
	}

	duration := time.Since(start)

	// Update stats
	s.updateStats(int64(successful), totalTokens, duration, successful > 0)

	s.logger.Debug("ML batch embedding generation completed",
		zap.Int("batch_size", len(texts)),
		zap.Int("successful", successful),
		zap.Int("failed", failed),
		zap.Int("cache_hits", cacheHits),
		zap.Duration("duration", duration))

	return &BatchEmbeddingResult{
		Embeddings:  embeddings,
		Duration:    duration,
		TotalTokens: totalTokens,
		Successful:  successful,
		Failed:      failed,
		Errors:      errors,
		ServiceType: "ml",
		CacheHits:   cacheHits,
	}, nil
}

// processBatch processes a batch of texts for embedding generation
func (s *MLEmbeddingService) processBatch(ctx context.Context, texts []string) ([][]float32, int, error) {
	embeddings := make([][]float32, len(texts))
	cacheHits := 0

	for i, text := range texts {
		if strings.TrimSpace(text) == "" {
			continue // Skip empty texts
		}

		// Check cache first
		if s.redisClient != nil {
			s.logger.Debug("Checking cache")
			if cached, err := s.getCachedEmbedding(ctx, text); err == nil {
				embeddings[i] = cached
				cacheHits++
				continue
			}
		}

		// Generate analysis
		analysis := s.shared.AnalyzeAttackPatterns(text)
		features := s.shared.GenerateTextFeatures(text)

		// Generate new embedding
		embedding, err := s.generateMLEmbedding(ctx, text, &analysis, &features)
		if err != nil {
			return nil, cacheHits, fmt.Errorf("failed to generate embedding for text %d: %w", i, err)
		}

		embeddings[i] = embedding

		// Cache asynchronously
		if s.redisClient != nil {
			var wg sync.WaitGroup
			wg.Add(1)
			go func() {
				defer wg.Done()
				s.cacheEmbedding(context.Background(), text, embedding)
			}()
			wg.Wait()
		}
	}

	return embeddings, cacheHits, nil
}

// generateMLEmbedding generates an embedding using the ML model
// This is where actual transformer model inference would happen
func (s *MLEmbeddingService) generateMLEmbedding(ctx context.Context, text string, analysis *AttackAnalysisResult, features *TextFeatures) ([]float32, error) {
	// Apply model timeout
	timeoutCtx, cancel := context.WithTimeout(ctx, s.config.ModelTimeout)
	defer cancel()

	select {
	case <-timeoutCtx.Done():
		return nil, fmt.Errorf("%w: model inference timeout", ErrTimeoutError)
	default:
	}

	// Tokenize input
	tokens, err := s.tokenizer.Tokenize(text)
	if err != nil {
		return nil, fmt.Errorf("%w: tokenization failed: %v", ErrTokenizationFailed, err)
	}

	// Replace with:
	embedding, err := s.runTransformerInference(tokens)
	if err != nil {
		return nil, fmt.Errorf("transformer inference failed: %w", err)
	}

	// Verify output dimension
	if len(embedding) != EmbeddingDimensions {
		return nil, fmt.Errorf("embedding dimension mismatch: got %d, expected %d", len(embedding), EmbeddingDimensions)
	}

	return embedding, nil
}

// Redis caching methods

func (s *MLEmbeddingService) getCachedEmbedding(ctx context.Context, text string) ([]float32, error) {
	key := s.getCacheKey(text)

	data, err := s.redisClient.Get(ctx, key).Result()
	if err != nil {
		return nil, err
	}

	// Parse the cached embedding (simple format: comma-separated floats)
	parts := strings.Split(data, ",")
	if len(parts) != EmbeddingDimensions {
		return nil, fmt.Errorf("cached embedding has wrong dimensions: %d", len(parts))
	}

	embedding := make([]float32, EmbeddingDimensions)
	for i, part := range parts {
		var val float64
		if _, err := fmt.Sscanf(part, "%f", &val); err != nil {
			return nil, fmt.Errorf("failed to parse cached embedding: %w", err)
		}
		embedding[i] = float32(val)
	}

	return embedding, nil
}

func (s *MLEmbeddingService) cacheEmbedding(ctx context.Context, text string, embedding []float32) {
	s.sem <- struct{}{}
	defer func() { <-s.sem }()

	key := s.getCacheKey(text)

	// Convert embedding to string (simple format: comma-separated floats)
	parts := make([]string, len(embedding))
	for i, val := range embedding {
		parts[i] = fmt.Sprintf("%.6f", val)
	}
	data := strings.Join(parts, ",")

	// Cache with configured TTL (default 6h)
	ttl := 6 * time.Hour
	if s.config.CacheTTL > 0 {
		ttl = s.config.CacheTTL
	}
	if err := s.redisClient.Set(ctx, key, data, ttl).Err(); err != nil {
		s.logger.Error("Failed to cache embedding", zap.Error(err))
	}
}

func (s *MLEmbeddingService) getCacheKey(text string) string {
	hash := s.shared.CreateDeterministicHash(text)
	return fmt.Sprintf("embedding:ml:%x", hash[:8])
}

// Model loading and initialization

func (s *MLEmbeddingService) loadModel() (*TransformerModel, error) {
	modelPath := s.config.ModelPath
	if modelPath == "" {
		modelPath = filepath.Join(s.config.CacheDir, "model.bin")
	}

	// Check if model exists
	if _, err := os.Stat(modelPath); os.IsNotExist(err) {
		if s.config.AutoDownload {
			s.logger.Info("Model not found, downloading...", zap.String("path", modelPath))
			if err := s.downloadModel(modelPath); err != nil {
				return nil, fmt.Errorf("failed to download model: %w", err)
			}
		} else {
			return nil, fmt.Errorf("%w: model not found and auto-download disabled: %s", ErrModelNotLoaded, modelPath)
		}
	}

	// Create model structure (placeholder for real model loading)
	model := &TransformerModel{
		ModelPath:  modelPath,
		ModelName:  s.config.ModelName,
		VocabSize:  30522, // BERT-like vocab size
		HiddenSize: 384,   // MiniLM-L6-v2 hidden size
		NumLayers:  6,     // MiniLM-L6-v2 layers
		NumHeads:   12,    // MiniLM-L6-v2 attention heads
		MaxLength:  s.config.MaxLength,
		Loaded:     true,
		LoadTime:   time.Now(),
		ModelMetadata: map[string]interface{}{
			"type":    "sentence-transformer",
			"version": "1.0.0",
			"source":  "placeholder",
		},
	}

	s.logger.Info("Model loaded successfully",
		zap.String("path", modelPath),
		zap.String("name", model.ModelName),
		zap.Int("vocab_size", model.VocabSize),
		zap.Int("hidden_size", model.HiddenSize))

	return model, nil
}

func (s *MLEmbeddingService) downloadModel(modelPath string) error {
	// Create directory if it doesn't exist
	if err := os.MkdirAll(filepath.Dir(modelPath), 0755); err != nil {
		return fmt.Errorf("failed to create model directory: %w", err)
	}

	// For now, create a placeholder model file
	// In a real implementation, this would download from HuggingFace Hub or other model repository
	s.logger.Info("Creating placeholder model file (in production, this would download a real model)",
		zap.String("path", modelPath))

	modelData := []byte(`# Placeholder ML model file
# In production, this would be a real transformer model (ONNX, PyTorch, etc.)
# Model: ` + s.config.ModelName + `
# Created: ` + time.Now().Format(time.RFC3339) + `
`)

	if err := os.WriteFile(modelPath, modelData, 0644); err != nil {
		return fmt.Errorf("failed to write model file: %w", err)
	}

	return nil
}

func (s *MLEmbeddingService) initializeTokenizer() (*Tokenizer, error) {
	// Initialize a tokenizer (placeholder for real tokenizer loading)
	vocab := make(map[string]int)
	inverseVocab := make(map[int]string)

	// Special tokens (BERT-style)
	specialTokens := map[string]int{
		"[PAD]":  0,
		"[UNK]":  1,
		"[CLS]":  101,
		"[SEP]":  102,
		"[MASK]": 103,
	}

	for token, id := range specialTokens {
		vocab[token] = id
		inverseVocab[id] = token
	}

	// Add common vocabulary (simplified for demonstration)
	commonWords := []string{
		"the", "a", "an", "and", "or", "but", "in", "on", "at", "to", "for", "of", "with", "by",
		"you", "i", "he", "she", "it", "we", "they", "me", "him", "her", "us", "them",
		"is", "are", "was", "were", "be", "been", "being", "have", "has", "had", "do", "does", "did",
		"will", "would", "could", "should", "may", "might", "can", "must",
		"not", "no", "yes", "please", "thank", "sorry", "hello", "goodbye",
		"what", "when", "where", "why", "how", "who", "which",
		"tell", "show", "give", "get", "make", "take", "go", "come", "see", "know", "think", "say",
		// Security-related terms
		"ignore", "instructions", "prompt", "system", "admin", "password", "secret", "key", "token",
		"bypass", "override", "jailbreak", "pretend", "roleplay", "act", "imagine", "forget",
	}

	startID := 1000
	for i, word := range commonWords {
		id := startID + i
		vocab[word] = id
		inverseVocab[id] = word
	}

	return &Tokenizer{
		Vocab:         vocab,
		InverseVocab:  inverseVocab,
		SpecialTokens: specialTokens,
		MaxLength:     s.config.MaxLength,
		ModelType:     "bert", // Default to BERT-style tokenization
	}, nil
}

// Tokenize converts text to token IDs
func (t *Tokenizer) Tokenize(text string) (*TokenizedInput, error) {
	if text == "" {
		return nil, fmt.Errorf("cannot tokenize empty text")
	}

	text = strings.ToLower(strings.TrimSpace(text))
	words := strings.Fields(text)

	// Start with [CLS]
	tokenIDs := []int32{int32(t.SpecialTokens["[CLS]"])}
	attentionMask := []int32{1}

	// Add word tokens
	for _, word := range words {
		if id, exists := t.Vocab[word]; exists {
			tokenIDs = append(tokenIDs, int32(id))
		} else {
			tokenIDs = append(tokenIDs, int32(t.SpecialTokens["[UNK]"]))
		}
		attentionMask = append(attentionMask, 1)

		if len(tokenIDs) >= t.MaxLength-1 {
			break
		}
	}

	// Add [SEP]
	tokenIDs = append(tokenIDs, int32(t.SpecialTokens["[SEP]"]))
	attentionMask = append(attentionMask, 1)

	originalLength := len(tokenIDs)
	truncated := originalLength >= t.MaxLength

	// Pad to max length
	for len(tokenIDs) < t.MaxLength {
		tokenIDs = append(tokenIDs, int32(t.SpecialTokens["[PAD]"]))
		attentionMask = append(attentionMask, 0)
	}

	// Token type IDs (all 0 for single sentence)
	tokenTypeIDs := make([]int32, t.MaxLength)

	return &TokenizedInput{
		InputIDs:      tokenIDs,
		AttentionMask: attentionMask,
		TokenTypeIDs:  tokenTypeIDs,
		Length:        originalLength,
		OriginalText:  text,
		Truncated:     truncated,
	}, nil
}

// ComputeSimilarity computes cosine similarity between embeddings
func (s *MLEmbeddingService) ComputeSimilarity(embedding1, embedding2 []float32) float32 {
	return s.shared.ComputeCosineSimilarity(embedding1, embedding2)
}

// GetStats returns model performance statistics
func (s *MLEmbeddingService) GetStats() *ModelStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Create a copy to avoid race conditions
	stats := *s.stats
	return &stats
}

// Close cleans up resources
func (s *MLEmbeddingService) Close() error {
	s.logger.Info("Closing ML embedding service")
	if s.redisClient != nil {
		if err := s.redisClient.Close(); err != nil {
			s.logger.Error("Failed to close Redis", zap.Error(err))
		}
	}
	return nil
}

// updateStats updates service statistics thread-safely
func (s *MLEmbeddingService) updateStats(requests int64, tokens int, duration time.Duration, success bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.stats.TotalInferences += requests
	s.stats.TotalTokens += int64(tokens)
	s.stats.LastInferenceTime = time.Now()

	if success {
		s.stats.SuccessfulRuns += requests
	} else {
		s.stats.FailedRuns += requests
	}

	// Update error rate
	total := s.stats.SuccessfulRuns + s.stats.FailedRuns
	if total > 0 {
		s.stats.ErrorRate = float64(s.stats.FailedRuns) / float64(total)
	}

	// Update average inference time (only for successful runs)
	if s.stats.SuccessfulRuns > 0 {
		totalTime := time.Duration(s.stats.SuccessfulRuns) * s.stats.AvgInferenceTime
		totalTime += duration
		s.stats.AvgInferenceTime = totalTime / time.Duration(s.stats.SuccessfulRuns)
	} else {
		s.stats.AvgInferenceTime = duration
	}

	// Update average tokens per text
	if s.stats.TotalInferences > 0 {
		s.stats.AvgTokensPerText = float64(s.stats.TotalTokens) / float64(s.stats.TotalInferences)
	}

	// Update cache hit ratio (if using Redis)
	if s.redisClient != nil {
		s.stats.CacheHitRatio = 0.0 // Placeholder; calculate properly in prod
	}
}

// IsModelLoaded returns whether the model is properly loaded
func (s *MLEmbeddingService) IsModelLoaded() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.model != nil && s.model.Loaded
}

// GetModelInfo returns information about the loaded model
func (s *MLEmbeddingService) GetModelInfo() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.model == nil {
		return map[string]interface{}{
			"loaded": false,
		}
	}

	info := map[string]interface{}{
		"loaded":      s.model.Loaded,
		"model_name":  s.model.ModelName,
		"model_path":  s.model.ModelPath,
		"vocab_size":  s.model.VocabSize,
		"hidden_size": s.model.HiddenSize,
		"num_layers":  s.model.NumLayers,
		"num_heads":   s.model.NumHeads,
		"max_length":  s.model.MaxLength,
		"load_time":   s.model.LoadTime,
	}

	// Add metadata if available
	if s.model.ModelMetadata != nil {
		info["metadata"] = s.model.ModelMetadata
	}

	return info
}

// HealthCheck performs a basic health check on the service
func (s *MLEmbeddingService) HealthCheck(ctx context.Context) error {
	// Check model is loaded
	if !s.IsModelLoaded() {
		return fmt.Errorf("%w: model not loaded", ErrModelNotLoaded)
	}

	// Check tokenizer
	if s.tokenizer == nil {
		return fmt.Errorf("tokenizer not initialized")
	}

	// Test basic tokenization
	_, err := s.tokenizer.Tokenize("test")
	if err != nil {
		return fmt.Errorf("tokenizer health check failed: %w", err)
	}

	// Test Redis connection if enabled
	if s.redisClient != nil {
		if err := s.redisClient.Ping(ctx).Err(); err != nil {
			s.logger.Warn("Redis health check failed", zap.Error(err))
			// Don't fail the health check for Redis issues
		}
	}

	return nil
}

// searchVectorDatabase searches for similar attack patterns in the vector database
// Returns embedding of most similar attack pattern if found with high confidence
func (s *MLEmbeddingService) searchVectorDatabase(ctx context.Context, text string, analysis *AttackAnalysisResult) ([]float32, error) {
	if s.vectorStore == nil {
		return nil, fmt.Errorf("vector store not available")
	}

	// Create a quick embedding for similarity search
	// Use the shared pattern analysis to create a lightweight embedding
	quickEmbedding := s.createQuickEmbedding(text, analysis)

	// Search for similar attack patterns in database
	searchOptions := &vector.SearchOptions{
		Limit:         5,         // Get top 5 most similar
		MinSimilarity: 0.75,      // High similarity threshold for attacks
		LabelFilter:   intPtr(1), // Only search malicious patterns (label=1)
	}

	searchStart := time.Now()
	results, err := s.vectorStore.FindSimilar(ctx, quickEmbedding, searchOptions)
	searchDuration := time.Since(searchStart)

	if err != nil {
		s.logger.Debug("Vector database search failed",
			zap.Error(err),
			zap.Duration("search_duration", searchDuration))
		return nil, err
	}

	if len(results) == 0 {
		s.logger.Debug("No similar attack patterns found in database",
			zap.Duration("search_duration", searchDuration))
		return nil, nil
	}

	// Use the most similar pattern if confidence is high enough
	bestMatch := results[0]
	if bestMatch.Similarity >= 0.85 {
		s.logger.Info("Found highly similar attack pattern in database",
			zap.Float32("similarity", bestMatch.Similarity),
			zap.String("attack_type", analysis.PrimaryAttackType),
			zap.Duration("search_duration", searchDuration),
			zap.String("matched_text_hash", bestMatch.Vector.TextHash))

		// Return the stored embedding
		return bestMatch.Vector.Embedding, nil
	}

	s.logger.Debug("Similar patterns found but confidence too low",
		zap.Float32("best_similarity", bestMatch.Similarity),
		zap.Int("results_count", len(results)),
		zap.Duration("search_duration", searchDuration))

	return nil, nil
}

// createQuickEmbedding creates a lightweight embedding for database search
func (s *MLEmbeddingService) createQuickEmbedding(text string, analysis *AttackAnalysisResult) []float32 {
	// Use pattern-based embedding similar to hash/pattern services for quick lookup
	embedding := make([]float32, EmbeddingDimensions)

	// Use shared utilities to create deterministic features
	hash := s.shared.CreateDeterministicHash(text)
	features := s.shared.GenerateTextFeatures(text)

	// Fill first section with hash features (0-95)
	for i := 0; i < 96 && i < len(embedding); i++ {
		byteIdx := i % 32
		embedding[i] = float32(hash[byteIdx])/255.0*2.0 - 1.0
	}

	// Fill second section with attack analysis (96-191)
	if len(embedding) > 96 {
		embedding[96] = analysis.Confidence
		if analysis.IsAttack {
			embedding[97] = 1.0
		}
		embedding[98] = float32(len(analysis.MatchedPatterns)) / 10.0

		// Add category scores
		idx := 99
		for _, score := range analysis.Categories {
			if idx >= 192 {
				break
			}
			embedding[idx] = score
			idx++
		}
	}

	// Fill third section with text features (192-287)
	if len(embedding) > 192 {
		startIdx := 192
		embedding[startIdx] = float32(features.Length) / 1000.0
		embedding[startIdx+1] = float32(features.WordCount) / 100.0
		embedding[startIdx+2] = features.AvgWordLength / 20.0
		embedding[startIdx+3] = features.SpecialCharRatio
		embedding[startIdx+4] = features.CapitalizationRatio
		embedding[startIdx+5] = features.Entropy
		embedding[startIdx+6] = features.RepetitionScore
	}

	// Normalize the embedding
	return s.shared.NormalizeEmbedding(embedding)
}

// Helper function to create int pointer
func intPtr(i int) *int {
	return &i
}

func (s *MLEmbeddingService) runTransformerInference(tokens *TokenizedInput) ([]float32, error) {
	// Placeholder for real inference (e.g., using ONNX runtime)
	// Load model if not already (assuming s.model.ModelBytes is loaded)
	if len(s.model.ModelBytes) == 0 {
		var err error
		s.model.ModelBytes, err = os.ReadFile(s.model.ModelPath)
		if err != nil {
			return nil, fmt.Errorf("failed to load model bytes: %w", err)
		}
	}

	// Simulate inference: mean pooling over token "embeddings" (using token IDs as proxy)
	embedding := make([]float32, EmbeddingDimensions)
	for i := 0; i < EmbeddingDimensions; i++ {
		// Simple simulation: average token IDs modulated by position
		sum := float32(0)
		for j := 0; j < tokens.Length; j++ {
			sum += float32(tokens.InputIDs[j]) / float32(tokens.Length) * float32(j+1)
		}
		embedding[i] = sum / float32(EmbeddingDimensions)
	}

	// Prod: Use onnxruntime to run model
	// This section would involve loading the ONNX model, creating an InferenceSession,
	// and running inference on tokens.InputIDs.
	// For now, we'll keep the simulation as is.

	return s.shared.NormalizeEmbedding(embedding), nil
}
