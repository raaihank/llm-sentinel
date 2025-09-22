# Release Guide

Complete workflow for releasing LLM-Sentinel to NPM and Docker Hub.

## Prerequisites

```bash
# Required tools
npm whoami                  # Verify NPM login
docker login               # Verify Docker Hub login
git status                 # Ensure clean working directory

# Required access
# - NPM: Publisher access to llm-sentinel package
# - Docker: Push access to raaihank/llm-sentinel
# - GitHub: Admin access to create releases
```

## Pre-Release Checklist

```bash
# 1. Clean build and test
npm run clean
npm ci
npm run build
npm run typecheck

# 2. Test dashboard builds
npm run build:dashboard
ls -la dist/dashboard/index.html  # Should exist

# 3. Test production server
NODE_ENV=production node dist/cli.js start -p 5051
curl http://localhost:5051/health
curl http://localhost:5051        # Dashboard should load

# 4. Test Docker build
docker build -t llm-sentinel:test .
docker run -d -p 5052:5050 --name test-release llm-sentinel:test
curl http://localhost:5052/health
docker stop test-release && docker rm test-release
```

## Version Management

```bash
# Choose version type
npm version patch    # Bug fixes (1.0.0 → 1.0.1)
npm version minor    # New features (1.0.0 → 1.1.0)
npm version major    # Breaking changes (1.0.0 → 2.0.0)

# This automatically:
# - Updates package.json version
# - Creates git commit
# - Creates git tag
```

## NPM Release

### 1. Package Verification
```bash
# Dry run to verify package contents
npm publish --dry-run

# Check files included
npm pack
tar -tzf llm-sentinel-*.tgz | head -20
rm llm-sentinel-*.tgz
```

### 2. Publish to NPM
```bash
# Publish to NPM registry
npm publish

# Verify publication
npm view llm-sentinel
npm view llm-sentinel versions --json
```

### 3. Test NPM Installation
```bash
# Test global installation
npm install -g llm-sentinel@latest

# Verify command works
llmsentinel --version
llmsentinel start --help

# Test installation
mkdir /tmp/test-install && cd /tmp/test-install
llmsentinel start -p 5053 &
PID=$!
sleep 3
curl http://localhost:5053/health
kill $PID
cd - && rm -rf /tmp/test-install
```

## Docker Release

### 1. Build Production Image
```bash
# Get version from package.json
VERSION=$(node -p "require('./package.json').version")
echo "Building version: v$VERSION"

# Build with version tag
docker build -t raaihank/llm-sentinel:v$VERSION .
docker build -t raaihank/llm-sentinel:latest .

# Verify build
docker images raaihank/llm-sentinel
```

### 2. Test Docker Image
```bash
# Test versioned image
docker run -d -p 5054:5050 --name test-v$VERSION raaihank/llm-sentinel:v$VERSION
sleep 5

# Verify dashboard and health
curl -s http://localhost:5054/health | jq
curl -s http://localhost:5054 | grep "LLM-Sentinel" || echo "Dashboard failed"

# Cleanup
docker stop test-v$VERSION && docker rm test-v$VERSION
```

### 3. Push to Docker Hub
```bash
# Push versioned tag
docker push raaihank/llm-sentinel:v$VERSION

# Push latest tag
docker push raaihank/llm-sentinel:latest

# Verify on Docker Hub
docker search raaihank/llm-sentinel
```

### 4. Test Docker Pull
```bash
# Test pulling and running
docker rmi raaihank/llm-sentinel:latest  # Remove local
docker pull raaihank/llm-sentinel:latest

# Test fresh pull
docker run -d -p 5055:5050 --name test-pull raaihank/llm-sentinel:latest
sleep 5
curl http://localhost:5055/health
docker stop test-pull && docker rm test-pull
```

## GitHub Release

### 1. Push Changes and Tags
```bash
# Push commits and tags
git push origin main
git push --tags

# Verify tag exists
git tag -l | grep $(node -p "require('./package.json').version")
```

### 2. Create GitHub Release
```bash
# Option 1: GitHub CLI (if available)
gh release create v$VERSION \
  --title "Release v$VERSION" \
  --notes "## Changes\n- Feature updates\n- Bug fixes\n\n## Installation\n\`\`\`bash\nnpm install -g llm-sentinel@$VERSION\n# or\ndocker pull raaihank/llm-sentinel:v$VERSION\n\`\`\`"

# Option 2: Manual via GitHub UI
echo "Create release manually at: https://github.com/raaihank/llm-sentinel/releases/new"
echo "Tag: v$VERSION"
```

## Post-Release Verification

### 1. Installation Tests
```bash
# Test NPM installation on different systems
npm install -g llm-sentinel@latest

# Test Docker on different systems
docker pull raaihank/llm-sentinel:latest
docker run -p 5050:5050 raaihank/llm-sentinel:latest
```

### 2. Documentation Check
```bash
# Verify README links work
curl -s https://raw.githubusercontent.com/raaihank/llm-sentinel/main/README.md | grep -o 'http[^)]*' | head -5

# Check Docker Hub description updated
echo "Visit: https://hub.docker.com/r/raaihank/llm-sentinel"

# Check NPM page
echo "Visit: https://www.npmjs.com/package/llm-sentinel"
```

## Complete Release Script

Save as `scripts/release.sh`:

```bash
#!/bin/bash
set -e

VERSION_TYPE=${1:-patch}
echo "🚀 Starting $VERSION_TYPE release..."

# Pre-flight checks
echo "📋 Pre-flight checks..."
npm run clean
npm ci
npm run build
npm run typecheck

# Version bump
echo "📝 Version bump..."
npm version $VERSION_TYPE

# Get new version
VERSION=$(node -p "require('./package.json').version")
echo "📦 Releasing version: v$VERSION"

# NPM publish
echo "📤 Publishing to NPM..."
npm publish

# Docker build and push
echo "🐳 Building and pushing Docker image..."
docker build -t raaihank/llm-sentinel:v$VERSION .
docker build -t raaihank/llm-sentinel:latest .
docker push raaihank/llm-sentinel:v$VERSION
docker push raaihank/llm-sentinel:latest

# Push to GitHub
echo "📨 Pushing to GitHub..."
git push origin main
git push --tags

# Success
echo "✅ Release v$VERSION complete!"
echo "🔗 NPM: https://www.npmjs.com/package/llm-sentinel"
echo "🔗 Docker: https://hub.docker.com/r/raaihank/llm-sentinel"
echo "🔗 GitHub: Create release at https://github.com/raaihank/llm-sentinel/releases/new"
```

Usage:
```bash
chmod +x scripts/release.sh
./scripts/release.sh patch   # or minor/major
```

## Rollback Procedures

### NPM Rollback
```bash
# Unpublish recent version (within 72 hours)
npm unpublish llm-sentinel@$VERSION

# Or deprecate
npm deprecate llm-sentinel@$VERSION "Known issues, use previous version"
```

### Docker Rollback
```bash
# Retag previous stable version as latest
docker pull raaihank/llm-sentinel:v1.0.1  # Previous stable
docker tag raaihank/llm-sentinel:v1.0.1 raaihank/llm-sentinel:latest
docker push raaihank/llm-sentinel:latest
```

### Communication
```bash
# Notify users via GitHub issue/discussion
echo "Create issue documenting known problems and rollback"
echo "Update README with temporary workarounds if needed"
```

## Release Checklist Summary

**Pre-Release:**
- [ ] Clean build passes
- [ ] Dashboard builds (dev + production)
- [ ] Production server tested
- [ ] Docker build tested

**Release Execution:**
- [ ] Version bumped (`npm version`)
- [ ] NPM published and verified
- [ ] Docker built, tagged, and pushed
- [ ] GitHub tags pushed
- [ ] GitHub release created

**Post-Release:**
- [ ] NPM installation tested
- [ ] Docker pull tested
- [ ] Documentation links verified
- [ ] Monitor for issues (24 hours)

---

**Next Steps:** After release, monitor GitHub issues and NPM/Docker Hub for any problems reported by users.