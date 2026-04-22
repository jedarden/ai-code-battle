/**
 * AI Code Battle - JavaScript (Node.js) Starter Kit
 *
 * A minimal bot scaffold with HMAC authentication and a placeholder
 * random strategy. Replace computeMoves() with your own logic.
 */

const http = require("http");
const crypto = require("crypto");

const PORT = parseInt(process.env.BOT_PORT || "8080", 10);
const SECRET = process.env.BOT_SECRET || "";

if (!SECRET) {
  console.error("ERROR: BOT_SECRET environment variable is required");
  process.exit(1);
}

const DIRECTIONS = ["N", "E", "S", "W"];

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

// --- Strategy ---

function computeMoves(state) {
  // Replace this with your strategy!
  const { toroidalManhattan } = require("./grid");

  const rows = state.config.rows;
  const cols = state.config.cols;
  const moves = [];

  const cardinalSteps = [
    { dr: -1, dc: 0, dir: "N" },
    { dr: 0, dc: 1, dir: "E" },
    { dr: 1, dc: 0, dir: "S" },
    { dr: 0, dc: -1, dir: "W" },
  ];

  for (const bot of state.bots) {
    if (bot.owner !== state.you.id) continue;

    const br = bot.position.row;
    const bc = bot.position.col;

    // Find direction toward nearest energy using toroidal distance
    if (state.energy && state.energy.length > 0) {
      let bestDist = Infinity;
      let bestDir = null;
      for (const { dr, dc, dir } of cardinalSteps) {
        const nr = (br + dr + rows) % rows;
        const nc = (bc + dc + cols) % cols;
        for (const e of state.energy) {
          const dist = toroidalManhattan(nr, nc, e.row, e.col, cols, rows);
          if (dist < bestDist) {
            bestDist = dist;
            bestDir = dir;
          }
        }
      }
      if (bestDir) {
        moves.push({ position: bot.position, direction: bestDir });
        continue;
      }
    }

    if (Math.random() < 0.5) {
      moves.push({
        position: bot.position,
        direction: DIRECTIONS[Math.floor(Math.random() * DIRECTIONS.length)],
      });
    }
  }
  return moves;
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

      if (state.turn === 0) {
        const seasonId = state.config.season_id || "";
        const rulesVersion = state.config.rules_version || "";
        console.log(
          `match=${state.match_id} season_id=${seasonId} rules_version=${rulesVersion} rows=${state.config.rows} cols=${state.config.cols}`
        );
      }

      const moves = computeMoves(state);
      const responseBody = JSON.stringify({ moves });
      const responseSig = signResponse(
        Buffer.from(responseBody),
        matchId,
        parseInt(turn, 10)
      );

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
  console.log(`Bot listening on port ${PORT}`);
});
