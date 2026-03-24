# Build stage
FROM node:22-alpine AS builder

WORKDIR /app
COPY package.json tsconfig.json ./
COPY src ./src

RUN npm install && npm run build

# Runtime stage
FROM node:22-alpine

WORKDIR /app
COPY --from=builder /app/dist ./dist
COPY --from=builder /app/node_modules ./node_modules
COPY package.json ./

ENV BOT_PORT=8084
ENV BOT_SECRET=""

EXPOSE 8084

CMD ["npm", "start"]
