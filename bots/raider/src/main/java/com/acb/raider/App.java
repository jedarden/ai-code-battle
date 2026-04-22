package com.acb.raider;

import io.javalin.Javalin;
import io.javalin.http.Context;

import javax.crypto.Mac;
import javax.crypto.spec.SecretKeySpec;
import java.nio.charset.StandardCharsets;
import java.security.MessageDigest;
import java.util.HexFormat;

/**
 * RaiderBot - Hit-and-run harasser archetype.
 *
 * Strategy: Attack weak targets, disengage before reinforcements arrive.
 * - Units scout for lone enemy bots (no allies within 2 cells)
 * - On finding one, attack from flank
 * - After 1-2 attack turns, retreat regardless of outcome
 * - Never attack groups of >=3 enemies
 * - Home base rotates: if own core under pressure, abandon raid and defend
 */
public class App {
    private static final int DEFAULT_PORT = 8086;
    private static String SECRET;
    private static final RaiderStrategy STRATEGY = new RaiderStrategy();

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
        System.out.println("RaiderBot starting on port " + port);
    }

    private static void handleTurn(Context ctx) {
        String matchId = ctx.header("X-ACB-Match-Id");
        String turnStr = ctx.header("X-ACB-Turn");
        String timestamp = ctx.header("X-ACB-Timestamp");
        String signature = ctx.header("X-ACB-Signature");

        if (matchId == null || turnStr == null || timestamp == null || signature == null) {
            ctx.status(401).result("Missing auth headers");
            return;
        }

        String body = ctx.body();

        if (!verifySignature(SECRET, matchId, turnStr, timestamp, body, signature)) {
            ctx.status(401).result("Invalid signature");
            return;
        }

        GameState state;
        try {
            state = GameState.fromJson(body);
        } catch (Exception e) {
            ctx.status(400).result("Invalid JSON: " + e.getMessage());
            return;
        }

        var moves = STRATEGY.computeMoves(state);
        int turn = Integer.parseInt(turnStr);

        System.out.println("Turn " + turn + ": " + moves.size() + " moves computed");

        String responseBody = MoveResponse.toJson(moves);
        String responseSig = signResponse(SECRET, matchId, turn, responseBody);

        ctx.header("X-ACB-Signature", responseSig);
        ctx.contentType("application/json");
        ctx.result(responseBody);
    }

    private static boolean verifySignature(String secret, String matchId, String turn,
                                           String timestamp, String body, String signature) {
        try {
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

    private static String signResponse(String secret, String matchId, int turn, String body) {
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

    private static String sha256Hex(String input) {
        try {
            MessageDigest digest = MessageDigest.getInstance("SHA-256");
            return HexFormat.of().formatHex(digest.digest(input.getBytes(StandardCharsets.UTF_8)));
        } catch (Exception e) {
            throw new RuntimeException("Failed to hash", e);
        }
    }
}
