# Build stage
FROM eclipse-temurin:21-jdk-alpine AS builder

WORKDIR /app
COPY pom.xml ./
COPY src ./src

# Install Maven and build
RUN apk add --no-cache maven && \
    mvn clean package -DskipTests

# Runtime stage
FROM eclipse-temurin:21-jre-alpine

WORKDIR /app
COPY --from=builder /app/target/hunter-bot-1.0.0.jar /app/hunter-bot.jar

ENV BOT_PORT=8085
ENV BOT_SECRET=""

EXPOSE 8085

CMD ["java", "-jar", "hunter-bot.jar"]
