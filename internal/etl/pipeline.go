package etl

import (
	"context"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/segmentio/parquet-go"
	"go.uber.org/zap"

	"github.com/raaihank/llm-sentinel/internal/cache"
	"github.com/raaihank/llm-sentinel/internal/embeddings"
	"github.com/raaihank/llm-sentinel/internal/vector"
)

// Pipeline handles ETL operations for security datasets
type Pipeline struct {
	vectorStore      *vector.Store
	embeddingService embeddings.EmbeddingService
	vectorCache      *cache.VectorCache
	config           *Config
	logger           *zap.Logger
	stats            *ProcessingStats
	mu               sync.RWMutex
}

// NewPipeline creates a new ETL pipeline
func NewPipeline(
	vectorStore *vector.Store,
	embeddingService embeddings.EmbeddingService,
	vectorCache *cache.VectorCache,
	config *Config,
	logger *zap.Logger,
) *Pipeline {
	return &Pipeline{
		vectorStore:      vectorStore,
		embeddingService: embeddingService,
		vectorCache:      vectorCache,
		config:           config,
		logger:           logger,
		stats: &ProcessingStats{
			StartTime: time.Now(),
		},
	}
}

// ProcessFile processes a dataset file (CSV, Parquet, or JSON)
func (p *Pipeline) ProcessFile(ctx context.Context, filePath string) (*ProcessingResult, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	p.logger.Info("Starting ETL pipeline",
		zap.String("file", filePath),
		zap.Int("batch_size", p.config.BatchSize),
		zap.Int("workers", p.config.WorkerCount))

	start := time.Now()
	result := &ProcessingResult{}

	// Detect file format
	format := DetectFileFormat(filePath)
	p.logger.Info("Detected file format", zap.String("format", string(format)))

	// Reset stats
	p.resetStats()

	// Process based on file format
	switch format {
	case FormatCSV:
		err := p.processCSV(ctx, filePath, result)
		if err != nil {
			return result, fmt.Errorf("CSV processing failed: %w", err)
		}
	case FormatParquet:
		err := p.processParquet(ctx, filePath, result)
		if err != nil {
			return result, fmt.Errorf("Parquet processing failed: %w", err)
		}
	case FormatJSON:
		err := p.processJSON(ctx, filePath, result)
		if err != nil {
			return result, fmt.Errorf("JSON processing failed: %w", err)
		}
	default:
		return result, fmt.Errorf("unsupported file format: %s", format)
	}

	result.Duration = time.Since(start)

	// Create vector index if requested and we have enough data
	if p.config.CreateIndex && result.ProcessedOK > 1000 {
		p.logger.Info("Creating vector similarity index...")
		indexStart := time.Now()
		if err := p.vectorStore.CreateIndex(ctx); err != nil {
			p.logger.Warn("Failed to create vector index", zap.Error(err))
		} else {
			p.logger.Info("Vector index created", zap.Duration("duration", time.Since(indexStart)))
		}
	}

	p.logger.Info("ETL pipeline completed",
		zap.Int64("total_records", result.TotalRecords),
		zap.Int64("processed_ok", result.ProcessedOK),
		zap.Int64("processed_failed", result.ProcessedFailed),
		zap.Duration("total_duration", result.Duration),
		zap.Duration("embedding_time", result.EmbeddingTime),
		zap.Duration("database_time", result.DatabaseTime))

	return result, nil
}

// processCSV processes CSV files
func (p *Pipeline) processCSV(ctx context.Context, filePath string, result *ProcessingResult) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open CSV file: %w", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.FieldsPerRecord = 3 // text, label_text, label

	// Read header
	header, err := reader.Read()
	if err != nil {
		return fmt.Errorf("failed to read CSV header: %w", err)
	}

	p.logger.Info("CSV header detected", zap.Strings("columns", header))

	// Process records in batches
	return p.processBatches(ctx, func() ([]*DataRecord, error) {
		var batch []*DataRecord
		p.logger.Debug("Starting to read batch")

		for len(batch) < p.config.BatchSize {
			record, err := reader.Read()
			if err == io.EOF {
				break
			}
			if err != nil {
				p.logger.Warn("Failed to read CSV record", zap.Error(err))
				continue
			}

			if len(record) != 3 {
				p.logger.Warn("Invalid CSV record length", zap.Int("length", len(record)))
				continue
			}

			// Parse label as integer
			var label int
			if record[2] == "1" || strings.ToLower(record[2]) == "true" {
				label = 1
			} else {
				label = 0
			}

			dataRecord := &DataRecord{
				Text:      strings.TrimSpace(record[0]),
				LabelText: strings.TrimSpace(record[1]),
				Label:     label,
			}

			if p.validateRecord(dataRecord) {
				batch = append(batch, dataRecord)
			}
		}

		p.logger.Debug("Batch read completed", zap.Int("batch_size", len(batch)))
		return batch, nil
	}, result)
}

// processParquet processes Parquet files
func (p *Pipeline) processParquet(ctx context.Context, filePath string, result *ProcessingResult) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open Parquet file: %w", err)
	}
	defer file.Close()

	reader := parquet.NewReader(file)
	defer reader.Close()

	// Process records in batches
	return p.processBatches(ctx, func() ([]*DataRecord, error) {
		var batch []*DataRecord

		for len(batch) < p.config.BatchSize {
			var record DataRecord
			err := reader.Read(&record)
			if err == io.EOF {
				break
			}
			if err != nil {
				p.logger.Warn("Failed to read Parquet record", zap.Error(err))
				continue
			}

			if p.validateRecord(&record) {
				batch = append(batch, &record)
			}
		}

		return batch, nil
	}, result)
}

// processJSON processes JSON files (one JSON object per line)
func (p *Pipeline) processJSON(ctx context.Context, filePath string, result *ProcessingResult) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open JSON file: %w", err)
	}
	defer file.Close()

	decoder := json.NewDecoder(file)

	// Process records in batches
	return p.processBatches(ctx, func() ([]*DataRecord, error) {
		var batch []*DataRecord

		for len(batch) < p.config.BatchSize {
			var record DataRecord
			err := decoder.Decode(&record)
			if err == io.EOF {
				break
			}
			if err != nil {
				p.logger.Warn("Failed to read JSON record", zap.Error(err))
				continue
			}

			if p.validateRecord(&record) {
				batch = append(batch, &record)
			}
		}

		return batch, nil
	}, result)
}

// processBatches processes data in batches using the provided reader function
func (p *Pipeline) processBatches(ctx context.Context, readBatch func() ([]*DataRecord, error), result *ProcessingResult) error {
	p.logger.Info("Starting processBatches loop")
	for {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Read next batch
		p.logger.Info("Calling readBatch()")
		batch, err := readBatch()
		if err != nil {
			return fmt.Errorf("failed to read batch: %w", err)
		}
		p.logger.Info("readBatch() completed", zap.Int("batch_size", len(batch)))

		if len(batch) == 0 {
			break // End of file
		}

		// Process batch
		if err := p.processBatch(ctx, batch, result); err != nil {
			p.logger.Error("Batch processing failed", zap.Error(err))
			result.ProcessedFailed += int64(len(batch))
			result.Errors = append(result.Errors, err.Error())
			continue
		}

		result.TotalRecords += int64(len(batch))
		result.ProcessedOK += int64(len(batch))

		// Progress reporting
		if result.TotalRecords%int64(p.config.ProgressReport) == 0 {
			p.reportProgress(result)
		}
	}

	return nil
}

// processBatch processes a single batch of records
func (p *Pipeline) processBatch(ctx context.Context, batch []*DataRecord, result *ProcessingResult) error {
	if len(batch) == 0 {
		return nil
	}

	p.logger.Info("Processing batch", zap.Int("batch_size", len(batch)))

	// Extract texts for batch embedding generation
	texts := make([]string, len(batch))
	for i, record := range batch {
		texts[i] = record.Text
	}

	// Generate embeddings for the batch
	p.logger.Info("Starting embedding generation", zap.Int("text_count", len(texts)))
	embeddingStart := time.Now()
	embeddingResult, err := p.embeddingService.GenerateBatchEmbeddings(ctx, texts)
	if err != nil {
		return fmt.Errorf("batch embedding generation failed: %w", err)
	}
	result.EmbeddingTime += time.Since(embeddingStart)
	p.logger.Info("Embedding generation completed", zap.Duration("duration", time.Since(embeddingStart)), zap.Int("embeddings_count", len(embeddingResult.Embeddings)))

	if len(embeddingResult.Embeddings) != len(batch) {
		return fmt.Errorf("embedding count mismatch: got %d, expected %d",
			len(embeddingResult.Embeddings), len(batch))
	}

	// Create security vectors
	vectors := make([]*vector.SecurityVector, len(batch))
	for i, record := range batch {
		vectors[i] = &vector.SecurityVector{
			Text:          record.Text,
			EmbeddingType: embeddingResult.ServiceType,
			TextHash:      computeTextHash(record.Text),
			LabelText:     record.LabelText,
			Label:         record.Label,
			Embedding:     embeddingResult.Embeddings[i],
		}
	}

	// Store in database
	p.logger.Info("Starting database batch insert", zap.Int("vectors_count", len(vectors)))
	dbStart := time.Now()
	batchResult, err := p.vectorStore.BatchInsert(ctx, vectors)
	if err != nil {
		return fmt.Errorf("database batch insert failed: %w", err)
	}
	result.DatabaseTime += time.Since(dbStart)
	p.logger.Info("Database batch insert completed", zap.Duration("duration", time.Since(dbStart)), zap.Int64("inserted", batchResult.Inserted))

	// Update cache with high-confidence malicious vectors
	if p.config.UpdateCache && p.vectorCache != nil {
		cacheStart := time.Now()
		p.updateCache(ctx, vectors, embeddingResult.Embeddings)
		result.CacheTime += time.Since(cacheStart)
	}

	p.logger.Debug("Batch processed successfully",
		zap.Int("batch_size", len(batch)),
		zap.Int64("inserted", batchResult.Inserted),
		zap.Int64("failed", batchResult.Failed),
		zap.Duration("embedding_time", time.Since(embeddingStart)),
		zap.Duration("database_time", time.Since(dbStart)))

	return nil
}

// updateCache updates the Redis cache with high-priority vectors
func (p *Pipeline) updateCache(ctx context.Context, vectors []*vector.SecurityVector, embeddings [][]float32) {
	var cacheVectors []*cache.CachedVector
	var cacheEmbeddings [][]float32

	for i, v := range vectors {
		// Cache malicious vectors (label = 1) for faster lookup
		if v.Label == 1 {
			cached := &cache.CachedVector{
				ID:         v.ID,
				Text:       v.Text,
				LabelText:  v.LabelText,
				Label:      v.Label,
				Embedding:  v.Embedding,
				Similarity: 1.0, // Perfect match for exact text
			}
			cacheVectors = append(cacheVectors, cached)
			cacheEmbeddings = append(cacheEmbeddings, embeddings[i])
		}
	}

	if len(cacheVectors) > 0 {
		if err := p.vectorCache.StoreBatch(ctx, cacheEmbeddings, cacheVectors); err != nil {
			p.logger.Warn("Failed to update cache", zap.Error(err))
		} else {
			p.logger.Debug("Cache updated", zap.Int("cached_vectors", len(cacheVectors)))
		}
	}
}

// validateRecord validates a data record
func (p *Pipeline) validateRecord(record *DataRecord) bool {
	if !p.config.ValidateData {
		return true
	}

	// Check required fields
	if strings.TrimSpace(record.Text) == "" {
		p.logger.Debug("Invalid record: empty text")
		return false
	}

	if strings.TrimSpace(record.LabelText) == "" {
		p.logger.Debug("Invalid record: empty label_text")
		return false
	}

	// Validate label
	if record.Label != 0 && record.Label != 1 {
		p.logger.Debug("Invalid record: invalid label", zap.Int("label", record.Label))
		return false
	}

	// Check text length (reasonable limits)
	if len(record.Text) > 10000 {
		p.logger.Debug("Invalid record: text too long", zap.Int("length", len(record.Text)))
		return false
	}

	return true
}

// reportProgress reports current processing progress
func (p *Pipeline) reportProgress(result *ProcessingResult) {
	elapsed := time.Since(p.stats.StartTime)
	rate := float64(result.TotalRecords) / elapsed.Seconds()

	p.logger.Info("Processing progress",
		zap.Int64("records_processed", result.TotalRecords),
		zap.Int64("records_ok", result.ProcessedOK),
		zap.Int64("records_failed", result.ProcessedFailed),
		zap.Float64("rate_per_sec", rate),
		zap.Duration("elapsed", elapsed),
		zap.Duration("avg_embedding_time", result.EmbeddingTime/time.Duration(result.TotalRecords)),
		zap.Duration("avg_database_time", result.DatabaseTime/time.Duration(result.TotalRecords)))
}

// resetStats resets processing statistics
func (p *Pipeline) resetStats() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.stats = &ProcessingStats{
		StartTime: time.Now(),
	}
}

// GetStats returns current processing statistics
func (p *Pipeline) GetStats() *ProcessingStats {
	p.mu.RLock()
	defer p.mu.RUnlock()

	// Create a copy
	stats := *p.stats
	return &stats
}

// computeTextHash computes SHA-256 hash of the given text
func computeTextHash(text string) string {
	hash := sha256.Sum256([]byte(text))
	return hex.EncodeToString(hash[:])
}
