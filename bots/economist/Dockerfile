FROM python:3.12-alpine

WORKDIR /app

# Install curl for health checks
RUN apk add --no-cache curl

COPY bot.py .

ENV BOT_PORT=8081
ENV BOT_SECRET=""

EXPOSE 8081

CMD ["python3", "bot.py"]
