package config

import "time"

// Config represents the main configuration structure
type Config struct {
	Server    ServerConfig    `yaml:"server" mapstructure:"server"`
	Privacy   PrivacyConfig   `yaml:"privacy" mapstructure:"privacy"`
	Security  SecurityConfig  `yaml:"security" mapstructure:"security"`
	Logging   LoggingConfig   `yaml:"logging" mapstructure:"logging"`
	Upstream  UpstreamConfig  `yaml:"upstream" mapstructure:"upstream"`
	WebSocket WebSocketConfig `yaml:"websocket" mapstructure:"websocket"`
}

// ServerConfig contains HTTP server configuration
type ServerConfig struct {
	Port         int           `yaml:"port" mapstructure:"port"`
	ReadTimeout  time.Duration `yaml:"read_timeout" mapstructure:"read_timeout"`
	WriteTimeout time.Duration `yaml:"write_timeout" mapstructure:"write_timeout"`
	IdleTimeout  time.Duration `yaml:"idle_timeout" mapstructure:"idle_timeout"`
}

// PrivacyConfig contains PII detection and masking configuration
type PrivacyConfig struct {
	Enabled   bool     `yaml:"enabled" mapstructure:"enabled"`
	Detectors []string `yaml:"detectors" mapstructure:"detectors"`
	Masking   struct {
		Type   string `yaml:"type" mapstructure:"type"`
		Format string `yaml:"format" mapstructure:"format"`
	} `yaml:"masking" mapstructure:"masking"`
	HeaderScrubbing struct {
		Enabled              bool     `yaml:"enabled" mapstructure:"enabled"`
		Headers              []string `yaml:"headers" mapstructure:"headers"`
		PreserveUpstreamAuth bool     `yaml:"preserve_upstream_auth" mapstructure:"preserve_upstream_auth"`
	} `yaml:"header_scrubbing" mapstructure:"header_scrubbing"`
}

// SecurityConfig contains basic security configuration
type SecurityConfig struct {
	Enabled        bool                 `yaml:"enabled" mapstructure:"enabled"`
	Mode           string               `yaml:"mode" mapstructure:"mode"` // block, log, or passthrough
	RateLimit      RateLimitConfig      `yaml:"rate_limit" mapstructure:"rate_limit"`
	VectorSecurity VectorSecurityConfig `yaml:"vector_security" mapstructure:"vector_security"`
}

// RateLimitConfig contains rate limiting configuration
type RateLimitConfig struct {
	Enabled        bool `yaml:"enabled" mapstructure:"enabled"`
	RequestsPerMin int  `yaml:"requests_per_min" mapstructure:"requests_per_min"`
	MaxRequestSize int  `yaml:"max_request_size" mapstructure:"max_request_size"` // bytes
	BurstLimit     int  `yaml:"burst_limit" mapstructure:"burst_limit"`
}

// VectorSecurityConfig contains vector-based security configuration
type VectorSecurityConfig struct {
	Enabled        bool            `yaml:"enabled" mapstructure:"enabled"`
	ServiceType    string          `yaml:"service_type" mapstructure:"service_type"` // "ml", "pattern", "hash"
	BlockThreshold float32         `yaml:"block_threshold" mapstructure:"block_threshold"`
	MaxBatchSize   int             `yaml:"max_batch_size" mapstructure:"max_batch_size"`
	Embedding      EmbeddingConfig `yaml:"embedding" mapstructure:"embedding"`
	Database       DatabaseConfig  `yaml:"database" mapstructure:"database"`
}

// EmbeddingConfig contains embedding service configuration
type EmbeddingConfig struct {
	ServiceType  string      `yaml:"service_type" mapstructure:"service_type"` // "ml", "pattern", "hash"
	Model        ModelConfig `yaml:"model" mapstructure:"model"`
	RedisEnabled bool        `yaml:"redis_enabled" mapstructure:"redis_enabled"`
	RedisURL     string      `yaml:"redis_url" mapstructure:"redis_url"`
	RedisDB      int         `yaml:"redis_db" mapstructure:"redis_db"`
}

// ModelConfig contains embedding model configuration
type ModelConfig struct {
	ModelName     string        `yaml:"model_name" mapstructure:"model_name"`
	ModelPath     string        `yaml:"model_path" mapstructure:"model_path"`
	TokenizerPath string        `yaml:"tokenizer_path" mapstructure:"tokenizer_path"`
	VocabPath     string        `yaml:"vocab_path" mapstructure:"vocab_path"`
	CacheDir      string        `yaml:"cache_dir" mapstructure:"cache_dir"`
	AutoDownload  bool          `yaml:"auto_download" mapstructure:"auto_download"`
	MaxLength     int           `yaml:"max_length" mapstructure:"max_length"`
	BatchSize     int           `yaml:"batch_size" mapstructure:"batch_size"`
	ModelTimeout  time.Duration `yaml:"model_timeout" mapstructure:"model_timeout"`
}

// DatabaseConfig contains vector database configuration
type DatabaseConfig struct {
	DatabaseURL     string        `yaml:"database_url" mapstructure:"database_url"`
	MaxOpenConns    int           `yaml:"max_open_conns" mapstructure:"max_open_conns"`
	MaxIdleConns    int           `yaml:"max_idle_conns" mapstructure:"max_idle_conns"`
	ConnMaxLifetime time.Duration `yaml:"conn_max_lifetime" mapstructure:"conn_max_lifetime"`
	ConnMaxIdleTime time.Duration `yaml:"conn_max_idle_time" mapstructure:"conn_max_idle_time"`
}

// LoggingConfig contains logging configuration
type LoggingConfig struct {
	Level  string `yaml:"level" mapstructure:"level"`
	Format string `yaml:"format" mapstructure:"format"` // json or console
	File   struct {
		Enabled  bool   `yaml:"enabled" mapstructure:"enabled"`
		Path     string `yaml:"path" mapstructure:"path"`
		MaxSize  int    `yaml:"max_size" mapstructure:"max_size"`
		MaxAge   int    `yaml:"max_age" mapstructure:"max_age"`
		Compress bool   `yaml:"compress" mapstructure:"compress"`
	} `yaml:"file" mapstructure:"file"`
}

// UpstreamConfig contains upstream service configuration
type UpstreamConfig struct {
	OpenAI    string        `yaml:"openai" mapstructure:"openai"`
	Anthropic string        `yaml:"anthropic" mapstructure:"anthropic"`
	Ollama    string        `yaml:"ollama" mapstructure:"ollama"`
	Timeout   time.Duration `yaml:"timeout" mapstructure:"timeout"`
}

// WebSocketConfig contains WebSocket configuration
type WebSocketConfig struct {
	Enabled         bool          `yaml:"enabled" mapstructure:"enabled"`
	Path            string        `yaml:"path" mapstructure:"path"`
	MaxConnections  int           `yaml:"max_connections" mapstructure:"max_connections"`
	ReadBufferSize  int           `yaml:"read_buffer_size" mapstructure:"read_buffer_size"`
	WriteBufferSize int           `yaml:"write_buffer_size" mapstructure:"write_buffer_size"`
	PingInterval    time.Duration `yaml:"ping_interval" mapstructure:"ping_interval"`
	PongTimeout     time.Duration `yaml:"pong_timeout" mapstructure:"pong_timeout"`
	WriteTimeout    time.Duration `yaml:"write_timeout" mapstructure:"write_timeout"`
	MaxMessageSize  int64         `yaml:"max_message_size" mapstructure:"max_message_size"`
	AllowedOrigins  []string      `yaml:"allowed_origins" mapstructure:"allowed_origins"`
	Events          struct {
		BroadcastPIIDetections  bool `yaml:"broadcast_pii_detections" mapstructure:"broadcast_pii_detections"`
		BroadcastVectorSecurity bool `yaml:"broadcast_vector_security" mapstructure:"broadcast_vector_security"`
		BroadcastSystem         bool `yaml:"broadcast_system" mapstructure:"broadcast_system"`
		BroadcastConnections    bool `yaml:"broadcast_connections" mapstructure:"broadcast_connections"`
	} `yaml:"events" mapstructure:"events"`
}

// GetDefaults returns a configuration with sensible defaults
func GetDefaults() *Config {
	return &Config{
		Server: ServerConfig{
			Port:         8080,
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 30 * time.Second,
			IdleTimeout:  60 * time.Second,
		},
		Privacy: PrivacyConfig{
			Enabled:   true,
			Detectors: []string{"all"},
			Masking: struct {
				Type   string `yaml:"type" mapstructure:"type"`
				Format string `yaml:"format" mapstructure:"format"`
			}{
				Type:   "deterministic",
				Format: "[MASKED_{{TYPE}}]",
			},
			HeaderScrubbing: struct {
				Enabled              bool     `yaml:"enabled" mapstructure:"enabled"`
				Headers              []string `yaml:"headers" mapstructure:"headers"`
				PreserveUpstreamAuth bool     `yaml:"preserve_upstream_auth" mapstructure:"preserve_upstream_auth"`
			}{
				Enabled:              true,
				Headers:              []string{"authorization", "x-api-key", "cookie"},
				PreserveUpstreamAuth: true,
			},
		},
		Security: SecurityConfig{
			Enabled: true,
			Mode:    "log", // Default to log mode, not block
			RateLimit: RateLimitConfig{
				Enabled:        true,
				RequestsPerMin: 60,
				MaxRequestSize: 1048576, // 1MB
				BurstLimit:     10,
			},
			VectorSecurity: VectorSecurityConfig{
				Enabled:        true,
				ServiceType:    "ml",
				BlockThreshold: 0.70,
				MaxBatchSize:   32,
				Embedding: EmbeddingConfig{
					ServiceType:  "ml",
					RedisEnabled: true,
					RedisURL:     "redis://localhost:6379",
					RedisDB:      0,
					Model: ModelConfig{
						ModelName:     "sentence-transformers/all-MiniLM-L6-v2",
						ModelPath:     "./models/minilm-l6-v2.onnx",
						TokenizerPath: "./models/tokenizer.json",
						VocabPath:     "./models/vocab.txt",
						CacheDir:      "./models/cache",
						AutoDownload:  true,
						MaxLength:     512,
						BatchSize:     32,
						ModelTimeout:  30 * time.Second,
					},
				},
				Database: DatabaseConfig{
					DatabaseURL:     "postgres://sentinel:sentinel_pass@localhost:5432/llm_sentinel?sslmode=disable",
					MaxOpenConns:    20,
					MaxIdleConns:    10,
					ConnMaxLifetime: time.Hour,
					ConnMaxIdleTime: 30 * time.Minute,
				},
			},
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "json",
			File: struct {
				Enabled  bool   `yaml:"enabled" mapstructure:"enabled"`
				Path     string `yaml:"path" mapstructure:"path"`
				MaxSize  int    `yaml:"max_size" mapstructure:"max_size"`
				MaxAge   int    `yaml:"max_age" mapstructure:"max_age"`
				Compress bool   `yaml:"compress" mapstructure:"compress"`
			}{
				Enabled:  false,
				Path:     "logs/sentinel.log",
				MaxSize:  100, // MB
				MaxAge:   30,  // days
				Compress: true,
			},
		},
		Upstream: UpstreamConfig{
			OpenAI:    "https://api.openai.com",
			Anthropic: "https://api.anthropic.com",
			Ollama:    "http://localhost:11434",
			Timeout:   30 * time.Second,
		},
		WebSocket: WebSocketConfig{
			Enabled:         true,
			Path:            "/ws",
			MaxConnections:  100,
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			PingInterval:    54 * time.Second,
			PongTimeout:     60 * time.Second,
			WriteTimeout:    10 * time.Second,
			MaxMessageSize:  512,
			AllowedOrigins:  []string{"*"}, // Allow all origins for development
			Events: struct {
				BroadcastPIIDetections  bool `yaml:"broadcast_pii_detections" mapstructure:"broadcast_pii_detections"`
				BroadcastVectorSecurity bool `yaml:"broadcast_vector_security" mapstructure:"broadcast_vector_security"`
				BroadcastSystem         bool `yaml:"broadcast_system" mapstructure:"broadcast_system"`
				BroadcastConnections    bool `yaml:"broadcast_connections" mapstructure:"broadcast_connections"`
			}{
				BroadcastPIIDetections:  true,
				BroadcastVectorSecurity: true,
				BroadcastSystem:         true,
				BroadcastConnections:    true,
			},
		},
	}
}
