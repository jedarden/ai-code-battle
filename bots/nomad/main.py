"""NomadBot — constant relocation archetype.

Mobile archetype that never stays in one region. Picks a target region every
~20 turns, migrates all units toward it, briefly engages enemies on arrival,
then picks a new region. Spawns join the current migration.

Archetype axes: Medium Aggression, Low Economy, High Exploration, Low Formation.
"""

import hashlib
import hmac
import json
import math
import os
import random
from http.server import HTTPServer, BaseHTTPRequestHandler

from grid import toroidal_manhattan, bfs

# ── Match state ──────────────────────────────────────────────────────────────

_matches: dict = {}  # match_id -> MatchState


class MatchState:
    """Persistent state for a single match."""

    def __init__(self, match_id: str):
        self.match_id = match_id
        self.target: tuple[int, int] | None = None
        self.target_turn: int = -999  # turn when current target was set
        self.arrived: bool = False
        self.arrive_turn: int = -999  # turn when group arrived at target

    def is_stale(self, turn: int) -> bool:
        return turn - self.target_turn > 200


def _get_state(match_id: str, turn: int) -> MatchState:
    if match_id not in _matches:
        _matches[match_id] = MatchState(match_id)
    state = _matches[match_id]
    if state.is_stale(turn):
        del _matches[match_id]
        _matches[match_id] = MatchState(match_id)
        state = _matches[match_id]
    return state


# ── Game state parsing ───────────────────────────────────────────────────────


class GameState:
    __slots__ = (
        "match_id", "turn", "rows", "cols",
        "vision_radius2", "attack_radius2", "spawn_cost", "energy_interval",
        "max_turns",
        "you_id", "you_energy", "you_score",
        "bots", "energy", "cores", "walls", "dead",
    )

    def __init__(self, raw: dict):
        self.match_id = raw["match_id"]
        self.turn = raw["turn"]
        cfg = raw["config"]
        self.rows = cfg["rows"]
        self.cols = cfg["cols"]
        self.vision_radius2 = cfg["vision_radius2"]
        self.attack_radius2 = cfg["attack_radius2"]
        self.spawn_cost = cfg["spawn_cost"]
        self.energy_interval = cfg["energy_interval"]
        self.max_turns = cfg["max_turns"]

        you = raw["you"]
        self.you_id = you["id"]
        self.you_energy = you["energy"]
        self.you_score = you["score"]

        self.bots = raw["bots"]
        self.energy = raw["energy"]
        self.cores = raw["cores"]
        self.walls = raw["walls"]
        self.dead = raw["dead"]


# ── Helpers ──────────────────────────────────────────────────────────────────

DIRECTIONS = {
    "N": (-1, 0),
    "S": (1, 0),
    "E": (0, 1),
    "W": (0, -1),
}


def _wrap(r: int, c: int, rows: int, cols: int) -> tuple[int, int]:
    return r % rows, c % cols


def _dist2(r1: int, c1: int, r2: int, c2: int, rows: int, cols: int) -> int:
    dr = abs(r1 - r2)
    dc = abs(c1 - c2)
    dr = min(dr, rows - dr)
    dc = min(dc, cols - dc)
    return dr * dr + dc * dc


def _centroid(positions: list[tuple[int, int]], rows: int, cols: int) -> tuple[int, int]:
    """Compute centroid of positions on a toroidal grid using mean of circular coords."""
    if not positions:
        return rows // 2, cols // 2
    sum_sin_r = sum(math.sin(2 * math.pi * r / rows) for r, _ in positions)
    sum_cos_r = sum(math.cos(2 * math.pi * r / rows) for r, _ in positions)
    sum_sin_c = sum(math.sin(2 * math.pi * c / cols) for _, c in positions)
    sum_cos_c = sum(math.cos(2 * math.pi * c / cols) for _, c in positions)
    n = len(positions)
    cr = (math.atan2(sum_sin_r / n, sum_cos_r / n) / (2 * math.pi) * rows) % rows
    cc = (math.atan2(sum_sin_c / n, sum_cos_c / n) / (2 * math.pi) * cols) % cols
    return int(round(cr)) % rows, int(round(cc)) % cols


def _cardinal_moves(r: int, c: int, rows: int, cols: int):
    for d, (dr, dc) in DIRECTIONS.items():
        yield _wrap(r + dr, c + dc, rows, cols), d


def _pick_target(
    centroid: tuple[int, int],
    rows: int, cols: int,
    my_cores: list[tuple[int, int]],
    enemy_cores: list[tuple[int, int]],
) -> tuple[int, int]:
    """Pick a migration target: random corner, opposite side, or enemy core."""
    candidates = []

    # Map corners
    corners = [
        (rows // 5, cols // 5),
        (rows // 5, 4 * cols // 5),
        (4 * rows // 5, cols // 5),
        (4 * rows // 5, 4 * cols // 5),
    ]
    candidates.extend(corners)

    # Opposite-of-centroid point
    opp = ((centroid[0] + rows // 2) % rows, (centroid[1] + cols // 2) % cols)
    candidates.append(opp)

    # Edge midpoints
    candidates.append((0, cols // 2))
    candidates.append((rows - 1, cols // 2))
    candidates.append((rows // 2, 0))
    candidates.append((rows // 2, cols - 1))

    # Enemy cores (high priority targets)
    candidates.extend(enemy_cores)

    # Filter out positions too close to current centroid
    far_enough = [
        p for p in candidates
        if _dist2(p[0], p[1], centroid[0], centroid[1], rows, cols)
        > (min(rows, cols) // 4) ** 2
    ]

    if not far_enough:
        far_enough = candidates

    # Weighted random: prefer enemy cores, then far candidates
    weights: list[float] = []
    for p in far_enough:
        if p in enemy_cores:
            weights.append(4.0)
        else:
            d = math.sqrt(_dist2(p[0], p[1], centroid[0], centroid[1], rows, cols))
            weights.append(d / max(rows, cols))

    return random.choices(far_enough, weights=weights, k=1)[0]


def _direction_toward(
    r: int, c: int, tr: int, tc: int, rows: int, cols: int,
    wall_set: set[tuple[int, int]],
    claimed: set[tuple[int, int]],
    enemy_positions: set[tuple[int, int]],
) -> tuple[int, int] | None:
    """Pick the cardinal direction that minimizes toroidal distance to target."""
    best_pos = None
    best_dir = None
    best_dist = float("inf")

    for (nr, nc), d in _cardinal_moves(r, c, rows, cols):
        if (nr, nc) in wall_set:
            continue
        if (nr, nc) in claimed:
            continue
        dist = toroidal_manhattan(nr, nc, tr, tc, rows, cols)
        # Avoid stepping directly onto enemies unless aggressive
        if (nr, nc) in enemy_positions:
            dist += 4  # penalty, not a hard block
        if dist < best_dist:
            best_dist = dist
            best_pos = (nr, nc)
            best_dir = d

    return best_pos, best_dir


def _flee_direction(
    r: int, c: int,
    enemies: list[tuple[int, int]],
    rows: int, cols: int,
    wall_set: set[tuple[int, int]],
    claimed: set[tuple[int, int]],
) -> tuple[int, int] | None:
    """Pick the direction that maximizes minimum distance from enemies."""
    if not enemies:
        return None

    best_pos = None
    best_dir = None
    best_min_dist = -1

    for (nr, nc), d in _cardinal_moves(r, c, rows, cols):
        if (nr, nc) in wall_set:
            continue
        if (nr, nc) in claimed:
            continue
        min_d = min(_dist2(nr, nc, er, ec, rows, cols) for er, ec in enemies)
        if min_d > best_min_dist:
            best_min_dist = min_d
            best_pos = (nr, nc)
            best_dir = d

    return best_pos, best_dir


# ── Core strategy ────────────────────────────────────────────────────────────

RELOCATE_INTERVAL = 20  # turns between relocations
ENGAGE_DURATION = 10    # turns to engage at destination before moving on
ARRIVE_RADIUS_FACTOR = 0.15  # fraction of map dimension = "arrived"
FLEE_RADIUS2_BONUS = 9  # extra buffer beyond attack radius


def compute_moves(state: GameState) -> list[dict]:
    rows, cols = state.rows, state.cols
    turn = state.turn
    attack_r2 = state.attack_radius2

    wall_set = {(w["row"], w["col"]) for w in state.walls}

    my_positions: list[tuple[int, int]] = []
    for b in state.bots:
        if b["owner"] == state.you_id:
            my_positions.append((b["position"]["row"], b["position"]["col"]))

    if not my_positions:
        return []

    enemy_positions: list[tuple[int, int]] = []
    for b in state.bots:
        if b["owner"] != state.you_id:
            enemy_positions.append((b["position"]["row"], b["position"]["col"]))
    enemy_set = set(enemy_positions)

    my_cores: list[tuple[int, int]] = []
    enemy_cores: list[tuple[int, int]] = []
    for c in state.cores:
        pos = (c["position"]["row"], c["position"]["col"])
        if c["owner"] == state.you_id and c["active"]:
            my_cores.append(pos)
        elif c["owner"] != state.you_id and c["active"]:
            enemy_cores.append(pos)

    centroid = _centroid(my_positions, rows, cols)
    ms = _get_state(state.match_id, turn)

    arrive_radius = int(min(rows, cols) * ARRIVE_RADIUS_FACTOR)

    # ── Decide if we need a new target ───────────────────────────────────
    need_new_target = False

    if ms.target is None:
        need_new_target = True
    elif ms.arrived:
        # Already arrived — stay for ENGAGE_DURATION then relocate
        if turn - ms.arrive_turn >= ENGAGE_DURATION:
            need_new_target = True
        # Also relocate if enemies are gone from our area
        elif not any(
            _dist2(centroid[0], centroid[1], er, ec, rows, cols)
            < (arrive_radius * 3) ** 2
            for er, ec in enemy_positions
        ):
            if turn - ms.arrive_turn >= 5:  # minimum stay time
                need_new_target = True
    else:
        # Haven't arrived yet — check if we've been stuck too long
        turns_traveling = turn - ms.target_turn
        if turns_traveling >= RELOCATE_INTERVAL * 2:
            need_new_target = True

    if need_new_target:
        ms.target = _pick_target(centroid, rows, cols, my_cores, enemy_cores)
        ms.target_turn = turn
        ms.arrived = False
        ms.arrive_turn = -999

    # ── Check arrival ────────────────────────────────────────────────────
    if not ms.arrived and ms.target:
        dist_to_target = toroidal_manhattan(
            centroid[0], centroid[1], ms.target[0], ms.target[1], rows, cols
        )
        if dist_to_target <= arrive_radius:
            ms.arrived = True
            ms.arrive_turn = turn

    target = ms.target

    # ── Compute moves for each bot ───────────────────────────────────────
    moves: list[dict] = []
    claimed: set[tuple[int, int]] = set()

    # Sort bots: process bots closest to enemies first (they may need to flee)
    def _enemy_threat(pos):
        if not enemy_positions:
            return 999
        return min(_dist2(pos[0], pos[1], e[0], e[1], rows, cols) for e in enemy_positions)

    sorted_positions = sorted(my_positions, key=_enemy_threat)

    for r, c in sorted_positions:
        flee_r2 = attack_r2 + FLEE_RADIUS2_BONUS
        nearby_enemies = [
            (er, ec) for er, ec in enemy_positions
            if _dist2(r, c, er, ec, rows, cols) <= flee_r2
        ]

        best_pos = None
        best_dir = None

        # Priority 1: Flee if enemy is dangerously close
        if nearby_enemies:
            result = _flee_direction(r, c, nearby_enemies, rows, cols, wall_set, claimed)
            if result:
                best_pos, best_dir = result

        # Priority 2: If arrived and engaging, chase nearby enemies
        if best_pos is None and ms.arrived and enemy_positions:
            # Find nearest enemy
            nearest_enemy = min(
                enemy_positions,
                key=lambda e: _dist2(r, c, e[0], e[1], rows, cols),
            )
            er, ec = nearest_enemy
            if _dist2(r, c, er, ec, rows, cols) <= (attack_r2 * 4):
                result = _direction_toward(
                    r, c, er, ec, rows, cols, wall_set, claimed, enemy_set,
                )
                if result:
                    best_pos, best_dir = result

        # Priority 3: Move toward migration target
        if best_pos is None and target:
            result = _direction_toward(
                r, c, target[0], target[1], rows, cols, wall_set, claimed, enemy_set,
            )
            if result:
                best_pos, best_dir = result

        # Priority 4: Spread from friendly bots (avoid clustering)
        if best_pos is None:
            best_spread_pos = None
            best_spread_dir = None
            best_min_friend_dist = -1
            for (nr, nc), d in _cardinal_moves(r, c, rows, cols):
                if (nr, nc) in wall_set or (nr, nc) in claimed:
                    continue
                min_friend = min(
                    (_dist2(nr, nc, fr, fc, rows, cols) for fr, fc in my_positions if (fr, fc) != (r, c)),
                    default=999,
                )
                if min_friend > best_min_friend_dist:
                    best_min_friend_dist = min_friend
                    best_spread_pos = (nr, nc)
                    best_spread_dir = d
            if best_spread_pos:
                best_pos = best_spread_pos
                best_dir = best_spread_dir

        if best_pos and best_dir:
            moves.append({"position": {"row": r, "col": c}, "direction": best_dir})
            claimed.add(best_pos)
        else:
            # Hold position
            claimed.add((r, c))

    return moves


# ── HTTP handler ─────────────────────────────────────────────────────────────

SECRET = os.environ.get("BOT_SECRET", "")


class NomadHandler(BaseHTTPRequestHandler):
    def _verify_signature(self, body: bytes) -> bool:
        sig = self.headers.get("X-ACB-Signature", "")
        match_id = self.headers.get("X-ACB-Match-Id", "")
        turn = self.headers.get("X-ACB-Turn", "")
        ts = self.headers.get("X-ACB-Timestamp", "")
        body_hash = hashlib.sha256(body).hexdigest()
        signing = f"{match_id}.{turn}.{ts}.{body_hash}"
        expected = hmac.new(SECRET.encode(), signing.encode(), hashlib.sha256).hexdigest()
        return hmac.compare_digest(sig, expected)

    def _sign_response(self, match_id: str, turn: str, body: bytes) -> str:
        body_hash = hashlib.sha256(body).hexdigest()
        signing = f"{match_id}.{turn}.{body_hash}"
        return hmac.new(SECRET.encode(), signing.encode(), hashlib.sha256).hexdigest()

    def do_GET(self):
        if self.path == "/health":
            self.send_response(200)
            self.end_headers()
            self.wfile.write(b"OK")
        else:
            self.send_response(404)
            self.end_headers()

    def do_POST(self):
        if self.path != "/turn":
            self.send_response(404)
            self.end_headers()
            return

        length = int(self.headers.get("Content-Length", 0))
        body = self.rfile.read(length)

        if not self._verify_signature(body):
            self.send_response(401)
            self.end_headers()
            return

        raw = json.loads(body)
        state = GameState(raw)
        moves = compute_moves(state)

        resp = json.dumps({"moves": moves}).encode()
        sig = self._sign_response(
            self.headers.get("X-ACB-Match-Id", ""),
            self.headers.get("X-ACB-Turn", ""),
            resp,
        )

        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        if sig:
            self.send_header("X-ACB-Signature", sig)
        self.end_headers()
        self.wfile.write(resp)

    def log_message(self, _format, *args):
        pass  # silence request logs


def main():
    if not SECRET:
        print("BOT_SECRET environment variable is required")
        exit(1)

    port = int(os.environ.get("BOT_PORT", "8080"))
    server = HTTPServer(("0.0.0.0", port), NomadHandler)
    print(f"NomadBot listening on :{port}")
    server.serve_forever()


if __name__ == "__main__":
    main()
