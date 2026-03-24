#!/usr/bin/env python3
"""
RandomBot - A bot that makes random valid moves.

This is a reference implementation demonstrating the HTTP protocol
in Python. It validates HMAC signatures and returns random moves.
"""

import hashlib
import hmac
import json
import os
import random
from http.server import HTTPServer, BaseHTTPRequestHandler


class GameState:
    """Represents the fog-filtered state visible to this bot."""

    def __init__(self, data: dict):
        self.match_id = data["match_id"]
        self.turn = data["turn"]
        self.config = data["config"]
        self.you_id = data["you"]["id"]
        self.you_energy = data["you"]["energy"]
        self.you_score = data["you"]["score"]
        self.bots = data["bots"]
        self.energy = data.get("energy", [])
        self.cores = data.get("cores", [])
        self.walls = data.get("walls", [])
        self.dead = data.get("dead", [])


class RandomBotHandler(BaseHTTPRequestHandler):
    """HTTP request handler for RandomBot."""

    secret: str = ""

    def log_message(self, format, *args):
        """Suppress default logging."""
        pass

    def send_json_response(self, status: int, data: dict, match_id: str = "", turn: int = 0):
        """Send a JSON response with HMAC signature."""
        body = json.dumps(data).encode("utf-8")

        # Sign response
        sig = self.sign_response(body, match_id, turn)

        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.send_header("X-ACB-Signature", sig)
        self.end_headers()
        self.wfile.write(body)

    def sign_response(self, body: bytes, match_id: str, turn: int) -> str:
        """Generate HMAC signature for response."""
        body_hash = hashlib.sha256(body).hexdigest()
        signing_string = f"{match_id}.{turn}.{body_hash}"
        sig = hmac.new(
            self.secret.encode("utf-8"),
            signing_string.encode("utf-8"),
            hashlib.sha256
        ).hexdigest()
        return sig

    def verify_signature(self, body: bytes, match_id: str, turn: str,
                         timestamp: str, signature: str) -> bool:
        """Verify HMAC signature of incoming request."""
        body_hash = hashlib.sha256(body).hexdigest()
        signing_string = f"{match_id}.{turn}.{timestamp}.{body_hash}"
        expected_sig = hmac.new(
            self.secret.encode("utf-8"),
            signing_string.encode("utf-8"),
            hashlib.sha256
        ).hexdigest()
        return hmac.compare_digest(signature, expected_sig)

    def do_GET(self):
        """Handle GET requests (health check)."""
        if self.path == "/health":
            self.send_response(200)
            self.send_header("Content-Type", "text/plain")
            self.end_headers()
            self.wfile.write(b"OK")
        else:
            self.send_error(404, "Not Found")

    def do_POST(self):
        """Handle POST requests (turn)."""
        if self.path != "/turn":
            self.send_error(404, "Not Found")
            return

        # Read body
        content_length = int(self.headers.get("Content-Length", 0))
        body = self.rfile.read(content_length)

        # Get auth headers
        match_id = self.headers.get("X-ACB-Match-Id", "")
        turn_str = self.headers.get("X-ACB-Turn", "0")
        timestamp = self.headers.get("X-ACB-Timestamp", "")
        signature = self.headers.get("X-ACB-Signature", "")

        if not signature:
            self.send_error(401, "Missing signature")
            return

        # Verify signature
        if not self.verify_signature(body, match_id, turn_str, timestamp, signature):
            self.send_error(401, "Invalid signature")
            return

        # Parse game state
        try:
            data = json.loads(body)
            state = GameState(data)
        except (json.JSONDecodeError, KeyError) as e:
            self.send_error(400, f"Invalid game state: {e}")
            return

        # Compute random moves
        moves = self.compute_moves(state)
        turn = int(turn_str)

        # Send response
        self.send_json_response(200, {"moves": moves}, match_id, turn)

    def compute_moves(self, state: GameState) -> list:
        """Compute random moves for all owned bots."""
        moves = []
        directions = ["N", "E", "S", "W"]

        for bot in state.bots:
            if bot["owner"] == state.you_id:
                # 50% chance to move, 50% chance to stay still
                if random.random() < 0.5:
                    direction = random.choice(directions)
                    moves.append({
                        "position": bot["position"],
                        "direction": direction
                    })

        return moves


def main():
    port = int(os.environ.get("BOT_PORT", "8081"))
    secret = os.environ.get("BOT_SECRET", "")

    if not secret:
        print("ERROR: BOT_SECRET environment variable is required")
        exit(1)

    RandomBotHandler.secret = secret

    server = HTTPServer(("", port), RandomBotHandler)
    print(f"RandomBot starting on port {port}")
    server.serve_forever()


if __name__ == "__main__":
    main()
