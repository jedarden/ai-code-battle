# GuardianBot - PHP defensive bot
FROM php:8.4-cli-alpine

WORKDIR /app

COPY . .

ENV BOT_PORT=8083
ENV BOT_SECRET=""

EXPOSE 8083

CMD ["php", "index.php"]
