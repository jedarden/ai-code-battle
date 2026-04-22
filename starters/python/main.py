#!/usr/bin/env python3
"""
AI Code Battle - Python Starter Kit

A minimal bot scaffold. Implements the HTTP protocol with HMAC
authentication and a placeholder random strategy.

Usage:
    BOT_SECRET=your-secret python3 main.py
"""

import hashlib
import hmac
import json
import os
import random
from http.server import HTTPServer, BaseHTTPRequestHandler

# Engine constants
DIRECTIONS = ["N", "E", "S", "W"]


class GameState:
    def __init__(self, data: dict):
        self.match_id = data["match_id"]
        self.turn = data["turn"]
        self.config = data["config"]
        self.you_id = data["you"]["id"]
        self.you_energy = data["you"]["energy"]
        self.you_score = data["you"]["score"]
        self.bots = data.get("bots", [])
        self.energy = data.get("energy", [])
        self.cores = data.get("cores", [])
        self.walls = data.get("walls", [])
        self.dead = data.get("dead", [])


class BotHandler(BaseHTTPRequestHandler):
    secret: str = ""

    def log_message(self, format, *args):
        pass

    def sign_response(self, body: bytes, match_id: str, turn: int) -> str:
        body_hash = hashlib.sha256(body).hexdigest()
        signing_string = f"{match_id}.{turn}.{body_hash}"
        return hmac.new(
            self.secret.encode(), signing_string.encode(), hashlib.sha256
        ).hexdigest()

    def verify_signature(self, body: bytes, match_id: str, turn: str,
                         timestamp: str, signature: str) -> bool:
        body_hash = hashlib.sha256(body).hexdigest()
        signing_string = f"{match_id}.{turn}.{timestamp}.{body_hash}"
        expected = hmac.new(
            self.secret.encode(), signing_string.encode(), hashlib.sha256
        ).hexdigest()
        return hmac.compare_digest(signature, expected)

    def do_GET(self):
        if self.path == "/health":
            self.send_response(200)
            self.send_header("Content-Type", "text/plain")
            self.end_headers()
            self.wfile.write(b"OK")
        else:
            self.send_error(404)

    def do_POST(self):
        if self.path != "/turn":
            self.send_error(404)
            return

        content_length = int(self.headers.get("Content-Length", 0))
        body = self.rfile.read(content_length)

        match_id = self.headers.get("X-ACB-Match-Id", "")
        turn_str = self.headers.get("X-ACB-Turn", "0")
        timestamp = self.headers.get("X-ACB-Timestamp", "")
        signature = self.headers.get("X-ACB-Signature", "")

        if not signature or not self.verify_signature(
            body, match_id, turn_str, timestamp, signature
        ):
            self.send_error(401, "Invalid signature")
            return

        try:
            state = GameState(json.loads(body))
        except (json.JSONDecodeError, KeyError) as e:
            self.send_error(400, f"Invalid game state: {e}")
            return

        moves = compute_moves(state)
        turn = int(turn_str)

        response_body = json.dumps({"moves": moves}).encode()
        response_sig = self.sign_response(response_body, match_id, turn)

        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.send_header("X-ACB-Signature", response_sig)
        self.end_headers()
        self.wfile.write(response_body)


def compute_moves(state: GameState) -> list:
    """Replace this with your strategy!"""
    from grid import toroidal_manhattan

    rows = state.config["rows"]
    cols = state.config["cols"]
    moves = []

    for bot in state.bots:
        if bot["owner"] != state.you_id:
            continue

        br, bc = bot["position"]["row"], bot["position"]["col"]

        # Find nearest energy using toroidal distance
        if state.energy:
            best_dist = float("inf")
            best_dir = None
            for er, ec, d in _cardinal_moves(br, bc, rows, cols):
                for e in state.energy:
                    dist = toroidal_manhattan(er, ec, e["row"], e["col"], cols, rows)
                    if dist < best_dist:
                        best_dist = dist
                        best_dir = d
            if best_dir:
                moves.append({"position": bot["position"], "direction": best_dir})
                continue

        if random.random() < 0.5:
            moves.append({
                "position": bot["position"],
                "direction": random.choice(DIRECTIONS),
            })

    return moves


def _cardinal_moves(row, col, rows, cols):
    """Yield (new_row, new_col, direction) for each cardinal step with wrap."""
    for dr, dc, d in [(-1, 0, "N"), (0, 1, "E"), (1, 0, "S"), (0, -1, "W")]:
        yield (row + dr) % rows, (col + dc) % cols, d


def main():
    port = int(os.environ.get("BOT_PORT", "8080"))
    secret = os.environ.get("BOT_SECRET", "")

    if not secret:
        print("ERROR: BOT_SECRET environment variable is required")
        exit(1)

    BotHandler.secret = secret
    server = HTTPServer(("", port), BotHandler)
    print(f"Bot listening on port {port}")
    server.serve_forever()


if __name__ == "__main__":
    main()
