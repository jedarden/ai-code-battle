FROM python:3.13-slim

WORKDIR /app

# Copy bot files
COPY main.py strategy.py ./

# Expose port
EXPOSE 8080

# Run the bot
CMD ["python", "main.py"]
