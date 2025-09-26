package proxy

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/yourusername/llm-sentinel/internal/config"
	"github.com/yourusername/llm-sentinel/internal/logger"
	"github.com/yourusername/llm-sentinel/internal/privacy"
	"github.com/yourusername/llm-sentinel/internal/web"
	"github.com/yourusername/llm-sentinel/internal/websocket"
	"go.uber.org/zap"
)

// Server represents the main proxy server
type Server struct {
	config   *config.Config
	logger   *logger.Logger
	detector *privacy.Detector
	router   *mux.Router
	server   *http.Server
	wsHub    *websocket.Hub
}

// New creates a new proxy server instance
func New(cfg *config.Config, log *logger.Logger) (*Server, error) {
	// Create PII detector
	detector, err := privacy.New(cfg.Privacy, log.WithComponent("privacy"))
	if err != nil {
		return nil, fmt.Errorf("failed to create privacy detector: %w", err)
	}

	// Create WebSocket hub
	wsHub := websocket.NewHub(log.WithComponent("websocket").Logger)

	// Create router
	router := mux.NewRouter()

	// Create server
	server := &Server{
		config:   cfg,
		logger:   log.WithComponent("proxy"),
		detector: detector,
		router:   router,
		wsHub:    wsHub,
	}

	// Setup routes
	server.setupRoutes()

	// Create HTTP server
	server.server = &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:      server.router,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
	}

	return server, nil
}

// setupRoutes configures all HTTP routes
func (s *Server) setupRoutes() {
	// Health check endpoint
	s.router.HandleFunc("/health", s.handleHealth).Methods("GET")

	// Info endpoint
	s.router.HandleFunc("/info", s.handleInfo).Methods("GET")

	// Dashboard endpoint - embedded HTML
	s.router.HandleFunc("/", web.ServeDashboard).Methods("GET")
	s.router.HandleFunc("/dashboard", web.ServeDashboard).Methods("GET")

	// WebSocket endpoint for dashboard
	s.router.HandleFunc("/ws", s.handleWebSocket).Methods("GET")

	// OpenAI proxy endpoints
	openaiRouter := s.router.PathPrefix("/openai").Subrouter()
	openaiRouter.Use(s.loggingMiddleware)
	openaiRouter.Use(s.privacyMiddleware)
	openaiRouter.PathPrefix("/").HandlerFunc(s.handleOpenAIProxy)

	// Ollama proxy endpoints
	ollamaRouter := s.router.PathPrefix("/ollama").Subrouter()
	ollamaRouter.Use(s.loggingMiddleware)
	ollamaRouter.Use(s.privacyMiddleware)
	ollamaRouter.PathPrefix("/").HandlerFunc(s.handleOllamaProxy)

	// Anthropic proxy endpoints
	anthropicRouter := s.router.PathPrefix("/anthropic").Subrouter()
	anthropicRouter.Use(s.loggingMiddleware)
	anthropicRouter.Use(s.privacyMiddleware)
	anthropicRouter.PathPrefix("/").HandlerFunc(s.handleAnthropicProxy)
}

// Start starts the HTTP server
func (s *Server) Start() error {
	s.logger.Info("Starting LLM-Sentinel proxy server",
		zap.Int("port", s.config.Server.Port),
		zap.String("upstream_openai", s.config.Upstream.OpenAI),
		zap.String("upstream_ollama", s.config.Upstream.Ollama),
		zap.String("upstream_anthropic", s.config.Upstream.Anthropic),
	)

	// Start WebSocket hub in a separate goroutine
	go s.wsHub.Run()

	return s.server.ListenAndServe()
}

// Stop gracefully stops the HTTP server
func (s *Server) Stop(ctx context.Context) error {
	s.logger.Info("Stopping LLM-Sentinel proxy server")
	return s.server.Shutdown(ctx)
}

// handleHealth handles health check requests
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{"status":"healthy","timestamp":"%s"}`, time.Now().Format(time.RFC3339))
}

// handleInfo handles info requests
func (s *Server) handleInfo(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{
		"name":"llm-sentinel",
		"version":"0.1.0",
		"privacy_enabled":%t,
		"security_enabled":%t,
		"detectors_count":%d
	}`, s.config.Privacy.Enabled, s.config.Security.Enabled, len(s.config.Privacy.Detectors))
}

// handleWebSocket handles WebSocket connections for the dashboard
func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	s.wsHub.HandleWebSocket(w, r)
}

// GetWebSocketHub returns the WebSocket hub for broadcasting events
func (s *Server) GetWebSocketHub() *websocket.Hub {
	return s.wsHub
}
