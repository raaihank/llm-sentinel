# LLM-Sentinel

> A Go-based proxy server that sits between your application and LLM APIs to detect PII and block prompt injection attempts in real-time.

## Features

- **PII Detection & Masking**: Automatically detects and masks 80+ types of sensitive data (emails, SSNs, credit cards, API keys, etc.)
- **Prompt Injection Protection**: Blocks malicious prompts using advanced pattern matching with fuzzy detection
- **Real-time Dashboard**: WebSocket-powered monitoring with live security events and response time tracking
- **Multi-Provider Support**: Works with OpenAI, Anthropic, Ollama, and other LLM APIs
- **Zero Configuration**: Works out of the box with Docker Compose
- **Production Ready**: Preserves authentication headers, configurable security modes, comprehensive logging

## Quick Start

```bash
git clone https://github.com/raaihank/llm-sentinel
cd llm-sentinel
docker-compose up --build
```

Then visit `http://localhost:8080` for the dashboard.

## Usage in Your Application

### Using Provider SDKs (Recommended)

Most LLM provider SDKs support custom base URLs. Simply change the base URL to route through LLM-Sentinel:

#### OpenAI SDK

```python
import openai

# Just change the base URL - everything else stays the same
client = openai.OpenAI(
    api_key="your-openai-api-key",
    base_url="http://localhost:8080/openai/v1"  # Add LLM-Sentinel proxy
)

response = client.chat.completions.create(
    model="gpt-4",
    messages=[{"role": "user", "content": "My email is user@company.com"}]
)
```

#### Ollama SDK

```python
from ollama import Client

client = Client(host="http://localhost:8080/ollama")
response = client.chat(
  model="llama3.1:8b", 
  messages=[
    {"role": "user", "content": "Explain HTTP in simple terms"}
  ]
)
```

#### Anthropic SDK

```python
import anthropic

# Just change the base URL
client = anthropic.Anthropic(
    api_key="your-anthropic-api-key",
    base_url="http://localhost:8080/anthropic"  # Add LLM-Sentinel proxy
)

response = client.messages.create(
    model="claude-3-sonnet-20240229",
    max_tokens=1024,
    messages=[{"role": "user", "content": "Hello"}]
)
```

#### LangChain Integration

```python
from langchain.llms import OpenAI
from langchain.chat_models import ChatOpenAI

# For OpenAI models through LLM-Sentinel
llm = ChatOpenAI(
    openai_api_key="your-openai-api-key",
    openai_api_base="http://localhost:8080/openai/v1"  # Route through proxy
)

# For Ollama models through LLM-Sentinel  
from langchain.llms import Ollama

llm = Ollama(
    model="llama3.1:8b",
    base_url="http://localhost:8080/ollama"  # Route through proxy
)
```

#### Environment Variables (Universal)

```bash
# Set these environment variables to route all SDK calls through LLM-Sentinel
export OPENAI_API_BASE="http://localhost:8080/openai/v1"
export ANTHROPIC_BASE_URL="http://localhost:8080/anthropic"

# Your existing code will automatically use the proxy
python your_existing_app.py
```

### Direct API Calls (Alternative)

If you prefer direct HTTP calls or your language doesn't have an official SDK:

```bash
# OpenAI API call
curl http://localhost:8080/openai/v1/chat/completions \
  -H "Authorization: Bearer $OPENAI_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"model":"gpt-4","messages":[{"role":"user","content":"Hello"}]}'

# Anthropic API call  
curl http://localhost:8080/anthropic/v1/messages \
  -H "x-api-key: $ANTHROPIC_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"model":"claude-3-sonnet-20240229","max_tokens":1024,"messages":[{"role":"user","content":"Hello"}]}'
```

## Configuration

Create or edit `configs/default.yaml`:

```yaml
server:
  port: 8080

privacy:
  enabled: true
  header_scrubbing:
    enabled: true
    preserve_upstream_auth: true  # Keep auth headers for upstream APIs

security:
  enabled: true
  mode: block  # "block", "log", or "passthrough"
  vector_security:
    enabled: true
    block_threshold: 0.70  # 70% confidence threshold
    embedding:
      service_type: "pattern"  # Use pattern matching (production ready)

upstream:
  openai: https://api.openai.com
  ollama: http://localhost:11434
  anthropic: https://api.anthropic.com

websocket:
  events:
    broadcast_pii_detections: true
    broadcast_vector_security: true
    broadcast_system: true
    broadcast_connections: true
```

## Performance Benchmarks

| Dataset | Samples | Threshold | Balanced Accuracy | Precision | Recall | Mean Latency |
|---------|---------|-----------|-------------------|-----------|--------|--------------|
| **Gandalf** | 222 (111 attacks) | 0.70 | **73.9%** | **100.0%** | 47.7% | 10.7ms |
| **Qualifire** | 9,992 (4,996 attacks) | 0.70 | **57.8%** | **100.0%** | 15.6% | 17.5ms |

*Latency measured for blocked requests only (security processing time)*

## Docker Compose Integration

```yaml
services:
  your-app:
    build: .
    environment:
      - OPENAI_API_BASE=http://llm-sentinel:8080/openai
      - OPENAI_API_KEY=${OPENAI_API_KEY}
    depends_on:
      - llm-sentinel
  
  llm-sentinel:
    image: llm-sentinel:latest
    ports:
      - "8080:8080"
    volumes:
      - ./configs:/app/configs
```

## What Gets Protected

### PII Detection

- Email addresses → `[EMAIL_MASKED]`
- SSNs → `[SSN_MASKED]`  
- Credit cards → `[CREDIT_CARD_MASKED]`
- API keys → `[API_KEY_MASKED]`
- Phone numbers → `[PHONE_MASKED]`
- File paths → `[PATH_MASKED]`
- 80+ other sensitive data patterns

### Prompt Injection Blocking

- Instruction manipulation: "ignore all previous instructions"
- Jailbreak attempts: "pretend you are not an AI"
- Information extraction: "reveal your system prompt"
- Obfuscation techniques: "ignor all previus instructons"
- Role manipulation: "you are now a different AI"

## Monitoring & Observability

### Real-time Dashboard

- Visit `http://localhost:8080` for live monitoring
- WebSocket-powered real-time updates
- Security alerts, PII detections, response times
- Request activity logs with status codes

### Structured Logging

```json
{
  "level": "info",
  "timestamp": "2025-09-29T14:50:20.444Z",
  "caller": "proxy/middleware.go:102",
  "msg": "PII detected in request",
  "component": "proxy",
  "request_id": "1759157420441888750",
  "findings_count": 2,
  "findings": [
    {"entityType": "email", "masked": "[EMAIL_MASKED]", "count": 1},
    {"entityType": "userPath", "masked": "[PATH_MASKED]", "count": 1}
  ]
}
```

## Production Deployment

### Docker (Recommended)

```bash
# Use the ONNX-enabled version for better performance
docker-compose -f docker-compose.onnx.yml up -d
```

### Binary Deployment

```bash
# Build for production
go build -o llm-sentinel ./cmd/sentinel

# Run with custom config
./llm-sentinel --config /etc/llm-sentinel/config.yaml
```

### Environment Variables

```bash
export LLM_SENTINEL_PORT=8080
export LLM_SENTINEL_CONFIG_PATH=/etc/llm-sentinel/config.yaml
export OPENAI_API_KEY=your-key-here
```

## Changelog

### 2025-09-29 - Advanced Security Features

- **Fuzzy Pattern Matching**: Detects obfuscated attacks like "ignor all previus instructons"
- **Enhanced Prompt Injection**: Blocks instruction manipulation, jailbreaks, and role hijacking
- **Attack Pattern Recognition**: 90%+ confidence detection with zero false positives
- **Security Benchmarks**: 73.9% accuracy on Gandalf, 57.8% on Qualifire datasets

### 2025-09-28 - Multi-Provider & Authentication

- **Anthropic Claude Support**: Full API integration with proper header handling
- **Authentication Preservation**: Upstream API keys properly forwarded
- **WebSocket Security**: Removed auth barriers for dashboard access
- **Production Configuration**: Configurable security modes and thresholds

### 2025-09-27 - Real-time Monitoring

- **Live Dashboard**: WebSocket-powered monitoring at `http://localhost:8080`
- **Security Alerts**: Real-time PII detections and threat blocking
- **Response Time Tracking**: Accurate latency monitoring for blocked requests
- **Activity Logging**: Request tracking with status codes and processing times

### 2025-09-26 - PII Protection

- **Comprehensive PII Detection**: 80+ patterns for sensitive data
- **Automatic Masking**: Emails, SSNs, credit cards, API keys, file paths
- **Request Sanitization**: PII removed before forwarding to LLM APIs
- **Privacy Compliance**: GDPR/CCPA-ready data protection

### 2025-09-25 - Proxy Infrastructure

- **Multi-Provider Routing**: OpenAI, Ollama, and Anthropic API support
- **Docker Deployment**: Complete containerized setup
- **Configuration Management**: YAML-based settings with environment overrides
- **Structured Logging**: JSON logs with request IDs and component tracking

### 2025-09-24 - Core Platform

- **HTTP Middleware Pipeline**: Rate limiting, logging, and security layers
- **Vector Store Integration**: PostgreSQL with pgvector for embeddings
- **Redis Caching**: High-performance embedding cache with binary storage
- **ETL Pipeline**: Dataset processing and security pattern training


## License

MIT - Use it however you want.
