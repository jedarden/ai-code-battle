# AI Code Battle Index Builder Container
# Generates static JSON index files and deploys to Cloudflare Pages

FROM node:20-alpine

WORKDIR /app

# Copy package files
COPY package*.json ./

# Install dependencies
RUN npm ci --only=production

# Copy source code
COPY tsconfig.json ./
COPY src/ ./src/

# Build TypeScript
RUN npm run build

# Create output directory
RUN mkdir -p /app/data

# Environment variables (set at runtime)
# API_URL - Worker API URL (e.g., https://api.aicodebattle.com)
# API_KEY - Worker API key
# OUTPUT_DIR - Output directory (default: /app/data)
# DEPLOY_COMMAND - Optional deploy command (e.g., wrangler pages deploy)

ENV OUTPUT_DIR=/app/data

# Run the index builder
CMD ["node", "dist/index.js"]
