// Defender strategy: core-hugging perimeter defense.
//
// Archetype: Low Aggression, Low Economy, Low Exploration, High Formation.
//
// Each bot is assigned to patrol an evenly-spaced slot around the nearest
// active core. If an enemy enters the perimeter, the closest defender
// intercepts. Bots never chase enemies past the perimeter radius.

using System.Collections.Generic;

static class DefenderStrategy
{
    const int PerimeterRadius = 8;
    const int MaxInterceptRadius = 12;

    static readonly (int dr, int dc, string dir)[] Cardinal =
    {
        (-1, 0, "N"), (0, 1, "E"), (1, 0, "S"), (0, -1, "W"),
    };

    public static List<Move> ComputeMoves(GameState state)
    {
        var rows = state.Config.Rows;
        var cols = state.Config.Cols;
        var myId = state.You.Id;
        var wallSet = BuildWallSet(state.Walls);

        var myBots = new List<VisibleBot>();
        var enemies = new List<VisibleBot>();
        foreach (var b in state.Bots)
        {
            if (b.Owner == myId) myBots.Add(b);
            else enemies.Add(b);
        }

        var myCores = new List<VisibleCore>();
        foreach (var c in state.Cores)
        {
            if (c.Owner == myId && c.Active) myCores.Add(c);
        }

        if (myCores.Count == 0)
            return GatherFallback(state, myBots, enemies, wallSet, rows, cols);

        // Assign each bot to its nearest active core.
        var botToCore = new Dictionary<VisibleBot, VisibleCore>();
        foreach (var bot in myBots)
        {
            var nearest = myCores[0];
            var bestDist = DistSq(bot.Position, nearest.Position, rows, cols);
            for (int i = 1; i < myCores.Count; i++)
            {
                var d = DistSq(bot.Position, myCores[i].Position, rows, cols);
                if (d < bestDist) { bestDist = d; nearest = myCores[i]; }
            }
            botToCore[bot] = nearest;
        }

        // Group bots by core.
        var groups = new Dictionary<VisibleCore, List<VisibleBot>>();
        foreach (var c in myCores) groups[c] = [];
        foreach (var kvp in botToCore) groups[kvp.Value].Add(kvp.Key);

        var moves = new List<Move>();
        var assignedThreats = new HashSet<(int, int)>();
        var claimedDests = new HashSet<(int, int)>();
        foreach (var b in myBots)
            claimedDests.Add((b.Position.Row, b.Position.Col));
        var perimSq = PerimeterRadius * PerimeterRadius;
        var interceptSq = MaxInterceptRadius * MaxInterceptRadius;

        foreach (var (core, bots) in groups)
        {
            if (bots.Count == 0) continue;

            // Compute patrol slots (evenly spaced circle around the core).
            var slots = ComputePatrolSlots(core.Position, bots.Count, rows, cols, wallSet);
            var usedSlots = new HashSet<int>();

            // Enemies within intercept range of this core.
            var threats = new List<VisibleBot>();
            foreach (var e in enemies)
            {
                if (DistSq(e.Position, core.Position, rows, cols) <= interceptSq)
                    threats.Add(e);
            }
            threats.Sort((a, b) =>
                DistSq(a.Position, core.Position, rows, cols)
                .CompareTo(DistSq(b.Position, core.Position, rows, cols)));

            // Sort bots: inner bots first (they're already in position, better interceptors).
            bots.Sort((a, b) =>
                DistSq(a.Position, core.Position, rows, cols)
                .CompareTo(DistSq(b.Position, core.Position, rows, cols)));

            foreach (var bot in bots)
            {
                Move? move = null;

                // Phase 1: intercept threats inside the perimeter.
                foreach (var threat in threats)
                {
                    var tKey = (threat.Position.Row, threat.Position.Col);
                    if (assignedThreats.Contains(tKey)) continue;

                    if (DistSq(threat.Position, core.Position, rows, cols) > perimSq)
                        continue; // outside perimeter — will be handled by outer ring

                    assignedThreats.Add(tKey);
                    move = GreedyMove(bot.Position, threat.Position, rows, cols, wallSet,
                        core.Position, interceptSq);
                    break;
                }

                // Phase 2: collect energy within perimeter if safe.
                if (move == null)
                {
                    var energyTarget = BestEnergy(state.Energy, bot, core, perimSq, rows, cols);
                    if (energyTarget != null)
                        move = GreedyMove(bot.Position, energyTarget, rows, cols, wallSet);
                }

                // Phase 3: patrol assigned slot.
                if (move == null)
                {
                    int bestSlotIdx = -1;
                    int bestSlotDist = int.MaxValue;
                    for (int i = 0; i < slots.Count; i++)
                    {
                        if (usedSlots.Contains(i)) continue;
                        var d = DistSq(bot.Position, slots[i], rows, cols);
                        if (d < bestSlotDist) { bestSlotDist = d; bestSlotIdx = i; }
                    }

                    if (bestSlotIdx >= 0)
                    {
                        usedSlots.Add(bestSlotIdx);
                        if (bestSlotDist > 2)
                            move = GreedyMove(bot.Position, slots[bestSlotIdx], rows, cols, wallSet);
                    }
                    else
                    {
                        // All slots taken — loiter near core.
                        var distToCore = DistSq(bot.Position, core.Position, rows, cols);
                        if (distToCore > perimSq)
                            move = GreedyMove(bot.Position, core.Position, rows, cols, wallSet);
                    }
                }

                if (move != null)
                {
                    var (dr, dc, _) = Array.Find(Cardinal, c => c.dir == move.Direction);
                    int nr = (move.Position.Row + dr + rows) % rows;
                    int nc = (move.Position.Col + dc + cols) % cols;
                    if (claimedDests.Contains((nr, nc)))
                        move = null; // destination occupied — hold
                    else
                    {
                        claimedDests.Remove((move.Position.Row, move.Position.Col));
                        claimedDests.Add((nr, nc));
                        moves.Add(move);
                    }
                }
            }
        }

        return moves;
    }

    // Find the best energy to collect: within perimeter, closest to bot, not
    // too far from core.
    static Position? BestEnergy(List<Position> energy, VisibleBot bot,
        VisibleCore core, int perimSq, int rows, int cols)
    {
        Position? best = null;
        int bestDist = int.MaxValue;
        foreach (var e in energy)
        {
            if (DistSq(e, core.Position, rows, cols) > perimSq) continue;
            var d = DistSq(e, bot.Position, rows, cols);
            if (d < bestDist && d <= 9) // only divert if very close (within 3 tiles)
            {
                bestDist = d;
                best = e;
            }
        }
        return best;
    }

    // Generate evenly-spaced patrol positions around a core.
    static List<Position> ComputePatrolSlots(Position core, int numBots,
        int rows, int cols, HashSet<(int, int)> wallSet)
    {
        var slots = new List<Position>();
        int n = Math.Max(numBots, 6);
        int candidates = n * 3; // extra to skip walls
        for (int i = 0; i < candidates && slots.Count < n; i++)
        {
            double angle = 2.0 * Math.PI * i / candidates;
            int dr = (int)Math.Round(PerimeterRadius * Math.Sin(angle));
            int dc = (int)Math.Round(PerimeterRadius * Math.Cos(angle));
            int r = (core.Row + dr + rows) % rows;
            int c = (core.Col + dc + cols) % cols;
            if (!wallSet.Contains((r, c)))
                slots.Add(new Position { Row = r, Col = c });
        }
        return slots;
    }

    // Greedy one-step move: pick the cardinal direction that minimizes
    // distance to target, optionally constrained to stay near an anchor.
    static Move? GreedyMove(Position from, Position to, int rows, int cols,
        HashSet<(int, int)> wallSet, Position? anchor = null, int maxAnchorDistSq = int.MaxValue)
    {
        string? bestDir = null;
        int bestDist = int.MaxValue;
        var targetDist = DistSq(from, to, rows, cols);

        foreach (var (dr, dc, dir) in Cardinal)
        {
            int nr = (from.Row + dr + rows) % rows;
            int nc = (from.Col + dc + cols) % cols;
            if (wallSet.Contains((nr, nc))) continue;

            // Don't move further from target.
            var newDist = DistSq(new Position { Row = nr, Col = nc }, to, rows, cols);
            if (newDist > targetDist && targetDist > 0) continue;

            // Stay near anchor if specified (prevents chasing past perimeter).
            if (anchor != null)
            {
                var dToAnchor = DistSq(new Position { Row = nr, Col = nc }, anchor, rows, cols);
                if (dToAnchor > maxAnchorDistSq) continue;
            }

            if (newDist < bestDist) { bestDist = newDist; bestDir = dir; }
        }

        // Fallback: any non-wall direction if all better moves blocked.
        if (bestDir == null)
        {
            foreach (var (dr, dc, dir) in Cardinal)
            {
                int nr = (from.Row + dr + rows) % rows;
                int nc = (from.Col + dc + cols) % cols;
                if (!wallSet.Contains((nr, nc)))
                {
                    if (anchor != null)
                    {
                        var dToAnchor = DistSq(new Position { Row = nr, Col = nc }, anchor, rows, cols);
                        if (dToAnchor > maxAnchorDistSq) continue;
                    }
                    bestDir = dir;
                    break;
                }
            }
        }

        if (bestDir == null) return null;
        return new Move { Position = from, Direction = bestDir };
    }

    // Fallback strategy when all cores are lost: gather energy, flee enemies.
    static List<Move> GatherFallback(GameState state, List<VisibleBot> myBots,
        List<VisibleBot> enemies, HashSet<(int, int)> wallSet, int rows, int cols)
    {
        var moves = new List<Move>();
        var claimedEnergy = new HashSet<(int, int)>();

        foreach (var bot in myBots)
        {
            // Flee from nearby enemies.
            var nearEnemy = ClosestEnemy(bot, enemies, state.Config.AttackRadius2 + 4, rows, cols);
            if (nearEnemy != null)
            {
                var flee = FleeMove(bot.Position, nearEnemy.Position, rows, cols, wallSet);
                if (flee != null) { moves.Add(flee); continue; }
            }

            // Collect nearest unclaimed energy.
            Position? bestEnergy = null;
            int bestDist = int.MaxValue;
            foreach (var e in state.Energy)
            {
                if (claimedEnergy.Contains((e.Row, e.Col))) continue;
                var d = DistSq(e, bot.Position, rows, cols);
                if (d < bestDist) { bestDist = d; bestEnergy = e; }
            }

            if (bestEnergy != null)
            {
                claimedEnergy.Add((bestEnergy.Row, bestEnergy.Col));
                var move = GreedyMove(bot.Position, bestEnergy, rows, cols, wallSet);
                if (move != null) moves.Add(move);
            }
        }

        return moves;
    }

    // Find the closest enemy within a squared distance threshold.
    static VisibleBot? ClosestEnemy(VisibleBot bot, List<VisibleBot> enemies,
        int thresholdSq, int rows, int cols)
    {
        VisibleBot? closest = null;
        int closestDist = int.MaxValue;
        foreach (var e in enemies)
        {
            var d = DistSq(bot.Position, e.Position, rows, cols);
            if (d <= thresholdSq && d < closestDist) { closestDist = d; closest = e; }
        }
        return closest;
    }

    // Pick the cardinal direction that maximizes distance from a threat.
    static Move? FleeMove(Position from, Position threat, int rows, int cols,
        HashSet<(int, int)> wallSet)
    {
        string? bestDir = null;
        int bestDist = -1;
        foreach (var (dr, dc, dir) in Cardinal)
        {
            int nr = (from.Row + dr + rows) % rows;
            int nc = (from.Col + dc + cols) % cols;
            if (wallSet.Contains((nr, nc))) continue;
            var d = DistSq(new Position { Row = nr, Col = nc }, threat, rows, cols);
            if (d > bestDist) { bestDist = d; bestDir = dir; }
        }
        if (bestDir == null) return null;
        return new Move { Position = from, Direction = bestDir };
    }

    // Squared Euclidean distance on a toroidal grid.
    static int DistSq(Position a, Position b, int rows, int cols)
    {
        int dr = Math.Min(Math.Abs(a.Row - b.Row), rows - Math.Abs(a.Row - b.Row));
        int dc = Math.Min(Math.Abs(a.Col - b.Col), cols - Math.Abs(a.Col - b.Col));
        return dr * dr + dc * dc;
    }

    static HashSet<(int, int)> BuildWallSet(List<Position> walls)
    {
        var set = new HashSet<(int, int)>(walls.Count);
        foreach (var w in walls) set.Add((w.Row, w.Col));
        return set;
    }
}
