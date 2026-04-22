FROM eclipse-temurin:21-jdk-alpine AS builder

WORKDIR /app
COPY pom.xml ./
COPY src ./src

RUN apk add --no-cache maven && \
    mvn clean package -DskipTests

FROM eclipse-temurin:21-jre-alpine

WORKDIR /app
COPY --from=builder /app/target/raider-bot-1.0.0.jar /app/raider-bot.jar

ENV BOT_PORT=8086
ENV BOT_SECRET=""

EXPOSE 8086

CMD ["java", "-jar", "raider-bot.jar"]
