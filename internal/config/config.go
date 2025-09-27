package config

import (
	"fmt"
	"strings"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
)

// Load loads configuration from file and environment variables
func Load(configPath string) (*Config, error) {
	// Set defaults
	config := GetDefaults()

	// Configure viper
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("./configs")
	viper.AddConfigPath("/etc/llm-sentinel/")
	viper.AddConfigPath("$HOME/.llm-sentinel/")

	// Environment variable overrides
	viper.SetEnvPrefix("SENTINEL")
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// Use specific config file if provided
	if configPath != "" {
		viper.SetConfigFile(configPath)
	}

	// Read configuration
	if err := viper.ReadInConfig(); err != nil {
		// Config file not found is not an error - we'll use defaults
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
	}

	// Unmarshal into config struct
	if err := viper.Unmarshal(config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Validate configuration
	if err := validateConfig(config); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return config, nil
}

// validateConfig validates the loaded configuration
func validateConfig(config *Config) error {
	// Server validation
	if config.Server.Port <= 0 || config.Server.Port > 65535 {
		return fmt.Errorf("invalid server port: %d", config.Server.Port)
	}

	// Security validation
	if config.Security.Mode != "block" && config.Security.Mode != "log" && config.Security.Mode != "passthrough" {
		return fmt.Errorf("invalid security mode: %s (must be block, log, or passthrough)", config.Security.Mode)
	}

	// Vector security validation
	if config.Security.VectorSecurity.Enabled {
		if config.Security.VectorSecurity.BlockThreshold < 0 || config.Security.VectorSecurity.BlockThreshold > 1 {
			return fmt.Errorf("invalid vector security block threshold: %f (must be between 0 and 1)", config.Security.VectorSecurity.BlockThreshold)
		}

		if config.Security.VectorSecurity.MaxBatchSize <= 0 {
			return fmt.Errorf("invalid vector security max batch size: %d (must be positive)", config.Security.VectorSecurity.MaxBatchSize)
		}

		// Embedding configuration validation
		if config.Security.VectorSecurity.Embedding.ServiceType == "" {
			return fmt.Errorf("embedding service type is required")
		}

		validServiceTypes := map[string]bool{"hash": true, "pattern": true, "ml": true}
		if !validServiceTypes[config.Security.VectorSecurity.Embedding.ServiceType] {
			return fmt.Errorf("invalid embedding service type: %s (must be hash, pattern, or ml)", config.Security.VectorSecurity.Embedding.ServiceType)
		}

		// Model configuration validation
		if config.Security.VectorSecurity.Embedding.Model.ModelName == "" {
			return fmt.Errorf("embedding model name is required")
		}

		if config.Security.VectorSecurity.Embedding.Model.MaxLength <= 0 {
			return fmt.Errorf("invalid embedding model max length: %d (must be positive)", config.Security.VectorSecurity.Embedding.Model.MaxLength)
		}

		if config.Security.VectorSecurity.Embedding.Model.BatchSize <= 0 {
			return fmt.Errorf("invalid embedding model batch size: %d (must be positive)", config.Security.VectorSecurity.Embedding.Model.BatchSize)
		}

		// Redis validation for ML service
		if config.Security.VectorSecurity.Embedding.ServiceType == "ml" && config.Security.VectorSecurity.Embedding.RedisEnabled && config.Security.VectorSecurity.Embedding.RedisURL == "" {
			return fmt.Errorf("redis URL is required when Redis is enabled for ML service")
		}

		// Database validation
		if config.Security.VectorSecurity.Database.DatabaseURL == "" {
			return fmt.Errorf("database URL is required for vector security")
		}

		if config.Security.VectorSecurity.Database.MaxOpenConns <= 0 {
			return fmt.Errorf("invalid database max open connections: %d (must be positive)", config.Security.VectorSecurity.Database.MaxOpenConns)
		}

		if config.Security.VectorSecurity.Database.MaxIdleConns <= 0 {
			return fmt.Errorf("invalid database max idle connections: %d (must be positive)", config.Security.VectorSecurity.Database.MaxIdleConns)
		}
	}

	// Rate limiting validation
	if config.Security.RateLimit.Enabled {
		if config.Security.RateLimit.RequestsPerMin <= 0 {
			return fmt.Errorf("invalid rate limit requests per minute: %d (must be positive)", config.Security.RateLimit.RequestsPerMin)
		}

		if config.Security.RateLimit.MaxRequestSize <= 0 {
			return fmt.Errorf("invalid rate limit max request size: %d (must be positive)", config.Security.RateLimit.MaxRequestSize)
		}

		if config.Security.RateLimit.BurstLimit <= 0 {
			return fmt.Errorf("invalid rate limit burst limit: %d (must be positive)", config.Security.RateLimit.BurstLimit)
		}
	}

	// Logging validation
	if config.Logging.Level != "debug" && config.Logging.Level != "info" && config.Logging.Level != "warn" && config.Logging.Level != "error" {
		return fmt.Errorf("invalid log level: %s (must be debug, info, warn, or error)", config.Logging.Level)
	}

	if config.Logging.Format != "json" && config.Logging.Format != "console" {
		return fmt.Errorf("invalid log format: %s (must be json or console)", config.Logging.Format)
	}

	// WebSocket validation
	if config.WebSocket.Enabled {
		if config.WebSocket.MaxConnections <= 0 {
			return fmt.Errorf("invalid websocket max connections: %d (must be positive)", config.WebSocket.MaxConnections)
		}

		if config.WebSocket.ReadBufferSize <= 0 {
			return fmt.Errorf("invalid websocket read buffer size: %d (must be positive)", config.WebSocket.ReadBufferSize)
		}

		if config.WebSocket.WriteBufferSize <= 0 {
			return fmt.Errorf("invalid websocket write buffer size: %d (must be positive)", config.WebSocket.WriteBufferSize)
		}
	}

	return nil
}

// Watch starts watching the configuration file for changes
func Watch(config *Config, callback func(*Config)) error {
	viper.WatchConfig()
	viper.OnConfigChange(func(e fsnotify.Event) {
		newConfig := &Config{}
		if err := viper.Unmarshal(newConfig); err != nil {
			// Log error but don't crash
			return
		}

		if err := validateConfig(newConfig); err != nil {
			// Log error but don't crash
			return
		}

		callback(newConfig)
	})

	return nil
}
