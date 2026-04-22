"""Grid utility functions for AI Code Battle.

Provides toroidal distance calculations, neighbor enumeration,
and BFS pathfinding on a wrapping grid.
"""

from collections import deque


def toroidal_manhattan(r1, c1, r2, c2, cols, rows):
    """Manhattan distance with wrap-around on a toroidal grid."""
    dr = abs(r1 - r2)
    dc = abs(c1 - c2)
    dr = min(dr, rows - dr)
    dc = min(dc, cols - dc)
    return dr + dc


def toroidal_chebyshev(r1, c1, r2, c2, cols, rows):
    """Chebyshev distance with wrap-around on a toroidal grid."""
    dr = abs(r1 - r2)
    dc = abs(c1 - c2)
    dr = min(dr, rows - dr)
    dc = min(dc, cols - dc)
    return max(dr, dc)


def neighbors(row, col, rows, cols):
    """Return 8-directional neighbors with wrap-around."""
    offsets = [(-1, -1), (-1, 0), (-1, 1),
               (0, -1),           (0, 1),
               (1, -1),  (1, 0),  (1, 1)]
    return [((row + dr) % rows, (col + dc) % cols) for dr, dc in offsets]


def bfs(start, goal, passable, rows, cols):
    """BFS pathfinding on a toroidal grid.

    Args:
        start: (row, col) tuple
        goal: (row, col) tuple
        passable: callable(row, col) -> bool
        rows, cols: grid dimensions

    Returns:
        List of (row, col) from start to goal (exclusive of start),
        or None if no path exists.
    """
    if start == goal:
        return []

    queue = deque([(start, [])])
    visited = {start}

    while queue:
        (r, c), path = queue.popleft()
        for nr, nc in neighbors(r, c, rows, cols):
            if (nr, nc) == goal:
                return path + [(nr, nc)]
            if (nr, nc) not in visited and passable(nr, nc):
                visited.add((nr, nc))
                queue.append(((nr, nc), path + [(nr, nc)]))

    return None
