package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"golang.org/x/time/rate"

	"github.com/raaihank/llm-sentinel/internal/security"
	"github.com/raaihank/llm-sentinel/internal/websocket"
	"go.uber.org/zap"
)

// loggingMiddleware logs HTTP requests and responses
func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Generate request ID
		requestID := generateRequestID()
		ctx := context.WithValue(r.Context(), "request_id", requestID)
		r = r.WithContext(ctx)

		// Create response writer wrapper to capture response data
		rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		// Log request
		s.logger.WithRequestID(requestID).Info("HTTP request started",
			zap.String("method", r.Method),
			zap.String("path", r.URL.Path),
			zap.String("remote_addr", r.RemoteAddr),
			zap.String("user_agent", r.UserAgent()),
		)

		// Process request
		next.ServeHTTP(rw, r)

		// Log response
		duration := time.Since(start)
		s.logger.WithRequestID(requestID).Info("HTTP request completed",
			zap.Int("status_code", rw.statusCode),
			zap.Duration("duration", duration),
			zap.Int("response_size", rw.size),
		)

		// Note: Only security issues (PII detections, vector threats) are broadcast via WebSocket
		// General request logs are only written to structured logs
	})
}

// privacyMiddleware applies PII detection and masking to requests
func (s *Server) privacyMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.config.Privacy.Enabled {
			next.ServeHTTP(w, r)
			return
		}

		requestID := getRequestID(r.Context())
		logger := s.logger.WithRequestID(requestID)

		// Store original headers in context before processing
		originalHeaders := make(map[string][]string)
		for key, values := range r.Header {
			originalHeaders[key] = make([]string, len(values))
			copy(originalHeaders[key], values)
		}
		ctx := context.WithValue(r.Context(), "original_headers", originalHeaders)
		r = r.WithContext(ctx)

		// Read request body
		body, err := io.ReadAll(r.Body)
		if err != nil {
			logger.Error("Failed to read request body", zap.Error(err))
			http.Error(w, "Failed to read request", http.StatusInternalServerError)
			return
		}
		r.Body.Close()

		// Process headers for sensitive data (for logging purposes)
		processedHeaders := s.detector.ProcessHeaders(r.Header)
		for key, values := range processedHeaders {
			r.Header.Del(key)
			for _, value := range values {
				r.Header.Add(key, value)
			}
		}

		// Process body for PII
		result := s.detector.ProcessText(string(body))

		// Log findings
		if len(result.Findings) > 0 {
			logger.Info("PII detected in request",
				zap.Int("findings_count", len(result.Findings)),
				zap.Any("findings", result.Findings),
			)

			// Broadcast PII detection event to WebSocket clients
			piiEvent := websocket.Event{
				Type:      websocket.EventTypePIIDetection,
				Timestamp: time.Now(),
				RequestID: requestID,
				Data: websocket.PIIDetectionEvent{
					RequestID:     requestID,
					Method:        r.Method,
					Path:          r.URL.Path,
					ClientIP:      getClientIP(r),
					UserAgent:     r.UserAgent(),
					Findings:      result.Findings,
					TotalFindings: len(result.Findings),
					MaskedContent: true,
					ProcessingMS:  float64(time.Since(time.Now()).Nanoseconds()) / 1e6,
				},
			}
			s.wsHub.BroadcastEvent(piiEvent)
		}

		// Replace request body with masked version
		r.Body = io.NopCloser(bytes.NewReader([]byte(result.MaskedText)))
		r.ContentLength = int64(len(result.MaskedText))

		// Store findings in context for metrics/dashboard
		ctx = context.WithValue(ctx, "privacy_findings", result.Findings)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// vectorSecurityMiddleware applies ML-based prompt security analysis
func (s *Server) vectorSecurityMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip if vector security is not enabled or not available
		if s.vectorSecurity == nil || !s.vectorSecurity.IsEnabled() {
			next.ServeHTTP(w, r)
			return
		}

		requestID := getRequestID(r.Context())
		logger := s.logger.WithRequestID(requestID)

		// Read request body
		body, err := io.ReadAll(r.Body)
		if err != nil {
			logger.Error("Failed to read request body for vector analysis", zap.Error(err))
			next.ServeHTTP(w, r)
			return
		}
		r.Body.Close()

		// Try to extract prompt from JSON body
		var requestData map[string]interface{}
		prompt := ""

		if err := json.Unmarshal(body, &requestData); err == nil {
			// Try common prompt fields
			if p, ok := requestData["prompt"].(string); ok {
				prompt = p
			} else if p, ok := requestData["input"].(string); ok {
				prompt = p
			} else if messages, ok := requestData["messages"].([]interface{}); ok {
				// Handle OpenAI-style messages
				if len(messages) > 0 {
					if msg, ok := messages[len(messages)-1].(map[string]interface{}); ok {
						if content, ok := msg["content"].(string); ok {
							prompt = content
						}
					}
				}
			}
		}

		// If we found a prompt, analyze it
		if prompt != "" {
			var result *security.SecurityResult
			for attempt := 0; attempt < 3; attempt++ {
				var err error
				result, err = s.vectorSecurity.AnalyzePrompt(r.Context(), prompt)
				if err == nil {
					break
				}
				logger.Warn("Vector analysis attempt failed", zap.Int("attempt", attempt), zap.Error(err))
				time.Sleep(100 * time.Millisecond) // Backoff
			}
			if result == nil {
				logger.Error("All vector analysis attempts failed, passing through")
				// Proceed without blocking
			} else {
				// Log the analysis result
				logger.Info("Vector security analysis completed",
					zap.Bool("is_malicious", result.IsMalicious),
					zap.String("attack_type", result.AttackType),
					zap.Float32("confidence", result.Confidence),
					zap.Duration("processing_time", result.ProcessingTime))

				// Broadcast vector security event
				if result.IsMalicious || result.Confidence > 0.5 { // Broadcast even medium confidence
					action := "logged"
					if result.IsMalicious && result.Confidence >= s.vectorSecurity.GetBlockThreshold() {
						action = "blocked"
					}

					vectorEvent := websocket.Event{
						Type:      websocket.EventTypeVectorSecurity,
						Timestamp: time.Now(),
						RequestID: requestID,
						Data: websocket.VectorSecurityEvent{
							RequestID:    requestID,
							Method:       r.Method,
							Path:         r.URL.Path,
							ClientIP:     getClientIP(r),
							UserAgent:    r.UserAgent(),
							IsMalicious:  result.IsMalicious,
							AttackType:   result.AttackType,
							Confidence:   result.Confidence,
							Similarity:   result.SimilarityScore,
							MatchedText:  result.MatchedText,
							Action:       action,
							ProcessingMS: float64(result.ProcessingTime.Nanoseconds()) / 1e6,
						},
					}
					s.wsHub.BroadcastEvent(vectorEvent)
				}

				// Block request if malicious and above threshold
				if result.IsMalicious && result.Confidence >= s.vectorSecurity.GetBlockThreshold() {
					logger.Warn("Blocking malicious request",
						zap.String("attack_type", result.AttackType),
						zap.Float32("confidence", result.Confidence))

					http.Error(w, fmt.Sprintf("Request blocked: %s detected (confidence: %.1f%%)",
						result.AttackType, result.Confidence*100), http.StatusForbidden)
					return
				}
			}
		}

		// Restore request body for next middleware
		r.Body = io.NopCloser(bytes.NewReader(body))
		r.ContentLength = int64(len(body))

		next.ServeHTTP(w, r)
	})
}

// rateLimiterMiddleware applies rate limiting to requests
func (s *Server) rateLimiterMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := getClientIP(r)
		s.mu.Lock()
		limiter, ok := s.rateLimiters[ip]
		if !ok {
			limiter = rate.NewLimiter(rate.Every(time.Minute/time.Duration(s.config.Security.RateLimit.RequestsPerMin)), s.config.Security.RateLimit.BurstLimit)
			s.rateLimiters[ip] = limiter
		}
		s.mu.Unlock()
		if !limiter.Allow() {
			http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// getClientIP extracts the client IP from the request
func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return xff
	}

	// Check X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}

	// Fall back to RemoteAddr
	return r.RemoteAddr
}

// responseWriter wraps http.ResponseWriter to capture response data
type responseWriter struct {
	http.ResponseWriter
	statusCode int
	size       int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	size, err := rw.ResponseWriter.Write(b)
	rw.size += size
	return size, err
}

// generateRequestID generates a unique request ID
func generateRequestID() string {
	// Simple implementation - in production, use UUID or similar
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

// getRequestID extracts request ID from context
func getRequestID(ctx context.Context) string {
	if requestID, ok := ctx.Value("request_id").(string); ok {
		return requestID
	}
	return "unknown"
}
