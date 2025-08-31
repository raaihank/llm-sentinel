# Contributing to LLM-Sentinel

Thank you for your interest in contributing to LLM-Sentinel! This guide will help you get started.

## Table of Contents

- [Code of Conduct](#code-of-conduct)
- [Getting Started](#getting-started)
- [Development Setup](#development-setup)
- [How to Contribute](#how-to-contribute)
- [Code Style](#code-style)
- [Testing](#testing)
- [Pull Request Process](#pull-request-process)
- [Issue Guidelines](#issue-guidelines)

## Code of Conduct

By participating in this project, you agree to abide by our Code of Conduct:

- Be respectful and inclusive
- Focus on constructive feedback
- Help others learn and grow
- Maintain professionalism in all interactions

## Getting Started

### Prerequisites

- Node.js 18+ 
- npm or yarn
- Git
- Basic understanding of TypeScript
- Familiarity with Express.js and HTTP proxies

### Development Setup

See [DEVELOPMENT.md](DEVELOPMENT.md) for detailed development setup instructions.

## How to Contribute

### Types of Contributions

We welcome contributions in the following areas:

1. **🐛 Bug Fixes** - Fix issues or improve existing functionality
2. **🚀 New Features** - Add new detection rules, endpoints, or capabilities
3. **📚 Documentation** - Improve docs, examples, or README
4. **🔧 Performance** - Optimize detection speed, memory usage, or proxy efficiency
5. **🧪 Testing** - Add tests, improve test coverage, or fix test issues
6. **🔐 Security** - Enhance security, add new detectors, or fix vulnerabilities

### Before You Start

1. **Check existing issues** - Look for related issues or feature requests
2. **Open an issue first** - For large changes, discuss your approach
3. **Fork the repository** - Create your own copy to work on
4. **Create a feature branch** - Use descriptive branch names

### Workflow

```bash
# 1. Fork and clone
git clone https://github.com/your-username/llm-sentinel.git
cd llm-sentinel

# 2. Create feature branch
git checkout -b feature/add-new-detector

# 3. Make your changes
npm run dev  # Development with hot reload

# 4. Test your changes
npm run build
npm run typecheck
npm test  # If tests exist

# 5. Commit and push
git add .
git commit -m "feat: add new API key detector for XYZ service"
git push origin feature/add-new-detector

# 6. Create pull request
```

## Code Style

### TypeScript Guidelines

- **Use TypeScript** for all new code
- **Prefer interfaces over types** for object definitions
- **Use strict null checks** - handle undefined/null explicitly
- **Avoid `any`** - use proper typing or `unknown`

### Code Formatting

```typescript
// ✅ Good
interface DetectionRule {
  name: string;
  pattern: RegExp;
  replacement: string;
  enabled: boolean;
}

const emailRule: DetectionRule = {
  name: 'email',
  pattern: /\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Z|a-z]{2,}\b/g,
  replacement: '[EMAIL_MASKED]',
  enabled: true
};

// ❌ Avoid
const rule: any = {
  name: 'email',
  pattern: /\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Z|a-z]{2,}\b/g,
  replacement: '[EMAIL_MASKED]'
};
```

### Naming Conventions

- **Variables/Functions**: `camelCase`
- **Classes/Interfaces**: `PascalCase`
- **Constants**: `UPPER_SNAKE_CASE`
- **Files**: `kebab-case.ts` or `camelCase.ts`

### File Organization

```
src/
├── cli.ts              # Command line interface
├── proxy-server.ts     # HTTP proxy server
├── detectors.ts        # Detection rules engine
├── config.ts           # Configuration management
├── logger.ts           # Structured logging
├── commands.ts         # CLI command implementations
└── types/              # Type definitions
    ├── config.ts
    ├── detection.ts
    └── server.ts
```

## Testing

### Running Tests

```bash
npm test              # Run all tests
npm run test:unit     # Unit tests only
npm run test:e2e      # End-to-end tests
npm run coverage      # Test coverage report
```

### Writing Tests

```typescript
// Example test for new detector
describe('API Key Detection', () => {
  test('should mask OpenAI API keys', () => {
    const input = 'My key is sk-1234567890abcdef';
    const result = maskSensitiveData(input);
    expect(result).toBe('My key is [OPENAI_API_KEY_MASKED]');
  });

  test('should handle multiple keys in same text', () => {
    const input = 'Keys: sk-abc123 and sk-def456';
    const result = maskSensitiveData(input);
    expect(result).toBe('Keys: [OPENAI_API_KEY_MASKED] and [OPENAI_API_KEY_MASKED]');
  });
});
```

### Test Requirements

- All new features must include tests
- Bug fixes should include regression tests
- Aim for >80% code coverage on new code
- Tests should be fast and deterministic

## Pull Request Process

### PR Checklist

Before submitting your PR, ensure:

- [ ] **Code builds successfully** (`npm run build`)
- [ ] **Types check** (`npm run typecheck`)
- [ ] **Tests pass** (`npm test`)
- [ ] **Documentation updated** if adding features
- [ ] **Commit messages follow convention** (see below)
- [ ] **No sensitive data** in code or commits
- [ ] **PR description** explains the change

### Commit Message Format

Use conventional commits for clear history:

```
type(scope): description

[optional body]

[optional footer]
```

**Types:**
- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation changes
- `style`: Code style changes (formatting, etc.)
- `refactor`: Code refactoring
- `test`: Adding or fixing tests
- `chore`: Build process or auxiliary tool changes

**Examples:**
```
feat(detector): add Stripe API key detection
fix(proxy): handle malformed JSON in requests
docs(readme): update installation instructions
test(masker): add tests for email detection
```

### PR Review Process

1. **Automated checks** - CI runs tests and type checking
2. **Code review** - Maintainers review for:
   - Code quality and style
   - Security implications
   - Performance impact
   - Test coverage
3. **Feedback incorporation** - Address review comments
4. **Final approval** - Maintainer approves and merges

## Issue Guidelines

### Bug Reports

Use the bug report template and include:

```markdown
**Bug Description**
Clear description of the issue

**Steps to Reproduce**
1. Start LLM-Sentinel
2. Send request with specific data
3. Observe incorrect behavior

**Expected Behavior**
What should happen

**Environment**
- OS: macOS 14.0
- Node.js: v18.17.0
- LLM-Sentinel: v1.0.0
- Installation: npm/docker/source
```

### Feature Requests

Use the feature request template:

```markdown
**Feature Description**
What new capability you'd like to see

**Use Case**
Why this would be valuable

**Proposed Implementation**
How it might work (optional)

**Alternatives Considered**
Other approaches you've thought about
```

### Security Issues

**Do not open public issues for security vulnerabilities.**

Instead, email security concerns to: [your-security-email]

## Development Guidelines

### Adding New Detectors

When adding detection rules for new services:

1. **Research the pattern** - Understand the format thoroughly
2. **Create comprehensive regex** - Handle variations and edge cases
3. **Add to rules array** - Include in `pii-masker.ts`
4. **Write tests** - Cover positive and negative cases
5. **Update documentation** - Add to README detector list

Example:
```typescript
// In detectors.ts
const newServiceApiKey: MaskingRule = {
  name: 'newServiceApiKey',
  pattern: /ns_[a-zA-Z0-9]{32}/g,
  replacement: '[NEW_SERVICE_API_KEY_MASKED]'
};
```

### Performance Considerations

- **Regex efficiency** - Use non-greedy quantifiers when possible
- **Memory usage** - Avoid storing sensitive data in memory
- **Proxy latency** - Minimize processing time per request
- **Logging overhead** - Keep log processing lightweight

### Security Best Practices

- **Never log sensitive data** - Even in debug mode
- **Validate all inputs** - Sanitize configuration and requests
- **Principle of least privilege** - Minimize permissions needed
- **Regular expression safety** - Avoid ReDoS vulnerabilities

## Getting Help

- **GitHub Issues** - For bugs and feature requests
- **GitHub Discussions** - For questions and general discussion
- **Documentation** - Check [DEVELOPMENT.md](DEVELOPMENT.md) for technical details

## Recognition

Contributors will be recognized in:
- GitHub contributors page
- Release notes for significant contributions
- Optional mention in documentation

Thank you for helping make LLM-Sentinel better! 🛡️