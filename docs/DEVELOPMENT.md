# Development Guide

## Quick Setup

```bash
# Clone and install
git clone https://github.com/raaihank/llm-sentinel.git
cd llm-sentinel
npm install

# Start development
npm run dev                    # Backend with hot reload
curl http://localhost:5050/health

# Dashboard development (separate terminal)
cd dashboard && npm run dev    # → http://localhost:3000
```

## Scripts

```bash
npm run dev           # Hot reload backend
npm run build         # Production build
npm run typecheck     # Type checking
npm run clean         # Remove build artifacts

# Dashboard
npm run dev:dashboard       # Dashboard dev mode
npm run build:dashboard     # Dashboard production build
```

## Architecture

```
Client → LLM-Sentinel → AI API
         ↓
    52 Detectors → Mask Data → Proxy Request
```

### File Structure
```
src/
├── cli.ts              # CLI entry point
├── proxy-server.ts     # Express proxy server
├── detectors.ts        # 50+ detection rules
├── config.ts           # Configuration management
├── logger.ts           # Structured logging
└── commands.ts         # CLI command handlers

dashboard/
├── src/app/            # Next.js dashboard
├── next.config.ts      # Dashboard build config
└── components/ui/      # UI components
```

## Core Components

### Detection Engine
```typescript
interface MaskingRule {
  name: string;
  pattern: RegExp;
  replacement: string;
}

const openaiApiKey: MaskingRule = {
  name: 'openaiApiKey',
  pattern: /sk-[a-zA-Z0-9]{48}/g,
  replacement: '[OPENAI_API_KEY_MASKED]'
};
```

### Proxy Flow
1. Intercept request → Parse JSON → Extract text
2. Run detectors → Mask sensitive data → Reconstruct request
3. Forward to AI API → Return response → Log events

### Configuration
```typescript
interface Config {
  server: { port: number; targets: string[] };
  detection: { enabled: boolean; rules: string[] };
  logging: { level: string; showEntities: boolean };
}

// Priority: CLI > ENV > File > Defaults
```

## Dashboard

### Development vs Production
| Mode | Port | Build | Usage |
|------|------|-------|-------|
| Dev | 3000 | Hot reload | `npm run dev:dashboard` |
| Prod | 5050 | Static export | Served by proxy server |

### Production Build
```bash
# Dashboard build triggers automatically
npm run build

# Manual dashboard build
NODE_ENV=production npm run build:dashboard
# → Generates static files in dist/dashboard/
```

## Adding Detectors

```typescript
// 1. Research API key format
// 2. Create detection rule in detectors.ts
const newServiceKey: MaskingRule = {
  name: 'newServiceApiKey',
  pattern: /ns_[a-zA-Z0-9]{32}/g,
  replacement: '[NEW_SERVICE_MASKED]'
};

// 3. Add to default rules in config.ts
const defaultRules = [..., 'newServiceApiKey'];

// 4. Write tests
test('detects new service keys', () => {
  expect(mask('key: ns_abc123')).toBe('key: [NEW_SERVICE_MASKED]');
});
```

## Testing

```bash
npm test              # Unit tests
npm run coverage      # Coverage report

# Manual testing
echo '{"prompt":"My key is sk-test123"}' | \
curl -X POST -H "Content-Type: application/json" \
     -d @- http://localhost:5050/openai/v1/chat/completions
```

## Configuration System

### Loading Order
1. CLI args: `--port 8080`
2. Environment: `LLM_SENTINEL_PORT=8080`
3. Config file: `~/.llm-sentinel/config.json`
4. Defaults: Built-in values

### Runtime Updates
```bash
llmsentinel port 8080           # Change port
llmsentinel rules:disable email # Disable email detection
llmsentinel debug               # Enable debug mode
```

## Docker

```dockerfile
# Multi-stage build
FROM node:20-alpine AS builder
WORKDIR /app
COPY . .
RUN npm ci && npm run build

FROM node:20-alpine
COPY --from=builder /app/dist ./dist
COPY package*.json ./
RUN npm ci --only=production
CMD ["node", "dist/cli.js", "start"]
```

## Performance

### Optimization Tips
- Use non-capturing groups: `(?:...)` not `(...)`
- Anchor patterns: `^pattern$` when possible
- Avoid backtracking: Specific quantifiers
- Stream large requests instead of buffering

### Memory Management
- Clean detection results after processing
- Limit request body size
- Use object pooling for frequent allocations

## 🚀 Release Checklist

### Pre-Release
- [ ] `npm run build && npm run typecheck` ✅
- [ ] Dashboard works: dev (`npm run dev:dashboard`) + prod (`npm run build`)
- [ ] Docker builds: `docker build -t test .`
- [ ] Production test: `NODE_ENV=production node dist/cli.js start`

### Testing
- [ ] Health endpoint: `/health`
- [ ] API endpoints: `/api/config`, `/api/stats`
- [ ] Proxy functionality: OpenAI + Ollama endpoints
- [ ] Dashboard loads at root path
- [ ] WebSocket real-time updates

### Release
- [ ] Version bump: `npm version patch/minor/major`
- [ ] NPM: `npm publish`
- [ ] Docker: `docker push raaihank/llm-sentinel:latest`
- [ ] GitHub: Create release from tag

### Verification
- [ ] NPM install: `npm install -g llm-sentinel`
- [ ] Docker pull: `docker pull raaihank/llm-sentinel:latest`
- [ ] Test both installations work

## Debugging

```bash
# Debug mode
llmsentinel debug
llmsentinel logs -n 100

# Health check
curl http://localhost:5050/health
curl http://localhost:5050/api/config

# Test detection
echo "test: sk-abc123" | node -e "
  const { maskSensitiveData } = require('./dist/detectors');
  process.stdin.on('data', d => console.log(maskSensitiveData(d.toString())));
"
```

## Common Issues

- **Port conflicts**: `lsof -ti:5050` then `kill <PID>`
- **Build failures**: `npm run clean && npm ci && npm run build`
- **Docker issues**: Check logs with `docker logs <container>`
- **Dashboard 404s**: Ensure `npm run build` completed successfully

---

For contributing guidelines, see [CONTRIBUTING.md](CONTRIBUTING.md).