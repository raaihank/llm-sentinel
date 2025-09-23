<!-- Badges -->
[![Build](https://github.com/raaihank/llm-sentinel/actions/workflows/build.yml/badge.svg?branch=main)](https://github.com/raaihank/llm-sentinel/actions/workflows/build.yml)
[![Test](https://github.com/raaihank/llm-sentinel/actions/workflows/test.yml/badge.svg?branch=main)](https://github.com/raaihank/llm-sentinel/actions/workflows/test.yml)
[![Security](https://github.com/raaihank/llm-sentinel/actions/workflows/security.yml/badge.svg?branch=main)](https://github.com/raaihank/llm-sentinel/actions/workflows/security.yml)

# LLM-Sentinel

> **Privacy-first proxy that automatically detects and masks sensitive data before it reaches AI models without compromising latency.**

## Installation

### Docker

```bash
docker pull raaihank/llm-sentinel:latest
docker run -p 5050:5050 raaihank/llm-sentinel:latest
```
<!-- 
### From Source

```bash
git clone https://github.com/raaihank/llm-sentinel.git
cd llm-sentinel
npm install && npm run build
llmsentinel start
``` -->

## Quick Start

Once running, replace your AI API base URLs:
- **OpenAI**: `http://localhost:5050/openai/v1` (instead of `https://api.openai.com/v1`)
- **Ollama**: `http://localhost:5050/ollama` (instead of `http://localhost:11434`)

## Usage Examples

### OpenAI SDK (Python)

```python
import openai

client = openai.OpenAI(
    api_key="sk-your-key-here",  
    base_url="http://localhost:5050/openai/v1"  # ← Add this line
)

# Your sensitive data is automatically protected
response = client.chat.completions.create(
    model="gpt-3.5-turbo",
    messages=[{
        "role": "user", 
        "content": "My API key is sk-abc123 and config at /Users/john/secrets"
    }]
)
# LLM receives: "My API key is [OPENAI_API_KEY_MASKED] and config at /Users/[USERNAME]/secrets"
```

### OpenAI SDK (JavaScript)

```javascript
import OpenAI from 'openai';

const openai = new OpenAI({
  apiKey: 'sk-your-key-here',
  baseURL: 'http://localhost:5050/openai/v1'  // ← Add this line
});

const response = await openai.chat.completions.create({
  model: 'gpt-3.5-turbo',
  messages: [{
    role: 'user',
    content: 'My AWS key is AKIAIOSFODNN7EXAMPLE and email user@company.com'
  }]
});
// LLM receives: "My AWS key is [AWS_ACCESS_KEY_MASKED] and email [EMAIL_MASKED]"
```

**Streaming Support:** All streaming requests are automatically supported - just add `stream: true` to your requests and LLM-Sentinel will mask sensitive data in real-time.

### Ollama SDK

```python
import ollama

client = ollama.Client(host='http://localhost:5050/ollama')  # ← Change this line

response = client.chat(
    model='llama2',
    messages=[{
        'role': 'user', 
        'content': 'My credit card is 4532-1234-5678-9012'
    }]
)
# LLM receives: "My credit card is [CREDIT_CARD_MASKED]"
```

**Streaming Support:** Ollama streaming requests work seamlessly - use `stream=True` in your client calls and all sensitive data will be masked in real-time.

### cURL Examples

**OpenAI:**
```bash
curl -X POST http://localhost:5050/openai/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer sk-your-key" \
  -d '{
    "model": "gpt-3.5-turbo",
    "messages": [{"role": "user", "content": "My SSH key is ssh-rsa AAAAB3..."}]
  }'
```

**Ollama:**
```bash
curl -X POST http://localhost:5050/ollama/api/generate \
  -H "Content-Type: application/json" \
  -d '{
    "model": "llama2",
    "prompt": "My database URL is postgresql://user:pass@localhost/db"
  }'
```

## What Gets Protected (52 Detectors)

- **🤖 AI/ML Services**: OpenAI, Claude, Google AI, Azure OpenAI, Cohere, HuggingFace
- **☁️ Cloud**: AWS keys, Azure subscriptions, GCP credentials, Heroku, Cloudflare
- **🔧 Development**: GitHub tokens, NPM tokens, PyPI, Docker Hub
- **💬 Services**: Slack, Discord, Twilio, SendGrid, Mailgun, Stripe
- **🗄️ Databases**: PostgreSQL, MongoDB, MySQL, Redis, Elasticsearch URLs
- **🔐 Security**: SSH keys, JWT tokens, Kubernetes tokens, PGP keys
- **👤 Personal**: Emails, phone numbers, credit cards, SSNs, IP addresses

## Commands

### Server Management
```bash
llmsentinel start [-p 5050] [-d]   # Start server (daemon with -d)
llmsentinel status                 # Check if running
llmsentinel stop                   # Stop daemon
llmsentinel restart                # Restart server
llmsentinel logs [-n 50]           # View logs
```

### Configuration
```bash
llmsentinel info                   # Show protection status
llmsentinel port 8080              # Change server port
llmsentinel rules                  # List all detectors
llmsentinel protect                # Enable all protection (default)
llmsentinel no-protect             # ⚠️ Disable all protection
```

### Fine-tuned Control
```bash
llmsentinel rules:disable email    # Disable email detection
llmsentinel rules:enable openaiApiKey # Enable OpenAI key detection
llmsentinel debug                  # Show detected entity types in logs
llmsentinel no-debug               # Hide sensitive details (default)
llmsentinel notifications          # Toggle desktop alerts
```

## Docker Usage

### Basic Run
```bash
docker run -p 5050:5050 raaihank/llm-sentinel:latest
```

### With Custom Configuration (Optional)
```bash
# Copy sample config (optional)
cp config.sample.json config.json
nano config.json

# Run with custom config
docker run -d \
  --name llm-sentinel \
  -p 5050:5050 \
  -v $(pwd)/config.json:/app/.llm-sentinel/config.json \
  -v $(pwd)/logs:/app/logs \
  raaihank/llm-sentinel:latest
```

### Docker Commands
```bash
docker logs llm-sentinel                    # View logs
docker exec -it llm-sentinel llmsentinel   # Run commands
docker stop llm-sentinel                   # Stop
docker restart llm-sentinel                # Restart
```

## Real-Time Dashboard 🔥

The dashboard provides beautiful monitoring and control interface:

### 📊 Dashboard Access
When using the npm package, the dashboard is automatically served at the same port as the proxy:
```bash
npm install -g llm-sentinel
llmsentinel start

# Dashboard automatically available at:
http://localhost:5050
```

### ✨ Features:
- **Real-time WebSocket monitoring** - Live updates of intercepted requests
- **Detailed event inspection** - Click on any event to see:
  - Complete request/response data
  - Headers (with API keys redacted)
  - Original vs masked content comparison
  - Processing logs and timing
  - Provider identification (OpenAI, Ollama, Claude, etc.)
- **OLED dark mode** - Battery-friendly pure black interface
- **Interactive controls** - View all 52 detectors, toggle settings
- **Complete configuration** - See all settings with CLI examples
- **Horizontal scrolling** - All JSON/logs properly readable

### 🎯 What You'll See:
- Live detection events as they happen
- Color-coded provider tags (OpenAI, Ollama, etc.)
- Processing time metrics
- Detection counts and statistics
- Masked sensitive data previews

### 📱 Docker Dashboard Access:
```bash
# Run with dashboard
docker run -p 5050:5050 raaihank/llm-sentinel:latest

# Dashboard available at:
http://localhost:5050
```

The dashboard automatically updates in real-time as your applications make AI API calls through LLM-Sentinel!

## Health Check

```bash
curl http://localhost:5050/health
# {"status":"healthy","uptime":3600}
```

## How It Works

```mermaid
graph LR
    A[Your App] -->|HTTP Request| B[LLM-Sentinel]
    B -->|Clean Request| C[AI Model]
    C -->|Response| B
    B -->|Response| A
    
    B --> D[52 Detectors]
    D --> E[API Keys]
    D --> F[Credentials] 
    D --> G[Personal Data]
    D --> H[Private Keys]
    
    style B fill:#e1f5fe
    style D fill:#f3e5f5
    style E fill:#fff3e0
    style F fill:#fff3e0
    style G fill:#fff3e0
    style H fill:#fff3e0
```

1. **Intercepts** requests to AI APIs
2. **Scans** content with 52 specialized detectors  
3. **Masks** sensitive data with safe placeholders
4. **Forwards** clean requests to AI models
5. **Logs** detections (secure by default)

## Configuration

LLM-Sentinel works out-of-the-box with secure defaults. Configuration is optional.

**Config file location:** `~/.llm-sentinel/config.json`

**Sample configuration:**
```json
{
  "server": { 
    "port": 5050,
    "openaiTarget": "https://api.openai.com",
    "ollamaTarget": "http://localhost:11434"
  },
  "detection": { 
    "enabled": true,
    "enabledRules": ["email", "openaiApiKey", "awsAccessKey"],
    "customRules": []
  },
  "logging": {
    "showDetectedEntity": false,
    "logLevel": "INFO",
    "logToConsole": true,
    "logToFile": true
  },
  "notifications": { 
    "enabled": true,
    "sound": false
  },
  "security": {
    "redactApiKeys": true,
    "redactCustomHeaders": ["x-api-key"]
  }
}
```

## Security Features

- ✅ **50+ specialized detectors** for comprehensive coverage
- ✅ **Streaming support** - works with real-time streaming requests
- ✅ **Privacy-first logging** - sensitive data never stored
- ✅ **Zero data retention** - proxy only, no storage
- ✅ **Configurable detection** - enable/disable specific types
- ✅ **Desktop notifications** - real-time detection alerts
- ✅ **API key redaction** - request headers cleaned in logs

## Links

- 📖 **[Development Guide](docs/DEVELOPMENT.md)** - Development setup, architecture, dashboard customization, adding detectors
- 🤝 **[Contributing](docs/CONTRIBUTING.md)** - How to contribute, code style, testing
- 🚀 **[Docker Hub](https://hub.docker.com/r/raaihank/llm-sentinel)** - Pre-built images
- 🐛 **[Issues](https://github.com/raaihank/llm-sentinel/issues)** - Bug reports and feature requests

## License

**Custom License** - Free for personal and non-commercial use. Commercial use requires explicit consent.

See [LICENSE](LICENSE) for full terms.

---

**🛡️ Protect your sensitive data. Enable all 52 detectors by default.**
