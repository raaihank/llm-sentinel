package logger

import (
	"os"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Logger wraps zap.Logger with additional functionality
type Logger struct {
	*zap.Logger
}

// Config contains logger configuration
type Config struct {
	Level  string
	Format string // json or console
	File   *FileConfig
}

// FileConfig contains file logging configuration
type FileConfig struct {
	Enabled  bool
	Path     string
	MaxSize  int
	MaxAge   int
	Compress bool
}

// New creates a new logger instance
func New(config Config) (*Logger, error) {
	// Parse log level
	level, err := zapcore.ParseLevel(config.Level)
	if err != nil {
		return nil, err
	}

	// Create encoder config
	var encoderConfig zapcore.EncoderConfig
	if config.Format == "console" {
		encoderConfig = zap.NewDevelopmentEncoderConfig()
		encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	} else {
		encoderConfig = zap.NewProductionEncoderConfig()
		encoderConfig.TimeKey = "timestamp"
		encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	}

	// Create encoder
	var encoder zapcore.Encoder
	if config.Format == "console" {
		encoder = zapcore.NewConsoleEncoder(encoderConfig)
	} else {
		encoder = zapcore.NewJSONEncoder(encoderConfig)
	}

	// Create core
	var cores []zapcore.Core

	// Console output
	consoleCore := zapcore.NewCore(
		encoder,
		zapcore.AddSync(os.Stdout),
		level,
	)
	cores = append(cores, consoleCore)

	// File output (if enabled)
	if config.File != nil && config.File.Enabled {
		file, err := os.OpenFile(config.File.Path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			return nil, err
		}

		fileCore := zapcore.NewCore(
			zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig()),
			zapcore.AddSync(file),
			level,
		)
		cores = append(cores, fileCore)
	}

	// Combine cores
	core := zapcore.NewTee(cores...)

	// Create logger
	logger := zap.New(core, zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel))

	return &Logger{Logger: logger}, nil
}

// WithRequestID adds a request ID to the logger context
func (l *Logger) WithRequestID(requestID string) *Logger {
	return &Logger{Logger: l.Logger.With(zap.String("request_id", requestID))}
}

// WithComponent adds a component name to the logger context
func (l *Logger) WithComponent(component string) *Logger {
	return &Logger{Logger: l.Logger.With(zap.String("component", component))}
}

// LogRequest logs an HTTP request with redacted sensitive data
func (l *Logger) LogRequest(method, path string, headers map[string][]string, body string, redacted bool) {
	// Redact sensitive headers
	safeHeaders := make(map[string]string)
	for k, v := range headers {
		if isSensitiveHeader(k) {
			safeHeaders[k] = "[REDACTED]"
		} else if len(v) > 0 {
			safeHeaders[k] = v[0]
		}
	}

	// Redact body if it contains sensitive data
	safeBody := body
	if redacted {
		safeBody = "[REDACTED - CONTAINS SENSITIVE DATA]"
	}

	l.Info("HTTP request",
		zap.String("method", method),
		zap.String("path", path),
		zap.Any("headers", safeHeaders),
		zap.String("body", safeBody),
	)
}

// LogResponse logs an HTTP response with redacted sensitive data
func (l *Logger) LogResponse(statusCode int, headers map[string][]string, body string, redacted bool) {
	// Redact sensitive headers
	safeHeaders := make(map[string]string)
	for k, v := range headers {
		if isSensitiveHeader(k) {
			safeHeaders[k] = "[REDACTED]"
		} else if len(v) > 0 {
			safeHeaders[k] = v[0]
		}
	}

	// Redact body if it contains sensitive data
	safeBody := body
	if redacted {
		safeBody = "[REDACTED - CONTAINS SENSITIVE DATA]"
	}

	l.Info("HTTP response",
		zap.Int("status_code", statusCode),
		zap.Any("headers", safeHeaders),
		zap.String("body", safeBody),
	)
}

// isSensitiveHeader checks if a header contains sensitive information
func isSensitiveHeader(header string) bool {
	sensitiveHeaders := []string{
		"authorization",
		"x-api-key",
		"cookie",
		"x-auth-token",
		"x-access-token",
		"bearer",
	}

	headerLower := strings.ToLower(header)
	for _, sensitive := range sensitiveHeaders {
		if strings.Contains(headerLower, sensitive) {
			return true
		}
	}
	return false
}
