package security

import (
	"sync"
	"time"

	"github.com/raaihank/llm-sentinel/internal/config"
)

// RateLimiter implements token bucket rate limiting for DoS protection
type RateLimiter struct {
	config  *config.SecurityConfig
	buckets map[string]*TokenBucket
	mu      sync.RWMutex
}

// TokenBucket represents a token bucket for rate limiting
type TokenBucket struct {
	tokens     float64
	capacity   float64
	refillRate float64
	lastRefill time.Time
	mu         sync.Mutex
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(cfg *config.SecurityConfig) *RateLimiter {
	return &RateLimiter{
		config:  cfg,
		buckets: make(map[string]*TokenBucket),
	}
}

// Allow checks if a request from the given client IP is allowed
func (r *RateLimiter) Allow(clientIP string) bool {
	if !r.config.RateLimit.Enabled {
		return true
	}

	bucket := r.getBucket(clientIP)
	return bucket.consume(1)
}

// GetCurrentRate returns the current rate for a client IP
func (r *RateLimiter) GetCurrentRate(clientIP string) float64 {
	r.mu.RLock()
	bucket, exists := r.buckets[clientIP]
	r.mu.RUnlock()

	if !exists {
		return 0
	}

	bucket.mu.Lock()
	defer bucket.mu.Unlock()

	return bucket.capacity - bucket.tokens
}

// getBucket gets or creates a token bucket for a client IP
func (r *RateLimiter) getBucket(clientIP string) *TokenBucket {
	r.mu.RLock()
	bucket, exists := r.buckets[clientIP]
	r.mu.RUnlock()

	if exists {
		return bucket
	}

	// Create new bucket
	r.mu.Lock()
	defer r.mu.Unlock()

	// Double-check after acquiring write lock
	if bucket, exists := r.buckets[clientIP]; exists {
		return bucket
	}

	bucket = &TokenBucket{
		tokens:     float64(r.config.RateLimit.RequestsPerMin),
		capacity:   float64(r.config.RateLimit.RequestsPerMin),
		refillRate: float64(r.config.RateLimit.RequestsPerMin) / 60.0, // per second
		lastRefill: time.Now(),
	}

	r.buckets[clientIP] = bucket
	return bucket
}

// consume attempts to consume tokens from the bucket
func (b *TokenBucket) consume(tokens float64) bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(b.lastRefill).Seconds()

	// Refill tokens based on elapsed time
	tokensToAdd := elapsed * b.refillRate
	b.tokens = min(b.capacity, b.tokens+tokensToAdd)
	b.lastRefill = now

	// Check if we have enough tokens
	if b.tokens >= tokens {
		b.tokens -= tokens
		return true
	}

	return false
}

// min returns the minimum of two float64 values
func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

// CleanupOldBuckets removes old, unused buckets to prevent memory leaks
func (r *RateLimiter) CleanupOldBuckets() {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-time.Hour) // Remove buckets older than 1 hour

	for ip, bucket := range r.buckets {
		bucket.mu.Lock()
		if bucket.lastRefill.Before(cutoff) {
			delete(r.buckets, ip)
		}
		bucket.mu.Unlock()
	}
}

// StartCleanupRoutine starts a background routine to clean up old buckets
func (r *RateLimiter) StartCleanupRoutine() {
	go func() {
		ticker := time.NewTicker(30 * time.Minute)
		defer ticker.Stop()

		for range ticker.C {
			r.CleanupOldBuckets()
		}
	}()
}
