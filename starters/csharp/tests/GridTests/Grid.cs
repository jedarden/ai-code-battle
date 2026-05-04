// Grid utility functions for AI Code Battle.
//
// Provides toroidal distance calculations, neighbor enumeration,
// and BFS pathfinding on a wrapping grid.

using System.Collections.Generic;

static class Grid
{
    private static readonly (int dr, int dc)[] Offsets =
    {
        (-1, -1), (-1, 0), (-1, 1),
        (0, -1),           (0, 1),
        (1, -1),  (1, 0),  (1, 1),
    };

    /// Manhattan distance with wrap-around on a toroidal grid.
    public static int ToroidalManhattan(int r1, int c1, int r2, int c2, int rows, int cols)
    {
        int dr = Math.Min(Math.Abs(r1 - r2), rows - Math.Abs(r1 - r2));
        int dc = Math.Min(Math.Abs(c1 - c2), cols - Math.Abs(c1 - c2));
        return dr + dc;
    }

    /// Chebyshev distance with wrap-around on a toroidal grid.
    public static int ToroidalChebyshev(int r1, int c1, int r2, int c2, int rows, int cols)
    {
        int dr = Math.Min(Math.Abs(r1 - r2), rows - Math.Abs(r1 - r2));
        int dc = Math.Min(Math.Abs(c1 - c2), cols - Math.Abs(c1 - c2));
        return Math.Max(dr, dc);
    }

    /// 8-directional neighbors with wrap-around.
    public static Position[] Neighbors(Position p, int rows, int cols)
    {
        var result = new Position[8];
        for (int i = 0; i < Offsets.Length; i++)
        {
            result[i] = new Position
            {
                Row = (p.Row + Offsets[i].dr + rows) % rows,
                Col = (p.Col + Offsets[i].dc + cols) % cols,
            };
        }
        return result;
    }

    /// BFS pathfinding on a toroidal grid.
    /// Returns path (excluding start) or null if unreachable.
    public static List<Position>? Bfs(Position start, Position goal,
        Func<Position, bool> passable, int rows, int cols)
    {
        if (start.Row == goal.Row && start.Col == goal.Col)
            return [];

        var visited = new HashSet<(int, int)> { (start.Row, start.Col) };
        var queue = new Queue<(Position pos, List<Position> path)>();
        queue.Enqueue((start, []));

        while (queue.Count > 0)
        {
            var (cur, path) = queue.Dequeue();
            foreach (var nb in Neighbors(cur, rows, cols))
            {
                var newPath = new List<Position>(path) { nb };
                if (nb.Row == goal.Row && nb.Col == goal.Col)
                    return newPath;

                var key = (nb.Row, nb.Col);
                if (!visited.Contains(key) && passable(nb))
                {
                    visited.Add(key);
                    queue.Enqueue((nb, newPath));
                }
            }
        }
        return null;
    }
}
