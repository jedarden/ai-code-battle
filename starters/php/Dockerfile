FROM php:8.4-cli-alpine

WORKDIR /app

COPY index.php strategy.php game.php composer.json ./

ENV BOT_PORT=8080
ENV BOT_SECRET=""

EXPOSE 8080

CMD ["php", "index.php"]
