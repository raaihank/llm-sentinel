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

// SecurityConfig contains security guardrails configuration
type SecurityConfig struct {
	Enabled    bool   `yaml:"enabled" mapstructure:"enabled"`
	Mode       string `yaml:"mode" mapstructure:"mode"` // block, log, or passthrough
	Thresholds struct {
		Injection float64 `yaml:"injection" mapstructure:"injection"`
		Jailbreak float64 `yaml:"jailbreak" mapstructure:"jailbreak"`
	} `yaml:"thresholds" mapstructure:"thresholds"`
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
		BroadcastRequests    bool `yaml:"broadcast_requests" mapstructure:"broadcast_requests"`
		BroadcastDetections  bool `yaml:"broadcast_detections" mapstructure:"broadcast_detections"`
		BroadcastSystem      bool `yaml:"broadcast_system" mapstructure:"broadcast_system"`
		BroadcastConnections bool `yaml:"broadcast_connections" mapstructure:"broadcast_connections"`
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
			Mode:    "block",
			Thresholds: struct {
				Injection float64 `yaml:"injection" mapstructure:"injection"`
				Jailbreak float64 `yaml:"jailbreak" mapstructure:"jailbreak"`
			}{
				Injection: 0.7,
				Jailbreak: 0.8,
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
				BroadcastRequests    bool `yaml:"broadcast_requests" mapstructure:"broadcast_requests"`
				BroadcastDetections  bool `yaml:"broadcast_detections" mapstructure:"broadcast_detections"`
				BroadcastSystem      bool `yaml:"broadcast_system" mapstructure:"broadcast_system"`
				BroadcastConnections bool `yaml:"broadcast_connections" mapstructure:"broadcast_connections"`
			}{
				BroadcastRequests:    true,
				BroadcastDetections:  true,
				BroadcastSystem:      true,
				BroadcastConnections: true,
			},
		},
	}
}
