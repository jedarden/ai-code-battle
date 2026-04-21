package com.acb.starter;

import com.fasterxml.jackson.databind.ObjectMapper;
import io.javalin.Javalin;
import io.javalin.http.Context;

import javax.crypto.Mac;
import javax.crypto.spec.SecretKeySpec;
import java.nio.charset.StandardCharsets;
import java.security.MessageDigest;
import java.security.SecureRandom;
import java.util.*;

/**
 * AI Code Battle - Java Starter Kit
 *
 * A minimal bot scaffold with HMAC authentication and a placeholder
 * random strategy. Replace computeMoves() with your own logic.
 */
public class App {

    private static final String[] DIRECTIONS = {"N", "E", "S", "W"};
    private static final SecureRandom RANDOM = new SecureRandom();
    private static final ObjectMapper MAPPER = new ObjectMapper();

    private static String secret;

    public static void main(String[] args) {
        String portStr = System.getenv().getOrDefault("BOT_PORT", "8080");
        secret = System.getenv().getOrDefault("BOT_SECRET", "");

        if (secret.isEmpty()) {
            System.err.println("ERROR: BOT_SECRET environment variable is required");
            System.exit(1);
        }

        int port = Integer.parseInt(portStr);

        Javalin app = Javalin.create()
                .start(port);

        app.get("/health", ctx -> ctx.result("OK"));
        app.post("/turn", App::handleTurn);

        System.out.println("Bot listening on port " + port);
    }

    private static void handleTurn(Context ctx) {
        String signature = ctx.header("X-ACB-Signature");
        String matchId = ctx.header("X-ACB-Match-Id");
        String turnStr = ctx.header("X-ACB-Turn");
        String timestamp = ctx.header("X-ACB-Timestamp");

        if (signature == null || signature.isEmpty()) {
            ctx.status(401).result("Missing signature");
            return;
        }

        String body = ctx.body();

        if (!verifySignature(matchId, turnStr, timestamp, body, signature)) {
            ctx.status(401).result("Invalid signature");
            return;
        }

        try {
            GameState state = MAPPER.readValue(body, GameState.class);
            List<Move> moves = computeMoves(state);

            String responseBody = MAPPER.writeValueAsString(new MoveResponse(moves));
            int turn = Integer.parseInt(turnStr != null ? turnStr : "0");
            String responseSig = signResponse(matchId, turn, responseBody);

            ctx.status(200);
            ctx.header("Content-Type", "application/json");
            ctx.header("X-ACB-Signature", responseSig);
            ctx.result(responseBody);
        } catch (Exception e) {
            ctx.status(400).result("Invalid game state");
        }
    }

    static List<Move> computeMoves(GameState state) {
        // Replace this with your strategy!
        List<Move> moves = new ArrayList<>();

        for (VisibleBot bot : state.bots) {
            if (bot.owner == state.you.id && RANDOM.nextDouble() < 0.5) {
                String dir = DIRECTIONS[RANDOM.nextInt(DIRECTIONS.length)];
                moves.add(new Move(bot.row, bot.col, dir));
            }
        }

        return moves;
    }

    // --- HMAC helpers ---

    static boolean verifySignature(String matchId, String turn, String timestamp,
                                    String body, String signature) {
        try {
            String bodyHash = sha256Hex(body.getBytes(StandardCharsets.UTF_8));
            String signingString = matchId + "." + turn + "." + timestamp + "." + bodyHash;
            String expected = hmacSha256(secret, signingString);
            return MessageDigest.isEqual(
                    expected.getBytes(StandardCharsets.UTF_8),
                    signature.getBytes(StandardCharsets.UTF_8)
            );
        } catch (Exception e) {
            return false;
        }
    }

    static String signResponse(String matchId, int turn, String body) {
        try {
            String bodyHash = sha256Hex(body.getBytes(StandardCharsets.UTF_8));
            String signingString = matchId + "." + turn + "." + bodyHash;
            return hmacSha256(secret, signingString);
        } catch (Exception e) {
            return "";
        }
    }

    static String hmacSha256(String key, String data) throws Exception {
        Mac mac = Mac.getInstance("HmacSHA256");
        mac.init(new SecretKeySpec(key.getBytes(StandardCharsets.UTF_8), "HmacSHA256"));
        byte[] hash = mac.doFinal(data.getBytes(StandardCharsets.UTF_8));
        return bytesToHex(hash);
    }

    static String sha256Hex(byte[] data) throws Exception {
        MessageDigest digest = MessageDigest.getInstance("SHA-256");
        return bytesToHex(digest.digest(data));
    }

    static String bytesToHex(byte[] bytes) {
        StringBuilder sb = new StringBuilder();
        for (byte b : bytes) {
            sb.append(String.format("%02x", b));
        }
        return sb.toString();
    }

    // --- Data classes ---

    public record GameConfig(int rows, int cols, int max_turns, int vision_radius2,
                             int attack_radius2, int spawn_cost, int energy_interval) {}

    public record You(int id, int energy, int score) {}

    public record VisibleBot(int row, int col, int owner) {}

    public record VisibleCore(int row, int col, int owner, boolean active) {}

    public record Position(int row, int col) {}

    public record GameState(String match_id, int turn, GameConfig config, You you,
                            List<VisibleBot> bots, List<Position> energy,
                            List<VisibleCore> cores, List<Position> walls,
                            List<VisibleBot> dead) {}

    public record Move(int row, int col, String direction) {}

    public record MoveResponse(List<Move> moves) {}
}
