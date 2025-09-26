package proxy

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/yourusername/llm-sentinel/internal/websocket"
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

		// Broadcast request log event to WebSocket clients
		requestLogEvent := websocket.Event{
			Type:      websocket.EventTypeRequestLog,
			Timestamp: time.Now(),
			RequestID: requestID,
			Data: websocket.RequestLogEvent{
				RequestID:    requestID,
				Method:       r.Method,
				Path:         r.URL.Path,
				StatusCode:   rw.statusCode,
				ClientIP:     getClientIP(r),
				UserAgent:    r.UserAgent(),
				Duration:     duration,
				RequestSize:  r.ContentLength,
				ResponseSize: int64(rw.size),
			},
		}
		s.wsHub.BroadcastEvent(requestLogEvent)
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

		// Read request body
		body, err := io.ReadAll(r.Body)
		if err != nil {
			logger.Error("Failed to read request body", zap.Error(err))
			http.Error(w, "Failed to read request", http.StatusInternalServerError)
			return
		}
		r.Body.Close()

		// Process headers for sensitive data
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
		ctx := context.WithValue(r.Context(), "privacy_findings", result.Findings)

		next.ServeHTTP(w, r.WithContext(ctx))
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
