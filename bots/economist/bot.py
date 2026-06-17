#!/usr/bin/env python3
"""
EconomistBot - Wins by energy starvation through node contesting.

This bot deliberately contests energy nodes that enemies are approaching,
destroying the contested energy rather than collecting it, while harvesting
uncontested nodes for its own economy.

Key mechanic: If two players each have a bot adjacent (≤√2) to an energy node
on the same collect phase, the energy is destroyed (neither gets it).
"""

import hashlib
import hmac
import json
import os
import random
import math
from http.server import HTTPServer, BaseHTTPRequestHandler
from typing import List, Dict, Optional, Tuple, Set


# Position tuple (row, col)
Position = Tuple[int, int]

# Game constants
ADJACENT_RADIUS2 = 2  # sqrt(2)^2 = 2
APPROACH_THRESHOLD = 4  # Consider enemies within this distance as "approaching"


class GameState:
    """Represents the fog-filtered state visible to this bot."""

    def __init__(self, data: dict):
        self.match_id = data["match_id"]
        self.turn = data["turn"]
        self.config = data["config"]
        self.you_id = data["you"]["id"]
        self.you_energy = data["you"]["energy"]
        self.you_score = data["you"]["score"]

        # Bots only have position and owner (no ID!)
        self.bots = data["bots"]
        self.energy = [tuple(int(v) for v in e) for e in data.get("energy", [])]
        self.cores = data.get("cores", [])
        self.walls = [tuple(int(v) for v in w) for w in data.get("walls", [])]
        self.dead = data.get("dead", [])

    @property
    def rows(self) -> int:
        return self.config["rows"]

    @property
    def cols(self) -> int:
        return self.config["cols"]

    @property
    def vision_radius2(self) -> int:
        return self.config["vision_radius2"]

    @property
    def attack_radius2(self) -> int:
        return self.config["attack_radius2"]

    @property
    def spawn_cost(self) -> int:
        return self.config["spawn_cost"]


class EconomistStrategy:
    """Implements energy-denial strategy through node contesting."""

    def __init__(self):
        # Use bot positions as keys (since bots have no IDs)
        self.contest_assignments: Dict[Position, Position] = {}  # bot_pos -> energy position to contest
        self.guard_assignments: Set[Position] = set()  # bot positions assigned to guard cores

    def compute_moves(self, state: GameState) -> List[dict]:
        """Compute moves for all owned bots."""
        # Separate bots by owner
        my_bots = [b for b in state.bots if b["owner"] == state.you_id]
        enemy_bots = [b for b in state.bots if b["owner"] != state.you_id]

        if not my_bots:
            return []

        # Build position maps
        enemy_positions = {tuple(b["position"]): b for b in enemy_bots}

        # Use positions as bot identifiers
        my_bot_positions = {}  # position -> bot dict
        for bot in my_bots:
            my_bot_positions[tuple(bot["position"])] = bot

        energy_positions = set(state.energy)

        # Clear previous assignments for bots that moved or died
        self._cleanup_assignments(my_bot_positions, energy_positions)

        # Assign bots to roles
        moves = []
        used_positions = set()

        # Priority 1: Maintain existing contest assignments
        for bot in my_bots:
            bot_pos = tuple(bot["position"])

            if bot_pos in self.contest_assignments:
                contest_pos = self.contest_assignments[bot_pos]
                if contest_pos in energy_positions:
                    used_positions.add(bot_pos)
                    # If already adjacent, stay to contest; otherwise move toward it
                    dist2 = self._distance2(bot_pos, contest_pos, state)
                    if dist2 <= ADJACENT_RADIUS2:
                        # Already adjacent - stay put to contest
                        pass  # Don't move, maintain contest
                    else:
                        # Move toward contest position
                        move = self._move_toward(bot_pos, contest_pos, state, enemy_positions, avoid_enemies=False)
                        if move:
                            moves.append({
                                "position": list(bot_pos),
                                "direction": move
                            })
                    continue

        # Priority 2: Contest visible energy nodes that enemies can also reach
        energy_by_priority = []

        for energy_pos in energy_positions:
            # Count how many of MY bots can reach this energy quickly (within 8 tiles = 2 turns)
            my_reachable = 0
            nearest_my_dist = float('inf')
            for bot in my_bots:
                bot_pos = tuple(bot["position"])
                if bot_pos in used_positions:
                    continue
                dist2 = self._distance2(bot_pos, energy_pos, state)
                if dist2 <= 64:  # Within 8 tiles
                    my_reachable += 1
                nearest_my_dist = min(nearest_my_dist, dist2)

            # Count how many ENEMY bots can reach this energy quickly
            enemy_reachable = 0
            nearest_enemy_dist = float('inf')
            if enemy_positions:
                for enemy_pos in enemy_positions:
                    dist2 = self._distance2(enemy_pos, energy_pos, state)
                    if dist2 <= 64:  # Within 8 tiles
                        enemy_reachable += 1
                    nearest_enemy_dist = min(nearest_enemy_dist, dist2)
            else:
                # No enemies visible yet - prioritize nodes closer to map center
                center = (state.rows // 2, state.cols // 2)
                dist_to_center = self._distance2(energy_pos, center, state)
                nearest_enemy_dist = dist_to_center  # Use as proxy
                enemy_reachable = 1 if dist_to_center < 100 else 0

            # Contest priority: nodes that BOTH sides can reach quickly
            if enemy_reachable > 0:
                # Enemies visible and reachable - HIGH contest priority
                priority = 10000.0 / (nearest_enemy_dist + nearest_my_dist + 1)
            elif nearest_enemy_dist < 100:
                # No enemies visible but node is near center - MEDIUM priority
                priority = 1000.0 / (nearest_my_dist + 1)
            else:
                # Node far from center - LOW priority
                priority = 100.0 / (nearest_my_dist + 1)

            energy_by_priority.append((energy_pos, priority, nearest_my_dist))

        # Sort by priority descending (highest first)
        energy_by_priority.sort(key=lambda x: x[1], reverse=True)

        for energy_pos, priority, nearest_my_dist in energy_by_priority:
            if energy_pos not in energy_positions:
                continue

            # Find nearest unassigned bot
            nearest_bot = self._find_nearest_bot(
                energy_pos, my_bots, used_positions, state
            )

            if nearest_bot:
                bot_pos = tuple(nearest_bot["position"])
                used_positions.add(bot_pos)
                self.contest_assignments[bot_pos] = energy_pos

                # Move toward energy to contest, but stay if already adjacent
                dist2 = self._distance2(bot_pos, energy_pos, state)
                if dist2 > ADJACENT_RADIUS2:
                    move = self._move_toward(bot_pos, energy_pos, state, enemy_positions, avoid_enemies=False)
                    if move:
                        moves.append({
                            "position": list(bot_pos),
                            "direction": move
                        })
                # If already adjacent, stay put to contest (no move added)

        # Priority 3: Remaining bots explore toward visible energy or map center
        center = (state.rows // 2, state.cols // 2)
        for bot in my_bots:
            bot_pos = tuple(bot["position"])
            if bot_pos not in used_positions:
                # Move toward center to find energy
                move = self._move_toward(bot_pos, center, state, enemy_positions, avoid_enemies=False)
                if move:
                    moves.append({
                        "position": list(bot_pos),
                        "direction": move
                    })

        return moves

    def _cleanup_assignments(self, current_bot_positions: Dict[Position, dict], energy_positions: Set[Position]):
        """Remove assignments for bots that moved or died, or consumed energy."""
        # Remove assignments for bots no longer at their assigned position
        to_remove = []
        for bot_pos in self.contest_assignments:
            if bot_pos not in current_bot_positions:
                # Bot moved or died
                to_remove.append(bot_pos)
            elif self.contest_assignments[bot_pos] not in energy_positions:
                # Energy was collected
                to_remove.append(bot_pos)

        for bot_pos in to_remove:
            del self.contest_assignments[bot_pos]

        # Clean up guard assignments
        self.guard_assignments = self.guard_assignments.intersection(current_bot_positions.keys())

    def _find_nearest_bot(
        self,
        target: Position,
        my_bots: List[dict],
        used_positions: Set[Position],
        state: GameState
    ) -> Optional[dict]:
        """Find the nearest unused bot to the target position."""
        nearest_bot = None
        nearest_dist = float('inf')

        for bot in my_bots:
            bot_pos = tuple(bot["position"])
            if bot_pos in used_positions:
                continue

            dist2 = self._distance2(bot_pos, target, state)
            if dist2 < nearest_dist:
                nearest_dist = dist2
                nearest_bot = bot

        return nearest_bot

    def _move_toward(
        self,
        from_pos: Position,
        to_pos: Position,
        state: GameState,
        enemy_positions: Dict[Position, dict],
        avoid_enemies: bool = False,
        stay_if_adjacent: bool = False
    ) -> Optional[str]:
        """Calculate best direction to move toward target.

        Args:
            from_pos: Starting position
            to_pos: Target position
            state: Game state
            enemy_positions: Map of enemy bot positions
            avoid_enemies: If True, avoid positions adjacent to enemies.
            stay_if_adjacent: If True and already adjacent to target, return None (stay put).
        """
        # If staying put is requested and we're already adjacent, don't move
        if stay_if_adjacent and self._distance2(from_pos, to_pos, state) <= ADJACENT_RADIUS2:
            return None

        directions = ["N", "E", "S", "W"]
        best_dir = None
        best_dist2 = float('inf')

        for direction in directions:
            new_pos = self._simulate_move(from_pos, direction, state)

            # Optionally avoid positions adjacent to enemies
            if avoid_enemies and self._is_near_enemy(new_pos, enemy_positions, state):
                continue

            dist2 = self._distance2(new_pos, to_pos, state)
            if dist2 < best_dist2:
                best_dist2 = dist2
                best_dir = direction

        return best_dir

    def _is_near_enemy(
        self,
        pos: Position,
        enemy_positions: Dict[Position, dict],
        state: GameState
    ) -> bool:
        """Check if position is adjacent to any enemy."""
        for direction in ["N", "E", "S", "W"]:
            adj_pos = self._simulate_move(pos, direction, state)
            if adj_pos in enemy_positions:
                return True
        return False

    def _distance2(self, a: Position, b: Position, state: GameState) -> int:
        """Calculate squared Euclidean distance with toroidal wrapping."""
        dr = abs(a[0] - b[0])
        dc = abs(a[1] - b[1])

        # Apply toroidal wrapping
        if dr > state.rows // 2:
            dr = state.rows - dr
        if dc > state.cols // 2:
            dc = state.cols - dc

        return dr * dr + dc * dc

    def _simulate_move(self, pos: Position, direction: str, state: GameState) -> Position:
        """Calculate new position after a move with toroidal wrapping."""
        row, col = pos

        if direction == "N":
            new_row = (row - 1 + state.rows) % state.rows
            new_col = col
        elif direction == "E":
            new_row = row
            new_col = (col + 1) % state.cols
        elif direction == "S":
            new_row = (row + 1) % state.rows
            new_col = col
        elif direction == "W":
            new_row = row
            new_col = (col - 1 + state.cols) % state.cols
        else:
            return pos

        return (new_row, new_col)


class EconomistBotHandler(BaseHTTPRequestHandler):
    """HTTP request handler for EconomistBot."""

    secret: str = ""
    strategy = EconomistStrategy()

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
        signing_string = f"{match_id}.{turn}.{body_hash}"
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

        # Compute moves
        moves = self.strategy.compute_moves(state)
        turn = int(turn_str)

        # Send response
        self.send_json_response(200, {"moves": moves}, match_id, turn)


def main():
    port = int(os.environ.get("BOT_PORT", "8081"))
    secret = os.environ.get("BOT_SECRET", "")

    if not secret:
        print("ERROR: BOT_SECRET environment variable is required")
        exit(1)

    EconomistBotHandler.secret = secret

    server = HTTPServer(("", port), EconomistBotHandler)
    print(f"EconomistBot starting on port {port}")
    server.serve_forever()


if __name__ == "__main__":
    main()
