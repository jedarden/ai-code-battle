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
  // The contract requires exactly 64 lowercase hex characters. The guard
  // must run before the byte comparison below: Buffer.from(..., "hex")
  // accepts uppercase, so an uppercase spelling of an otherwise valid
  // signature would decode to the same bytes and verify.
  if (!/^[0-9a-f]{64}$/.test(signature)) return false;
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
  // Response signing string: {match_id}.{turn}.{sha256_hex(body)}
  const bodyHash = createHash("sha256").update(body).digest("hex");
  const signingString = `${matchId}.${turn}.${bodyHash}`;
  return createHmac("sha256", secret).update(signingString).digest("hex");
}

/**
 * Verify that a timestamp is within the allowed window.
 * Prevents replay attacks.
 *
 * @param timestamp - Unix timestamp in seconds
 * @param windowSeconds - Allowed window (default: 30)
 * @returns true if timestamp is valid
 */
export function verifyTimestamp(
  timestamp: string,
  windowSeconds: number = 30
): boolean {
  const seconds = Number(timestamp);
  return (
    /^(0|[1-9]\d*)$/.test(timestamp) &&
    Number.isSafeInteger(seconds) &&
    Math.abs(Date.now() / 1000 - seconds) <= windowSeconds
  );
}

/**
 * Extract auth headers from a Fastify request.
 */
export function getAuthHeaders(
  headers: Record<string, string | string[] | undefined>
): {
  matchId: string;
  turn: string;
  timestamp: string;
  botId: string;
  signature: string;
} {
  return {
    matchId: getHeaderValue(headers, "x-acb-match-id"),
    turn: getHeaderValue(headers, "x-acb-turn"),
    timestamp: getHeaderValue(headers, "x-acb-timestamp"),
    botId: getHeaderValue(headers, "x-acb-bot-id"),
    signature: getHeaderValue(headers, "x-acb-signature"),
  };
}

function getHeaderValue(
  headers: Record<string, string | string[] | undefined>,
  name: string
): string {
  const value = headers[name];
  return typeof value === "string" ? value : "";
}
