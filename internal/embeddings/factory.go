package embeddings

import (
	"context"
	"fmt"

	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"

	"github.com/raaihank/llm-sentinel/internal/vector"
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
	if err := ValidateServiceConfig(config); err != nil {
		return nil, err
	}
	switch config.Type {
	case HashEmbedding:
		service, err := NewHashEmbeddingService(&config.ModelConfig, f.logger)
		if err != nil {
			return nil, err
		}
		f.logger.Info("Created hash embedding service")
		return service, nil
	case PatternEmbedding:
		service, err := NewPatternEmbeddingService(&config.ModelConfig, f.logger)
		if err != nil {
			return nil, err
		}
		f.logger.Info("Created pattern embedding service")
		return service, nil
	case MLEmbedding:
		var redisClient *redis.Client
		if config.RedisEnabled {
			redisClient = redis.NewClient(&redis.Options{Addr: config.RedisURL})
			if err := redisClient.Ping(context.Background()).Err(); err != nil {
				f.logger.Warn("Redis connection failed, disabling cache", zap.Error(err))
				redisClient = nil
			}
		}
		var vectorStore *vector.Store = nil
		service, err := NewMLEmbeddingService(&config.ModelConfig, f.logger, redisClient, vectorStore)
		if err != nil {
			return nil, err
		}
		f.logger.Info("Created ML embedding service")
		return service, nil
	default:
		return nil, fmt.Errorf("unknown embedding service type: %s", config.Type)
	}
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
	return []ServiceType{MLEmbedding}
}

// GetServiceCapabilities returns the capabilities of a service type
func GetServiceCapabilities(serviceType ServiceType) map[string]interface{} {
	switch serviceType {
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
	ServiceType    ServiceType `json:"service_type"`
	AvgLatencyMs   float64     `json:"avg_latency_ms"`
	MemoryUsageMB  int         `json:"memory_usage_mb"`
	AccuracyScore  float64     `json:"accuracy_score"`
	ThroughputRPS  int         `json:"throughput_rps"`
	CacheHitRatio  float64     `json:"cache_hit_ratio"`
	RecommendedUse string      `json:"recommended_use"`
}

// GetPerformanceMetrics returns estimated performance metrics for each service type
func GetPerformanceMetrics() map[ServiceType]ServicePerformanceMetrics {
	return map[ServiceType]ServicePerformanceMetrics{
		MLEmbedding: {
			ServiceType:    MLEmbedding,
			AvgLatencyMs:   35.0,
			MemoryUsageMB:  80,
			AccuracyScore:  0.85,
			ThroughputRPS:  50,
			CacheHitRatio:  0.70,
			RecommendedUse: "Maximum accuracy, when caching is beneficial",
		},
	}
}
