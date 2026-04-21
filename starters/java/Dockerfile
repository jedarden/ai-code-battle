FROM eclipse-temurin:21-alpine AS builder

WORKDIR /app
COPY pom.xml .
COPY src ./src

RUN apk add --no-cache maven && mvn package -DskipTests

FROM eclipse-temurin:21-jre-alpine

WORKDIR /app
COPY --from=builder /app/target/starter-bot-1.0.0.jar /app/bot.jar

ENV BOT_PORT=8080
ENV BOT_SECRET=""

EXPOSE 8080

CMD ["java", "-jar", "/app/bot.jar"]
