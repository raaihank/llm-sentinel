package proxy

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"github.com/raaihank/llm-sentinel/internal/config"
	"github.com/raaihank/llm-sentinel/internal/embeddings"
	"github.com/raaihank/llm-sentinel/internal/logger"
	"github.com/raaihank/llm-sentinel/internal/privacy"
	"github.com/raaihank/llm-sentinel/internal/security"
	"github.com/raaihank/llm-sentinel/internal/vector"
	"github.com/raaihank/llm-sentinel/internal/web"
	"github.com/raaihank/llm-sentinel/internal/websocket"
	"go.uber.org/zap"
	"golang.org/x/time/rate"
)

// Server represents the main proxy server
type Server struct {
	config         *config.Config
	logger         *logger.Logger
	detector       *privacy.Detector
	vectorSecurity security.VectorSecurityAnalyzer
	router         *mux.Router
	server         *http.Server
	wsHub          *websocket.Hub
	mu             sync.Mutex
	rateLimiters   map[string]*rate.Limiter
}

// New creates a new proxy server instance
func New(cfg *config.Config, log *logger.Logger) (*Server, error) {
	// Create PII detector
	detector, err := privacy.New(cfg.Privacy, log)
	if err != nil {
		return nil, fmt.Errorf("failed to create privacy detector: %w", err)
	}

	// Create vector security engine if enabled
	var vectorSecurity security.VectorSecurityAnalyzer
	if cfg.Security.VectorSecurity.Enabled {
		// Create simple embedding service
		embeddingModelConfig := embeddings.ModelConfig{
			ModelName:    cfg.Security.VectorSecurity.Embedding.Model.ModelName,
			ModelPath:    cfg.Security.VectorSecurity.Embedding.Model.ModelPath,
			CacheDir:     cfg.Security.VectorSecurity.Embedding.Model.CacheDir,
			AutoDownload: cfg.Security.VectorSecurity.Embedding.Model.AutoDownload,
			MaxLength:    cfg.Security.VectorSecurity.Embedding.Model.MaxLength,
			BatchSize:    cfg.Security.VectorSecurity.Embedding.Model.BatchSize,
		}
		var embeddingService embeddings.EmbeddingService
		var err error

		// Create embedding service using factory
		factory := embeddings.NewFactory(log.WithComponent("embeddings-factory").Logger)

		serviceConfig := embeddings.ServiceConfig{
			Type:         embeddings.ServiceType(cfg.Security.VectorSecurity.Embedding.ServiceType),
			ModelConfig:  embeddingModelConfig,
			RedisEnabled: cfg.Security.VectorSecurity.Embedding.RedisEnabled,
			RedisURL:     cfg.Security.VectorSecurity.Embedding.RedisURL,
		}

		// Validate configuration
		if err := embeddings.ValidateServiceConfig(serviceConfig); err != nil {
			log.Error("Invalid embedding service configuration", zap.Error(err))
			log.Info("Falling back to hash embedding service")
			serviceConfig.Type = embeddings.HashEmbedding
			serviceConfig.RedisEnabled = false
		}

		embeddingService, err = factory.CreateService(serviceConfig)
		if err != nil {
			log.Warn("Failed to create embedding service, vector security disabled", zap.Error(err))
		} else {
			// Attempt to initialize vector store and attach to ML embedding service
			if mlService, ok := embeddingService.(*embeddings.MLEmbeddingService); ok {
				dbCfg := &vector.Config{
					DatabaseURL:     cfg.Security.VectorSecurity.Database.DatabaseURL,
					MaxOpenConns:    cfg.Security.VectorSecurity.Database.MaxOpenConns,
					MaxIdleConns:    cfg.Security.VectorSecurity.Database.MaxIdleConns,
					ConnMaxLifetime: cfg.Security.VectorSecurity.Database.ConnMaxLifetime,
					ConnMaxIdleTime: cfg.Security.VectorSecurity.Database.ConnMaxIdleTime,
				}
				store, sErr := vector.NewStore(dbCfg, log.WithComponent("vector-store").Logger)
				if sErr != nil {
					log.Warn("Vector store initialization failed; continuing without DB lookups", zap.Error(sErr))
				} else {
					mlService.SetVectorStore(store)
					log.Info("Vector store attached to ML embedding service")
				}
			}

			vectorSecurity = security.NewSimpleVectorSecurityEngine(
				embeddingService,
				&cfg.Security.VectorSecurity,
				log.WithComponent("vector-security").Logger,
			)
			log.Info("Vector security engine initialized")
		}
	}

	// Create WebSocket hub with configuration
	hubConfig := &websocket.HubConfig{
		BroadcastPIIDetections:  cfg.WebSocket.Events.BroadcastPIIDetections,
		BroadcastVectorSecurity: cfg.WebSocket.Events.BroadcastVectorSecurity,
		BroadcastSystem:         cfg.WebSocket.Events.BroadcastSystem,
		BroadcastConnections:    cfg.WebSocket.Events.BroadcastConnections,
	}
	wsHub := websocket.NewHub(hubConfig, log.WithComponent("websocket").Logger)

	// Create router
	router := mux.NewRouter()

	// Create server
	server := &Server{
		config:         cfg,
		logger:         log.WithComponent("proxy"),
		detector:       detector,
		vectorSecurity: vectorSecurity,
		router:         router,
		wsHub:          wsHub,
		mu:             sync.Mutex{},
		rateLimiters:   make(map[string]*rate.Limiter),
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
	openaiRouter.Use(s.vectorSecurityMiddleware)
	openaiRouter.PathPrefix("/").HandlerFunc(s.handleOpenAIProxy)

	// Ollama proxy endpoints
	ollamaRouter := s.router.PathPrefix("/ollama").Subrouter()
	ollamaRouter.Use(s.loggingMiddleware)
	ollamaRouter.Use(s.privacyMiddleware)
	ollamaRouter.Use(s.vectorSecurityMiddleware)
	ollamaRouter.PathPrefix("/").HandlerFunc(s.handleOllamaProxy)

	// Anthropic proxy endpoints
	anthropicRouter := s.router.PathPrefix("/anthropic").Subrouter()
	anthropicRouter.Use(s.loggingMiddleware)
	anthropicRouter.Use(s.privacyMiddleware)
	anthropicRouter.Use(s.vectorSecurityMiddleware)
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
