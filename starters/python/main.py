#!/usr/bin/env python3
"""AI Code Battle - Python Starter Kit.

Flask-based HTTP bot with HMAC authentication. Implement your strategy
in compute_moves() below.

Usage:
    BOT_SECRET=your-secret python3 main.py
"""

import hashlib
import hmac
import json
import os
import time
from typing import List, Dict, Any

from flask import Flask, request, jsonify

app = Flask(__name__)
SECRET = os.environ.get("BOT_SECRET", "")
DIRECTIONS = ["N", "E", "S", "W"]


def verify_signature(body: bytes, match_id: str, turn: str,
                     timestamp: str, signature: str) -> bool:
    """Verify HMAC-SHA256 signature from engine."""
    try:
        ts = int(timestamp)
        now = int(time.time())
        if abs(now - ts) > 30:
            return False
    except (ValueError, TypeError):
        return False

    body_hash = hashlib.sha256(body).hexdigest()
    signing_string = f"{match_id}.{turn}.{timestamp}.{body_hash}"
    expected = hmac.new(
        SECRET.encode(), signing_string.encode(), hashlib.sha256
    ).hexdigest()
    return hmac.compare_digest(signature, expected)


def sign_response(body: bytes, match_id: str, turn: int) -> str:
    """Generate HMAC-SHA256 signature for response."""
    body_hash = hashlib.sha256(body).hexdigest()
    signing_string = f"{match_id}.{turn}.{body_hash}"
    return hmac.new(
        SECRET.encode(), signing_string.encode(), hashlib.sha256
    ).hexdigest()


@app.route("/health", methods=["GET"])
def health():
    """Health check endpoint."""
    return "OK", 200


@app.route("/turn", methods=["POST"])
def turn():
    """Main game turn endpoint."""
    match_id = request.headers.get("X-ACB-Match-Id", "")
    turn_str = request.headers.get("X-ACB-Turn", "0")
    timestamp = request.headers.get("X-ACB-Timestamp", "")
    signature = request.headers.get("X-ACB-Signature", "")

    body = request.get_data()
    if not signature or not verify_signature(body, match_id, turn_str, timestamp, signature):
        return "Invalid signature", 401

    try:
        state = json.loads(body)
    except json.JSONDecodeError:
        return "Invalid JSON", 400

    moves = compute_moves(state)
    turn_num = int(turn_str)

    response_data = {"moves": moves}
    response_body = json.dumps(response_data).encode()
    response_sig = sign_response(response_body, match_id, turn_num)

    response = jsonify(response_data)
    response.headers["X-ACB-Signature"] = response_sig
    return response


def compute_moves(state: Dict[str, Any]) -> List[Dict[str, Any]]:
    """YOUR STRATEGY GOES HERE.

    Args:
        state: Game state dict with keys:
            - match_id: str
            - turn: int
            - config: dict (rows, cols, max_turns, vision_radius2, attack_radius2, etc.)
            - you: dict (id, energy, score)
            - bots: list of dict (each has row, col, owner)
            - energy: list of dict (each has row, col)
            - cores: list of dict (each has row, col, owner, active)
            - walls: list of dict (each has row, col)
            - dead: list of dict (each has row, col, owner)

    Returns:
        List of move dicts, each with:
            - row: int (your bot's current row)
            - col: int (your bot's current col)
            - direction: "N" | "E" | "S" | "W"
    """
    import random

    my_id = state["you"]["id"]
    rows, cols = state["config"]["rows"], state["config"]["cols"]

    moves = []
    for bot in state.get("bots", []):
        if bot["owner"] != my_id:
            continue

        # STUB: hold all bots in place (return empty list)
        # Replace this with your strategy!
        pass

    return moves


if __name__ == "__main__":
    if not SECRET:
        print("ERROR: BOT_SECRET environment variable is required")
        exit(1)

    port = int(os.environ.get("BOT_PORT", "8080"))
    app.run(host="0.0.0.0", port=port, debug=False)
