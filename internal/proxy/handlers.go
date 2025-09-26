package proxy

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"go.uber.org/zap"
)

// handleOpenAIProxy handles requests to OpenAI API
func (s *Server) handleOpenAIProxy(w http.ResponseWriter, r *http.Request) {
	target, err := url.Parse(s.config.Upstream.OpenAI)
	if err != nil {
		s.logger.Error("Failed to parse OpenAI target URL", zap.Error(err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Remove /openai prefix from path
	r.URL.Path = strings.TrimPrefix(r.URL.Path, "/openai")
	if r.URL.Path == "" {
		r.URL.Path = "/"
	}

	s.proxyRequest(w, r, target, "openai")
}

// handleOllamaProxy handles requests to Ollama API
func (s *Server) handleOllamaProxy(w http.ResponseWriter, r *http.Request) {
	target, err := url.Parse(s.config.Upstream.Ollama)
	if err != nil {
		s.logger.Error("Failed to parse Ollama target URL", zap.Error(err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Remove /ollama prefix from path
	r.URL.Path = strings.TrimPrefix(r.URL.Path, "/ollama")
	if r.URL.Path == "" {
		r.URL.Path = "/"
	}

	s.proxyRequest(w, r, target, "ollama")
}

// handleAnthropicProxy handles requests to Anthropic API
func (s *Server) handleAnthropicProxy(w http.ResponseWriter, r *http.Request) {
	target, err := url.Parse(s.config.Upstream.Anthropic)
	if err != nil {
		s.logger.Error("Failed to parse Anthropic target URL", zap.Error(err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Remove /anthropic prefix from path
	r.URL.Path = strings.TrimPrefix(r.URL.Path, "/anthropic")
	if r.URL.Path == "" {
		r.URL.Path = "/"
	}

	s.proxyRequest(w, r, target, "anthropic")
}

// proxyRequest proxies the request to the target URL
func (s *Server) proxyRequest(w http.ResponseWriter, r *http.Request, target *url.URL, provider string) {
	requestID := getRequestID(r.Context())
	logger := s.logger.WithRequestID(requestID)

	// Create reverse proxy
	proxy := httputil.NewSingleHostReverseProxy(target)

	// Configure proxy
	proxy.Director = func(req *http.Request) {
		req.URL.Scheme = target.Scheme
		req.URL.Host = target.Host
		req.Host = target.Host

		// Preserve upstream authentication headers
		if s.config.Privacy.Enabled && s.config.Privacy.HeaderScrubbing.Enabled && s.config.Privacy.HeaderScrubbing.PreserveUpstreamAuth {
			// Get the original request from context to restore auth headers
			if originalHeaders, ok := req.Context().Value("original_headers").(map[string][]string); ok {
				// Restore auth headers that were scrubbed
				for key, values := range originalHeaders {
					if s.detector.IsAuthHeaderPublic(key) {
						req.Header.Del(key)
						for _, value := range values {
							req.Header.Add(key, value)
						}
					}
				}
			}
		}

		// Preserve original headers
		if _, ok := req.Header["User-Agent"]; !ok {
			req.Header.Set("User-Agent", "LLM-Sentinel/0.1.0")
		}

		logger.Debug("Proxying request",
			zap.String("provider", provider),
			zap.String("target_url", req.URL.String()),
			zap.String("method", req.Method),
		)
	}

	// Handle errors
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		logger.Error("Proxy error",
			zap.String("provider", provider),
			zap.Error(err),
		)

		http.Error(w, fmt.Sprintf("Proxy error: %v", err), http.StatusBadGateway)
	}

	// Set timeout
	proxy.Transport = &http.Transport{
		ResponseHeaderTimeout: s.config.Upstream.Timeout,
	}

	// Execute proxy request
	start := time.Now()
	proxy.ServeHTTP(w, r)
	duration := time.Since(start)

	logger.Info("Request proxied",
		zap.String("provider", provider),
		zap.Duration("upstream_duration", duration),
	)
}
