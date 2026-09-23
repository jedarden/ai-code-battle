/**
 * PacifistBot - Non-aggressive attrition archetype for AI Code Battle.
 *
 * Never attacks. Survives by evasion and hopes to outlast opponents
 * whose bots kill each other off.
 *
 * Uses the JavaScript starter kit pattern (zero external dependencies).
 */

const http = require("http");
const crypto = require("crypto");
const { computeMoves } = require("./strategy");

const PORT = parseInt(process.env.BOT_PORT || "8080", 10);
const SECRET = process.env.BOT_SECRET || "";

if (!SECRET) {
  console.error("ERROR: BOT_SECRET environment variable is required");
  process.exit(1);
}

// --- HMAC helpers ---

function getHeader(req, name) {
  const value = req.headers[name];
  return typeof value === "string" ? value : "";
}

function parseTurn(turn) {
  if (!/^(0|[1-9]\d*)$/.test(turn)) return null;
  const value = Number(turn);
  return Number.isSafeInteger(value) ? value : null;
}

function verifySignature(body, matchId, turn, timestamp, signature) {
  if (!/^[0-9a-fA-F]{64}$/.test(signature)) return false;
  const bodyHash = crypto.createHash("sha256").update(body).digest("hex");
  const signingString = `${matchId}.${turn}.${timestamp}.${bodyHash}`;
  const expected = crypto
    .createHmac("sha256", SECRET)
    .update(signingString)
    .digest("hex");
  const provided = Buffer.from(signature, "hex");
  const expectedBytes = Buffer.from(expected, "hex");
  return (
    provided.length === expectedBytes.length &&
    crypto.timingSafeEqual(provided, expectedBytes)
  );
}

function verifyTimestamp(timestamp) {
  const seconds = Number(timestamp);
  return (
    /^(0|[1-9]\d*)$/.test(timestamp) &&
    Number.isSafeInteger(seconds) &&
    Math.abs(Date.now() / 1000 - seconds) <= 30
  );
}

function signResponse(body, matchId, turn) {
  const bodyHash = crypto.createHash("sha256").update(body).digest("hex");
  const signingString = `${matchId}.${turn}.${bodyHash}`;
  return crypto
    .createHmac("sha256", SECRET)
    .update(signingString)
    .digest("hex");
}

// --- HTTP server ---

const server = http.createServer((req, res) => {
  if (req.method === "GET" && req.url === "/health") {
    res.writeHead(200, { "Content-Type": "text/plain" });
    res.end("OK");
    return;
  }

  if (req.method === "POST" && req.url === "/turn") {
    const chunks = [];
    req.on("data", (chunk) => chunks.push(chunk));
    req.on("end", () => {
      const body = Buffer.concat(chunks);

      const matchId = getHeader(req, "x-acb-match-id");
      const turn = getHeader(req, "x-acb-turn");
      const timestamp = getHeader(req, "x-acb-timestamp");
      const botId = getHeader(req, "x-acb-bot-id");
      const signature = getHeader(req, "x-acb-signature");
      const turnNumber = parseTurn(turn);

      if (
        !matchId ||
        turnNumber === null ||
        !timestamp ||
        !botId ||
        !signature
      ) {
        res.writeHead(401, { "Content-Type": "text/plain" });
        res.end("Missing auth headers");
        return;
      }

      if (!verifySignature(body, matchId, turn, timestamp, signature)) {
        res.writeHead(401, { "Content-Type": "text/plain" });
        res.end("Invalid signature");
        return;
      }

      if (!verifyTimestamp(timestamp)) {
        res.writeHead(401, { "Content-Type": "text/plain" });
        res.end("Invalid timestamp");
        return;
      }

      let state;
      try {
        state = JSON.parse(body.toString("utf8"));
      } catch {
        res.writeHead(400, { "Content-Type": "text/plain" });
        res.end("Invalid JSON");
        return;
      }

      if (
        state === null ||
        typeof state !== "object" ||
        state.match_id !== matchId ||
        state.turn !== turnNumber
      ) {
        res.writeHead(401, { "Content-Type": "text/plain" });
        res.end("Invalid request identity");
        return;
      }

      const moves = computeMoves(state);
      const responseBody = Buffer.from(JSON.stringify({ moves }));
      const responseSig = signResponse(responseBody, matchId, turnNumber);

      console.log(`Turn ${state.turn}: ${moves.length} moves`);

      res.writeHead(200, {
        "Content-Type": "application/json",
        "X-ACB-Signature": responseSig,
      });
      res.end(responseBody);
    });
    return;
  }

  res.writeHead(404);
  res.end("Not Found");
});

server.listen(PORT, () => {
  console.log(`PacifistBot listening on port ${PORT}`);
});
