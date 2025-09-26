package etl

import (
	"time"
)

// DataRecord represents a single record from the input dataset
type DataRecord struct {
	Text      string `csv:"text" parquet:"text" json:"text"`
	LabelText string `csv:"label_text" parquet:"label_text" json:"label_text"`
	Label     int    `csv:"label" parquet:"label" json:"label"`
}

// ProcessingResult represents the result of processing a dataset
type ProcessingResult struct {
	TotalRecords    int64         `json:"total_records"`
	ProcessedOK     int64         `json:"processed_ok"`
	ProcessedFailed int64         `json:"processed_failed"`
	Duplicates      int64         `json:"duplicates"`
	Duration        time.Duration `json:"duration"`
	EmbeddingTime   time.Duration `json:"embedding_time"`
	DatabaseTime    time.Duration `json:"database_time"`
	CacheTime       time.Duration `json:"cache_time"`
	Errors          []string      `json:"errors,omitempty"`
}

// Config contains ETL pipeline configuration
type Config struct {
	BatchSize      int           `yaml:"batch_size" mapstructure:"batch_size"`           // 1000
	WorkerCount    int           `yaml:"worker_count" mapstructure:"worker_count"`       // 4
	MaxRetries     int           `yaml:"max_retries" mapstructure:"max_retries"`         // 3
	RetryDelay     time.Duration `yaml:"retry_delay" mapstructure:"retry_delay"`         // 5s
	SkipDuplicates bool          `yaml:"skip_duplicates" mapstructure:"skip_duplicates"` // true
	ValidateData   bool          `yaml:"validate_data" mapstructure:"validate_data"`     // true
	CreateIndex    bool          `yaml:"create_index" mapstructure:"create_index"`       // true
	UpdateCache    bool          `yaml:"update_cache" mapstructure:"update_cache"`       // true
	ProgressReport int           `yaml:"progress_report" mapstructure:"progress_report"` // 1000
}

// ValidationError represents a data validation error
type ValidationError struct {
	Row     int64  `json:"row"`
	Field   string `json:"field"`
	Value   string `json:"value"`
	Message string `json:"message"`
}

// ProcessingStats tracks real-time processing statistics
type ProcessingStats struct {
	StartTime      time.Time `json:"start_time"`
	RecordsRead    int64     `json:"records_read"`
	RecordsValid   int64     `json:"records_valid"`
	RecordsInvalid int64     `json:"records_invalid"`
	EmbeddingsGen  int64     `json:"embeddings_generated"`
	DatabaseWrites int64     `json:"database_writes"`
	CacheWrites    int64     `json:"cache_writes"`
	CurrentBatch   int64     `json:"current_batch"`
	EstimatedTotal int64     `json:"estimated_total"`
	ProcessingRate float64   `json:"processing_rate"` // records per second
}

// FileFormat represents supported file formats
type FileFormat string

const (
	FormatCSV     FileFormat = "csv"
	FormatParquet FileFormat = "parquet"
	FormatJSON    FileFormat = "json"
)

// DetectFileFormat detects file format from extension
func DetectFileFormat(filename string) FileFormat {
	switch {
	case len(filename) >= 4 && filename[len(filename)-4:] == ".csv":
		return FormatCSV
	case len(filename) >= 8 && filename[len(filename)-8:] == ".parquet":
		return FormatParquet
	case len(filename) >= 5 && filename[len(filename)-5:] == ".json":
		return FormatJSON
	default:
		return FormatCSV // Default to CSV
	}
}
