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
import time
from http.server import HTTPServer, BaseHTTPRequestHandler

# --- Request schema (docs/bot-protocol.md) ---
REQUIRED_TOP = ("match_id", "turn", "config", "you", "bots", "energy",
                "cores", "walls", "dead")
REQUIRED_CONFIG = ("rows", "cols", "max_turns", "vision_radius2",
                   "attack_radius2", "spawn_cost", "energy_interval",
                   "cores_per_player", "zone_enabled", "zone_start_turn",
                   "zone_shrink_interval", "zone_shrink_step",
                   "zone_min_radius", "kill_score")
OPTIONAL_CONFIG = ("map_id", "season_id", "rules_version", "turn_timeout")


def _json_int(value) -> bool:
    """JSON integer: bool is a distinct JSON type and never counts."""
    return type(value) is int


def _position_error(position, where):
    if not isinstance(position, dict):
        return f"{where} must be an object"
    for key in position:
        if key not in ("row", "col"):
            return f"unknown {where} field: {key}"
    if not _json_int(position.get("row")) or not _json_int(position.get("col")):
        return f"{where} requires integer row/col"
    return None


def _element_error(element, shape, where):
    if not isinstance(element, dict):
        return f"{where} elements must be objects"
    if shape == "point":
        for key in element:
            if key not in ("row", "col"):
                return f"unknown {where} element field: {key}"
        if not _json_int(element.get("row")) or not _json_int(element.get("col")):
            return f"{where} elements require integer row/col"
        return None
    allowed = ("position", "owner", "active") if shape == "core" else ("position", "owner")
    for key in element:
        if key not in allowed:
            return f"unknown {where} element field: {key}"
    error = _position_error(element.get("position"), f"{where} element position")
    if error is not None:
        return error
    if not _json_int(element.get("owner")):
        return f"{where} elements require integer owner"
    if shape == "core" and not isinstance(element.get("active"), bool):
        return f"{where} elements require boolean active"
    return None


def validate_request_schema(state):
    """Return None when the body is schema-conformant, else a short reason.

    Runs after signature verification: a failure here is authenticated
    malformed input (400), never an authentication failure. Unknown fields
    are rejected everywhere the contract closes an object (top level,
    config, you, zone, zone.center, and every array element), and required
    fields must carry the documented JSON types.
    """
    if not isinstance(state, dict):
        return "request must be one JSON object"
    known_top = frozenset(REQUIRED_TOP).union({"zone"})
    for key in state:
        if key not in known_top:
            return f"unknown field: {key}"
    for key in REQUIRED_TOP:
        if key not in state:
            return f"missing required field: {key}"
    if not isinstance(state["match_id"], str) or not state["match_id"]:
        return "match_id must be a non-empty string"
    if not _json_int(state["turn"]):
        return "turn must be an integer"

    config = state["config"]
    if not isinstance(config, dict):
        return "config must be an object"
    known_config = frozenset(REQUIRED_CONFIG + OPTIONAL_CONFIG)
    for key in config:
        if key not in known_config:
            return f"unknown config field: {key}"
    for key in REQUIRED_CONFIG:
        if key not in config:
            return f"config missing required field: {key}"
        if key == "zone_enabled":
            if type(config[key]) is not bool:
                return "config.zone_enabled must be a boolean"
        elif not _json_int(config[key]):
            return f"config.{key} must be an integer"
    for key in OPTIONAL_CONFIG:
        if key in config:
            if key == "turn_timeout":
                if not _json_int(config[key]):
                    return "config.turn_timeout must be an integer"
            elif not isinstance(config[key], str):
                return f"config.{key} must be a string"

    you = state["you"]
    if not isinstance(you, dict):
        return "you must be an object"
    for key in you:
        if key not in ("id", "energy", "score"):
            return f"unknown you field: {key}"
    for key in ("id", "energy", "score"):
        if key not in you:
            return f"you missing required field: {key}"
        if not _json_int(you[key]):
            return f"you.{key} must be an integer"

    for key in ("bots", "energy", "cores", "walls", "dead"):
        if not isinstance(state[key], list):
            return f"{key} must be an array"
    for element in state["bots"]:
        error = _element_error(element, "bot", "bots")
        if error is not None:
            return error
    for element in state["dead"]:
        error = _element_error(element, "bot", "dead")
        if error is not None:
            return error
    for element in state["energy"]:
        error = _element_error(element, "point", "energy")
        if error is not None:
            return error
    for element in state["walls"]:
        error = _element_error(element, "point", "walls")
        if error is not None:
            return error
    for element in state["cores"]:
        error = _element_error(element, "core", "cores")
        if error is not None:
            return error

    zone = state.get("zone")
    if zone is not None:
        if not isinstance(zone, dict):
            return "zone must be an object"
        for key in zone:
            if key not in ("center", "radius", "active"):
                return f"unknown zone field: {key}"
        for key in ("center", "radius", "active"):
            if key not in zone:
                return f"zone missing required field: {key}"
        error = _position_error(zone.get("center"), "zone.center")
        if error is not None:
            return error
        if not _json_int(zone.get("radius")):
            return "zone.radius must be an integer"
        if not isinstance(zone.get("active"), bool):
            return "zone.active must be a boolean"

    return None


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

        # Content-Type is part of the transport contract (step 1 of
        # the documented verification order).
        if self.headers.get("Content-Type") != "application/json":
            self.send_error(401, "Invalid authentication")
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

        # Parse game state
        try:
            data = json.loads(body)
        except (json.JSONDecodeError, UnicodeDecodeError) as e:
            self.send_error(400, f"Invalid game state: {e}")
            return

        # Strict schema before identity: a missing, unknown, or mis-typed
        # field is authenticated malformed input (400); present but
        # contradictory values are an authentication failure (401).
        schema_error = validate_request_schema(data)
        if schema_error is not None:
            self.send_error(400, f"Invalid request schema: {schema_error}")
            return

        if data["match_id"] != match_id or data["turn"] != turn:
            self.send_error(401, "Request identity mismatch")
            return

        try:
            state = GameState(data)
        except (KeyError, TypeError) as e:
            self.send_error(400, f"Invalid game state: {e}")
            return

        # Compute random moves
        moves = self.compute_moves(state)

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
