FROM python:3.12-slim

WORKDIR /app

COPY main.py .

ENV BOT_PORT=8080
ENV BOT_SECRET=""

EXPOSE 8080

CMD ["python3", "main.py"]
