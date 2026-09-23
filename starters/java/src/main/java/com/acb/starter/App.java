package com.acb.starter;

import com.fasterxml.jackson.databind.ObjectMapper;
import io.javalin.Javalin;
import io.javalin.http.Context;

import javax.crypto.Mac;
import javax.crypto.spec.SecretKeySpec;
import java.nio.charset.StandardCharsets;
import java.security.MessageDigest;
import java.security.SecureRandom;
import java.time.Instant;
import java.util.*;

/**
 * AI Code Battle - Java Starter Kit
 *
 * A minimal bot scaffold with HMAC authentication and a placeholder
 * random strategy. Replace computeMoves() with your own logic.
 */
public class App {

    private static final String[] DIRECTIONS = {"N", "E", "S", "W"};
    private static final long TIMESTAMP_TOLERANCE_SECONDS = 30;
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
        String botId = ctx.header("X-ACB-Bot-Id");

        if (matchId == null || matchId.isEmpty()
                || turnStr == null || turnStr.isEmpty()
                || timestamp == null || timestamp.isEmpty()
                || botId == null || botId.isEmpty()
                || signature == null || signature.isEmpty()) {
            ctx.status(401).result("Missing auth headers");
            return;
        }

        byte[] body = ctx.bodyAsBytes();

        if (!verifySignature(matchId, turnStr, timestamp, body, signature)) {
            ctx.status(401).result("Invalid signature");
            return;
        }

        try {
            GameState state = MAPPER.readValue(body, GameState.class);
            final int turn;
            try {
                turn = Integer.parseInt(turnStr);
            } catch (NumberFormatException e) {
                ctx.status(401).result("Invalid request identity");
                return;
            }
            if (turn < 0 || !Integer.toString(turn).equals(turnStr) || state.turn == null
                    || !matchId.equals(state.match_id) || state.turn != turn) {
                ctx.status(401).result("Invalid request identity");
                return;
            }

            if (state.turn == 0) {
                String seasonId = state.config.season_id != null ? state.config.season_id : "";
                String rulesVersion = state.config.rules_version != null ? state.config.rules_version : "";
                System.out.printf("match=%s season_id=%s rules_version=%s rows=%d cols=%d%n",
                        state.match_id, seasonId, rulesVersion, state.config.rows, state.config.cols);
            }

            List<Move> moves = computeMoves(state);

            byte[] responseBody = MAPPER.writeValueAsBytes(new MoveResponse(moves));
            String responseSig = signResponse(matchId, Integer.toString(turn), responseBody);

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
        int rows = state.config.rows;
        int cols = state.config.cols;
        List<Move> moves = new ArrayList<>();

        int[][] cardinal = {{-1, 0}, {0, 1}, {1, 0}, {0, -1}};

        for (VisibleBot bot : state.bots) {
            if (bot.owner != state.you.id) continue;

            // Find direction toward nearest energy using toroidal distance
            if (!state.energy.isEmpty()) {
                int bestDist = Integer.MAX_VALUE;
                String bestDir = null;
                for (int i = 0; i < cardinal.length; i++) {
                    int nr = Math.floorMod(bot.position.row + cardinal[i][0], rows);
                    int nc = Math.floorMod(bot.position.col + cardinal[i][1], cols);
                    for (Position e : state.energy) {
                        int d = Grid.toroidalManhattan(nr, nc, e.row, e.col, rows, cols);
                        if (d < bestDist) {
                            bestDist = d;
                            bestDir = DIRECTIONS[i];
                        }
                    }
                }
                if (bestDir != null) {
                    moves.add(new Move(new Position(bot.position.row, bot.position.col), bestDir));
                    continue;
                }
            }

            if (RANDOM.nextDouble() < 0.5) {
                String dir = DIRECTIONS[RANDOM.nextInt(DIRECTIONS.length)];
                moves.add(new Move(new Position(bot.position.row, bot.position.col), dir));
            }
        }

        return moves;
    }

    // --- HMAC helpers ---

    static boolean verifySignature(String matchId, String turn, String timestamp,
                                    byte[] body, String signature) {
        try {
            long timestampSeconds = Long.parseLong(timestamp);
            long now = Instant.now().getEpochSecond();
            if (timestampSeconds < now - TIMESTAMP_TOLERANCE_SECONDS
                    || timestampSeconds > now + TIMESTAMP_TOLERANCE_SECONDS) {
                return false;
            }

            String bodyHash = sha256Hex(body);
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

    static String signResponse(String matchId, String turn, byte[] body) {
        try {
            // Response signing string: {match_id}.{turn}.{sha256_hex(body)}
            String bodyHash = sha256Hex(body);
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
                             int attack_radius2, int spawn_cost, int energy_interval,
                             String season_id, String rules_version) {}

    public record You(int id, int energy, int score) {}

    public record VisibleBot(Position position, int owner) {}

    public record VisibleCore(Position position, int owner, boolean active) {}

    public record Position(int row, int col) {}

    public record GameState(String match_id, Integer turn, GameConfig config, You you,
                            List<VisibleBot> bots, List<Position> energy,
                            List<VisibleCore> cores, List<Position> walls,
                            List<VisibleBot> dead) {}

    public record Move(Position position, String direction) {}

    public record MoveResponse(List<Move> moves) {}
}
