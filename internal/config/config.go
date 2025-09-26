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
	if config.Server.Port <= 0 || config.Server.Port > 65535 {
		return fmt.Errorf("invalid server port: %d", config.Server.Port)
	}

	if config.Security.Mode != "block" && config.Security.Mode != "log" && config.Security.Mode != "passthrough" {
		return fmt.Errorf("invalid security mode: %s (must be block, log, or passthrough)", config.Security.Mode)
	}

	if config.Logging.Level != "debug" && config.Logging.Level != "info" && config.Logging.Level != "warn" && config.Logging.Level != "error" {
		return fmt.Errorf("invalid log level: %s (must be debug, info, warn, or error)", config.Logging.Level)
	}

	if config.Logging.Format != "json" && config.Logging.Format != "console" {
		return fmt.Errorf("invalid log format: %s (must be json or console)", config.Logging.Format)
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
