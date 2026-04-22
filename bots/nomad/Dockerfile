FROM python:3.13-slim

WORKDIR /app
COPY main.py grid.py .

ENV BOT_PORT=8080
ENV BOT_SECRET=""

EXPOSE 8080

CMD ["python3", "main.py"]
