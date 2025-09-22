# Contributing to LLM-Sentinel

## Quick Start

```bash
# Fork & clone
git clone https://github.com/your-username/llm-sentinel.git
cd llm-sentinel

# Setup & develop
npm install
npm run dev

# Test & commit
npm run build && npm run typecheck
git checkout -b feature/your-change
git commit -m "feat: description"
git push origin feature/your-change
```

## What We Accept

- **🐛 Bug fixes** - Fix issues or improve functionality
- **🔐 New detectors** - Add API keys, tokens, PII detection
- **🚀 Features** - CLI commands, proxy enhancements, dashboard
- **📚 Documentation** - README, examples, API docs
- **🧪 Tests** - Unit tests, integration tests

## Code Guidelines

### TypeScript Style
```typescript
// ✅ Good
interface DetectionRule {
  name: string;
  pattern: RegExp;
  replacement: string;
}

const rule: DetectionRule = {
  name: 'email',
  pattern: /[\w._%+-]+@[\w.-]+\.[A-Z|a-z]{2,}/g,
  replacement: '[EMAIL_MASKED]'
};

// ❌ Avoid
const rule: any = { ... };
```

### Naming
- Variables: `camelCase`
- Classes: `PascalCase`
- Files: `kebab-case.ts`

## Adding Detectors

1. **Research pattern** - Study API key format
2. **Add to `detectors.ts`**:
```typescript
const newServiceKey: MaskingRule = {
  name: 'newServiceApiKey',
  pattern: /ns_[a-zA-Z0-9]{32}/g,
  replacement: '[NEW_SERVICE_KEY_MASKED]'
};
```
3. **Write tests** - Positive and negative cases
4. **Update README** - Add to detector count

## Commit Format

```
type(scope): description

feat(detector): add Stripe API key detection
fix(proxy): handle malformed JSON
docs(readme): update examples
```

**Types**: `feat`, `fix`, `docs`, `test`, `refactor`, `chore`

## PR Checklist

- [ ] `npm run build` succeeds
- [ ] `npm run typecheck` passes
- [ ] Tests added for new features
- [ ] Documentation updated
- [ ] No sensitive data in commits
- [ ] Descriptive PR title/description

## Development Tips

### File Structure
```
src/
├── cli.ts           # CLI entry
├── proxy-server.ts  # HTTP proxy
├── detectors.ts     # Detection rules
├── config.ts        # Configuration
└── commands.ts      # CLI commands
```

### Dashboard Development
```bash
# Backend
npm run dev

# Dashboard (separate terminal)
cd dashboard && npm run dev
# → http://localhost:3000
```

### Testing
```bash
npm test                    # All tests
npm run test:unit          # Unit only
```

## Performance & Security

- **Regex efficiency** - Avoid backtracking
- **No sensitive logging** - Even in debug mode
- **Memory conscious** - Clean up after requests
- **Validate inputs** - Sanitize all user data

## Bug Reports

Include:
- Steps to reproduce
- Expected vs actual behavior
- Environment (OS, Node.js version)
- Installation method (npm/docker/source)

## Feature Requests

Include:
- Use case and value
- Proposed implementation
- Alternatives considered

## Security Issues

**Email privately** - Don't open public issues for vulnerabilities.

## Getting Help

- **GitHub Issues** - Bugs and features
- **GitHub Discussions** - Questions
- **Documentation** - Check [DEVELOPMENT.md](DEVELOPMENT.md)

## Recognition

Contributors are recognized in:
- Release notes
- GitHub contributors page
- Project documentation

---

**Ready to contribute?** Open an issue first for large changes, then submit a PR. We appreciate all contributions! 🛡️