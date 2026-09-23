#!/usr/bin/env python3
"""
AI Code Battle - Python Starter Bot

A minimal HTTP bot server with HMAC authentication.
"""

import hashlib
import hmac
import json
import os
import time
from http.server import HTTPServer, BaseHTTPRequestHandler
from strategy import compute_moves


class BotHandler(BaseHTTPRequestHandler):
    """HTTP request handler for the bot."""

    # Class variable set from environment
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
        if not all(value and value.strip() for value in
                   (match_id, turn, timestamp, signature)):
            return False
        if not self.secret:
            return False
        try:
            turn_number = int(turn)
            request_time = int(timestamp)
        except (TypeError, ValueError):
            return False
        if turn_number < 0:
            return False
        try:
            if abs(time.time() - request_time) > 30:
                return False
        except OverflowError:
            return False
        body_hash = hashlib.sha256(body).hexdigest()
        signing_string = f"{match_id}.{turn}.{timestamp}.{body_hash}"
        expected_sig = hmac.new(
            self.secret.encode("utf-8"),
            signing_string.encode("utf-8"),
            hashlib.sha256
        ).hexdigest()
        return hmac.compare_digest(signature.encode("utf-8"), expected_sig.encode("ascii"))

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
        turn_str = self.headers.get("X-ACB-Turn", "")
        timestamp = self.headers.get("X-ACB-Timestamp", "")
        bot_id = self.headers.get("X-ACB-Bot-Id", "")
        signature = self.headers.get("X-ACB-Signature", "")

        if (not all(value and value.strip() for value in
                    (match_id, turn_str, timestamp, bot_id, signature))
                or not self.verify_signature(body, match_id, turn_str, timestamp, signature)):
            self.send_error(401, "Invalid authentication")
            return

        try:
            turn = int(turn_str)
        except (TypeError, ValueError):
            self.send_error(401, "Invalid authentication")
            return

        # Parse state
        try:
            state = json.loads(body.decode("utf-8"))
        except (json.JSONDecodeError, UnicodeDecodeError):
            self.send_error(400, "Invalid JSON")
            return

        if (not isinstance(state, dict)
                or state.get("match_id") != match_id
                or type(state.get("turn")) is not int
                or state.get("turn") != turn):
            self.send_error(401, "Request identity mismatch")
            return

        # Compute moves using your strategy
        moves = compute_moves(state)

        # Send response
        self.send_json_response(200, {"moves": moves}, match_id, turn)


def main():
    """Start the bot server."""
    # Get shared secret from environment
    secret = os.environ.get("SHARED_SECRET", "")
    if not secret:
        raise ValueError("SHARED_SECRET environment variable must be set")

    BotHandler.secret = secret

    # Start server
    port = int(os.environ.get("PORT", "8080"))
    server = HTTPServer(("0.0.0.0", port), BotHandler)
    print(f"Bot listening on port {port}")
    server.serve_forever()


if __name__ == "__main__":
    main()
