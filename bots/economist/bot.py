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
        self.bots = data["bots"]
        self.energy = [tuple(e) for e in data.get("energy", [])]
        self.cores = data.get("cores", [])
        self.walls = [tuple(w) for w in data.get("walls", [])]
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
        self.contest_assignments: Dict[int, Position] = {}  # bot_id -> energy position to contest
        self.guard_assignments: Set[int] = set()  # bot_ids assigned to guard cores

    def compute_moves(self, state: GameState) -> List[dict]:
        """Compute moves for all owned bots."""
        # Separate bots by owner
        my_bots = [b for b in state.bots if b["owner"] == state.you_id]
        enemy_bots = [b for b in state.bots if b["owner"] != state.you_id]

        if not my_bots:
            return []

        # Build position maps
        enemy_positions = {tuple(b["position"]): b for b in enemy_bots}
        my_bot_positions = {tuple(b["position"]): b for b in my_bots}
        energy_positions = set(state.energy)

        # Build my cores positions
        my_core_positions = set()
        for core in state.cores:
            if core["owner"] == state.you_id:
                my_core_positions.add(tuple(core["position"]))

        # Check if we should switch to pure collection mode
        score_lead = state.you_score
        for enemy in enemy_bots:
            # Assume enemy has similar score if we can't see it
            pass  # Simplified - we use our score as proxy

        pure_collection = (state.turn > 200 and
                          score_lead >= 3)

        # Analyze energy nodes
        energy_analysis = self._analyze_energy_nodes(
            state.energy, my_bot_positions, enemy_positions, state
        )

        # Clear previous assignments for dead/moved bots
        self._cleanup_assignments(my_bots, energy_positions)

        # Assign bots to roles
        moves = []
        used_bots = set()

        # Priority 1: Maintain existing contest assignments (stay put)
        for bot in my_bots:
            bot_id = bot["id"]
            bot_pos = tuple(bot["position"])

            if bot_id in self.contest_assignments:
                contest_pos = self.contest_assignments[bot_id]
                if contest_pos in energy_positions:
                    # Check if still contesting effectively
                    dist2 = self._distance2(bot_pos, contest_pos, state)
                    if dist2 <= APPROACH_THRESHOLD * APPROACH_THRESHOLD:
                        # Still approaching, continue
                        used_bots.add(bot_id)
                        # Only move if not already adjacent
                        if dist2 > ADJACENT_RADIUS2:
                            move = self._move_toward(bot_pos, contest_pos, state, enemy_positions)
                            if move:
                                moves.append({
                                    "position": list(bot_pos),
                                    "direction": move
                                })
                        continue

        # Priority 2: Contest threatened nodes (highest priority)
        if not pure_collection:
            threatened_energy = [
                (pos, analysis) for pos, analysis in energy_analysis.items()
                if analysis["enemy_nearby"] > 0 and analysis["my_adjacent"] == 0
            ]

            # Sort by contest priority
            threatened_energy.sort(
                key=lambda x: x[1]["contest_priority"], reverse=True
            )

            for energy_pos, analysis in threatened_energy:
                if energy_pos not in energy_positions:
                    continue

                # Find nearest unassigned bot
                nearest_bot = self._find_nearest_bot(
                    energy_pos, my_bots, used_bots, state
                )

                if nearest_bot:
                    bot_id = nearest_bot["id"]
                    bot_pos = tuple(nearest_bot["position"])

                    used_bots.add(bot_id)
                    self.contest_assignments[bot_id] = energy_pos

                    # Move toward energy to contest
                    dist2 = self._distance2(bot_pos, energy_pos, state)
                    if dist2 > ADJACENT_RADIUS2:
                        move = self._move_toward(bot_pos, energy_pos, state, enemy_positions)
                        if move:
                            moves.append({
                                "position": list(bot_pos),
                                "direction": move
                            })

        # Priority 3: Reserve 1-2 bots to guard cores
        guards_needed = min(2, len(my_bots) // 3)
        guards_assigned = 0

        for core_pos in my_core_positions:
            if guards_assigned >= guards_needed:
                break

            # Find nearest unassigned bot
            nearest_bot = self._find_nearest_bot(
                core_pos, my_bots, used_bots, state
            )

            if nearest_bot:
                bot_id = nearest_bot["id"]
                bot_pos = tuple(nearest_bot["position"])

                dist2 = self._distance2(bot_pos, core_pos, state)
                if dist2 > ADJACENT_RADIUS2:
                    # Move toward core to guard
                    move = self._move_toward(bot_pos, core_pos, state, enemy_positions)
                    if move:
                        moves.append({
                            "position": list(bot_pos),
                            "direction": move
                        })

                used_bots.add(bot_id)
                self.guard_assignments.add(bot_id)
                guards_assigned += 1

        # Priority 4: Collect uncontested energy
        unthreatened_energy = [
            pos for pos, analysis in energy_analysis.items()
            if analysis["enemy_nearby"] == 0 and pos in energy_positions
        ]

        for energy_pos in unthreatened_energy:
            # Find nearest unassigned bot
            nearest_bot = self._find_nearest_bot(
                energy_pos, my_bots, used_bots, state
            )

            if nearest_bot:
                bot_id = nearest_bot["id"]
                bot_pos = tuple(nearest_bot["position"])

                used_bots.add(bot_id)

                # Move toward energy to collect
                dist2 = self._distance2(bot_pos, energy_pos, state)
                if dist2 > ADJACENT_RADIUS2:
                    move = self._move_toward(bot_pos, energy_pos, state, enemy_positions)
                    if move:
                        moves.append({
                            "position": list(bot_pos),
                            "direction": move
                        })

        # Remaining bots: explore toward map center
        center = (state.rows // 2, state.cols // 2)
        for bot in my_bots:
            if bot["id"] not in used_bots:
                bot_pos = tuple(bot["position"])
                move = self._move_toward(bot_pos, center, state, enemy_positions)
                if move:
                    moves.append({
                        "position": list(bot_pos),
                        "direction": move
                    })

        return moves

    def _analyze_energy_nodes(
        self,
        energy_nodes: List[Position],
        my_bot_positions: Dict[Position, dict],
        enemy_positions: Dict[Position, dict],
        state: GameState
    ) -> Dict[Position, dict]:
        """Analyze energy nodes to determine contest priorities."""
        analysis = {}

        for energy_pos in energy_nodes:
            enemy_nearby = 0
            enemy_approaching = 0
            my_adjacent = 0
            my_nearby = 0
            nearest_enemy_dist = float('inf')
            nearest_my_dist = float('inf')

            # Count nearby bots
            for bot_pos, bot in my_bot_positions.items():
                dist2 = self._distance2(bot_pos, energy_pos, state)
                if dist2 <= ADJACENT_RADIUS2:
                    my_adjacent += 1
                if dist2 <= APPROACH_THRESHOLD * APPROACH_THRESHOLD:
                    my_nearby += 1
                nearest_my_dist = min(nearest_my_dist, dist2)

            for enemy_pos, enemy in enemy_positions.items():
                dist2 = self._distance2(enemy_pos, energy_pos, state)
                if dist2 <= APPROACH_THRESHOLD * APPROACH_THRESHOLD:
                    enemy_nearby += 1
                    nearest_enemy_dist = min(nearest_enemy_dist, dist2)
                    if dist2 <= APPROACH_THRESHOLD * APPROACH_THRESHOLD:
                        enemy_approaching += 1

            # Calculate contest priority
            # Priority = (enemy_count_nearby * energy_value) / distance
            # Energy value is always 1 in current rules
            distance_factor = math.sqrt(max(nearest_enemy_dist, 1))
            contest_priority = (enemy_approaching * 1.0) / distance_factor

            analysis[energy_pos] = {
                "enemy_nearby": enemy_nearby,
                "enemy_approaching": enemy_approaching,
                "my_adjacent": my_adjacent,
                "my_nearby": my_nearby,
                "nearest_enemy_dist": nearest_enemy_dist,
                "contest_priority": contest_priority if enemy_approaching > 0 else 0,
            }

        return analysis

    def _cleanup_assignments(self, my_bots: List[dict], energy_positions: Set[Position]):
        """Remove assignments for dead bots or consumed energy."""
        active_bot_ids = {b["id"] for b in my_bots}

        # Remove assignments for dead bots
        to_remove = []
        for bot_id in self.contest_assignments:
            if bot_id not in active_bot_ids:
                to_remove.append(bot_id)
            elif self.contest_assignments[bot_id] not in energy_positions:
                to_remove.append(bot_id)

        for bot_id in to_remove:
            del self.contest_assignments[bot_id]

        # Clean up guard assignments
        self.guard_assignments = self.guard_assignments.intersection(active_bot_ids)

    def _find_nearest_bot(
        self,
        target: Position,
        my_bots: List[dict],
        used_bots: Set[int],
        state: GameState
    ) -> Optional[dict]:
        """Find the nearest unused bot to the target position."""
        nearest_bot = None
        nearest_dist = float('inf')

        for bot in my_bots:
            if bot["id"] in used_bots:
                continue

            dist2 = self._distance2(tuple(bot["position"]), target, state)
            if dist2 < nearest_dist:
                nearest_dist = dist2
                nearest_bot = bot

        return nearest_bot

    def _move_toward(
        self,
        from_pos: Position,
        to_pos: Position,
        state: GameState,
        enemy_positions: Dict[Position, dict]
    ) -> Optional[str]:
        """Calculate best direction to move toward target, avoiding enemies."""
        directions = ["N", "E", "S", "W"]
        best_dir = None
        best_dist2 = float('inf')

        for direction in directions:
            new_pos = self._simulate_move(from_pos, direction, state)

            # Avoid positions adjacent to enemies (combat avoidance)
            if self._is_near_enemy(new_pos, enemy_positions, state):
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
