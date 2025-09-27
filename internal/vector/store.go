package vector

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"go.uber.org/zap"
)

// Store handles vector storage operations with PostgreSQL + pgvector
type Store struct {
	db     *sqlx.DB
	logger *zap.Logger
}

// Config contains database configuration
type Config struct {
	DatabaseURL     string        `yaml:"database_url" mapstructure:"database_url"`
	MaxOpenConns    int           `yaml:"max_open_conns" mapstructure:"max_open_conns"`
	MaxIdleConns    int           `yaml:"max_idle_conns" mapstructure:"max_idle_conns"`
	ConnMaxLifetime time.Duration `yaml:"conn_max_lifetime" mapstructure:"conn_max_lifetime"`
	ConnMaxIdleTime time.Duration `yaml:"conn_max_idle_time" mapstructure:"conn_max_idle_time"`
}

// NewStore creates a new vector store instance
func NewStore(config *Config, logger *zap.Logger) (*Store, error) {
	db, err := sqlx.Connect("postgres", config.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(config.MaxOpenConns)
	db.SetMaxIdleConns(config.MaxIdleConns)
	db.SetConnMaxLifetime(config.ConnMaxLifetime)
	db.SetConnMaxIdleTime(config.ConnMaxIdleTime)

	store := &Store{
		db:     db,
		logger: logger,
	}

	// Test connection and ensure pgvector extension
	if err := store.initialize(); err != nil {
		return nil, fmt.Errorf("failed to initialize store: %w", err)
	}

	logger.Info("Vector store initialized successfully",
		zap.String("database_url", maskDatabaseURL(config.DatabaseURL)),
		zap.Int("max_open_conns", config.MaxOpenConns),
		zap.Int("max_idle_conns", config.MaxIdleConns))

	return store, nil
}

// initialize checks database connection and ensures pgvector extension
func (s *Store) initialize() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Test connection
	if err := s.db.PingContext(ctx); err != nil {
		return fmt.Errorf("database ping failed: %w", err)
	}

	// Check if pgvector extension is available
	var extensionExists bool
	query := "SELECT EXISTS(SELECT 1 FROM pg_extension WHERE extname = 'vector')"
	if err := s.db.GetContext(ctx, &extensionExists, query); err != nil {
		return fmt.Errorf("failed to check pgvector extension: %w", err)
	}

	if !extensionExists {
		return fmt.Errorf("pgvector extension is not installed")
	}

	s.logger.Info("Database initialized with pgvector extension")
	return nil
}

// Insert adds a new security vector to the database
func (s *Store) Insert(ctx context.Context, vector *SecurityVector) error {
	query := `
		INSERT INTO security_vectors (text, text_hash, label_text, label, embedding)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at, updated_at`

	embeddingStr := formatEmbedding(vector.Embedding)

	err := s.db.QueryRowContext(ctx, query,
		vector.Text,
		vector.TextHash,
		vector.LabelText,
		vector.Label,
		embeddingStr,
	).Scan(&vector.ID, &vector.CreatedAt, &vector.UpdatedAt)

	if err != nil {
		s.logger.Error("Failed to insert vector",
			zap.Error(err),
			zap.String("label_text", vector.LabelText),
			zap.Int("label", vector.Label))
		return fmt.Errorf("failed to insert vector: %w", err)
	}

	s.logger.Debug("Vector inserted successfully",
		zap.Int64("id", vector.ID),
		zap.String("label_text", vector.LabelText))

	return nil
}

// BatchInsert adds multiple security vectors efficiently
func (s *Store) BatchInsert(ctx context.Context, vectors []*SecurityVector) (*BatchInsertResult, error) {
	if len(vectors) == 0 {
		return &BatchInsertResult{}, nil
	}

	start := time.Now()
	result := &BatchInsertResult{}

	// Prepare batch insert
	valueStrings := make([]string, 0, len(vectors))
	valueArgs := make([]interface{}, 0, len(vectors)*5)

	for i, vector := range vectors {
		valueStrings = append(valueStrings, fmt.Sprintf("($%d, $%d, $%d, $%d, $%d)", i*5+1, i*5+2, i*5+3, i*5+4, i*5+5))
		valueArgs = append(valueArgs,
			vector.Text,
			vector.TextHash,
			vector.LabelText,
			vector.Label,
			formatEmbedding(vector.Embedding),
		)
	}

	query := fmt.Sprintf(`
		INSERT INTO security_vectors (text, text_hash, label_text, label, embedding)
		VALUES %s
		ON CONFLICT (text_hash) DO NOTHING`,
		strings.Join(valueStrings, ","))

	res, err := s.db.ExecContext(ctx, query, valueArgs...)
	if err != nil {
		result.Failed = int64(len(vectors))
		result.Errors = []error{err}
		s.logger.Error("Batch insert failed", zap.Error(err))
		return result, fmt.Errorf("batch insert failed: %w", err)
	}

	inserted, err := res.RowsAffected()
	if err != nil {
		s.logger.Warn("Could not get rows affected", zap.Error(err))
		inserted = int64(len(vectors)) // Assume all inserted
	}

	result.Inserted = inserted
	result.Failed = int64(len(vectors)) - inserted
	result.Duration = time.Since(start)
	duplicates := int64(len(vectors)) - inserted - result.Failed

	s.logger.Info("Batch insert completed",
		zap.Int64("inserted", result.Inserted),
		zap.Int64("duplicates_skipped", duplicates),
		zap.Int64("failed", result.Failed),
		zap.Duration("duration", result.Duration))

	return result, nil
}

// FindSimilar finds vectors similar to the given embedding
func (s *Store) FindSimilar(ctx context.Context, embedding []float32, options *SearchOptions) ([]*SimilarityResult, error) {
	if options == nil {
		options = &SearchOptions{
			Limit:         5,
			MinSimilarity: 0.7,
		}
	}

	embeddingStr := formatEmbedding(embedding)

	// Build query with optional filters
	whereClause := "WHERE (1 - (embedding <=> $1)) >= $2"
	args := []interface{}{embeddingStr, options.MinSimilarity}
	argIndex := 3

	if options.LabelFilter != nil {
		whereClause += fmt.Sprintf(" AND label = $%d", argIndex)
		args = append(args, *options.LabelFilter)
		argIndex++
	}

	if options.LabelTextFilter != "" {
		whereClause += fmt.Sprintf(" AND label_text = $%d", argIndex)
		args = append(args, options.LabelTextFilter)
		argIndex++
	}

	query := fmt.Sprintf(`
		SELECT 
			id, text, label_text, label, embedding,
			created_at, updated_at,
			(1 - (embedding <=> $1)) as similarity,
			(embedding <=> $1) as distance
		FROM security_vectors
		%s
		ORDER BY embedding <=> $1
		LIMIT $%d`, whereClause, argIndex)

	args = append(args, options.Limit)

	start := time.Now()
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		s.logger.Error("Similarity search failed", zap.Error(err))
		return nil, fmt.Errorf("similarity search failed: %w", err)
	}
	defer rows.Close()

	var results []*SimilarityResult
	for rows.Next() {
		var result SimilarityResult
		var vector SecurityVector
		var embeddingStr string

		err := rows.Scan(
			&vector.ID,
			&vector.Text,
			&vector.LabelText,
			&vector.Label,
			&embeddingStr,
			&vector.CreatedAt,
			&vector.UpdatedAt,
			&result.Similarity,
			&result.Distance,
		)
		if err != nil {
			s.logger.Error("Failed to scan similarity result", zap.Error(err))
			continue
		}

		// Parse embedding back to float32 slice
		vector.Embedding, err = parseEmbedding(embeddingStr)
		if err != nil {
			s.logger.Error("Failed to parse embedding", zap.Error(err))
			continue
		}

		result.Vector = &vector
		results = append(results, &result)
	}

	searchDuration := time.Since(start)
	s.logger.Debug("Similarity search completed",
		zap.Int("results", len(results)),
		zap.Duration("duration", searchDuration),
		zap.Float32("min_similarity", options.MinSimilarity))

	return results, nil
}

// GetStats returns database statistics
func (s *Store) GetStats(ctx context.Context) (*VectorStats, error) {
	stats := &VectorStats{}

	// Get vector counts
	query := `
		SELECT 
			COUNT(*) as total,
			COUNT(CASE WHEN label = 1 THEN 1 END) as malicious,
			COUNT(CASE WHEN label = 0 THEN 1 END) as safe
		FROM security_vectors`

	err := s.db.QueryRowContext(ctx, query).Scan(
		&stats.TotalVectors,
		&stats.MaliciousCount,
		&stats.SafeCount,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get vector stats: %w", err)
	}

	// Get cache stats if available
	cacheQuery := `
		SELECT 
			COALESCE(avg_search_time_ms, 0) as avg_search_time,
			CASE 
				WHEN total_searches > 0 THEN (cache_hits::float / total_searches::float) * 100
				ELSE 0 
			END as cache_hit_rate
		FROM vector_cache_stats 
		LIMIT 1`

	err = s.db.QueryRowContext(ctx, cacheQuery).Scan(
		&stats.AvgSearchTimeMs,
		&stats.CacheHitRate,
	)
	if err != nil && err != sql.ErrNoRows {
		s.logger.Warn("Failed to get cache stats", zap.Error(err))
	}

	return stats, nil
}

// CreateIndex creates the vector similarity index for better performance
func (s *Store) CreateIndex(ctx context.Context) error {
	// Only create index if we have enough vectors
	var count int64
	if err := s.db.GetContext(ctx, &count, "SELECT COUNT(*) FROM security_vectors"); err != nil {
		return fmt.Errorf("failed to count vectors: %w", err)
	}

	if count < 1000 {
		s.logger.Info("Skipping index creation, not enough vectors", zap.Int64("count", count))
		return nil
	}

	s.logger.Info("Creating vector similarity index...", zap.Int64("vector_count", count))

	query := `
		CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_security_vectors_embedding 
		ON security_vectors USING ivfflat (embedding vector_cosine_ops) 
		WITH (lists = 100)`

	_, err := s.db.ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to create vector index: %w", err)
	}

	s.logger.Info("Vector similarity index created successfully")
	return nil
}

// Close closes the database connection
func (s *Store) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

// Helper functions

// formatEmbedding converts float32 slice to PostgreSQL vector format
func formatEmbedding(embedding []float32) string {
	if len(embedding) == 0 {
		return "[]"
	}

	parts := make([]string, len(embedding))
	for i, v := range embedding {
		parts[i] = fmt.Sprintf("%g", v)
	}
	return "[" + strings.Join(parts, ",") + "]"
}

// parseEmbedding converts PostgreSQL vector format back to float32 slice
func parseEmbedding(embeddingStr string) ([]float32, error) {
	// Remove brackets and split by comma
	embeddingStr = strings.Trim(embeddingStr, "[]")
	if embeddingStr == "" {
		return []float32{}, nil
	}

	parts := strings.Split(embeddingStr, ",")
	embedding := make([]float32, len(parts))

	for i, part := range parts {
		var val float32
		if _, err := fmt.Sscanf(strings.TrimSpace(part), "%g", &val); err != nil {
			return nil, fmt.Errorf("failed to parse embedding value: %w", err)
		}
		embedding[i] = val
	}

	return embedding, nil
}

// maskDatabaseURL masks sensitive information in database URL for logging
func maskDatabaseURL(url string) string {
	// Simple masking - replace password with ***
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
