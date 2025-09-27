package websocket

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"
	"encoding/base64"
)

const (
	// Time allowed to write a message to the peer
	writeWait = 10 * time.Second
	// Time allowed to read the next pong message from the peer
	pongWait = 60 * time.Second
	// Send pings to peer with this period. Must be less than pongWait
	pingPeriod = (pongWait * 9) / 10
	// Maximum message size allowed from peer
	maxMessageSize = 512
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// Allow connections from any origin for now
		// In production, you should validate the origin
		return true
	},
}

// HubConfig contains configuration for the WebSocket hub
type HubConfig struct {
	BroadcastPIIDetections  bool
	BroadcastVectorSecurity bool
	BroadcastSystem         bool
	BroadcastConnections    bool
	WebSocketUsername       string
	WebSocketPassword       string
}

// Hub maintains the set of active clients and broadcasts messages to the clients
type Hub struct {
	// Registered clients
	clients map[*Client]bool

	// Inbound messages from the clients
	broadcast chan Event

	// Register requests from the clients
	register chan *Client

	// Unregister requests from clients
	unregister chan *Client

	// Configuration for event broadcasting
	config *HubConfig

	// Logger
	logger *zap.Logger

	// Mutex for thread-safe operations
	mu sync.RWMutex

	// Statistics
	stats *HubStats
}

// HubStats tracks WebSocket hub statistics
type HubStats struct {
	TotalConnections   int64
	ActiveConnections  int64
	TotalMessages      int64
	TotalBroadcasts    int64
	LastConnectionTime time.Time
	LastDisconnectTime time.Time
	LastBroadcastTime  time.Time
}

// NewHub creates a new WebSocket hub
func NewHub(config *HubConfig, logger *zap.Logger) *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		broadcast:  make(chan Event, 256),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		config:     config,
		logger:     logger,
		stats:      &HubStats{},
	}
}

// Run starts the hub and handles client registration/unregistration and broadcasting
func (h *Hub) Run() {
	h.logger.Info("Starting WebSocket hub", zap.String("component", "websocket"))

	for {
		select {
		case client := <-h.register:
			h.registerClient(client)

		case client := <-h.unregister:
			h.unregisterClient(client)

		case event := <-h.broadcast:
			h.broadcastEvent(event)
		}
	}
}

// registerClient registers a new client
func (h *Hub) registerClient(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.clients[client] = true
	h.stats.TotalConnections++
	h.stats.ActiveConnections++
	h.stats.LastConnectionTime = time.Now()

	h.logger.Info("Client connected",
		zap.String("component", "websocket"),
		zap.String("client_id", client.ID),
		zap.String("client_ip", client.IP),
		zap.Int64("active_connections", h.stats.ActiveConnections),
	)

	// Send connection event to other clients
	connectionEvent := Event{
		Type:      EventTypeConnection,
		Timestamp: time.Now(),
		Data: ConnectionEvent{
			Action:    "connected",
			ClientID:  client.ID,
			ClientIP:  client.IP,
			UserAgent: client.UserAgent,
			Message:   fmt.Sprintf("Client %s connected", client.ID),
		},
	}

	// Broadcast to other clients (not the newly connected one)
	go h.broadcastToOthers(connectionEvent, client)
}

// unregisterClient unregisters a client
func (h *Hub) unregisterClient(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if _, ok := h.clients[client]; ok {
		delete(h.clients, client)
		close(client.Send)
		h.stats.ActiveConnections--
		h.stats.LastDisconnectTime = time.Now()

		h.logger.Info("Client disconnected",
			zap.String("component", "websocket"),
			zap.String("client_id", client.ID),
			zap.String("client_ip", client.IP),
			zap.Int64("active_connections", h.stats.ActiveConnections),
		)

		// Send disconnection event to other clients
		connectionEvent := Event{
			Type:      EventTypeConnection,
			Timestamp: time.Now(),
			Data: ConnectionEvent{
				Action:    "disconnected",
				ClientID:  client.ID,
				ClientIP:  client.IP,
				UserAgent: client.UserAgent,
				Message:   fmt.Sprintf("Client %s disconnected", client.ID),
			},
		}

		// Broadcast to remaining clients
		go h.BroadcastEvent(connectionEvent)
	}
}

// broadcastEvent broadcasts an event to all registered clients
func (h *Hub) broadcastEvent(event Event) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	h.stats.TotalBroadcasts++
	h.stats.LastBroadcastTime = time.Now()

	for client := range h.clients {
		if h.shouldSendToClient(client, event) {
			select {
			case client.Send <- event:
				h.stats.TotalMessages++
			default:
				// Client's send channel is full, close it
				h.logger.Warn("Client send channel full, closing connection",
					zap.String("component", "websocket"),
					zap.String("client_id", client.ID),
				)
				delete(h.clients, client)
				close(client.Send)
				h.stats.ActiveConnections--
			}
		}
	}
}

// broadcastToOthers broadcasts an event to all clients except the specified one
func (h *Hub) broadcastToOthers(event Event, excludeClient *Client) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for client := range h.clients {
		if client != excludeClient && h.shouldSendToClient(client, event) {
			select {
			case client.Send <- event:
				h.stats.TotalMessages++
			default:
				// Client's send channel is full, close it
				delete(h.clients, client)
				close(client.Send)
				h.stats.ActiveConnections--
			}
		}
	}
}

// shouldSendToClient determines if an event should be sent to a specific client based on their subscription
func (h *Hub) shouldSendToClient(client *Client, event Event) bool {
	if client.Subscription == nil {
		// No subscription filter, send all events
		return true
	}

	// Check if client is subscribed to this event type
	subscribed := false
	for _, eventType := range client.Subscription.Events {
		if eventType == event.Type {
			subscribed = true
			break
		}
	}

	if !subscribed {
		return false
	}

	// Apply additional filters if present
	if client.Subscription.Filter != nil {
		return h.applyEventFilter(client.Subscription.Filter, event)
	}

	return true
}

// applyEventFilter applies filtering logic to determine if an event should be sent
func (h *Hub) applyEventFilter(filter *EventFilter, event Event) bool {
	// Add filtering logic as needed for security events
	// - MinSeverity filtering
	// - RuleTypes filtering
	// - IPWhitelist filtering
	// - PathPatterns filtering

	return true
}

// BroadcastEvent sends an event to all connected clients (only if enabled in config)
func (h *Hub) BroadcastEvent(event Event) {
	// Check if this event type should be broadcast based on configuration
	if !h.shouldBroadcastEvent(event.Type) {
		return
	}

	select {
	case h.broadcast <- event:
	default:
		h.logger.Warn("Broadcast channel full, dropping event",
			zap.String("component", "websocket"),
			zap.String("event_type", string(event.Type)),
		)
	}
}

// shouldBroadcastEvent checks if an event type should be broadcast based on configuration
func (h *Hub) shouldBroadcastEvent(eventType EventType) bool {
	if h.config == nil {
		return false
	}

	switch eventType {
	case EventTypePIIDetection:
		return h.config.BroadcastPIIDetections
	case EventTypeVectorSecurity:
		return h.config.BroadcastVectorSecurity
	case EventTypeSystemStatus:
		return h.config.BroadcastSystem
	case EventTypeConnection:
		return h.config.BroadcastConnections
	default:
		return false
	}
}

// HandleWebSocket handles WebSocket connections
func (h *Hub) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	typ, data, err := parseBasicAuth(auth)
	if err != nil || typ != "Basic" {
		http.Error(w, "Invalid auth", http.StatusUnauthorized)
		return
	}
	user, pass, ok := parseCredentials(data)
	if !ok || user != h.config.WebSocketUsername || pass != h.config.WebSocketPassword {
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.logger.Error("Failed to upgrade WebSocket connection",
			zap.String("component", "websocket"),
			zap.Error(err),
		)
		return
	}

	client := &Client{
		ID:          generateClientID(),
		Conn:        conn,
		Send:        make(chan Event, 256),
		ConnectedAt: time.Now(),
		LastPing:    time.Now(),
		IP:          getClientIP(r),
		UserAgent:   r.UserAgent(),
	}

	h.register <- client

	// Start goroutines for handling the client
	go h.handleClientWrite(client)
	go h.handleClientRead(client)
}

// handleClientWrite handles writing messages to the client
func (h *Hub) handleClientWrite(client *Client) {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		if conn, ok := client.Conn.(*websocket.Conn); ok {
			conn.Close()
		}
	}()

	for {
		select {
		case event, channelOk := <-client.Send:
			if conn, ok := client.Conn.(*websocket.Conn); ok {
				conn.SetWriteDeadline(time.Now().Add(writeWait))
				if !channelOk {
					conn.WriteMessage(websocket.CloseMessage, []byte{})
					return
				}

				if err := conn.WriteJSON(event); err != nil {
					h.logger.Error("Failed to write WebSocket message",
						zap.String("component", "websocket"),
						zap.String("client_id", client.ID),
						zap.Error(err),
					)
					return
				}
			}

		case <-ticker.C:
			if conn, ok := client.Conn.(*websocket.Conn); ok {
				conn.SetWriteDeadline(time.Now().Add(writeWait))
				if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
					return
				}
			}
		}
	}
}

// handleClientRead handles reading messages from the client
func (h *Hub) handleClientRead(client *Client) {
	defer func() {
		h.unregister <- client
		if conn, ok := client.Conn.(*websocket.Conn); ok {
			conn.Close()
		}
	}()

	if conn, ok := client.Conn.(*websocket.Conn); ok {
		conn.SetReadLimit(maxMessageSize)
		conn.SetReadDeadline(time.Now().Add(pongWait))
		conn.SetPongHandler(func(string) error {
			client.LastPing = time.Now()
			conn.SetReadDeadline(time.Now().Add(pongWait))
			return nil
		})

		for {
			var msg ClientMessage
			err := conn.ReadJSON(&msg)
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					h.logger.Error("WebSocket error",
						zap.String("component", "websocket"),
						zap.String("client_id", client.ID),
						zap.Error(err),
					)
				}
				break
			}

			h.handleClientMessage(client, msg)
		}
	}
}

// handleClientMessage handles messages received from clients
func (h *Hub) handleClientMessage(client *Client, msg ClientMessage) {
	switch msg.Type {
	case "subscribe":
		if data, ok := msg.Data.(map[string]interface{}); ok {
			jsonData, _ := json.Marshal(data)
			var subscription SubscriptionRequest
			if err := json.Unmarshal(jsonData, &subscription); err == nil {
				client.Subscription = &subscription
				h.logger.Info("Client subscription updated",
					zap.String("component", "websocket"),
					zap.String("client_id", client.ID),
					zap.Any("subscription", subscription),
				)
			}
		}
	case "ping":
		// Respond with pong
		pongEvent := Event{
			Type:      "pong",
			Timestamp: time.Now(),
			Data:      map[string]string{"message": "pong"},
		}
		select {
		case client.Send <- pongEvent:
		default:
		}
	}
}

// GetStats returns current hub statistics
func (h *Hub) GetStats() HubStats {
	h.mu.RLock()
	defer h.mu.RUnlock()

	stats := *h.stats
	stats.ActiveConnections = int64(len(h.clients))
	return stats
}

// generateClientID generates a unique client ID
func generateClientID() string {
	return fmt.Sprintf("client_%d", time.Now().UnixNano())
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

func parseBasicAuth(auth string) (typ string, data string, err error) {
	parts := strings.SplitN(auth, " ", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid auth format")
	}
	return parts[0], parts[1], nil
}

func parseCredentials(data string) (string, string, bool) {
	decoded, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return "", "", false
	}
	parts := strings.SplitN(string(decoded), ":", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	return parts[0], parts[1], true
}
