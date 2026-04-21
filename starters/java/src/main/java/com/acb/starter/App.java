package com.acb.starter;

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
            GameState state = parseGameState(body);
            List<Map<String, Object>> moves = computeMoves(state);

            String responseBody = toJsonMoves(moves);
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

    static List<Map<String, Object>> computeMoves(GameState state) {
        // Replace this with your strategy!
        List<Map<String, Object>> moves = new ArrayList<>();

        for (Map<String, Object> bot : state.bots) {
            int owner = ((Number) bot.get("owner")).intValue();
            if (owner == state.youId && RANDOM.nextDouble() < 0.5) {
                String dir = DIRECTIONS[RANDOM.nextInt(DIRECTIONS.length)];
                Map<String, Object> move = new LinkedHashMap<>();
                move.put("position", bot.get("position"));
                move.put("direction", dir);
                moves.add(move);
            }
        }

        return moves;
    }

    // --- JSON helpers ---

    static GameState parseGameState(String json) {
        // Minimal JSON parser for the game state
        GameState state = new GameState();
        Map<String, Object> map = parseJson(json);
        state.matchId = (String) map.get("match_id");
        state.turn = ((Number) map.get("turn")).intValue();
        state.config = (Map<String, Object>) map.get("config");

        Map<String, Object> you = (Map<String, Object>) map.get("you");
        state.youId = ((Number) you.get("id")).intValue();
        state.youEnergy = ((Number) you.get("energy")).intValue();
        state.youScore = ((Number) you.get("score")).intValue();

        state.bots = (List<Map<String, Object>>) map.get("bots");
        state.energy = (List<Map<String, Object>>) map.get("energy");
        state.cores = (List<Map<String, Object>>) map.get("cores");
        state.walls = (List<Map<String, Object>>) map.get("walls");
        state.dead = (List<Map<String, Object>>) map.get("dead");

        return state;
    }

    static String toJsonMoves(List<Map<String, Object>> moves) {
        StringBuilder sb = new StringBuilder("{\"moves\":[");
        for (int i = 0; i < moves.size(); i++) {
            if (i > 0) sb.append(",");
            Map<String, Object> move = moves.get(i);
            Map<String, Object> pos = (Map<String, Object>) move.get("position");
            sb.append("{\"position\":{\"row\":")
              .append(pos.get("row")).append(",\"col\":").append(pos.get("col"))
              .append("},\"direction\":\"").append(move.get("direction")).append("\"}");
        }
        sb.append("]}");
        return sb.toString();
    }

    @SuppressWarnings("unchecked")
    static Map<String, Object> parseJson(String json) {
        return new io.javalin.json.JavalinJackson().fromJsonString(json, Map.class);
    }

    // --- HMAC helpers ---

    static boolean verifySignature(String matchId, String turn, String timestamp,
                                    String body, String signature) {
        try {
            String bodyHash = sha256Hex(body.getBytes(StandardCharsets.UTF_8));
            String signingString = matchId + "." + turn + "." + timestamp + "." + bodyHash;
            String expected = hmacSha256(secret, signingString);
            return expected.equals(signature);
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

    static class GameState {
        String matchId;
        int turn;
        Map<String, Object> config;
        int youId;
        int youEnergy;
        int youScore;
        List<Map<String, Object>> bots;
        List<Map<String, Object>> energy;
        List<Map<String, Object>> cores;
        List<Map<String, Object>> walls;
        List<Map<String, Object>> dead;
    }
}
