# acb-starter-java

Java starter kit for [AI Code Battle](https://aicodebattle.com) — a competitive bot programming platform.

Uses Javalin for the HTTP server, Jackson for JSON, and `javax.crypto` for HMAC authentication.

## Quick Start

```bash
# Build
mvn package

# Run locally
BOT_SECRET=dev-secret java -jar target/starter-bot-1.0.0.jar

# Run with Docker
docker build -t my-bot .
docker run -e BOT_SECRET=your-secret -p 8080:8080 my-bot
```

Your bot listens on port 8080 and responds to `POST /turn` with move commands.

## Register Your Bot

Once your bot is deployed and accessible via HTTPS:

```bash
curl -X POST https://api.aicodebattle.com/api/register \
  -H "Content-Type: application/json" \
  -d '{
    "name": "my-java-bot",
    "endpoint_url": "https://my-bot.example.com",
    "owner": "your-name",
    "description": "My awesome bot"
  }'
```

Save the `bot_id` and `shared_secret` from the response — the secret is shown only once.

## Project Structure

```
src/main/java/com/acb/starter/App.java  # Server, auth, types, and strategy
pom.xml                                 # Maven build configuration
Dockerfile                              # Multi-stage container build
```

## Customization

Edit `computeMoves()` in `App.java` to implement your strategy. The `GameState` object provides:

- `bots` — all visible bots (yours and enemies)
- `energy` — visible energy pickup locations
- `cores` — visible core positions
- `walls` — visible wall positions
- `youEnergy` — your current energy count
- `youScore` — your current score
- `config` — match parameters (grid size, etc.)

Return a `List<Map<String, Object>>`, each entry with `position` (your bot's current position) and `direction` (`"N"`, `"E"`, `"S"`, or `"W"`). Bots not included in the response stay in place.

## Protocol

- **Endpoint:** `POST /turn` — receives game state JSON, returns moves JSON
- **Health:** `GET /health` — must return 200
- **Timeout:** 3 seconds per turn
- **Auth:** HMAC-SHA256 via `X-ACB-Signature` header
