package cache

import (
	"context"
	"encoding/json"
	"time"
)

// CachedVector represents a cached vector with similarity score
type CachedVector struct {
	ID         int64     `json:"id"`
	Text       string    `json:"text"`
	LabelText  string    `json:"label_text"`
	Label      int       `json:"label"`
	Embedding  []float32 `json:"embedding"`
	Similarity float32   `json:"similarity"`
	Distance   float32   `json:"distance"`
	CachedAt   time.Time `json:"cached_at"`
	TTL        int64     `json:"ttl"`
}

// SearchResult represents a cache search result
type SearchResult struct {
	Vector   *CachedVector `json:"vector"`
	CacheHit bool          `json:"cache_hit"`
}

// CacheStats represents cache performance statistics
type CacheStats struct {
	Hits            int64   `json:"hits"`
	Misses          int64   `json:"misses"`
	HitRate         float64 `json:"hit_rate"`
	TotalKeys       int64   `json:"total_keys"`
	MemoryUsage     int64   `json:"memory_usage_bytes"`
	AvgResponseTime float64 `json:"avg_response_time_ms"`
}

// Config contains cache configuration
type Config struct {
	RedisURL        string        `yaml:"redis_url" mapstructure:"redis_url"`
	MaxConnections  int           `yaml:"max_connections" mapstructure:"max_connections"`
	MinIdleConns    int           `yaml:"min_idle_conns" mapstructure:"min_idle_conns"`
	ConnMaxLifetime time.Duration `yaml:"conn_max_lifetime" mapstructure:"conn_max_lifetime"`
	DefaultTTL      time.Duration `yaml:"default_ttl" mapstructure:"default_ttl"`
	MaxCacheSize    int           `yaml:"max_cache_size" mapstructure:"max_cache_size"`
	KeyPrefix       string        `yaml:"key_prefix" mapstructure:"key_prefix"`
}

// SearchOptions contains options for cache search
type SearchOptions struct {
	MinSimilarity   float32 `json:"min_similarity"`
	MaxResults      int     `json:"max_results"`
	LabelFilter     *int    `json:"label_filter,omitempty"`
	LabelTextFilter string  `json:"label_text_filter,omitempty"`
}

func (vc *VectorCache) Set(ctx context.Context, key string, embedding []float32) error {
	// Serialize embedding (e.g., to JSON or binary)
	data, err := json.Marshal(embedding)
	if err != nil {
		return err
	}
	return vc.client.Set(ctx, key, data, 0).Err()
}
