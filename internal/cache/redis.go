package cache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"
)

// VectorCache handles Redis-based caching for vector similarity searches
type VectorCache struct {
	client *redis.Client
	config *Config
	logger *zap.Logger
	stats  *cacheStats
}

// cacheStats tracks cache performance metrics
type cacheStats struct {
	hits   int64
	misses int64
}

// NewVectorCache creates a new Redis-based vector cache
func NewVectorCache(config *Config, logger *zap.Logger) (*VectorCache, error) {
	// Parse Redis URL
	opts, err := redis.ParseURL(config.RedisURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Redis URL: %w", err)
	}

	// Configure connection pool
	opts.PoolSize = config.MaxConnections
	opts.MinIdleConns = config.MinIdleConns
	// Note: ConnMaxLifetime not available in this Redis client version

	client := redis.NewClient(opts)

	cache := &VectorCache{
		client: client,
		config: config,
		logger: logger,
		stats:  &cacheStats{},
	}

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := cache.ping(ctx); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	logger.Info("Vector cache initialized successfully",
		zap.String("redis_url", maskRedisURL(config.RedisURL)),
		zap.Int("max_connections", config.MaxConnections),
		zap.Duration("default_ttl", config.DefaultTTL))

	return cache, nil
}

// ping tests the Redis connection
func (vc *VectorCache) ping(ctx context.Context) error {
	_, err := vc.client.Ping(ctx).Result()
	return err
}

// SearchSimilar searches for similar vectors in the cache
func (vc *VectorCache) SearchSimilar(ctx context.Context, embedding []float32, options *SearchOptions) (*SearchResult, error) {
	if options == nil {
		options = &SearchOptions{
			MinSimilarity: 0.8,
			MaxResults:    1,
		}
	}

	start := time.Now()

	// Generate cache key from embedding hash
	cacheKey := vc.generateEmbeddingKey(embedding)

	// Try to get from cache
	cachedData, err := vc.client.Get(ctx, cacheKey).Result()
	if err == redis.Nil {
		// Cache miss
		vc.stats.misses++
		vc.logger.Debug("Cache miss", zap.String("key", cacheKey))
		return &SearchResult{CacheHit: false}, nil
	} else if err != nil {
		vc.logger.Error("Cache lookup failed", zap.Error(err))
		return &SearchResult{CacheHit: false}, nil
	}

	// Parse cached vector
	var cachedVector CachedVector
	if err := json.Unmarshal([]byte(cachedData), &cachedVector); err != nil {
		vc.logger.Error("Failed to unmarshal cached vector", zap.Error(err))
		// Delete corrupted cache entry
		vc.client.Del(ctx, cacheKey)
		return &SearchResult{CacheHit: false}, nil
	}

	// Check if cached result meets criteria
	if cachedVector.Similarity < options.MinSimilarity {
		vc.stats.misses++
		return &SearchResult{CacheHit: false}, nil
	}

	// Apply filters
	if options.LabelFilter != nil && cachedVector.Label != *options.LabelFilter {
		vc.stats.misses++
		return &SearchResult{CacheHit: false}, nil
	}

	if options.LabelTextFilter != "" && cachedVector.LabelText != options.LabelTextFilter {
		vc.stats.misses++
		return &SearchResult{CacheHit: false}, nil
	}

	// Cache hit!
	vc.stats.hits++
	duration := time.Since(start)

	vc.logger.Debug("Cache hit",
		zap.String("key", cacheKey),
		zap.Float32("similarity", cachedVector.Similarity),
		zap.Duration("duration", duration))

	return &SearchResult{
		Vector:   &cachedVector,
		CacheHit: true,
	}, nil
}

// Store caches a vector with its similarity score
func (vc *VectorCache) Store(ctx context.Context, embedding []float32, vector *CachedVector) error {
	cacheKey := vc.generateEmbeddingKey(embedding)

	// Set cache timestamp and TTL
	vector.CachedAt = time.Now()
	vector.TTL = int64(vc.config.DefaultTTL.Seconds())

	// Serialize vector
	data, err := json.Marshal(vector)
	if err != nil {
		return fmt.Errorf("failed to marshal vector for caching: %w", err)
	}

	// Store in Redis with TTL
	err = vc.client.Set(ctx, cacheKey, data, vc.config.DefaultTTL).Err()
	if err != nil {
		vc.logger.Error("Failed to cache vector", zap.Error(err))
		return fmt.Errorf("failed to cache vector: %w", err)
	}

	vc.logger.Debug("Vector cached successfully",
		zap.String("key", cacheKey),
		zap.String("label_text", vector.LabelText),
		zap.Float32("similarity", vector.Similarity))

	return nil
}

// StoreBatch caches multiple vectors efficiently using Redis pipeline
func (vc *VectorCache) StoreBatch(ctx context.Context, embeddings [][]float32, vectors []*CachedVector) error {
	if len(embeddings) != len(vectors) {
		return fmt.Errorf("embeddings and vectors length mismatch")
	}

	if len(vectors) == 0 {
		return nil
	}

	pipe := vc.client.Pipeline()

	for i, vector := range vectors {
		cacheKey := vc.generateEmbeddingKey(embeddings[i])

		// Set cache timestamp and TTL
		vector.CachedAt = time.Now()
		vector.TTL = int64(vc.config.DefaultTTL.Seconds())

		// Serialize vector
		data, err := json.Marshal(vector)
		if err != nil {
			vc.logger.Error("Failed to marshal vector for batch caching", zap.Error(err))
			continue
		}

		pipe.Set(ctx, cacheKey, data, vc.config.DefaultTTL)
	}

	// Execute pipeline
	_, err := pipe.Exec(ctx)
	if err != nil {
		vc.logger.Error("Batch cache operation failed", zap.Error(err))
		return fmt.Errorf("batch cache operation failed: %w", err)
	}

	vc.logger.Debug("Batch cache operation completed",
		zap.Int("cached_vectors", len(vectors)))

	return nil
}

// GetStats returns cache performance statistics
func (vc *VectorCache) GetStats(ctx context.Context) (*CacheStats, error) {
	// Get Redis info
	info, err := vc.client.Info(ctx, "memory", "stats").Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get Redis info: %w", err)
	}

	stats := &CacheStats{
		Hits:   vc.stats.hits,
		Misses: vc.stats.misses,
	}

	// Calculate hit rate
	total := stats.Hits + stats.Misses
	if total > 0 {
		stats.HitRate = float64(stats.Hits) / float64(total) * 100
	}

	// Parse memory usage from Redis info
	lines := strings.Split(info, "\r\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "used_memory:") {
			if memStr := strings.TrimPrefix(line, "used_memory:"); memStr != "" {
				if mem, err := strconv.ParseInt(memStr, 10, 64); err == nil {
					stats.MemoryUsage = mem
				}
			}
		}
	}

	// Get total keys count
	keys, err := vc.client.DBSize(ctx).Result()
	if err == nil {
		stats.TotalKeys = keys
	}

	return stats, nil
}

// Clear removes all cached vectors
func (vc *VectorCache) Clear(ctx context.Context) error {
	pattern := vc.config.KeyPrefix + "*"

	// Use SCAN to find all keys with our prefix
	iter := vc.client.Scan(ctx, 0, pattern, 0).Iterator()
	var keys []string

	for iter.Next(ctx) {
		keys = append(keys, iter.Val())
	}

	if err := iter.Err(); err != nil {
		return fmt.Errorf("failed to scan cache keys: %w", err)
	}

	if len(keys) == 0 {
		return nil
	}

	// Delete keys in batches
	batchSize := 100
	for i := 0; i < len(keys); i += batchSize {
		end := i + batchSize
		if end > len(keys) {
			end = len(keys)
		}

		if err := vc.client.Del(ctx, keys[i:end]...).Err(); err != nil {
			vc.logger.Error("Failed to delete cache keys", zap.Error(err))
			return fmt.Errorf("failed to delete cache keys: %w", err)
		}
	}

	vc.logger.Info("Cache cleared", zap.Int("deleted_keys", len(keys)))
	return nil
}

// Close closes the Redis connection
func (vc *VectorCache) Close() error {
	if vc.client != nil {
		return vc.client.Close()
	}
	return nil
}

// generateEmbeddingKey creates a cache key from an embedding vector
func (vc *VectorCache) generateEmbeddingKey(embedding []float32) string {
	// Create a hash of the embedding for consistent cache keys
	hasher := sha256.New()

	// Convert embedding to bytes for hashing
	for _, val := range embedding {
		// Quantize to reduce precision and improve cache hit rate
		quantized := math.Round(float64(val)*1000) / 1000
		hasher.Write([]byte(fmt.Sprintf("%.3f,", quantized)))
	}

	hash := hex.EncodeToString(hasher.Sum(nil))
	return fmt.Sprintf("%s:emb:%s", vc.config.KeyPrefix, hash[:16]) // Use first 16 chars
}

// maskRedisURL masks sensitive information in Redis URL for logging
func maskRedisURL(url string) string {
	if strings.Contains(url, "@") {
		parts := strings.Split(url, "@")
		if len(parts) >= 2 {
			userPart := parts[0]
			if strings.Contains(userPart, ":") {
				userParts := strings.Split(userPart, ":")
				if len(userParts) >= 3 {
					userParts[len(userParts)-1] = "***"
					parts[0] = strings.Join(userParts, ":")
				}
			}
			return strings.Join(parts, "@")
		}
	}
	return url
}
