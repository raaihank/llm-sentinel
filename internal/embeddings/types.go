package embeddings

import (
	"time"
)

// ModelConfig contains embedding model configuration
type ModelConfig struct {
	ModelName     string        `yaml:"model_name" mapstructure:"model_name"`         // "sentence-transformers/all-MiniLM-L6-v2"
	ModelPath     string        `yaml:"model_path" mapstructure:"model_path"`         // "./models/minilm-l6-v2.onnx"
	TokenizerPath string        `yaml:"tokenizer_path" mapstructure:"tokenizer_path"` // "./models/tokenizer.json"
	VocabPath     string        `yaml:"vocab_path" mapstructure:"vocab_path"`         // "./models/vocab.txt"
	CacheDir      string        `yaml:"cache_dir" mapstructure:"cache_dir"`           // "./models/cache"
	AutoDownload  bool          `yaml:"auto_download" mapstructure:"auto_download"`   // true
	MaxLength     int           `yaml:"max_length" mapstructure:"max_length"`         // 512
	BatchSize     int           `yaml:"batch_size" mapstructure:"batch_size"`         // 32
	ModelTimeout  time.Duration `yaml:"model_timeout" mapstructure:"model_timeout"`   // 30s
	CacheTTL      time.Duration `yaml:"cache_ttl" mapstructure:"cache_ttl"`           // 6h
}

// EmbeddingResult represents the result of embedding generation
type EmbeddingResult struct {
	Embedding   []float32             `json:"embedding"`
	Duration    time.Duration         `json:"duration"`
	TokenCount  int                   `json:"token_count"`
	Analysis    *AttackAnalysisResult `json:"analysis,omitempty"`
	Features    *TextFeatures         `json:"features,omitempty"`
	ServiceType string                `json:"service_type"`
	CacheHit    bool                  `json:"cache_hit"`
}

// BatchEmbeddingResult represents the result of batch embedding generation
type BatchEmbeddingResult struct {
	Embeddings  [][]float32   `json:"embeddings"`
	Duration    time.Duration `json:"duration"`
	TotalTokens int           `json:"total_tokens"`
	Successful  int           `json:"successful"`
	Failed      int           `json:"failed"`
	Errors      []error       `json:"errors,omitempty"`
	ServiceType string        `json:"service_type"`
	CacheHits   int           `json:"cache_hits"`
}

// ModelStats represents model performance statistics
type ModelStats struct {
	TotalInferences   int64         `json:"total_inferences"`
	TotalTokens       int64         `json:"total_tokens"`
	SuccessfulRuns    int64         `json:"successful_runs"`
	FailedRuns        int64         `json:"failed_runs"`
	AvgInferenceTime  time.Duration `json:"avg_inference_time"`
	AvgTokensPerText  float64       `json:"avg_tokens_per_text"`
	ModelLoadTime     time.Duration `json:"model_load_time"`
	ModelMemoryUsage  int64         `json:"model_memory_usage_bytes"`
	LastInferenceTime time.Time     `json:"last_inference_time"`
	CacheHitRatio     float64       `json:"cache_hit_ratio"`
	ErrorRate         float64       `json:"error_rate"`
	ServiceType       string        `json:"service_type"`
	StartTime         time.Time     `json:"start_time"`
}

// TokenizerResult represents tokenization result
type TokenizerResult struct {
	InputIDs      []int32 `json:"input_ids"`
	AttentionMask []int32 `json:"attention_mask"`
	TokenTypeIDs  []int32 `json:"token_type_ids"`
	TokenCount    int     `json:"token_count"`
	OriginalText  string  `json:"original_text"`
	Truncated     bool    `json:"truncated"`
}

// EmbeddingErrors define custom error types
type EmbeddingError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
	Code    int    `json:"code"`
}

func (e *EmbeddingError) Error() string {
	return e.Message
}

// Common error types
var (
	ErrInvalidInput        = &EmbeddingError{Type: "invalid_input", Message: "invalid input text", Code: 1001}
	ErrModelNotLoaded      = &EmbeddingError{Type: "model_not_loaded", Message: "model not loaded", Code: 1002}
	ErrInferenceFailed     = &EmbeddingError{Type: "inference_failed", Message: "inference failed", Code: 1003}
	ErrCacheError          = &EmbeddingError{Type: "cache_error", Message: "cache operation failed", Code: 1004}
	ErrConfigError         = &EmbeddingError{Type: "config_error", Message: "configuration error", Code: 1005}
	ErrNetworkError        = &EmbeddingError{Type: "network_error", Message: "network operation failed", Code: 1006}
	ErrTimeoutError        = &EmbeddingError{Type: "timeout_error", Message: "operation timed out", Code: 1007}
	ErrTokenizationFailed  = &EmbeddingError{Type: "tokenization_failed", Message: "tokenization failed", Code: 1008}
	ErrModelDownloadFailed = &EmbeddingError{Type: "model_download_failed", Message: "model download failed", Code: 1009}
	ErrInsufficientMemory  = &EmbeddingError{Type: "insufficient_memory", Message: "insufficient memory for model", Code: 1010}
)
