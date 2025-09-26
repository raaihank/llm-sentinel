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
}

// EmbeddingResult represents the result of embedding generation
type EmbeddingResult struct {
	Embedding  []float32     `json:"embedding"`
	Duration   time.Duration `json:"duration"`
	TokenCount int           `json:"token_count"`
}

// BatchEmbeddingResult represents the result of batch embedding generation
type BatchEmbeddingResult struct {
	Embeddings  [][]float32   `json:"embeddings"`
	Duration    time.Duration `json:"duration"`
	TotalTokens int           `json:"total_tokens"`
	Failed      int           `json:"failed"`
	Errors      []error       `json:"errors,omitempty"`
}

// ModelStats represents model performance statistics
type ModelStats struct {
	TotalInferences   int64         `json:"total_inferences"`
	TotalTokens       int64         `json:"total_tokens"`
	AvgInferenceTime  time.Duration `json:"avg_inference_time"`
	AvgTokensPerText  float64       `json:"avg_tokens_per_text"`
	ModelLoadTime     time.Duration `json:"model_load_time"`
	ModelMemoryUsage  int64         `json:"model_memory_usage_bytes"`
	LastInferenceTime time.Time     `json:"last_inference_time"`
}

// TokenizerResult represents tokenization result
type TokenizerResult struct {
	InputIDs      []int32 `json:"input_ids"`
	AttentionMask []int32 `json:"attention_mask"`
	TokenCount    int     `json:"token_count"`
}
