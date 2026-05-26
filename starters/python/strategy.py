"""
Strategy module for AI Code Battle bot.

Implement your bot logic in the compute_moves() function.
"""

from typing import List, Dict, Any


def compute_moves(state: Dict[str, Any]) -> List[Dict[str, Any]]:
    """
    Compute moves for all visible bots.

    Args:
        state: Game state with visible bots, energy, cores, walls
               - bots: List of visible bots with position and owner
               - energy: List of energy positions
               - cores: List of core positions
               - walls: List of wall positions
               - you: Your bot's ID, energy, and score

    Returns:
        List of move objects, one per bot:
        {
            "position": {"row": int, "col": int},
            "direction": "N" | "E" | "S" | "W" | ""
        }

    Example:
        # Move all bots toward nearest energy
        moves = []
        for bot in state["bots"]:
            if bot["owner"] == state["you"]["id"]:
                # TODO: Implement your strategy here
                moves.append({
                    "position": bot["position"],
                    "direction": ""  # Hold position
                })
        return moves
    """
    moves = []

    # TODO: Implement your strategy here
    # This stub holds all bots in place
    for bot in state.get("bots", []):
        if bot.get("owner") == state["you"]["id"]:
            moves.append({
                "position": bot["position"],
                "direction": ""  # Empty string = no move
            })

    return moves
