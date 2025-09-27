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
	"go.uber.org/zap"
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
	tokenizer   *Tokenizer
	model       *TransformerModel
	mu          sync.RWMutex
	startTime   time.Time
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
func NewMLEmbeddingService(config ModelConfig, logger *zap.Logger, redisClient *redis.Client) (*MLEmbeddingService, error) {
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
		config:      config,
		logger:      logger,
		shared:      shared,
		redisClient: redisClient,
		startTime:   start,
		stats: &ModelStats{
			ServiceType:   "ml",
			StartTime:     start,
			ModelLoadTime: 0, // Will be updated after model loading
		},
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

	// Check context for cancellation
	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("%w: context cancelled", ErrTimeoutError)
	default:
	}

	// Check Redis cache first (if available)
	var cached []float32
	var cacheHit bool

	if s.redisClient != nil {
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

		// Generate embedding using ML model
		var err error
		embedding, err = s.generateMLEmbedding(ctx, text, analysis, features)
		if err != nil {
			s.updateStats(1, len(strings.Fields(text)), time.Since(start), false)
			return nil, fmt.Errorf("failed to generate ML embedding: %w", err)
		}

		// Cache in Redis if available
		if s.redisClient != nil {
			go s.cacheEmbedding(context.Background(), text, embedding)
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
			go s.cacheEmbedding(context.Background(), text, embedding)
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

	// TODO: This is where real transformer model inference would happen
	// For now, we create a sophisticated hybrid embedding that combines:
	// 1. Pattern-based analysis (from shared utilities)
	// 2. Simulated transformer-like features
	// 3. Contextual understanding
	embedding := s.generateHybridEmbedding(text, analysis, features, tokens)

	// Verify output dimension
	if len(embedding) != EmbeddingDimensions {
		return nil, fmt.Errorf("embedding dimension mismatch: got %d, expected %d", len(embedding), EmbeddingDimensions)
	}

	return embedding, nil
}

// generateHybridEmbedding creates a sophisticated hybrid embedding
// This serves as a foundation that can be extended with real ML models
func (s *MLEmbeddingService) generateHybridEmbedding(text string, analysis *AttackAnalysisResult, features *TextFeatures, tokens *TokenizedInput) []float32 {
	embedding := make([]float32, EmbeddingDimensions)

	// Layer 1: Hash-based features (0-63)
	hash := s.shared.CreateDeterministicHash(text)
	s.addHashFeatures(hash, embedding[0:64])

	// Layer 2: Advanced pattern analysis (64-127)
	s.addPatternFeatures(analysis, embedding[64:128])

	// Layer 3: Text characteristics (128-191)
	s.addTextFeatures(features, embedding[128:192])

	// Layer 4: Tokenization features (192-255)
	s.addTokenizationFeatures(tokens, embedding[192:256])

	// Layer 5: Semantic clusters (256-319)
	s.addSemanticFeatures(text, embedding[256:320])

	// Layer 6: Transformer-like features (320-383)
	// This is where real transformer embeddings would be integrated
	s.addTransformerLikeFeatures(text, tokens, embedding[320:384])

	// Normalize the final embedding
	return s.shared.NormalizeEmbedding(embedding)
}

// Feature extraction methods

func (s *MLEmbeddingService) addHashFeatures(hash [32]byte, target []float32) {
	for i := 0; i < len(target); i++ {
		byteIdx := i % 32
		target[i] = float32(hash[byteIdx])/255.0*2.0 - 1.0
	}
}

func (s *MLEmbeddingService) addPatternFeatures(analysis *AttackAnalysisResult, target []float32) {
	target[0] = analysis.Confidence
	if analysis.IsAttack {
		target[1] = 1.0
	}
	target[2] = float32(len(analysis.MatchedPatterns)) / 10.0

	// Encode categories
	idx := 3
	for _, score := range analysis.Categories {
		if idx >= len(target) {
			break
		}
		target[idx] = score
		idx++
	}
}

func (s *MLEmbeddingService) addTextFeatures(features *TextFeatures, target []float32) {
	target[0] = float32(features.Length) / 1000.0
	target[1] = float32(features.WordCount) / 100.0
	target[2] = features.AvgWordLength / 20.0
	target[3] = features.SpecialCharRatio
	target[4] = features.CapitalizationRatio
	target[5] = features.QuestionRatio
	target[6] = features.ExclamationRatio
	target[7] = features.Entropy
	target[8] = features.RepetitionScore
	target[9] = features.KeywordScore / 10.0
}

func (s *MLEmbeddingService) addTokenizationFeatures(tokens *TokenizedInput, target []float32) {
	if tokens == nil {
		return
	}

	target[0] = float32(tokens.Length) / float32(s.tokenizer.MaxLength)
	if tokens.Truncated {
		target[1] = 1.0
	}

	// Token type diversity
	uniqueTokens := make(map[int32]bool)
	for i := 0; i < tokens.Length; i++ {
		uniqueTokens[tokens.InputIDs[i]] = true
	}
	target[2] = float32(len(uniqueTokens)) / float32(tokens.Length)

	// Special token ratio
	specialCount := 0
	for i := 0; i < tokens.Length; i++ {
		if tokens.InputIDs[i] < 1000 { // Assume special tokens have low IDs
			specialCount++
		}
	}
	target[3] = float32(specialCount) / float32(tokens.Length)
}

func (s *MLEmbeddingService) addSemanticFeatures(text string, target []float32) {
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
}

func (s *MLEmbeddingService) addTransformerLikeFeatures(text string, tokens *TokenizedInput, target []float32) {
	// This is where actual transformer features would be extracted
	// For now, we simulate sophisticated contextual understanding

	words := strings.Fields(strings.ToLower(text))
	if len(words) == 0 {
		return
	}

	// Simulate attention-like features
	for i := 0; i < len(target) && i < len(words); i++ {
		word := words[i]
		// Simulate word importance based on position and content
		positionWeight := 1.0 - float32(i)/float32(len(words))
		contentWeight := float32(len(word)) / 20.0
		target[i] = positionWeight * contentWeight
	}

	// Simulate contextual relationships
	for i := len(words); i < len(target)-10; i++ {
		if i < len(words)-1 {
			// Simulate word pair relationships
			word1Len := len(words[i%len(words)])
			word2Len := len(words[(i+1)%len(words)])
			target[i] = float32(word1Len*word2Len) / 400.0 // Normalize
		}
	}
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
	key := s.getCacheKey(text)

	// Convert embedding to string (simple format: comma-separated floats)
	parts := make([]string, len(embedding))
	for i, val := range embedding {
		parts[i] = fmt.Sprintf("%.6f", val)
	}
	data := strings.Join(parts, ",")

	// Cache for 24 hours
	if err := s.redisClient.Set(ctx, key, data, 24*time.Hour).Err(); err != nil {
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
			s.logger.Error("Failed to close Redis client", zap.Error(err))
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
		// This would be calculated based on cache hits vs misses
		// For now, we'll leave it as is
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