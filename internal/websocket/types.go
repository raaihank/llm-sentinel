package websocket

import (
	"time"

	"github.com/raaihank/llm-sentinel/internal/privacy"
)

// EventType represents the type of WebSocket event
type EventType string

const (
	// EventTypePIIDetection represents a PII detection event
	EventTypePIIDetection EventType = "pii_detection"
	// EventTypeVectorSecurity represents a vector security threat detection
	EventTypeVectorSecurity EventType = "vector_security"
	// EventTypeSystemStatus represents a system status event
	EventTypeSystemStatus EventType = "system_status"
	// EventTypeConnection represents connection events
	EventTypeConnection EventType = "connection"
	// EventTypeRequestCompletion represents request completion for response time tracking
	EventTypeRequestCompletion EventType = "request_completion"
)

// Event represents a WebSocket event sent to clients
type Event struct {
	Type      EventType   `json:"type"`
	Timestamp time.Time   `json:"timestamp"`
	Data      interface{} `json:"data"`
	RequestID string      `json:"request_id,omitempty"`
}

// PIIDetectionEvent represents a PII detection event
type PIIDetectionEvent struct {
	RequestID     string            `json:"request_id"`
	Method        string            `json:"method"`
	Path          string            `json:"path"`
	ClientIP      string            `json:"client_ip"`
	UserAgent     string            `json:"user_agent,omitempty"`
	Findings      []privacy.Finding `json:"findings"`
	TotalFindings int               `json:"total_findings"`
	MaskedContent bool              `json:"masked_content"`
	ProcessingMS  float64           `json:"processing_ms"`
}

// VectorSecurityEvent represents a vector security threat detection
type VectorSecurityEvent struct {
	RequestID    string  `json:"request_id"`
	Method       string  `json:"method"`
	Path         string  `json:"path"`
	ClientIP     string  `json:"client_ip"`
	UserAgent    string  `json:"user_agent,omitempty"`
	IsMalicious  bool    `json:"is_malicious"`
	AttackType   string  `json:"attack_type"`
	Confidence   float32 `json:"confidence"`
	Similarity   float32 `json:"similarity"`
	MatchedText  string  `json:"matched_text,omitempty"`
	Action       string  `json:"action"` // "blocked", "logged", "allowed"
	ProcessingMS float64 `json:"processing_ms"`
}

// SystemStatusEvent represents system status information
type SystemStatusEvent struct {
	Status           string `json:"status"`
	Uptime           string `json:"uptime"`
	TotalRequests    int64  `json:"total_requests"`
	TotalDetections  int64  `json:"total_detections"`
	ActiveRules      int    `json:"active_rules"`
	ConnectedClients int    `json:"connected_clients"`
	MemoryUsage      string `json:"memory_usage"`
	CPUUsage         string `json:"cpu_usage,omitempty"`
}

// ConnectionEvent represents WebSocket connection events
type ConnectionEvent struct {
	Action    string `json:"action"` // "connected", "disconnected"
	ClientID  string `json:"client_id"`
	ClientIP  string `json:"client_ip"`
	UserAgent string `json:"user_agent,omitempty"`
	Message   string `json:"message,omitempty"`
}

// RequestCompletionEvent represents request completion for response time tracking
type RequestCompletionEvent struct {
	RequestID    string  `json:"request_id"`
	Method       string  `json:"method"`
	Path         string  `json:"path"`
	StatusCode   int     `json:"status_code"`
	ResponseTime float64 `json:"response_time"` // in milliseconds
	ResponseSize int     `json:"response_size"` // in bytes
}

// ClientMessage represents messages sent from clients to server
type ClientMessage struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

// SubscriptionRequest represents a client subscription request
type SubscriptionRequest struct {
	Events []EventType  `json:"events"`
	Filter *EventFilter `json:"filter,omitempty"`
}

// EventFilter represents filtering options for events
type EventFilter struct {
	MinSeverity   string   `json:"min_severity,omitempty"`
	RuleTypes     []string `json:"rule_types,omitempty"`
	IPWhitelist   []string `json:"ip_whitelist,omitempty"`
	PathPatterns  []string `json:"path_patterns,omitempty"`
	ExcludeHealth bool     `json:"exclude_health,omitempty"`
}

// Client represents a WebSocket client connection
type Client struct {
	ID           string
	Conn         interface{} // Will be *websocket.Conn
	Send         chan Event
	Subscription *SubscriptionRequest
	ConnectedAt  time.Time
	LastPing     time.Time
	IP           string
	UserAgent    string
}
