"use strict";
var __createBinding = (this && this.__createBinding) || (Object.create ? (function(o, m, k, k2) {
    if (k2 === undefined) k2 = k;
    var desc = Object.getOwnPropertyDescriptor(m, k);
    if (!desc || ("get" in desc ? !m.__esModule : desc.writable || desc.configurable)) {
      desc = { enumerable: true, get: function() { return m[k]; } };
    }
    Object.defineProperty(o, k2, desc);
}) : (function(o, m, k, k2) {
    if (k2 === undefined) k2 = k;
    o[k2] = m[k];
}));
var __setModuleDefault = (this && this.__setModuleDefault) || (Object.create ? (function(o, v) {
    Object.defineProperty(o, "default", { enumerable: true, value: v });
}) : function(o, v) {
    o["default"] = v;
});
var __importStar = (this && this.__importStar) || (function () {
    var ownKeys = function(o) {
        ownKeys = Object.getOwnPropertyNames || function (o) {
            var ar = [];
            for (var k in o) if (Object.prototype.hasOwnProperty.call(o, k)) ar[ar.length] = k;
            return ar;
        };
        return ownKeys(o);
    };
    return function (mod) {
        if (mod && mod.__esModule) return mod;
        var result = {};
        if (mod != null) for (var k = ownKeys(mod), i = 0; i < k.length; i++) if (k[i] !== "default") __createBinding(result, mod, k[i]);
        __setModuleDefault(result, mod);
        return result;
    };
})();
var __importDefault = (this && this.__importDefault) || function (mod) {
    return (mod && mod.__esModule) ? mod : { "default": mod };
};
Object.defineProperty(exports, "__esModule", { value: true });
exports.ProxyServer = void 0;
const express_1 = __importDefault(require("express"));
const http_proxy_middleware_1 = require("http-proxy-middleware");
const node_notifier_1 = __importDefault(require("node-notifier"));
const path = __importStar(require("path"));
const fs = __importStar(require("fs"));
const WebSocket = require('ws');
const { Server: WebSocketServer } = require('ws');
const detectors_1 = require("./detectors");
const logger_1 = require("./logger");
const config_1 = require("./config");
class ProxyServer {
    app;
    server;
    wss;
    config;
    appConfig;
    configManager;
    sensitiveDataDetector;
    logger;
    activeConnections = new Set();
    requestMetrics = new Map();
    constructor(config) {
        this.config = config;
        this.configManager = new config_1.ConfigManager();
        this.appConfig = this.configManager.getConfig();
        this.app = (0, express_1.default)();
        this.sensitiveDataDetector = new detectors_1.SensitiveDataDetector();
        this.logger = new logger_1.Logger(this.appConfig.logging.logToConsole, this.appConfig.logging.logToFile);
        this.setupProxies();
    }
    broadcastToClients(data) {
        const message = JSON.stringify(data);
        this.activeConnections.forEach((ws) => {
            if (ws.readyState === WebSocket.OPEN) {
                ws.send(message);
            }
        });
    }
    generateRequestId() {
        return Math.random().toString(36).substr(2, 9);
    }
    onProxyReqHandler = (proxyReq, req, _res) => {
        const requestId = this.generateRequestId();
        const startTime = Date.now();
        // Store request in req object for later use
        req.requestId = requestId;
        req.startTime = startTime;
        // Get the raw body if it exists
        if (req.body) {
            let bodyData = req.body;
            // Convert to string if it's a Buffer
            if (Buffer.isBuffer(bodyData)) {
                bodyData = bodyData.toString();
            }
            else if (typeof bodyData === 'object') {
                bodyData = JSON.stringify(bodyData);
            }
            try {
                // Only mask if PII detection is enabled
                let maskedBody = bodyData;
                let findings = [];
                if (this.appConfig.detection.enabled) {
                    [maskedBody, findings] = this.sensitiveDataDetector.mask(bodyData);
                }
                // Filter sensitive headers for storage
                const sanitizedHeaders = { ...req.headers };
                if (this.appConfig.security.redactApiKeys) {
                    if (sanitizedHeaders.authorization) {
                        sanitizedHeaders.authorization = '[REDACTED]';
                    }
                    this.appConfig.security.redactCustomHeaders.forEach(header => {
                        const lowerHeader = header.toLowerCase();
                        if (sanitizedHeaders[lowerHeader]) {
                            sanitizedHeaders[lowerHeader] = '[REDACTED]';
                        }
                    });
                }
                // Create request metrics with full data
                const metrics = {
                    id: requestId,
                    timestamp: new Date().toISOString(),
                    endpoint: req.path || req.url,
                    method: req.method,
                    processingTimeMs: Date.now() - startTime,
                    detections: findings,
                    contentLength: bodyData.length,
                    status: 'processing',
                    requestHeaders: sanitizedHeaders,
                    requestBody: bodyData,
                    maskedRequestBody: maskedBody,
                    logs: [{
                            timestamp: new Date().toISOString(),
                            level: 'INFO',
                            message: 'Request started',
                            data: {
                                url: req.url,
                                method: req.method,
                                contentLength: bodyData.length,
                                detectionsFound: findings.length
                            }
                        }]
                };
                this.requestMetrics.set(requestId, metrics);
                // Broadcast comprehensive request event with detection data
                this.broadcastToClients({
                    type: 'request_event',
                    data: {
                        id: requestId,
                        timestamp: metrics.timestamp,
                        endpoint: metrics.endpoint,
                        method: metrics.method,
                        status: 'processing',
                        hasDetections: findings.length > 0,
                        detections: findings,
                        requestHeaders: sanitizedHeaders,
                        originalRequestBody: bodyData,
                        maskedRequestBody: maskedBody,
                        processingTimeMs: metrics.processingTimeMs,
                        logs: metrics.logs
                    }
                });
                // Log complete request processing information
                this.logger.log("Request processed", {
                    request: {
                        url: req.url,
                        method: req.method,
                        headers: sanitizedHeaders,
                        originalContentLength: bodyData.length
                    },
                    sensitiveDataDetection: {
                        enabled: this.appConfig.detection.enabled,
                        sensitiveDataFound: findings.length > 0,
                        totalEntityTypes: findings.length,
                        detectedEntities: findings.map(f => ({
                            entityType: this.appConfig.logging.showDetectedEntity ? f.entityType : '[ENTITY_TYPE_HIDDEN]',
                            maskedAs: this.appConfig.logging.showMaskedValue ? f.masked : '[MASKED_VALUE_HIDDEN]',
                            occurrences: this.appConfig.logging.showOccurrenceCount ? f.count : undefined
                        })).filter(entity => entity.occurrences !== undefined || !this.appConfig.logging.showOccurrenceCount)
                    },
                    llmPayload: {
                        maskedContentLength: maskedBody.length,
                        contentReduced: bodyData.length > maskedBody.length,
                        bytesReduced: bodyData.length - maskedBody.length,
                        maskedContent: this.appConfig.logging.truncatePayload && maskedBody.length > this.appConfig.logging.maxPayloadLogLength
                            ? maskedBody.substring(0, this.appConfig.logging.maxPayloadLogLength) + "...[truncated]"
                            : maskedBody
                    }
                });
                // Send notifications for findings based on config
                if (this.appConfig.notifications.enabled && findings.length > 0) {
                    findings.forEach(finding => {
                        let message = '';
                        if (this.appConfig.notifications.showEntityType) {
                            message += finding.entityType;
                        }
                        else {
                            message += 'Sensitive data';
                        }
                        if (this.appConfig.notifications.showCount && finding.count > 1) {
                            message += ` (${finding.count} occurrences)`;
                        }
                        message += ` → ${finding.masked}`;
                        this.notifySensitiveData(message);
                    });
                }
                // Write the masked body
                const bodyBuffer = Buffer.from(maskedBody);
                proxyReq.setHeader('Content-Length', bodyBuffer.length);
                proxyReq.write(bodyBuffer);
                proxyReq.end();
            }
            catch (e) {
                this.logger.error('Error processing request body:', e);
                // Still forward the original body if masking fails
                (0, http_proxy_middleware_1.fixRequestBody)(proxyReq, req);
            }
        }
        else {
            // No body to process, use default handler
            (0, http_proxy_middleware_1.fixRequestBody)(proxyReq, req);
        }
    };
    onProxyResHandler = (proxyRes, req, _res) => {
        const requestId = req.requestId;
        const startTime = req.startTime;
        if (requestId && startTime) {
            const totalTime = Date.now() - startTime;
            const metrics = this.requestMetrics.get(requestId);
            if (metrics) {
                // Capture response data
                let responseBody = '';
                const chunks = [];
                proxyRes.on('data', (chunk) => {
                    chunks.push(chunk);
                });
                proxyRes.on('end', () => {
                    responseBody = Buffer.concat(chunks).toString();
                    // Update metrics with response data
                    metrics.status = 'completed';
                    metrics.apiResponseTimeMs = totalTime - metrics.processingTimeMs;
                    metrics.responseStatus = proxyRes.statusCode || 200;
                    metrics.responseHeaders = proxyRes.headers;
                    metrics.responseBody = responseBody;
                    // Add completion log
                    metrics.logs?.push({
                        timestamp: new Date().toISOString(),
                        level: 'INFO',
                        message: 'Response received',
                        data: {
                            statusCode: proxyRes.statusCode,
                            responseSize: responseBody.length,
                            totalTimeMs: totalTime,
                            apiResponseTimeMs: metrics.apiResponseTimeMs
                        }
                    });
                    // Broadcast updated comprehensive event
                    this.broadcastToClients({
                        type: 'request_event',
                        data: {
                            id: requestId,
                            timestamp: metrics.timestamp,
                            endpoint: metrics.endpoint,
                            method: metrics.method,
                            status: 'completed',
                            hasDetections: metrics.detections.length > 0,
                            detections: metrics.detections,
                            requestHeaders: metrics.requestHeaders,
                            originalRequestBody: metrics.requestBody,
                            maskedRequestBody: metrics.maskedRequestBody,
                            responseHeaders: metrics.responseHeaders,
                            responseBody: metrics.responseBody,
                            responseStatus: metrics.responseStatus,
                            processingTimeMs: metrics.processingTimeMs,
                            apiResponseTimeMs: metrics.apiResponseTimeMs,
                            totalTimeMs: totalTime,
                            logs: metrics.logs
                        }
                    });
                });
            }
        }
    };
    notifySensitiveData(finding) {
        const title = 'Sensitive Data Masked';
        const message = `LLM-Sentinel masked: ${finding}`;
        node_notifier_1.default.notify({
            title: title,
            message: message,
            sound: this.appConfig.notifications.sound,
            wait: false
        });
        if (this.appConfig.logging.logLevel === 'DEBUG') {
            this.logger.log('Notification sent', { masked: true });
        }
    }
    setupProxies() {
        // Add body parsing middleware
        this.app.use(express_1.default.raw({ type: '*/*', limit: '10mb' }));
        // Health check endpoint
        this.app.get('/health', (_req, res) => {
            res.json({ status: 'healthy', uptime: process.uptime() });
        });
        // WebSocket statistics endpoint
        this.app.get('/api/stats', (_req, res) => {
            const recentMetrics = Array.from(this.requestMetrics.values())
                .sort((a, b) => new Date(b.timestamp).getTime() - new Date(a.timestamp).getTime())
                .slice(0, 10);
            res.json({
                totalRequests: this.requestMetrics.size,
                activeConnections: this.activeConnections.size,
                recentMetrics
            });
        });
        // Detailed request information endpoint
        this.app.get('/api/request/:id', (req, res) => {
            const requestId = req.params.id;
            const metrics = this.requestMetrics.get(requestId);
            if (!metrics) {
                res.status(404).json({ error: 'Request not found' });
                return;
            }
            res.json({
                ...metrics,
                // Ensure response body is truncated if too large for display
                responseBody: metrics.responseBody && metrics.responseBody.length > 10000
                    ? metrics.responseBody.substring(0, 10000) + '\n\n[Response truncated - full response was ' + metrics.responseBody.length + ' characters]'
                    : metrics.responseBody
            });
        });
        // Configuration endpoint - read-only view
        this.app.get('/api/config', (_req, res) => {
            res.json({
                config: this.appConfig,
                note: "To modify these settings, use the 'llmsentinel' command line tool",
                examples: {
                    "Enable debug mode": "llmsentinel debug",
                    "Disable all protection": "llmsentinel no-protect",
                    "Change server port": "llmsentinel port 8080",
                    "Disable email detection": "llmsentinel rules:disable email"
                }
            });
        });
        // Dashboard static files - serve the built Next.js dashboard at root
        // Try multiple possible paths for dashboard files
        let dashboardPath = path.join(__dirname, 'dashboard');
        // Check if dashboard exists at the default path, if not try alternate paths
        if (!fs.existsSync(dashboardPath)) {
            // Try relative to the package root
            const packageRoot = path.dirname(__dirname);
            dashboardPath = path.join(packageRoot, 'dashboard');
            if (!fs.existsSync(dashboardPath)) {
                // Try in the same directory as the executable
                dashboardPath = path.join(__dirname, '..', 'dashboard');
            }
        }
        // Serve dashboard at root
        this.app.get('/', (_req, res) => {
            const indexPath = path.join(dashboardPath, 'index.html');
            if (fs.existsSync(indexPath)) {
                // Use absolute path for Express sendFile
                const absolutePath = path.resolve(indexPath);
                res.sendFile(absolutePath);
            }
            else {
                // Dashboard not built - serve development message
                res.send(`
          <!DOCTYPE html>
          <html>
            <head>
              <title>LLM-Sentinel</title>
              <style>
                body { font-family: system-ui; margin: 40px; background: #000; color: #fff; }
                .container { max-width: 600px; margin: 0 auto; text-align: center; }
                .status { color: #10b981; margin: 20px 0; }
                .endpoints { text-align: left; background: #111; padding: 20px; border-radius: 8px; margin: 20px 0; }
                .endpoint { margin: 10px 0; font-family: monospace; }
                .note { color: #94a3b8; font-size: 14px; }
              </style>
            </head>
            <body>
              <div class="container">
                <h1>🛡️ LLM-Sentinel</h1>
                <div class="status">✅ Server Running</div>
                <p>Privacy-first proxy for AI APIs</p>

                <div class="endpoints">
                  <h3>Available Endpoints:</h3>
                  <div class="endpoint">📊 <strong>/health</strong> - Health check</div>
                  <div class="endpoint">📈 <strong>/api/stats</strong> - Statistics</div>
                  <div class="endpoint">🔧 <strong>/api/config</strong> - Configuration</div>
                  <div class="endpoint">🤖 <strong>/openai/*</strong> - OpenAI proxy</div>
                  <div class="endpoint">🦙 <strong>/ollama/*</strong> - Ollama proxy</div>
                </div>

                <div class="note">
                  <p><strong>Dashboard:</strong> Run <code>npm run build</code> to build the full dashboard interface.</p>
                  <p><strong>Development:</strong> Use <code>npm run dev:dashboard</code> for dashboard development.</p>
                </div>
              </div>
            </body>
          </html>
        `);
            }
        });
        // Serve static dashboard assets
        this.app.use(express_1.default.static(dashboardPath));
        // Proxy for OpenAI
        const openaiOptions = {
            target: this.config.openaiTarget,
            changeOrigin: true,
            selfHandleResponse: false,
            on: {
                proxyReq: this.onProxyReqHandler,
                proxyRes: this.onProxyResHandler
            },
            pathRewrite: {
                '^/openai': '',
            },
            logger: console,
            onError: (err, _req, res) => {
                this.logger.error('OpenAI proxy error:', err);
                if (res && typeof res.writeHead === 'function') {
                    res.writeHead(500, { 'Content-Type': 'text/plain' });
                    res.end('Proxy error occurred');
                }
            }
        };
        // Proxy for Ollama
        const ollamaOptions = {
            target: this.config.ollamaTarget,
            changeOrigin: true,
            selfHandleResponse: false,
            on: {
                proxyReq: this.onProxyReqHandler,
                proxyRes: this.onProxyResHandler
            },
            pathRewrite: {
                '^/ollama': '',
            },
            logger: console,
            onError: (err, _req, res) => {
                this.logger.error('Ollama proxy error:', err);
                if (res && typeof res.writeHead === 'function') {
                    res.writeHead(500, { 'Content-Type': 'text/plain' });
                    res.end('Proxy error occurred');
                }
            }
        };
        this.app.use('/openai', (0, http_proxy_middleware_1.createProxyMiddleware)(openaiOptions));
        this.app.use('/ollama', (0, http_proxy_middleware_1.createProxyMiddleware)(ollamaOptions));
    }
    start() {
        return new Promise((resolve, reject) => {
            try {
                this.server = this.app.listen(this.config.port, () => {
                    this.logger.log(`LLM-Sentinel is running on http://localhost:${this.config.port}`);
                    this.logger.log(`-> Forwarding /openai requests to ${this.config.openaiTarget}`);
                    this.logger.log(`-> Forwarding /ollama requests to ${this.config.ollamaTarget}`);
                    // Setup WebSocket server
                    this.setupWebSocket();
                    resolve();
                });
                this.server.on('error', (error) => {
                    this.logger.error('Server error:', error);
                    reject(error);
                });
            }
            catch (error) {
                reject(error);
            }
        });
    }
    setupWebSocket() {
        if (!this.server)
            return;
        this.wss = new WebSocketServer({ server: this.server, path: '/ws' });
        this.wss.on('connection', (ws) => {
            this.activeConnections.add(ws);
            this.logger.log('WebSocket client connected');
            // Send initial stats
            ws.send(JSON.stringify({
                type: 'stats',
                data: {
                    totalRequests: this.requestMetrics.size,
                    activeConnections: this.activeConnections.size,
                    uptime: process.uptime()
                }
            }));
            ws.on('close', () => {
                this.activeConnections.delete(ws);
                this.logger.log('WebSocket client disconnected');
            });
            ws.on('error', (error) => {
                this.logger.error('WebSocket error:', error);
                this.activeConnections.delete(ws);
            });
        });
    }
    stop() {
        return new Promise((resolve) => {
            if (this.server) {
                this.server.close(() => {
                    this.logger.log('LLM-Sentinel server stopped');
                    resolve();
                });
            }
            else {
                resolve();
            }
        });
    }
}
exports.ProxyServer = ProxyServer;
//# sourceMappingURL=proxy-server.js.map