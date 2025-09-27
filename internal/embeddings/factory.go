package embeddings

import (
	"fmt"

	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"
)

// ServiceType represents the type of embedding service
type ServiceType string

const (
	// HashEmbedding uses deterministic hash-based embeddings with keyword boosting
	HashEmbedding ServiceType = "hash"

	// PatternEmbedding uses sophisticated pattern matching with contextual analysis
	PatternEmbedding ServiceType = "pattern"

	// MLEmbedding uses transformer models with Redis caching for semantic understanding
	MLEmbedding ServiceType = "ml"
)

// ServiceConfig contains configuration for embedding service selection
type ServiceConfig struct {
	Type         ServiceType `yaml:"type" mapstructure:"type"`                   // Service type to use
	ModelConfig  ModelConfig `yaml:"model" mapstructure:"model"`                 // Model configuration
	RedisEnabled bool        `yaml:"redis_enabled" mapstructure:"redis_enabled"` // Enable Redis caching
	RedisURL     string      `yaml:"redis_url" mapstructure:"redis_url"`         // Redis connection URL
}

// Factory creates embedding services based on configuration
type Factory struct {
	logger *zap.Logger
}

// NewFactory creates a new embedding service factory
func NewFactory(logger *zap.Logger) *Factory {
	return &Factory{
		logger: logger,
	}
}

// CreateService creates an embedding service based on the configuration
func (f *Factory) CreateService(config ServiceConfig) (EmbeddingService, error) {
	f.logger.Info("Creating embedding service",
		zap.String("type", string(config.Type)),
		zap.String("model", config.ModelConfig.ModelName),
		zap.Bool("redis_enabled", config.RedisEnabled))

	switch config.Type {
	case HashEmbedding:
		return f.createHashService(config)
	case PatternEmbedding:
		return f.createPatternService(config)
	case MLEmbedding:
		return f.createMLService(config)
	default:
		return nil, fmt.Errorf("unknown embedding service type: %s", config.Type)
	}
}

// createHashService creates a hash-based embedding service
func (f *Factory) createHashService(config ServiceConfig) (EmbeddingService, error) {
	f.logger.Info("Initializing hash-based embedding service")

	service, err := NewHashEmbeddingService(&config.ModelConfig, f.logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create hash embedding service: %w", err)
	}

	return service, nil
}

// createPatternService creates a pattern-based embedding service
func (f *Factory) createPatternService(config ServiceConfig) (EmbeddingService, error) {
	f.logger.Info("Initializing pattern-based embedding service")

	service, err := NewPatternEmbeddingService(config.ModelConfig, f.logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create pattern embedding service: %w", err)
	}

	return service, nil
}

// createMLService creates an ML-based embedding service with optional Redis caching
func (f *Factory) createMLService(config ServiceConfig) (EmbeddingService, error) {
	f.logger.Info("Initializing ML-based embedding service",
		zap.Bool("redis_enabled", config.RedisEnabled))

	var redisClient *redis.Client

	// Initialize Redis client if enabled
	if config.RedisEnabled && config.RedisURL != "" {
		redisClient = redis.NewClient(&redis.Options{
			Addr: config.RedisURL,
		})

		// Test Redis connection
		if err := redisClient.Ping(redisClient.Context()).Err(); err != nil {
			f.logger.Warn("Redis connection failed, proceeding without cache", zap.Error(err))
			redisClient = nil
		} else {
			f.logger.Info("Redis cache enabled for ML embeddings")
		}
	}

	service, err := NewMLEmbeddingService(config.ModelConfig, f.logger, redisClient)
	if err != nil {
		if redisClient != nil {
			redisClient.Close()
		}
		return nil, fmt.Errorf("failed to create ML embedding service: %w", err)
	}

	return service, nil
}

// GetRecommendedService returns the recommended service type based on use case
func GetRecommendedService() ServiceType {
	// ML embedding is the most advanced and recommended
	return MLEmbedding
}

// GetServiceDescription returns a description of each service type
func GetServiceDescription(serviceType ServiceType) string {
	switch serviceType {
	case HashEmbedding:
		return "Fast hash-based embeddings with basic keyword detection. Good for simple use cases."
	case PatternEmbedding:
		return "Advanced pattern matching with contextual analysis. Excellent balance of speed and accuracy."
	case MLEmbedding:
		return "Transformer-based semantic embeddings with Redis caching. Best accuracy for complex threats."
	default:
		return "Unknown service type"
	}
}

// ValidateServiceConfig validates the embedding service configuration
func ValidateServiceConfig(config ServiceConfig) error {
	// Validate service type
	switch config.Type {
	case HashEmbedding, PatternEmbedding, MLEmbedding:
		// Valid types
	default:
		return fmt.Errorf("invalid service type: %s (must be one of: hash, pattern, ml)", config.Type)
	}

	// Validate model configuration
	if config.ModelConfig.ModelName == "" {
		return fmt.Errorf("model name is required")
	}

	if config.ModelConfig.MaxLength <= 0 {
		return fmt.Errorf("max_length must be positive")
	}

	if config.ModelConfig.BatchSize <= 0 {
		return fmt.Errorf("batch_size must be positive")
	}

	// Validate Redis configuration if ML service with Redis is enabled
	if config.Type == MLEmbedding && config.RedisEnabled && config.RedisURL == "" {
		return fmt.Errorf("redis_url is required when redis_enabled is true for ML service")
	}

	return nil
}

// GetAllServiceTypes returns all available service types
func GetAllServiceTypes() []ServiceType {
	return []ServiceType{HashEmbedding, PatternEmbedding, MLEmbedding}
}

// GetServiceCapabilities returns the capabilities of a service type
func GetServiceCapabilities(serviceType ServiceType) map[string]interface{} {
	switch serviceType {
	case HashEmbedding:
		return map[string]interface{}{
			"deterministic":    true,
			"caching":          false,
			"pattern_matching": false,
			"ml_inference":     false,
			"redis_support":    false,
			"speed":            "very_fast",
			"accuracy":         "basic",
			"memory_usage":     "low",
		}
	case PatternEmbedding:
		return map[string]interface{}{
			"deterministic":    true,
			"caching":          false,
			"pattern_matching": true,
			"ml_inference":     false,
			"redis_support":    false,
			"speed":            "fast",
			"accuracy":         "good",
			"memory_usage":     "medium",
		}
	case MLEmbedding:
		return map[string]interface{}{
			"deterministic":    false,
			"caching":          true,
			"pattern_matching": true,
			"ml_inference":     true,
			"redis_support":    true,
			"speed":            "medium",
			"accuracy":         "high",
			"memory_usage":     "high",
		}
	default:
		return map[string]interface{}{}
	}
}

// CreateDefaultConfig creates a default configuration for a service type
func CreateDefaultConfig(serviceType ServiceType) ServiceConfig {
	baseConfig := ServiceConfig{
		Type: serviceType,
		ModelConfig: ModelConfig{
			ModelName:    "default",
			MaxLength:    512,
			BatchSize:    16,
			ModelTimeout: 30000000000, // 30 seconds in nanoseconds
			AutoDownload: true,
		},
		RedisEnabled: false,
	}

	switch serviceType {
	case HashEmbedding:
		baseConfig.ModelConfig.ModelName = "hash-deterministic"
	case PatternEmbedding:
		baseConfig.ModelConfig.ModelName = "pattern-advanced"
	case MLEmbedding:
		baseConfig.ModelConfig.ModelName = "sentence-transformers/all-MiniLM-L6-v2"
		baseConfig.ModelConfig.CacheDir = "./models"
		baseConfig.RedisEnabled = true
		baseConfig.RedisURL = "localhost:6379"
	}

	return baseConfig
}

// ServicePerformanceMetrics contains performance metrics for different service types
type ServicePerformanceMetrics struct {
	ServiceType      ServiceType `json:"service_type"`
	AvgLatencyMs     float64     `json:"avg_latency_ms"`
	MemoryUsageMB    int         `json:"memory_usage_mb"`
	AccuracyScore    float64     `json:"accuracy_score"`
	ThroughputRPS    int         `json:"throughput_rps"`
	CacheHitRatio    float64     `json:"cache_hit_ratio"`
	RecommendedUse   string      `json:"recommended_use"`
}

// GetPerformanceMetrics returns estimated performance metrics for each service type
func GetPerformanceMetrics() map[ServiceType]ServicePerformanceMetrics {
	return map[ServiceType]ServicePerformanceMetrics{
		HashEmbedding: {
			ServiceType:      HashEmbedding,
			AvgLatencyMs:     2.0,
			MemoryUsageMB:    5,
			AccuracyScore:    0.65,
			ThroughputRPS:    1000,
			CacheHitRatio:    0.0,
			RecommendedUse:   "High-speed processing, deterministic results",
		},
		PatternEmbedding: {
			ServiceType:      PatternEmbedding,
			AvgLatencyMs:     10.0,
			MemoryUsageMB:    15,
			AccuracyScore:    0.75,
			ThroughputRPS:    200,
			CacheHitRatio:    0.0,
			RecommendedUse:   "Production use, balanced performance",
		},
		MLEmbedding: {
			ServiceType:      MLEmbedding,
			AvgLatencyMs:     35.0,
			MemoryUsageMB:    80,
			AccuracyScore:    0.85,
			ThroughputRPS:    50,
			CacheHitRatio:    0.70,
			RecommendedUse:   "Maximum accuracy, when caching is beneficial",
		},
	}
}
