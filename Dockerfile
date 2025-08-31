# Multi-stage build for optimal image size
FROM node:20-alpine AS builder

# Set working directory
WORKDIR /app

# Copy package files
COPY package*.json ./
COPY tsconfig.json ./

# Copy source code first
COPY src/ ./src/

# Copy dashboard source files
COPY dashboard/ ./dashboard/

# Install all dependencies (including dev dependencies for building)
RUN npm ci && npm cache clean --force

# Build the main application
RUN npm run build

# Build the dashboard
WORKDIR /app/dashboard
RUN npm ci && npm cache clean --force
RUN npm run build

# Return to main directory
WORKDIR /app

# Production stage
FROM node:20-alpine AS production

# Install dumb-init for proper signal handling
RUN apk add --no-cache dumb-init

# Create non-root user for security
RUN addgroup -g 1001 -S nodejs && \
    adduser -S llmsentinel -u 1001

# Set working directory
WORKDIR /app

# Copy package files
COPY package*.json ./

# Install only production dependencies (skip postinstall build script)
RUN npm ci --only=production --ignore-scripts && npm cache clean --force

# Copy built application from builder stage
COPY --from=builder /app/dist ./dist

# Copy built dashboard files to dist directory (where the server expects them)
COPY --from=builder /app/dist/dashboard ./dist/dashboard

# Create logs directory with proper permissions
RUN mkdir -p /app/logs && chown -R llmsentinel:nodejs /app/logs

# Create config directory for user configs
RUN mkdir -p /app/.llm-sentinel && chown -R llmsentinel:nodejs /app/.llm-sentinel

# Create global symlink for llmsentinel command
RUN ln -s /app/dist/cli.js /usr/local/bin/llmsentinel && \
    chmod +x /app/dist/cli.js

# Switch to non-root user
USER llmsentinel

# Expose the default port
EXPOSE 5050

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD node -e "require('http').get('http://localhost:5050/health', (res) => { \
        process.exit(res.statusCode === 200 ? 0 : 1) \
    }).on('error', () => process.exit(1))"

# Use dumb-init to handle signals properly
ENTRYPOINT ["dumb-init", "--"]

# Start the application
CMD ["node", "dist/cli.js", "start"]