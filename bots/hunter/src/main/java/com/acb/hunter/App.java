package com.acb.hunter;

import io.javalin.Javalin;
import io.javalin.http.Context;

import javax.crypto.Mac;
import javax.crypto.spec.SecretKeySpec;
import java.nio.charset.StandardCharsets;
import java.security.MessageDigest;
import java.time.Instant;
import java.util.HexFormat;

/**
 * HunterBot - A bot that targets isolated enemies for efficient kills.
 *
 * Strategy: Target isolated enemy bots for efficient kills.
 * - Identify enemy bots that are >=4 tiles from their nearest friendly bot (isolated targets)
 * - Send pairs of bots to intercept isolated enemies (2v1 wins cleanly)
 * - If no isolated targets, default to gatherer behavior
 * - Maintain a map of known enemy positions across turns, predict movement
 * - Avoid engaging formations of 3+ enemy bots
 * - Opportunistic energy collection when not actively hunting
 */
public class App {
    private static final int DEFAULT_PORT = 8085;
    private static final long TIMESTAMP_TOLERANCE_SECONDS = 30;
    private static String SECRET;
    private static final HunterStrategy STRATEGY = new HunterStrategy();

    public static void main(String[] args) {
        String portStr = System.getenv("BOT_PORT");
        int port = portStr != null ? Integer.parseInt(portStr) : DEFAULT_PORT;

        SECRET = System.getenv("BOT_SECRET");
        if (SECRET == null || SECRET.isEmpty()) {
            System.err.println("ERROR: BOT_SECRET environment variable is required");
            System.exit(1);
        }

        Javalin app = Javalin.create();

        app.get("/health", ctx -> ctx.result("OK"));

        app.post("/turn", App::handleTurn);

        app.start(port);
        System.out.println("HunterBot starting on port " + port);
    }

    private static void handleTurn(Context ctx) {
        // Extract auth headers
        String matchId = ctx.header("X-ACB-Match-Id");
        String turnStr = ctx.header("X-ACB-Turn");
        String timestamp = ctx.header("X-ACB-Timestamp");
        String botId = ctx.header("X-ACB-Bot-Id");
        String signature = ctx.header("X-ACB-Signature");

        if (matchId == null || matchId.isEmpty()
                || turnStr == null || turnStr.isEmpty()
                || timestamp == null || timestamp.isEmpty()
                || botId == null || botId.isEmpty()
                || signature == null || signature.isEmpty()) {
            ctx.status(401).result("Missing auth headers");
            return;
        }

        byte[] body = ctx.bodyAsBytes();

        // Verify signature
        if (!verifySignature(SECRET, matchId, turnStr, timestamp, body, signature)) {
            ctx.status(401).result("Invalid signature");
            return;
        }

        // Parse game state
        GameState state;
        try {
            state = GameState.fromJson(new String(body, StandardCharsets.UTF_8));
        } catch (Exception e) {
            ctx.status(400).result("Invalid JSON: " + e.getMessage());
            return;
        }

        int turn;
        try {
            turn = Integer.parseInt(turnStr);
        } catch (NumberFormatException e) {
            ctx.status(401).result("Invalid request identity");
            return;
        }
        if (turn < 0 || !Integer.toString(turn).equals(turnStr) || state.getTurn() == null
                || !matchId.equals(state.getMatchId()) || state.getTurn() != turn) {
            ctx.status(401).result("Invalid request identity");
            return;
        }

        // Compute moves
        var moves = STRATEGY.computeMoves(state);

        System.out.println("Turn " + turn + ": " + moves.size() + " moves computed");

        // Build response
        byte[] responseBody = MoveResponse.toJson(moves).getBytes(StandardCharsets.UTF_8);

        // Sign response
        String responseSig = signResponse(SECRET, matchId, turnStr, responseBody);

        ctx.header("X-ACB-Signature", responseSig);
        ctx.contentType("application/json");
        ctx.result(responseBody);
    }

    private static boolean verifySignature(String secret, String matchId, String turn,
                                           String timestamp, byte[] body, String signature) {
        try {
            long timestampSeconds = Long.parseLong(timestamp);
            long now = Instant.now().getEpochSecond();
            if (timestampSeconds < now - TIMESTAMP_TOLERANCE_SECONDS
                    || timestampSeconds > now + TIMESTAMP_TOLERANCE_SECONDS) {
                return false;
            }

            String bodyHash = sha256Hex(body);
            String signingString = matchId + "." + turn + "." + timestamp + "." + bodyHash;

            Mac mac = Mac.getInstance("HmacSHA256");
            SecretKeySpec keySpec = new SecretKeySpec(secret.getBytes(StandardCharsets.UTF_8), "HmacSHA256");
            mac.init(keySpec);
            byte[] expected = mac.doFinal(signingString.getBytes(StandardCharsets.UTF_8));

            return MessageDigest.isEqual(
                    HexFormat.of().parseHex(signature),
                    expected
            );
        } catch (Exception e) {
            return false;
        }
    }

    private static String signResponse(String secret, String matchId, String turn, byte[] body) {
        try {
            String bodyHash = sha256Hex(body);
            String signingString = matchId + "." + turn + "." + bodyHash;

            Mac mac = Mac.getInstance("HmacSHA256");
            SecretKeySpec keySpec = new SecretKeySpec(secret.getBytes(StandardCharsets.UTF_8), "HmacSHA256");
            mac.init(keySpec);
            return HexFormat.of().formatHex(mac.doFinal(signingString.getBytes(StandardCharsets.UTF_8)));
        } catch (Exception e) {
            throw new RuntimeException("Failed to sign response", e);
        }
    }

    private static String sha256Hex(byte[] input) {
        try {
            MessageDigest digest = MessageDigest.getInstance("SHA-256");
            return HexFormat.of().formatHex(digest.digest(input));
        } catch (Exception e) {
            throw new RuntimeException("Failed to hash", e);
        }
    }
}
