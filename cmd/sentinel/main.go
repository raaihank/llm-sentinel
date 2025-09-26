package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/raaihank/llm-sentinel/internal/config"
	"github.com/raaihank/llm-sentinel/internal/logger"
	"github.com/raaihank/llm-sentinel/internal/proxy"
	"go.uber.org/zap"
)

var (
	version = "0.1.0"
	commit  = "dev"
	date    = "unknown"
)

func main() {
	// Parse command line flags
	var (
		configPath  = flag.String("config", "", "Path to configuration file")
		showVersion = flag.Bool("version", false, "Show version information")
		healthCheck = flag.Bool("health-check", false, "Perform health check and exit")
	)
	flag.Parse()

	// Show version and exit
	if *showVersion {
		fmt.Printf("LLM-Sentinel %s (commit: %s, built: %s)\n", version, commit, date)
		os.Exit(0)
	}

	// Perform health check and exit
	if *healthCheck {
		performHealthCheck()
		return
	}

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger
	loggerConfig := logger.Config{
		Level:  cfg.Logging.Level,
		Format: cfg.Logging.Format,
	}

	if cfg.Logging.File.Enabled {
		loggerConfig.File = &logger.FileConfig{
			Enabled:  cfg.Logging.File.Enabled,
			Path:     cfg.Logging.File.Path,
			MaxSize:  cfg.Logging.File.MaxSize,
			MaxAge:   cfg.Logging.File.MaxAge,
			Compress: cfg.Logging.File.Compress,
		}
	}

	log, err := logger.New(loggerConfig)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer log.Sync()

	log.Info("Starting LLM-Sentinel",
		zap.String("version", version),
		zap.String("commit", commit),
		zap.String("build_date", date),
		zap.Int("port", cfg.Server.Port),
	)

	// Create proxy server
	server, err := proxy.New(cfg, log)
	if err != nil {
		log.Fatal("Failed to create proxy server", zap.Error(err))
	}

	// Start server in goroutine
	serverErrors := make(chan error, 1)
	go func() {
		log.Info("HTTP server listening", zap.Int("port", cfg.Server.Port))
		serverErrors <- server.Start()
	}()

	// Setup graceful shutdown
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, syscall.SIGINT, syscall.SIGTERM)

	// Wait for shutdown signal or server error
	select {
	case err := <-serverErrors:
		log.Error("Server error", zap.Error(err))
	case sig := <-shutdown:
		log.Info("Shutdown signal received", zap.String("signal", sig.String()))

		// Give outstanding requests 30 seconds to complete
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := server.Stop(ctx); err != nil {
			log.Error("Failed to shutdown server gracefully", zap.Error(err))
			os.Exit(1)
		}

		log.Info("Server shutdown complete")
	}
}

// performHealthCheck performs a health check against the running server
func performHealthCheck() {
	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	resp, err := client.Get("http://localhost:8080/health")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Health check failed: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stderr, "Health check failed: HTTP %d\n", resp.StatusCode)
		os.Exit(1)
	}

	fmt.Println("Health check passed")
	os.Exit(0)
}
