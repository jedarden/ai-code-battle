FROM node:22-alpine

WORKDIR /app
COPY package.json .
COPY index.js grid.js .

ENV BOT_PORT=8080
ENV BOT_SECRET=""

EXPOSE 8080

CMD ["node", "index.js"]
