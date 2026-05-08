/**
 * AI Code Battle - HMAC Authentication
 *
 * Implements HMAC-SHA256 signing and verification for the game protocol.
 */

import { createHash, createHmac, timingSafeEqual } from "node:crypto";

/**
 * Verify the HMAC signature on an incoming request.
 *
 * @param body - Raw request body as Buffer
 * @param matchId - Match ID from header
 * @param turn - Turn number from header
 * @param timestamp - Timestamp from header
 * @param signature - X-ACB-Signature header value
 * @param secret - Your bot's shared secret
 * @returns true if signature is valid
 */
export function verifySignature(
  body: Buffer,
  matchId: string,
  turn: string,
  timestamp: string,
  signature: string,
  secret: string
): boolean {
  const bodyHash = createHash("sha256").update(body).digest("hex");
  const signingString = `${matchId}.${turn}.${timestamp}.${bodyHash}`;
  const expected = createHmac("sha256", secret)
    .update(signingString)
    .digest("hex");

  // Constant-time comparison to prevent timing attacks
  try {
    return timingSafeEqual(
      Buffer.from(signature, "hex"),
      Buffer.from(expected, "hex")
    );
  } catch {
    return false;
  }
}

/**
 * Generate HMAC signature for a response.
 *
 * @param body - Response body as string or Buffer
 * @param matchId - Match ID
 * @param turn - Turn number
 * @param secret - Your bot's shared secret
 * @returns Hex-encoded signature
 */
export function signResponse(
  body: string | Buffer,
  matchId: string,
  turn: number,
  secret: string
): string {
  const bodyStr = typeof body === "string" ? body : body.toString();
  const bodyHash = createHash("sha256").update(bodyStr).digest("hex");
  const signingString = `${matchId}.${turn}.${bodyHash}`;
  return createHmac("sha256", secret).update(signingString).digest("hex");
}

/**
 * Verify that a timestamp is within the allowed window.
 * Prevents replay attacks.
 *
 * @param timestamp - Unix timestamp or ISO 8601 string
 * @param windowSeconds - Allowed window (default: 30)
 * @returns true if timestamp is valid
 */
export function verifyTimestamp(
  timestamp: string,
  windowSeconds: number = 30
): boolean {
  let ts: Date;

  // Try ISO 8601 first
  const parsed = new Date(timestamp);
  if (!isNaN(parsed.getTime())) {
    ts = parsed;
  } else {
    // Try Unix timestamp (seconds since epoch)
    const seconds = parseInt(timestamp, 10);
    if (isNaN(seconds)) {
      return false;
    }
    ts = new Date(seconds * 1000);
  }

  const now = new Date();
  const diff = (now.getTime() - ts.getTime()) / 1000;
  return diff >= -windowSeconds && diff <= windowSeconds;
}

/**
 * Extract auth headers from a Fastify request.
 */
export function getAuthHeaders(headers: Record<string, string>): {
  matchId: string;
  turn: string;
  timestamp: string;
  botId: string;
  signature: string;
} {
  return {
    matchId: headers["x-acb-match-id"] || "",
    turn: headers["x-acb-turn"] || "0",
    timestamp: headers["x-acb-timestamp"] || "",
    botId: headers["x-acb-bot-id"] || "",
    signature: headers["x-acb-signature"] || "",
  };
}
