#!/usr/bin/env python3
"""
ScoutBot - Exploration-maximizing archetype.

Strategy:
- Maintains a last-seen tick counter per cell (from observation memory)
- Each unit moves toward the stalest nearby unobserved cell
- Flees if an enemy is within extended combat range
- Multiple bots are assigned to different map zones to maximize coverage
- New bots head to their assigned zone before exploring locally

Archetype axes: Low Aggression, Low Economy, High Exploration, Low Formation
"""

import hashlib
import hmac
import json
import math
import os
import time
from http.server import HTTPServer, BaseHTTPRequestHandler

DIRECTIONS = [("N", -1, 0), ("E", 0, 1), ("S", 1, 0), ("W", 0, -1)]

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

# Per-match persistent state
_seen = {}       # match_id -> dict of (row,col) -> last_seen_turn
_walls = {}      # match_id -> set of (row,col)


def _wrap(r, c, rows, cols):
    return r % rows, c % cols


def _dist2(r1, c1, r2, c2, rows, cols):
    dr = abs(r1 - r2)
    dc = abs(c1 - c2)
    dr = min(dr, rows - dr)
    dc = min(dc, cols - dc)
    return dr * dr + dc * dc


def _manhattan(r1, c1, r2, c2, rows, cols):
    dr = abs(r1 - r2)
    dc = abs(c1 - c2)
    dr = min(dr, rows - dr)
    dc = min(dc, cols - dc)
    return dr + dc


def _cardinal_neighbors(r, c, rows, cols):
    for d, dr, dc in DIRECTIONS:
        yield d, (r + dr) % rows, (c + dc) % cols


def _update_visibility(state):
    """Mark all cells within vision of owned bots as seen this turn."""
    mid = state.match_id
    turn = state.turn
    rows = state.config["rows"]
    cols = state.config["cols"]
    vision_r2 = state.config.get("vision_radius2", 49)
    vr = int(math.isqrt(vision_r2)) + 1

    if mid not in _seen:
        _seen[mid] = {}
        _walls[mid] = set()

    seen = _seen[mid]
    walls = _walls[mid]

    for w in state.walls:
        walls.add((w["row"], w["col"]))

    for bot in state.bots:
        if bot["owner"] != state.you_id:
            continue
        br = bot["position"]["row"]
        bc = bot["position"]["col"]
        for dr in range(-vr, vr + 1):
            for dc in range(-vr, vr + 1):
                if dr * dr + dc * dc > vision_r2:
                    continue
                r, c = _wrap(br + dr, bc + dc, rows, cols)
                seen[(r, c)] = turn


def _enemy_positions(state):
    return [
        (b["position"]["row"], b["position"]["col"])
        for b in state.bots
        if b["owner"] != state.you_id
    ]


def _should_flee(br, bc, enemies, rows, cols, attack_r2):
    flee_r2 = attack_r2 + 9
    for er, ec in enemies:
        if _dist2(br, bc, er, ec, rows, cols) <= flee_r2:
            return True
    return False


def _flee_direction(br, bc, enemies, rows, cols, walls):
    """Pick cardinal direction maximizing distance from enemies, avoiding walls."""
    best_dir = None
    best_min_dist = -1

    for d, nr, nc in _cardinal_neighbors(br, bc, rows, cols):
        if (nr, nc) in walls:
            continue
        min_dist = min(_dist2(nr, nc, er, ec, rows, cols) for er, ec in enemies)
        if min_dist > best_min_dist:
            best_min_dist = min_dist
            best_dir = d

    if best_dir is None:
        best_dir = "N"
    return best_dir


def _best_explore_direction(br, bc, seen, turn, rows, cols, walls, look_ahead=12):
    """Score each cardinal direction by staleness of cells in a forward cone.

    Each cell contributes (turns_since_last_seen) to the direction's score.
    Never-seen cells contribute (turn + 1), making them highest priority.
    This creates long sweeping motions across unexplored territory.
    """
    best_dir = None
    best_score = -1

    for d, dr, dc in DIRECTIONS:
        score = 0
        for step in range(1, look_ahead + 1):
            if dr != 0:  # Moving N/S — sample along column spread
                r = (br + dr * step) % rows
                spread = min(step + 2, cols // 2)
                for c_off in range(-spread, spread + 1):
                    c = (bc + c_off) % cols
                    if (r, c) in walls:
                        continue
                    last_seen = seen.get((r, c))
                    if last_seen is None:
                        score += turn + 1
                    else:
                        staleness = turn - last_seen
                        if staleness > 0:
                            score += staleness
            else:  # Moving E/W — sample along row spread
                c = (bc + dc * step) % cols
                spread = min(step + 2, rows // 2)
                for r_off in range(-spread, spread + 1):
                    r = (br + r_off) % rows
                    if (r, c) in walls:
                        continue
                    last_seen = seen.get((r, c))
                    if last_seen is None:
                        score += turn + 1
                    else:
                        staleness = turn - last_seen
                        if staleness > 0:
                            score += staleness
        if score > best_score:
            best_score = score
            best_dir = d

    return best_dir


def _direction_toward(br, bc, tr, tc, rows, cols, walls):
    """Pick cardinal direction that minimizes distance to target, avoiding walls."""
    best_dir = None
    best_dist = _manhattan(br, bc, tr, tc, rows, cols)

    for d, nr, nc in _cardinal_neighbors(br, bc, rows, cols):
        if (nr, nc) in walls:
            continue
        dist = _manhattan(nr, nc, tr, tc, rows, cols)
        if dist < best_dist:
            best_dist = dist
            best_dir = d

    return best_dir


def _assign_zone(bot_idx, total_bots, rows, cols, my_cores):
    """Assign an exploration zone for this bot based on its index.

    Distributes bots across the map using angular sectors from the
    core position, sending each bot to a different region.
    """
    if total_bots <= 1:
        return None

    if my_cores:
        cr = sum(c["position"]["row"] for c in my_cores) / len(my_cores)
        cc = sum(c["position"]["col"] for c in my_cores) / len(my_cores)
    else:
        cr, cc = rows / 2, cols / 2

    angle = 2 * math.pi * bot_idx / total_bots + math.pi
    tr = int(cr + rows * 0.4 * math.sin(angle)) % rows
    tc = int(cc + cols * 0.4 * math.cos(angle)) % cols
    return (tr, tc)


def compute_moves(state):
    """Compute exploration-focused moves for all owned bots."""
    rows = state.config["rows"]
    cols = state.config["cols"]
    attack_r2 = state.config.get("attack_radius2", 5)

    _update_visibility(state)

    seen = _seen.get(state.match_id, {})
    walls = _walls.get(state.match_id, set())
    turn = state.turn

    enemies = _enemy_positions(state)
    my_bots = [b for b in state.bots if b["owner"] == state.you_id]
    my_cores = [c for c in state.cores
                if c["owner"] == state.you_id and c.get("active", True)]

    moves = []
    claimed = set()
    for b in my_bots:
        claimed.add((b["position"]["row"], b["position"]["col"]))

    for idx, bot in enumerate(my_bots):
        br = bot["position"]["row"]
        bc = bot["position"]["col"]

        def _try_dir(d):
            """Return direction if destination is unclaimed, else None."""
            if d is None:
                return None
            for name, dr, dc in DIRECTIONS:
                if name == d:
                    nr, nc = (br + dr) % rows, (bc + dc) % cols
                    if (nr, nc) not in claimed:
                        return d
            return None

        # Priority 1: Flee if enemy nearby
        if enemies and _should_flee(br, bc, enemies, rows, cols, attack_r2):
            d = _try_dir(_flee_direction(br, bc, enemies, rows, cols, walls))
            if d:
                for name, dr, dc in DIRECTIONS:
                    if name == d:
                        claimed.discard((br, bc))
                        claimed.add(((br + dr) % rows, (bc + dc) % cols))
                moves.append({"position": bot["position"], "direction": d})
            continue

        # Priority 2: Multi-bot coordination — head to assigned zone if far
        if len(my_bots) > 1:
            zone = _assign_zone(idx, len(my_bots), rows, cols, my_cores)
            if zone:
                zr, zc = zone
                dist_to_zone = _manhattan(br, bc, zr, zc, rows, cols)
                if dist_to_zone > max(rows, cols) // 3:
                    d = _try_dir(_direction_toward(br, bc, zr, zc, rows, cols, walls))
                    if d:
                        for name, dr, dc in DIRECTIONS:
                            if name == d:
                                claimed.discard((br, bc))
                                claimed.add(((br + dr) % rows, (bc + dc) % cols))
                        moves.append({"position": bot["position"], "direction": d})
                    continue

        # Priority 3: Explore — move toward the direction with stalest territory
        d = _try_dir(_best_explore_direction(br, bc, seen, turn, rows, cols, walls))
        if d:
            for name, dr, dc in DIRECTIONS:
                if name == d:
                    claimed.discard((br, bc))
                    claimed.add(((br + dr) % rows, (bc + dc) % cols))
            moves.append({"position": bot["position"], "direction": d})
            continue

        # Fallback: spread from friendly bots to maximize coverage
        best_dir = "N"
        best_spread = -1
        for d, nr, nc in _cardinal_neighbors(br, bc, rows, cols):
            if (nr, nc) in walls:
                continue
            min_friend = float("inf")
            for other in my_bots:
                if other is bot:
                    continue
                or_, oc = other["position"]["row"], other["position"]["col"]
                min_friend = min(min_friend, _dist2(nr, nc, or_, oc, rows, cols))
            if min_friend > best_spread:
                best_spread = min_friend
                best_dir = d

        moves.append({"position": bot["position"], "direction": best_dir})

    return moves


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


class ScoutBotHandler(BaseHTTPRequestHandler):
    secret: str = ""

    def log_message(self, format, *args):
        pass

    def sign_response(self, body: bytes, match_id: str, turn: int) -> str:
        body_hash = hashlib.sha256(body).hexdigest()
        signing_string = f"{match_id}.{turn}.{body_hash}"
        return hmac.new(
            self.secret.encode("utf-8"),
            signing_string.encode("utf-8"),
            hashlib.sha256,
        ).hexdigest()

    def verify_signature(self, body: bytes, match_id: str, turn: str,
                         timestamp: str, signature: str) -> bool:
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
        expected = hmac.new(
            self.secret.encode("utf-8"),
            signing_string.encode("utf-8"),
            hashlib.sha256,
        ).hexdigest()
        return hmac.compare_digest(signature.encode("utf-8"), expected.encode("ascii"))

    def do_GET(self):
        if self.path == "/health":
            self.send_response(200)
            self.send_header("Content-Type", "text/plain")
            self.end_headers()
            self.wfile.write(b"OK")
        else:
            self.send_error(404, "Not Found")

    def do_POST(self):
        if self.path != "/turn":
            self.send_error(404, "Not Found")
            return

        # Content-Type is part of the transport contract (step 1 of
        # the documented verification order).
        if self.headers.get("Content-Type") != "application/json":
            self.send_error(401, "Invalid authentication")
            return

        content_length = int(self.headers.get("Content-Length", 0))
        body = self.rfile.read(content_length)

        match_id = self.headers.get("X-ACB-Match-Id", "")
        turn_str = self.headers.get("X-ACB-Turn", "")
        timestamp = self.headers.get("X-ACB-Timestamp", "")
        bot_id = self.headers.get("X-ACB-Bot-Id", "")
        signature = self.headers.get("X-ACB-Signature", "")

        if (not all(value and value.strip() for value in
                    (match_id, turn_str, timestamp, bot_id, signature))
                or not self.verify_signature(
                    body, match_id, turn_str, timestamp, signature
                )):
            self.send_error(401, "Invalid authentication")
            return

        try:
            turn = int(turn_str)
        except (TypeError, ValueError):
            self.send_error(401, "Invalid authentication")
            return

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

        moves = compute_moves(state)

        response_body = json.dumps({"moves": moves}).encode("utf-8")
        response_sig = self.sign_response(response_body, match_id, turn)

        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.send_header("X-ACB-Signature", response_sig)
        self.end_headers()
        self.wfile.write(response_body)


def main():
    port = int(os.environ.get("BOT_PORT", "8080"))
    secret = os.environ.get("BOT_SECRET", "")

    if not secret:
        print("ERROR: BOT_SECRET environment variable is required")
        exit(1)

    ScoutBotHandler.secret = secret
    server = HTTPServer(("", port), ScoutBotHandler)
    print(f"ScoutBot listening on port {port}")
    server.serve_forever()


if __name__ == "__main__":
    main()
