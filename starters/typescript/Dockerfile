# AI Code Battle - TypeScript Starter Bot
FROM node:22-alpine AS builder

WORKDIR /app

# Copy package files
COPY package.json package-lock.json* ./

# Install all dependencies (including devDependencies for TypeScript build)
RUN npm ci

# Copy source code
COPY src/ ./src/
COPY tsconfig.json ./

# Build TypeScript
RUN npx tsc

# Production stage
FROM node:22-alpine

WORKDIR /app

# Copy only production dependencies and built code
COPY package.json package-lock.json* ./
RUN npm ci --omit=dev
COPY --from=builder /app/dist ./dist

# Set non-root user for security
RUN addgroup -g 1001 -S acb && \
    adduser -S -D -H -u 1001 -s /sbin/nologin -G acb -g acb acb
USER acb

# Expose bot port
EXPOSE 8080

# Set required environment variable (default for dev)
ENV BOT_SECRET=dev-secret

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD node -e "require('http').get('http://localhost:8080/health', (r) => { process.exit(r.statusCode === 200 ? 0 : 1) })"

# Start the bot
CMD ["node", "dist/index.js"]
