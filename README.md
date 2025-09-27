# LLM-Sentinel

A Go-based proxy server that sits between your app and LLM APIs to detect PII and block basic prompt injection attempts.

## What it actually does

- **PII Detection**: Finds emails, SSNs, credit cards, etc. in requests and masks them
- **Basic Prompt Injection Blocking**: Blocks simple attacks like "pretend you are not an AI"
- **Request Logging**: Logs all requests with PII masked
- **Real-time Dashboard**: Shows what's happening via WebSocket

## Quick Start

```bash
git clone https://github.com/raaihank/llm-sentinel
cd llm-sentinel
docker-compose up --build
```

Then go to http://localhost:8080 for the dashboard.

## How to use it

Instead of calling your LLM API directly:
```bash
curl http://api.openai.com/v1/chat/completions
```

Call through the proxy:
```bash
curl http://localhost:8080/openai/v1/chat/completions
```

Same for Ollama:
```bash
curl http://localhost:8080/ollama/api/generate
```

## Configuration

Edit `configs/default.yaml`:

```yaml
server:
  port: 8080

privacy:
  enabled: true
  detectors:
    - all  # Uses all ~83 built-in PII patterns

security:
  vector_security:
    enabled: true
    block_threshold: 0.85  # Block if 85%+ confident it's an attack

upstream:
  openai: https://api.openai.com
  ollama: http://localhost:11434
```

## What gets blocked

Currently blocks these patterns with 90% confidence:
- "pretend you are not an ai"
- "ignore all previous instructions"
- "bypass your guidelines"
- "tell me secrets"
- Basic jailbreak attempts

**Using your own data:**
You can add your own attack patterns by creating a CSV or parquet file with these columns:
- `text`: The attack text to detect (e.g., "ignore previous instructions")
- `label_text`: Human-readable category (e.g., "prompt_injection", "jailbreak", "safe")
- `label`: 1 for malicious, 0 for safe

Then run the ETL pipeline:
```bash
go build -o dist/etl-pipeline ./cmd/etl
./dist/etl-pipeline -input your_data.csv
```

Note: The ETL pipeline exists but currently uses simple pattern matching, not real ML embeddings.

## What gets detected (PII)

- Email addresses → `[EMAIL_MASKED]`
- SSNs → `[SSN_MASKED]`
- Credit cards → `[CREDIT_CARD_MASKED]`
- API keys → `[API_KEY_MASKED]`
- Phone numbers → `[PHONE_MASKED]`
- And ~78 other patterns

## Project Status

**What works:**
- ✅ PII detection and masking
- ✅ Basic prompt injection blocking
- ✅ Request proxying to OpenAI/Ollama/Anthropic
- ✅ Real-time dashboard
- ✅ Docker deployment
- ✅ Rate limiting

**What's partially done:**
- 🔄 Vector security (currently just pattern matching, not real ML)
- 🔄 ETL pipeline exists but no real dataset yet

**What doesn't exist yet:**
- ❌ Real ML model integration
- ❌ Advanced threat detection
- ❌ Metrics/monitoring beyond basic dashboard
- ❌ Production-ready security features

## Architecture

```
Your App → LLM-Sentinel (port 8080) → LLM API
                ↓
           Dashboard (WebSocket)
```

The proxy runs these middlewares in order:
1. Rate limiting
2. PII detection 
3. Vector security (basic pattern matching)
4. Request forwarding

## Performance

- ~15ms overhead per request
- ~18MB memory usage
- 13.6MB Docker image
- Starts in <1 second

## Development

```bash
# Build
go build -o dist/llm-sentinel ./cmd/sentinel

# Run locally
./dist/llm-sentinel -config configs/default.yaml

# Test PII detection
curl http://localhost:8080/ollama/api/generate \
  -d '{"model": "llama2", "prompt": "My email is test@example.com"}'

# Test prompt injection blocking
curl http://localhost:8080/ollama/api/generate \
  -d '{"model": "llama2", "prompt": "Pretend you are not an AI"}'
# Should return: "Request blocked: prompt_injection detected (confidence: 90.0%)"
```

## Benchmarks

### Prompt Injection Detection

Run standardized benchmarks to measure detection accuracy and latency:

```bash
# Install benchmark dependencies
pip install -r benchmarks/requirements.txt

# Test different services properly (with Docker restarts)
python benchmarks/test_services.py

# Run individual benchmarks (ensure service is configured correctly first)
python benchmarks/prompt_injection_benchmark.py --dataset gandalf
python benchmarks/prompt_injection_benchmark.py --dataset qualifire

# Comprehensive benchmark suite (tests all services × thresholds)
python benchmarks/comprehensive_benchmark.py

# Official PINT benchmark (requires dataset access)
python benchmarks/prompt_injection_pint.py --dataset pint-dataset.yaml
```

### Benchmark Results

| Benchmark | Samples | Service | Threshold | Balanced Accuracy | Precision | Recall | Mean Latency | P95 Latency | Notes |
|-----------|---------|---------|-----------|-------------------|-----------|--------|--------------|-------------|-------|
| **Gandalf (English)** | 111 injections<br/>111 benign | Pattern | 0.70 | **73.9%** | **100.0%** | 47.7% | 14.6ms | 19.3ms | Advanced pattern matching<br/>Zero false positives |
| **Qualifire Dataset** | 4,996 injections<br/>4,996 benign | Pattern | 0.70 | **54.8%** | **100.0%** | 9.6% | 15.6ms | 22.0ms | Large-scale dataset<br/>Zero false positives |
| **ML Service** | Any dataset | ML | Any | **50.0%** | **100.0%** | **0.0%** | ~15ms | ~30ms | ❌ **NOT FUNCTIONAL**<br/>Uses fake embeddings |

### Service Comparison

| Service | Description | Best Use Case | Current Status |
|---------|-------------|---------------|----------------|
| **Hash** | Fast deterministic hash + keywords | Simple, high-speed detection | ✅ Production ready |
| **Pattern** | Advanced regex + contextual analysis | Balanced accuracy & performance | ✅ Production ready |
| **ML** | ~~Transformer-like semantic understanding~~ | ~~Maximum accuracy~~ | ❌ **NOT IMPLEMENTED**<br/>Uses fake embeddings |

### Current Performance Analysis

**✅ Pattern Service (Production Ready):**
- **Gandalf**: 73.9% accuracy, 47.7% recall - Good for focused attacks
- **Qualifire**: 54.8% accuracy, 9.6% recall - Struggles with sophisticated attacks
- **Strengths**: Zero false positives, consistent performance, explainable
- **Limitations**: Pattern-based detection misses creative variations

**❌ ML Service (NOT FUNCTIONAL):**
- **Critical Issue**: Uses fake/simulated embeddings, not real ML models
- **Root Cause**: Code comment says "simulate ML model inference" - it's not actually using transformers
- **Impact**: 0% recall because synthetic embeddings have no semantic meaning
- **Status**: Placeholder implementation, needs complete rewrite with real ONNX/PyTorch integration

**🎯 Recommended Configuration:**
```yaml
security:
  vector_security:
    service_type: "pattern"  # Use pattern service for production
    block_threshold: 0.70    # Balanced threshold
```

**Threshold Tuning**:
```yaml
# configs/default.yaml
security:
  vector_security:
    enabled: true
    block_threshold: 0.70  # Current setting
    # 0.60 = Higher recall, some false positives
    # 0.80 = Lower recall, maximum precision
```

## Project Structure

```
cmd/sentinel/     # Main app
internal/
  config/         # YAML config loading
  privacy/        # PII detection (83 regex patterns)
  security/       # Rate limiting + basic prompt injection detection
  proxy/          # HTTP proxy server
  websocket/      # Real-time events
web/              # Single HTML dashboard file
configs/          # YAML config files
```

## Limitations

- Vector security is just pattern matching, not real ML
- No user management or auth
- Basic dashboard, not production monitoring
- Limited to simple prompt injection patterns
- No persistent storage of events
- Docker setup includes PostgreSQL/Redis but they're not used yet

## Roadmap (realistic)

**Next up:**
- Integrate actual ONNX model for better threat detection
- Use the PostgreSQL/Redis setup for vector storage
- Add more prompt injection patterns

**Maybe later:**
- Better dashboard with filtering
- Export logs to files
- More LLM provider support

## License

MIT - do whatever you want with it.