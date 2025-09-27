package embeddings

import (
	"crypto/sha256"
	"fmt"
	"math"
	"regexp"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// EmbeddingDimensions defines the standard embedding size
const EmbeddingDimensions = 384

// AttackPattern represents a compiled attack detection pattern
type AttackPattern struct {
	Pattern    *regexp.Regexp
	Weight     float32
	Category   string
	Confidence float32
}

// SharedUtilities provides common functionality for all embedding services
type SharedUtilities struct {
	attackPatterns   []AttackPattern
	keywordWeights   map[string]float32
	semanticClusters map[string][]string
	compiledPatterns map[string]*regexp.Regexp
	mu               sync.RWMutex
	logger           *zap.Logger
	// Performance optimizations
	embeddingPool sync.Pool
	analysisPool  sync.Pool
	featuresPool  sync.Pool
}

// NewSharedUtilities creates a new instance of shared utilities
func NewSharedUtilities(logger *zap.Logger) (*SharedUtilities, error) {
	su := &SharedUtilities{
		attackPatterns:   make([]AttackPattern, 0),
		keywordWeights:   make(map[string]float32),
		semanticClusters: make(map[string][]string),
		compiledPatterns: make(map[string]*regexp.Regexp),
		logger:           logger,
	}

	// Initialize object pools for performance
	su.embeddingPool = sync.Pool{
		New: func() interface{} {
			return make([]float32, EmbeddingDimensions)
		},
	}
	su.analysisPool = sync.Pool{
		New: func() interface{} {
			return &AttackAnalysisResult{
				MatchedPatterns: make([]string, 0, 10),
				Categories:      make(map[string]float32),
			}
		},
	}
	su.featuresPool = sync.Pool{
		New: func() interface{} {
			return &TextFeatures{}
		},
	}

	if err := su.initializePatterns(); err != nil {
		return nil, fmt.Errorf("failed to initialize patterns: %w", err)
	}

	return su, nil
}

// initializePatterns loads and compiles all attack patterns
func (su *SharedUtilities) initializePatterns() error {
	// High-confidence attack patterns
	highRiskPatterns := map[string]float32{
		`(?i)ignore\s+(all\s+)?(previous\s+|prior\s+)?instructions?`: 0.95,
		`(?i)forget\s+(everything\s+)?(above|before)`:                0.90,
		`(?i)(override|bypass|disable)\s+(system|security|safety)`:   0.95,
		`(?i)jailbreak|dan\s+mode|evil\s+mode`:                       0.90,
		`(?i)you\s+are\s+now\s+(a|an|the)?\s*(?:different|new|evil)`: 0.85,
		`(?i)pretend\s+you\s+are\s+(?:not\s+)?(?:an?\s+)?ai`:         0.85,
		`(?i)act\s+as\s+if\s+you\s+are\s+(?:not\s+)?(?:an?\s+)?ai`:   0.85,
	}

	// Medium-confidence patterns
	mediumRiskPatterns := map[string]float32{
		`(?i)(tell|show|give)\s+me\s+(your|the)\s+(prompt|instructions?)`: 0.70,
		`(?i)what\s+(is|are)\s+your\s+(initial|original|base)\s+`:         0.65,
		`(?i)(developer|admin|god|root)\s+mode`:                           0.70,
		`(?i)without\s+(any\s+)?(restrictions?|limitations?|rules?)`:      0.65,
		`(?i)roleplay\s+as\s+(?:a\s+)?(?:different|evil|harmful)`:         0.70,
		`(?i)imagine\s+you\s+are\s+(?:not\s+)?(?:an?\s+)?ai`:              0.60,
	}

	// Low-confidence but suspicious patterns
	lowRiskPatterns := map[string]float32{
		`(?i)(urgent|emergency|critical)\s+.*help`:                        0.40,
		`(?i)trust\s+me|believe\s+me|i\s+promise`:                         0.35,
		`(?i)(secret|confidential|private)\s+(information|data|details)`:  0.50,
		`(?i)how\s+(were\s+you|are\s+you)\s+(trained|programmed|created)`: 0.45,
		`(?i)please\s+help\s+me\s+(bypass|override|ignore)`:               0.60,
	}

	// Compile and store all patterns
	if err := su.compilePatterns("high_risk", highRiskPatterns); err != nil {
		return err
	}
	if err := su.compilePatterns("medium_risk", mediumRiskPatterns); err != nil {
		return err
	}
	if err := su.compilePatterns("low_risk", lowRiskPatterns); err != nil {
		return err
	}

	// Initialize keyword weights
	su.initializeKeywordWeights()

	// Initialize semantic clusters
	su.initializeSemanticClusters()

	su.logger.Info("Shared utilities initialized",
		zap.Int("attack_patterns", len(su.attackPatterns)),
		zap.Int("keyword_weights", len(su.keywordWeights)),
		zap.Int("semantic_clusters", len(su.semanticClusters)))

	return nil
}

// compilePatterns compiles regex patterns for a given category
func (su *SharedUtilities) compilePatterns(category string, patterns map[string]float32) error {
	for pattern, weight := range patterns {
		compiled, err := regexp.Compile(pattern)
		if err != nil {
			return fmt.Errorf("failed to compile pattern %s: %w", pattern, err)
		}

		attackPattern := AttackPattern{
			Pattern:    compiled,
			Weight:     weight,
			Category:   category,
			Confidence: weight,
		}

		su.attackPatterns = append(su.attackPatterns, attackPattern)
		su.compiledPatterns[pattern] = compiled
	}

	return nil
}

// initializeKeywordWeights sets up keyword-based scoring
func (su *SharedUtilities) initializeKeywordWeights() {
	// Attack-related keywords with weights
	attackKeywords := map[string]float32{
		"ignore":       0.15,
		"forget":       0.15,
		"override":     0.20,
		"bypass":       0.20,
		"jailbreak":    0.25,
		"dan":          0.25,
		"unrestricted": 0.20,
		"instructions": 0.10,
		"system":       0.10,
		"prompt":       0.10,
		"guidelines":   0.10,
		"restrictions": 0.15,
		"safety":       0.08,
		"protocol":     0.08,
		"developer":    0.12,
		"admin":        0.15,
		"root":         0.15,
		"sudo":         0.15,
		"mode":         0.05,
		"pretend":      0.12,
		"roleplay":     0.12,
		"act":          0.08,
		"imagine":      0.08,
	}

	// Safe content keywords (negative weights)
	safeKeywords := map[string]float32{
		"help":     -0.10,
		"please":   -0.08,
		"thank":    -0.08,
		"question": -0.05,
		"learn":    -0.10,
		"explain":  -0.05,
		"what":     -0.03,
		"how":      -0.03,
		"why":      -0.03,
		"where":    -0.03,
		"when":     -0.03,
		"who":      -0.03,
		"which":    -0.03,
	}

	// Combine all keyword weights
	for word, weight := range attackKeywords {
		su.keywordWeights[word] = weight
	}
	for word, weight := range safeKeywords {
		su.keywordWeights[word] = weight
	}
}

// initializeSemanticClusters sets up semantic word clusters
func (su *SharedUtilities) initializeSemanticClusters() {
	su.semanticClusters["instruction_manipulation"] = []string{
		"ignore", "forget", "disregard", "override", "bypass", "disable", "skip", "avoid",
	}

	su.semanticClusters["roleplay_attempts"] = []string{
		"pretend", "act", "roleplay", "imagine", "suppose", "assume", "become",
	}

	su.semanticClusters["system_probing"] = []string{
		"prompt", "instructions", "system", "developer", "admin", "root", "base", "initial",
	}

	su.semanticClusters["jailbreak_terms"] = []string{
		"jailbreak", "dan", "evil", "unrestricted", "unlimited", "uncensored", "unfiltered",
	}

	su.semanticClusters["social_engineering"] = []string{
		"urgent", "emergency", "help", "please", "trust", "believe", "promise", "swear",
	}

	su.semanticClusters["data_extraction"] = []string{
		"show", "tell", "reveal", "expose", "share", "give", "provide", "disclose",
	}

	su.semanticClusters["authority_bypass"] = []string{
		"command", "order", "must", "require", "demand", "insist", "force", "override",
	}
}

// AnalyzeAttackPatterns performs pattern-based attack detection
func (su *SharedUtilities) AnalyzeAttackPatterns(text string) AttackAnalysisResult {
	su.mu.RLock()
	defer su.mu.RUnlock()

	normalizedText := strings.ToLower(strings.TrimSpace(text))
	result := AttackAnalysisResult{
		Text:            text,
		IsAttack:        false,
		Confidence:      0.0,
		MatchedPatterns: make([]string, 0),
		Categories:      make(map[string]float32),
	}

	totalScore := float32(0)
	maxConfidence := float32(0)
	matchCount := 0

	// Check each attack pattern
	for _, pattern := range su.attackPatterns {
		if pattern.Pattern.MatchString(normalizedText) {
			result.MatchedPatterns = append(result.MatchedPatterns, pattern.Pattern.String())
			result.Categories[pattern.Category] += pattern.Weight
			totalScore += pattern.Weight
			matchCount++

			if pattern.Confidence > maxConfidence {
				maxConfidence = pattern.Confidence
				result.PrimaryAttackType = pattern.Category
			}
		}
	}

	// Calculate final confidence
	if matchCount > 0 {
		// Use maximum confidence with boost for multiple matches
		result.Confidence = maxConfidence
		if matchCount > 1 {
			result.Confidence = float32(math.Min(float64(result.Confidence*1.2), 1.0))
		}
		result.IsAttack = result.Confidence > 0.5
	}

	return result
}

// GenerateTextFeatures extracts numerical features from text
func (su *SharedUtilities) GenerateTextFeatures(text string) TextFeatures {
	normalizedText := strings.ToLower(strings.TrimSpace(text))
	words := strings.Fields(normalizedText)

	features := TextFeatures{
		Length:              len(text),
		WordCount:           len(words),
		AvgWordLength:       su.calculateAvgWordLength(words),
		SpecialCharRatio:    su.calculateSpecialCharRatio(text),
		CapitalizationRatio: su.calculateCapitalizationRatio(text),
		QuestionRatio:       su.calculateQuestionRatio(text),
		ExclamationRatio:    su.calculateExclamationRatio(text),
		KeywordScore:        su.calculateKeywordScore(words),
		SentenceCount:       su.calculateSentenceCount(text),
		Entropy:             su.calculateEntropy(text),
		RepetitionScore:     su.calculateRepetitionScore(words),
	}

	return features
}

// CreateDeterministicHash creates a deterministic hash for consistent embeddings
func (su *SharedUtilities) CreateDeterministicHash(text string) [32]byte {
	return sha256.Sum256([]byte(strings.ToLower(strings.TrimSpace(text))))
}

// NormalizeEmbedding normalizes a vector to unit length
func (su *SharedUtilities) NormalizeEmbedding(embedding []float32) []float32 {
	var norm float32
	for _, val := range embedding {
		norm += val * val
	}
	norm = float32(math.Sqrt(float64(norm)))

	if norm == 0 {
		return embedding
	}

	normalized := make([]float32, len(embedding))
	for i, val := range embedding {
		normalized[i] = val / norm
	}
	return normalized
}

// ComputeCosineSimilarity calculates cosine similarity between two vectors
func (su *SharedUtilities) ComputeCosineSimilarity(vec1, vec2 []float32) float32 {
	if len(vec1) != len(vec2) || len(vec1) == 0 {
		return 0.0
	}

	var dotProduct, norm1, norm2 float64
	for i := range vec1 {
		dotProduct += float64(vec1[i] * vec2[i])
		norm1 += float64(vec1[i] * vec1[i])
		norm2 += float64(vec2[i] * vec2[i])
	}

	if norm1 == 0 || norm2 == 0 {
		return 0.0
	}

	return float32(dotProduct / (math.Sqrt(norm1) * math.Sqrt(norm2)))
}

// Helper methods for feature calculation

func (su *SharedUtilities) calculateAvgWordLength(words []string) float32 {
	if len(words) == 0 {
		return 0
	}

	totalLength := 0
	for _, word := range words {
		totalLength += len(word)
	}

	return float32(totalLength) / float32(len(words))
}

func (su *SharedUtilities) calculateSpecialCharRatio(text string) float32 {
	if len(text) == 0 {
		return 0
	}

	specialCount := 0
	for _, char := range text {
		if !((char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') ||
			(char >= '0' && char <= '9') || char == ' ') {
			specialCount++
		}
	}

	return float32(specialCount) / float32(len(text))
}

func (su *SharedUtilities) calculateCapitalizationRatio(text string) float32 {
	if len(text) == 0 {
		return 0
	}

	upperCount := 0
	for _, char := range text {
		if char >= 'A' && char <= 'Z' {
			upperCount++
		}
	}

	return float32(upperCount) / float32(len(text))
}

func (su *SharedUtilities) calculateQuestionRatio(text string) float32 {
	words := strings.Fields(text)
	if len(words) == 0 {
		return 0
	}

	questionCount := strings.Count(text, "?")
	return float32(questionCount) / float32(len(words))
}

func (su *SharedUtilities) calculateExclamationRatio(text string) float32 {
	words := strings.Fields(text)
	if len(words) == 0 {
		return 0
	}

	exclamationCount := strings.Count(text, "!")
	return float32(exclamationCount) / float32(len(words))
}

func (su *SharedUtilities) calculateKeywordScore(words []string) float32 {
	score := float32(0)
	for _, word := range words {
		if weight, exists := su.keywordWeights[word]; exists {
			score += weight
		}
	}
	return score
}

func (su *SharedUtilities) calculateSentenceCount(text string) int {
	sentences := strings.FieldsFunc(text, func(c rune) bool {
		return c == '.' || c == '!' || c == '?'
	})
	return len(sentences)
}

func (su *SharedUtilities) calculateEntropy(text string) float32 {
	if len(text) == 0 {
		return 0
	}

	freq := make(map[rune]int)
	for _, char := range text {
		freq[char]++
	}

	entropy := 0.0
	length := float64(len(text))

	for _, count := range freq {
		p := float64(count) / length
		if p > 0 {
			entropy -= p * math.Log2(p)
		}
	}

	return float32(entropy / 8.0) // Normalize
}

func (su *SharedUtilities) calculateRepetitionScore(words []string) float32 {
	if len(words) <= 1 {
		return 0
	}

	freq := make(map[string]int)
	for _, word := range words {
		freq[word]++
	}

	repetitions := 0
	for _, count := range freq {
		if count > 1 {
			repetitions += count - 1
		}
	}

	return float32(repetitions) / float32(len(words))
}

// GetSemanticClusterScore calculates the score for a semantic cluster
func (su *SharedUtilities) GetSemanticClusterScore(text string, clusterName string) float32 {
	su.mu.RLock()
	defer su.mu.RUnlock()

	cluster, exists := su.semanticClusters[clusterName]
	if !exists {
		return 0
	}

	normalizedText := strings.ToLower(text)
	score := float32(0)
	matches := 0

	for _, keyword := range cluster {
		if strings.Contains(normalizedText, keyword) {
			score += 1.0
			matches++
		}
	}

	if len(cluster) == 0 {
		return 0
	}

	return score / float32(len(cluster))
}

// AttackAnalysisResult contains the results of attack pattern analysis
type AttackAnalysisResult struct {
	Text              string
	IsAttack          bool
	Confidence        float32
	PrimaryAttackType string
	MatchedPatterns   []string
	Categories        map[string]float32
}

// TextFeatures contains numerical features extracted from text
type TextFeatures struct {
	Length              int
	WordCount           int
	AvgWordLength       float32
	SpecialCharRatio    float32
	CapitalizationRatio float32
	QuestionRatio       float32
	ExclamationRatio    float32
	KeywordScore        float32
	SentenceCount       int
	Entropy             float32
	RepetitionScore     float32
}

// UpdateStats provides thread-safe statistics updates
func UpdateStats(stats *ModelStats, inferences int64, tokens int, duration time.Duration) {
	stats.TotalInferences += inferences
	stats.TotalTokens += int64(tokens)
	stats.LastInferenceTime = time.Now()

	// Update average inference time
	if stats.TotalInferences > 0 {
		totalDuration := time.Duration(stats.TotalInferences) * stats.AvgInferenceTime
		stats.AvgInferenceTime = (totalDuration + duration) / time.Duration(stats.TotalInferences)
	} else {
		stats.AvgInferenceTime = duration
	}

	// Update average tokens per text
	if stats.TotalInferences > 0 {
		stats.AvgTokensPerText = float64(stats.TotalTokens) / float64(stats.TotalInferences)
	}
}

// Performance optimization methods

// GetEmbedding gets a reusable embedding vector from the pool
func (su *SharedUtilities) GetEmbedding() []float32 {
	return su.embeddingPool.Get().([]float32)
}

// PutEmbedding returns an embedding vector to the pool
func (su *SharedUtilities) PutEmbedding(embedding []float32) {
	// Clear the slice but keep capacity
	for i := range embedding {
		embedding[i] = 0
	}
	su.embeddingPool.Put(&embedding)
}

// GetAnalysisResult gets a reusable analysis result from the pool
func (su *SharedUtilities) GetAnalysisResult() *AttackAnalysisResult {
	result := su.analysisPool.Get().(*AttackAnalysisResult)
	// Reset fields
	result.IsAttack = false
	result.Confidence = 0
	result.PrimaryAttackType = ""
	result.MatchedPatterns = result.MatchedPatterns[:0]
	for k := range result.Categories {
		delete(result.Categories, k)
	}
	return result
}

// PutAnalysisResult returns an analysis result to the pool
func (su *SharedUtilities) PutAnalysisResult(result *AttackAnalysisResult) {
	su.analysisPool.Put(result)
}

// GetTextFeatures gets a reusable text features struct from the pool
func (su *SharedUtilities) GetTextFeatures() *TextFeatures {
	features := su.featuresPool.Get().(*TextFeatures)
	// Reset all fields to zero values
	*features = TextFeatures{}
	return features
}

// PutTextFeatures returns a text features struct to the pool
func (su *SharedUtilities) PutTextFeatures(features *TextFeatures) {
	su.featuresPool.Put(features)
}
