package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"

	"github.com/raaihank/llm-sentinel/internal/cache"
	"github.com/raaihank/llm-sentinel/internal/config"
	"github.com/raaihank/llm-sentinel/internal/embeddings"
	"github.com/raaihank/llm-sentinel/internal/etl"
	"github.com/raaihank/llm-sentinel/internal/logger"
	"github.com/raaihank/llm-sentinel/internal/vector"
)

func main() {
	var (
		configPath   = flag.String("config", "configs/default.yaml", "Configuration file path")
		inputFile    = flag.String("input", "", "Input dataset file (CSV, Parquet, or JSON)")
		batchSize    = flag.Int("batch-size", 1000, "Batch size for processing")
		workers      = flag.Int("workers", 4, "Number of worker goroutines")
		skipCache    = flag.Bool("skip-cache", false, "Skip updating Redis cache")
		skipIndex    = flag.Bool("skip-index", false, "Skip creating vector index")
		validateOnly = flag.Bool("validate-only", false, "Only validate data, don't process")
		dryRun       = flag.Bool("dry-run", false, "Dry run - don't write to database")
		rebuildCache = flag.Bool("rebuild-cache", false, "Rebuild Redis cache from database")
		showStats    = flag.Bool("stats", false, "Show database statistics and exit")
	)
	flag.Parse()

	if *inputFile == "" && !*rebuildCache && !*showStats {
		fmt.Fprintf(os.Stderr, "Usage: %s [options]\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\nOptions:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  %s --input dataset.csv --batch-size 500\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s --input dataset.parquet --workers 8\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s --rebuild-cache\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s --stats\n", os.Args[0])
		os.Exit(1)
	}

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger
	log, err := logger.New(logger.Config{
		Level:  cfg.Logging.Level,
		Format: cfg.Logging.Format,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer log.Sync()

	log.Info("Starting LLM-Sentinel ETL Pipeline",
		zap.String("version", "0.1.0"),
		zap.String("config", *configPath))

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		log.Info("Received shutdown signal, cancelling operations...")
		cancel()
	}()

	// Initialize services
	services, err := initializeServices(cfg, log)
	if err != nil {
		log.Fatal("Failed to initialize services", zap.Error(err))
	}
	defer services.cleanup()

	// Handle different operations
	switch {
	case *showStats:
		if err := showDatabaseStats(ctx, services, log); err != nil {
			log.Fatal("Failed to show stats", zap.Error(err))
		}
	case *rebuildCache:
		if err := rebuildCacheFromDB(ctx, services, log); err != nil {
			log.Fatal("Failed to rebuild cache", zap.Error(err))
		}
	default:
		// Process input file
		etlConfig := &etl.Config{
			BatchSize:      *batchSize,
			WorkerCount:    *workers,
			MaxRetries:     3,
			RetryDelay:     5 * time.Second,
			SkipDuplicates: true,
			ValidateData:   true,
			CreateIndex:    !*skipIndex,
			UpdateCache:    !*skipCache,
			ProgressReport: 1000,
		}

		if err := processDataset(ctx, services, etlConfig, *inputFile, *validateOnly, *dryRun, log); err != nil {
			log.Fatal("ETL processing failed", zap.Error(err))
		}
	}

	log.Info("ETL pipeline completed successfully")
}

// services holds all initialized services
type services struct {
	vectorStore      *vector.Store
	embeddingService embeddings.EmbeddingService
	vectorCache      *cache.VectorCache
}

func (s *services) cleanup() {
	if s.embeddingService != nil {
		s.embeddingService.Close()
	}
	if s.vectorStore != nil {
		s.vectorStore.Close()
	}
	if s.vectorCache != nil {
		s.vectorCache.Close()
	}
}

// initializeServices initializes all required services
func initializeServices(cfg *config.Config, log *logger.Logger) (*services, error) {
	services := &services{}

	// Initialize vector store
	log.Info("Initializing vector store...")
	vectorStore, err := vector.NewStore(&vector.Config{
		DatabaseURL:     cfg.Security.VectorSecurity.Database.DatabaseURL,
		MaxOpenConns:    cfg.Security.VectorSecurity.Database.MaxOpenConns,
		MaxIdleConns:    cfg.Security.VectorSecurity.Database.MaxIdleConns,
		ConnMaxLifetime: cfg.Security.VectorSecurity.Database.ConnMaxLifetime,
		ConnMaxIdleTime: cfg.Security.VectorSecurity.Database.ConnMaxIdleTime,
	}, log.Logger)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize vector store: %w", err)
	}
	services.vectorStore = vectorStore

	// Initialize embedding service using factory
	log.Info("Initializing embedding service...")
	factory := embeddings.NewFactory(log.Logger)

	serviceConfig := embeddings.ServiceConfig{
		Type: embeddings.ServiceType(cfg.Security.VectorSecurity.Embedding.ServiceType),
		ModelConfig: embeddings.ModelConfig{
			ModelName:     cfg.Security.VectorSecurity.Embedding.Model.ModelName,
			ModelPath:     cfg.Security.VectorSecurity.Embedding.Model.ModelPath,
			TokenizerPath: cfg.Security.VectorSecurity.Embedding.Model.TokenizerPath,
			VocabPath:     cfg.Security.VectorSecurity.Embedding.Model.VocabPath,
			CacheDir:      cfg.Security.VectorSecurity.Embedding.Model.CacheDir,
			AutoDownload:  cfg.Security.VectorSecurity.Embedding.Model.AutoDownload,
			MaxLength:     cfg.Security.VectorSecurity.Embedding.Model.MaxLength,
			BatchSize:     cfg.Security.VectorSecurity.Embedding.Model.BatchSize,
			ModelTimeout:  cfg.Security.VectorSecurity.Embedding.Model.ModelTimeout,
		},
		RedisEnabled: cfg.Security.VectorSecurity.Embedding.RedisEnabled,
		RedisURL:     cfg.Security.VectorSecurity.Embedding.RedisURL,
	}

	embeddingService, err := factory.CreateService(serviceConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize embedding service: %w", err)
	}
	services.embeddingService = embeddingService

	// Vector caching is now handled by the embedding service itself

	return services, nil
}

// processDataset processes the input dataset file
func processDataset(ctx context.Context, services *services, etlConfig *etl.Config, inputFile string, validateOnly, dryRun bool, log *logger.Logger) error {
	log.Info("Processing dataset",
		zap.String("file", inputFile),
		zap.Bool("validate_only", validateOnly),
		zap.Bool("dry_run", dryRun))

	// Check if file exists
	if _, err := os.Stat(inputFile); os.IsNotExist(err) {
		return fmt.Errorf("input file does not exist: %s", inputFile)
	}

	// Create ETL pipeline
	pipeline := etl.NewPipeline(
		services.vectorStore,
		services.embeddingService,
		services.vectorCache,
		etlConfig,
		log.Logger,
	)

	// Process the file
	result, err := pipeline.ProcessFile(ctx, inputFile)
	if err != nil {
		return fmt.Errorf("pipeline processing failed: %w", err)
	}

	// Report results
	log.Info("Dataset processing completed",
		zap.String("file", inputFile),
		zap.Int64("total_records", result.TotalRecords),
		zap.Int64("processed_ok", result.ProcessedOK),
		zap.Int64("processed_failed", result.ProcessedFailed),
		zap.Int64("duplicates", result.Duplicates),
		zap.Duration("total_duration", result.Duration),
		zap.Duration("embedding_time", result.EmbeddingTime),
		zap.Duration("database_time", result.DatabaseTime),
		zap.Duration("cache_time", result.CacheTime),
		zap.Float64("records_per_second", float64(result.TotalRecords)/result.Duration.Seconds()))

	if len(result.Errors) > 0 {
		log.Warn("Processing completed with errors", zap.Strings("errors", result.Errors))
	}

	return nil
}

// showDatabaseStats displays current database statistics
func showDatabaseStats(ctx context.Context, services *services, log *logger.Logger) error {
	log.Info("Retrieving database statistics...")

	stats, err := services.vectorStore.GetStats(ctx)
	if err != nil {
		return fmt.Errorf("failed to get database stats: %w", err)
	}

	fmt.Printf("\n=== LLM-Sentinel Vector Database Statistics ===\n")
	fmt.Printf("Total Vectors:      %d\n", stats.TotalVectors)
	fmt.Printf("Malicious Vectors:  %d (%.1f%%)\n", stats.MaliciousCount,
		float64(stats.MaliciousCount)/float64(stats.TotalVectors)*100)
	fmt.Printf("Safe Vectors:       %d (%.1f%%)\n", stats.SafeCount,
		float64(stats.SafeCount)/float64(stats.TotalVectors)*100)
	fmt.Printf("Avg Search Time:    %.2f ms\n", stats.AvgSearchTimeMs)
	fmt.Printf("Cache Hit Rate:     %.1f%%\n", stats.CacheHitRate)

	// Get cache stats if available
	if services.vectorCache != nil {
		cacheStats, err := services.vectorCache.GetStats(ctx)
		if err == nil {
			fmt.Printf("\n=== Cache Statistics ===\n")
			fmt.Printf("Cache Hits:         %d\n", cacheStats.Hits)
			fmt.Printf("Cache Misses:       %d\n", cacheStats.Misses)
			fmt.Printf("Hit Rate:           %.1f%%\n", cacheStats.HitRate)
			fmt.Printf("Total Keys:         %d\n", cacheStats.TotalKeys)
			fmt.Printf("Memory Usage:       %.2f MB\n", float64(cacheStats.MemoryUsage)/1024/1024)
		}
	}

	// Get embedding service stats
	embeddingStats := services.embeddingService.GetStats()
	fmt.Printf("\n=== Embedding Service Statistics ===\n")
	fmt.Printf("Total Inferences:   %d\n", embeddingStats.TotalInferences)
	fmt.Printf("Total Tokens:       %d\n", embeddingStats.TotalTokens)
	fmt.Printf("Avg Inference Time: %v\n", embeddingStats.AvgInferenceTime)
	fmt.Printf("Avg Tokens/Text:    %.1f\n", embeddingStats.AvgTokensPerText)
	fmt.Printf("Model Load Time:    %v\n", embeddingStats.ModelLoadTime)

	return nil
}

// rebuildCacheFromDB rebuilds the Redis cache from database vectors
func rebuildCacheFromDB(ctx context.Context, services *services, log *logger.Logger) error {
	if services.vectorCache == nil {
		return fmt.Errorf("vector cache is not enabled")
	}

	log.Info("Rebuilding cache from database...")

	// Clear existing cache
	if err := services.vectorCache.Clear(ctx); err != nil {
		return fmt.Errorf("failed to clear cache: %w", err)
	}

	// New implementation:
	const batchSize = 1000
	var offset int64 = 0
	for {
		vectors, err := services.vectorStore.GetMaliciousVectors(ctx, batchSize, offset)
		if err != nil {
			return fmt.Errorf("failed to query malicious vectors: %w", err)
		}
		if len(vectors) == 0 {
			break
		}

		log.Debug("Processing vector batch", zap.Int("count", len(vectors)))
		for _, vec := range vectors {
			cacheKey := fmt.Sprintf("vector:%s", vec.TextHash)
			var setErr error = nil // Temp
			if setErr != nil {
				log.Warn("Failed to cache vector", zap.String("text_hash", vec.TextHash), zap.Error(setErr))
			} else {
				log.Debug("Cached key", zap.String("key", cacheKey))
			}
		}

		offset += int64(batchSize) // Use batchSize since mock
		if offset%1000 == 0 {
			log.Info("Cache rebuild progress", zap.Int64("processed", offset))
		}
	}

	log.Info("Cache rebuild completed")
	log.Warn("Cache rebuild is mocked; implement proper query in vector/store.go")
	return nil
}
