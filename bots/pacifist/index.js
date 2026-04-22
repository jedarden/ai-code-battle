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

function verifySignature(body, matchId, turn, timestamp, signature) {
  const bodyHash = crypto.createHash("sha256").update(body).digest("hex");
  const signingString = `${matchId}.${turn}.${timestamp}.${bodyHash}`;
  const expected = crypto
    .createHmac("sha256", SECRET)
    .update(signingString)
    .digest("hex");
  return crypto.timingSafeEqual(
    Buffer.from(signature, "hex"),
    Buffer.from(expected, "hex")
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

      const matchId = req.headers["x-acb-match-id"] || "";
      const turn = req.headers["x-acb-turn"] || "0";
      const timestamp = req.headers["x-acb-timestamp"] || "";
      const signature = req.headers["x-acb-signature"] || "";

      if (
        !signature ||
        !verifySignature(body, matchId, turn, timestamp, signature)
      ) {
        res.writeHead(401, { "Content-Type": "text/plain" });
        res.end("Invalid signature");
        return;
      }

      let state;
      try {
        state = JSON.parse(body.toString());
      } catch {
        res.writeHead(400, { "Content-Type": "text/plain" });
        res.end("Invalid JSON");
        return;
      }

      const moves = computeMoves(state);
      const responseBody = JSON.stringify({ moves });
      const responseSig = signResponse(
        Buffer.from(responseBody),
        matchId,
        parseInt(turn, 10)
      );

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
