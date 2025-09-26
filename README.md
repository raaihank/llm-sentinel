# 🛡️ LLM-Sentinel: Go-Powered AI Security Proxy

A high-performance security proxy for LLM applications. Detect and mask PII, prevent prompt injections, and add real-time monitoring to any AI workflow—with zero code changes.

**Built with Go for maximum performance and minimal footprint.**

## 🚀 Quick Start

### Option 1: Docker Hub (Easiest)
```bash
# Pull and run from Docker Hub
docker run -p 5052:8080 --name llm-sentinel raaihank/llm-sentinel:latest

# Access dashboard
open http://localhost:5052
```

### Option 2: Docker Compose (Recommended for Development)
```bash
# Clone and start with Docker Compose
git clone https://github.com/raaihank/llm-sentinel
cd llm-sentinel
docker-compose up -d

# Access dashboard
open http://localhost:8080
```

### Option 3: Binary Release
```bash
# Download latest release
curl -L https://github.com/raaihank/llm-sentinel/releases/latest/download/sentinel-linux-amd64 -o sentinel
chmod +x sentinel
./sentinel --config configs/default.yaml
```

### Option 4: Build from Source
```bash
git clone https://github.com/raaihank/llm-sentinel
cd llm-sentinel
make build
./bin/sentinel --config configs/default.yaml
```

## ✨ What's New in Go Version

### 🔄 Complete Architecture Transformation
- **Language**: TypeScript → **Go 1.23+**
- **Size**: ~200MB → **13.6MB Docker image**
- **Performance**: **3-5x faster** response times
- **Memory**: ~100MB → **<20MB runtime**
- **Dependencies**: Node.js ecosystem → **Zero runtime dependencies**


## 🎯 Core Features

### 🔒 Data Privacy Protection
- **50+ Sensitvie Data Detectors**: Credit cards, SSNs, emails, API keys, tokens, certificates
- **Smart Context Matching**: Reduces false positives with keyword-aware patterns
- **Deterministic Masking**: Consistent `[MASKED_TYPE]` placeholders
- **Header Scrubbing**: Automatic removal of sensitive headers
- **Real-time Alerts**: Live dashboard notifications

### 🛡️ Security Guardrails
- **Prompt Injection Detection**: Block manipulation attempts
- **OWASP LLM Top 10**: Protection against common AI threats
- **Configurable Thresholds**: Adjust sensitivity per environment
- **Request/Response Logging**: Full audit trail with PII masking

### 🔌 Zero Integration
- **Transparent Proxy**: Drop-in replacement for AI API endpoints
- **Multiple Providers**: OpenAI, Anthropic, Ollama support
- **Streaming Compatible**: Handles SSE and chunked responses
- **Configuration-Only**: No code changes required

## 🏗️ Architecture

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Your App      │───▶│  LLM-Sentinel   │───▶│   AI Provider   │
│                 │    │                 │    │                 │
│ localhost:3000  │    │ localhost:8080  │    │ api.******.com  │
└─────────────────┘    └─────────────────┘    └─────────────────┘
                              │
                              ▼
                       ┌─────────────────┐
                       │   Dashboard     │
                       │ Real-time UI    │
                       │ ws://localhost  │
                       └─────────────────┘
```

## 📋 API Endpoints

### Proxy Endpoints
- `POST /openai/*` → OpenAI API
- `POST /ollama/*` → Ollama API  
- `POST /anthropic/*` → Anthropic API

### Management Endpoints
- `GET /` → Dashboard UI
- `GET /health` → Health check
- `GET /info` → System information
- `WS /ws` → WebSocket for real-time events

## ⚙️ Configuration

### Basic Configuration (`configs/default.yaml`)
```yaml
server:
  port: 8080

privacy:
  enabled: true
  detectors:
    - all  # Enable all 50+ detectors
  masking:
    type: deterministic
    format: "[MASKED_{{TYPE}}]"

upstream:
  openai: https://api.openai.com
  anthropic: https://api.anthropic.com
  ollama: http://localhost:11434

websocket:
  enabled: true
  path: /ws
  events:
    broadcast_detections: true
    broadcast_requests: true
```

### Docker Configuration
For containerized deployments, use `configs/docker.yaml` which includes:
```yaml
upstream:
  ollama: http://host.docker.internal:11434  # Connects to host machine
```

## 🐳 Docker Deployment

### Development
```bash
# Start with Docker Compose
docker-compose up -d

# View logs
docker-compose logs -f

# Stop
docker-compose down
```

## 🔧 Usage Examples

### OpenAI Integration
```bash
# Before (direct to OpenAI)
curl http://localhost:8080/openai/v1/chat/completions \
  -H "Authorization: Bearer $OPENAI_API_KEY" \
  -d '{"model": "gpt-4", "messages": [{"role": "user", "content": "My SSN is 123-45-6789"}]}'

# After (through LLM-Sentinel)
curl http://localhost:8080/openai/v1/chat/completions \
  -H "Authorization: Bearer $OPENAI_API_KEY" \
  -d '{"model": "gpt-4", "messages": [{"role": "user", "content": "My SSN is 123-45-6789"}]}'
# → SSN automatically masked as [MASKED_SSN]
```

### Ollama Integration
```bash
# Start Ollama on host
ollama serve
ollama pull llama2

# Proxy through LLM-Sentinel
curl http://localhost:8080/ollama/api/generate \
  -d '{"model": "llama2", "prompt": "My email is user@company.com", "stream": false}'
# → Email masked as [MASKED_EMAIL]
```

### Real-time Monitoring
```javascript
// Connect to WebSocket for live events
const ws = new WebSocket('ws://localhost:8080/ws');

ws.onopen = () => {
  // Subscribe to PII detection events
  ws.send(JSON.stringify({
    type: 'subscribe',
    data: { events: ['pii_detection', 'request_log'] }
  }));
};

ws.onmessage = (event) => {
  const data = JSON.parse(event.data);
  if (data.type === 'pii_detection') {
    console.log(`🚨 PII detected: ${data.data.total_findings} findings`);
  }
};
```

## 📊 Performance Benchmarks

| Metric | TypeScript Version | Go Version | Improvement |
|--------|-------------------|------------|-------------|
| **Response Time** | ~50ms | ~15ms | **3.3x faster** |
| **Memory Usage** | ~100MB | ~18MB | **5.5x less** |
| **Binary Size** | ~200MB+ | 13.6MB | **15x smaller** |
| **Startup Time** | ~3s | <1s | **3x faster** |
| **Docker Image** | ~200MB | 13.6MB | **15x smaller** |

## 🛠️ Development

### Prerequisites
- Go 1.23+
- Docker & Docker Compose (optional)
- Make

### Build Commands
```bash
# Build binary
make build

# Run tests
make test

# Run locally
make run

# Build Docker image
make docker

# Clean build artifacts
make clean
```

### Project Structure
```
llm-sentinel/
├── cmd/sentinel/          # Main application entry point
├── internal/
│   ├── config/           # Configuration management
│   ├── logger/           # Structured logging
│   ├── privacy/          # PII detection engine
│   ├── proxy/            # HTTP proxy server
│   ├── web/              # Dashboard handler
│   └── websocket/        # Real-time events
├── configs/              # Configuration files
├── web/                  # Dashboard HTML
├── docker-compose.yml    # Development deployment
└── Dockerfile           # Multi-stage Docker build
```

## 🔒 Security Features

### PII Detection Rules (50+ Patterns)
- **Financial**: Credit cards, bank accounts, routing numbers
- **Identity**: SSNs, driver licenses, passports
- **Contact**: Emails, phone numbers, addresses
- **API Keys**: OpenAI, AWS, Google Cloud, GitHub, etc.
- **Certificates**: X.509, SSH keys, PGP keys
- **Database**: Connection strings, Redis URLs
- **Infrastructure**: Kubernetes tokens, Docker registry auth

### Security Hardening
- **Non-root container** execution (UID 65534)
- **Read-only filesystem** with minimal attack surface
- **No shell or package manager** in production image
- **Scratch-based image** with zero vulnerabilities
- **Resource limits** and security policies

## 📈 Monitoring & Observability

### Built-in Metrics
- Request/response times and sizes
- PII detection counts by type
- Error rates and status codes
- WebSocket connection counts
- Memory and CPU usage

### Integration Examples
```bash
# Prometheus metrics (coming soon)
curl http://localhost:8080/metrics

# Health check for monitoring
curl http://localhost:8080/health
# {"status":"healthy","timestamp":"2025-09-24T20:33:34Z"}

# System information
curl http://localhost:8080/info
# {"name":"llm-sentinel","version":"0.1.0","privacy_enabled":true}
```

## 🤝 Contributing

We welcome contributions! Please open an issue or submit a pull request.

### Development Workflow
1. Fork the repository
2. Create a feature branch: `git checkout -b feature/amazing-feature`
3. Make your changes and add tests
4. Run tests: `make test`
5. Commit changes: `git commit -m 'Add amazing feature'`
6. Push to branch: `git push origin feature/amazing-feature`
7. Open a Pull Request

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

---

**Made with ❤️ for the AI community. Secure your LLMs without compromising performance.**
