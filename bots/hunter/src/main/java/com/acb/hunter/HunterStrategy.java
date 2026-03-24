package com.acb.hunter;

import java.util.*;
import java.util.stream.Collectors;

/**
 * HunterBot strategy: target isolated enemies for efficient kills.
 *
 * Strategy: Target isolated enemy bots for efficient kills.
 * - Identify enemy bots that are >=4 tiles from their nearest friendly bot (isolated targets)
 * - Send pairs of bots to intercept isolated enemies (2v1 wins cleanly)
 * - If no isolated targets, default to gatherer behavior
 * - Maintain a map of known enemy positions across turns, predict movement
 * - Avoid engaging formations of 3+ enemy bots
 * - Opportunistic energy collection when not actively hunting
 */
public class HunterStrategy {
    private static final int ISOLATION_THRESHOLD = 16; // Squared distance (4 tiles)
    private static final int FORMATION_SIZE = 3; // Avoid groups of 3+ enemies

    // Track known enemy positions for prediction
    private final Map<String, EnemyTracker> enemyTrackers = new HashMap<>();

    /**
     * Compute moves for all owned bots
     */
    public List<Move> computeMoves(GameState state) {
        int myId = state.getYou().getId();
        GameConfig config = state.getConfig();
        int rows = config.getRows();
        int cols = config.getCols();

        // Separate my bots from enemies
        List<VisibleBot> myBots = new ArrayList<>();
        List<VisibleBot> enemyBots = new ArrayList<>();

        for (VisibleBot bot : state.getBots()) {
            if (bot.getOwner() == myId) {
                myBots.add(bot);
            } else {
                enemyBots.add(bot);
            }
        }

        if (myBots.isEmpty()) {
            return Collections.emptyList();
        }

        // Update enemy trackers
        updateEnemyTrackers(enemyBots, rows, cols);

        // Build position lookups
        Set<String> walls = buildPositionSet(state.getWalls());
        Set<String> enemyPositions = buildPositionSet(
                enemyBots.stream().map(VisibleBot::getPosition).collect(Collectors.toList())
        );
        Set<String> myBotPositions = buildPositionSet(
                myBots.stream().map(VisibleBot::getPosition).collect(Collectors.toList())
        );

        // Find isolated enemy targets
        List<VisibleBot> isolatedEnemies = findIsolatedEnemies(enemyBots, rows, cols);

        // Find energy positions
        Set<String> energyPositions = buildPositionSet(state.getEnergy());

        // Assign bots to targets
        List<Move> moves = new ArrayList<>();
        Set<String> usedEnergy = new HashSet<>();
        Set<Position> assignedTargets = new HashSet<>();

        // First, assign hunters to isolated enemies
        Map<VisibleBot, VisibleBot> hunterAssignments = assignHunters(myBots, isolatedEnemies, rows, cols);

        for (Map.Entry<VisibleBot, VisibleBot> entry : hunterAssignments.entrySet()) {
            VisibleBot hunter = entry.getKey();
            VisibleBot target = entry.getValue();

            // Get predicted position of target
            Position predictedPos = predictPosition(target, rows, cols);
            assignedTargets.add(predictedPos);

            Move move = computeHunterMove(hunter, predictedPos, enemyPositions, walls, myBotPositions, rows, cols);
            if (move != null) {
                moves.add(move);
                // Mark this bot as assigned
                myBotPositions.remove(hunter.getPosition().key());
            }
        }

        // Second, assign remaining bots to gather or explore
        for (VisibleBot bot : myBots) {
            if (!myBotPositions.contains(bot.getPosition().key())) {
                continue; // Already assigned
            }

            Move move;
            if (!energyPositions.isEmpty()) {
                move = computeGatherMove(bot, energyPositions, usedEnergy, enemyPositions, walls, rows, cols);
            } else {
                move = computeExploreMove(bot, enemyPositions, walls, rows, cols);
            }

            if (move != null) {
                moves.add(move);
            }
        }

        return moves;
    }

    /**
     * Update enemy position trackers for prediction
     */
    private void updateEnemyTrackers(List<VisibleBot> enemyBots, int rows, int cols) {
        for (VisibleBot bot : enemyBots) {
            String key = bot.getPosition().key();
            EnemyTracker tracker = enemyTrackers.computeIfAbsent(key, k -> new EnemyTracker());
            tracker.update(bot.getPosition(), rows, cols);
        }
    }

    /**
     * Find isolated enemy bots (>=4 tiles from nearest friendly)
     */
    private List<VisibleBot> findIsolatedEnemies(List<VisibleBot> enemyBots, int rows, int cols) {
        List<VisibleBot> isolated = new ArrayList<>();

        for (VisibleBot bot : enemyBots) {
            boolean isIsolated = true;
            int nearestDist = Integer.MAX_VALUE;

            for (VisibleBot other : enemyBots) {
                if (bot == other) continue;

                int dist = bot.getPosition().distance2(other.getPosition(), rows, cols);
                nearestDist = Math.min(nearestDist, dist);
            }

            // Isolated if nearest friendly is >= 4 tiles away (squared distance 16)
            // or if it's the only enemy bot
            if (nearestDist >= ISOLATION_THRESHOLD || enemyBots.size() == 1) {
                isolated.add(bot);
            }
        }

        return isolated;
    }

    /**
     * Assign hunters to isolated targets using greedy matching
     */
    private Map<VisibleBot, VisibleBot> assignHunters(
            List<VisibleBot> myBots,
            List<VisibleBot> isolatedEnemies,
            int rows, int cols
    ) {
        Map<VisibleBot, VisibleBot> assignments = new HashMap<>();

        if (isolatedEnemies.isEmpty()) {
            return assignments;
        }

        // Sort my bots by distance to nearest isolated enemy
        List<VisibleBot> availableHunters = new ArrayList<>(myBots);

        // Assign 2 hunters per target when possible
        for (VisibleBot target : isolatedEnemies) {
            int huntersNeeded = 2;

            // Sort available hunters by distance to target
            availableHunters.sort((a, b) -> {
                int distA = a.getPosition().distance2(target.getPosition(), rows, cols);
                int distB = b.getPosition().distance2(target.getPosition(), rows, cols);
                return Integer.compare(distA, distB);
            });

            int assigned = 0;
            Iterator<VisibleBot> iter = availableHunters.iterator();
            while (iter.hasNext() && assigned < huntersNeeded) {
                VisibleBot hunter = iter.next();
                assignments.put(hunter, target);
                iter.remove();
                assigned++;
            }
        }

        return assignments;
    }

    /**
     * Predict where an enemy will be next turn
     */
    private Position predictPosition(VisibleBot enemy, int rows, int cols) {
        String key = enemy.getPosition().key();
        EnemyTracker tracker = enemyTrackers.get(key);

        if (tracker != null && tracker.hasPrediction()) {
            return tracker.predictNextPosition(rows, cols);
        }

        return enemy.getPosition();
    }

    /**
     * Compute move for a hunter bot toward a target
     */
    private Move computeHunterMove(
            VisibleBot bot,
            Position target,
            Set<String> enemyPositions,
            Set<String> walls,
            Set<String> myBotPositions,
            int rows, int cols
    ) {
        Direction bestDir = null;
        int bestScore = Integer.MIN_VALUE;

        for (Direction dir : Direction.all()) {
            Position newPos = bot.getPosition().moveToward(dir, rows, cols);
            String newPosKey = newPos.key();

            // Can't move into walls
            if (walls.contains(newPosKey)) {
                continue;
            }

            // Avoid self-collision
            if (myBotPositions.contains(newPosKey)) {
                continue;
            }

            // Score: prefer getting closer to target
            int distToTarget = newPos.distance2(target, rows, cols);
            int currentDistToTarget = bot.getPosition().distance2(target, rows, cols);
            int score = currentDistToTarget - distToTarget;

            // Bonus for being in attack range of target
            if (distToTarget <= 5) { // attack_radius2
                score += 20;
            }

            // Penalty for moving adjacent to multiple enemies
            int adjacentEnemies = 0;
            for (String enemyPosKey : enemyPositions) {
                String[] parts = enemyPosKey.split(",");
                Position enemyPos = new Position(Integer.parseInt(parts[0]), Integer.parseInt(parts[1]));
                if (newPos.distance2(enemyPos, rows, cols) <= 2) {
                    adjacentEnemies++;
                }
            }
            score -= adjacentEnemies * 10;

            if (score > bestScore) {
                bestScore = score;
                bestDir = dir;
            }
        }

        if (bestDir != null) {
            return new Move(bot.getPosition(), bestDir);
        }

        return null;
    }

    /**
     * Compute move for a gatherer bot
     */
    private Move computeGatherMove(
            VisibleBot bot,
            Set<String> energyPositions,
            Set<String> usedEnergy,
            Set<String> enemyPositions,
            Set<String> walls,
            int rows, int cols
    ) {
        // Find nearest untargeted energy
        Position nearestEnergy = null;
        int nearestDist = Integer.MAX_VALUE;

        for (String energyKey : energyPositions) {
            if (usedEnergy.contains(energyKey)) continue;

            String[] parts = energyKey.split(",");
            Position energyPos = new Position(Integer.parseInt(parts[0]), Integer.parseInt(parts[1]));
            int dist = bot.getPosition().distance2(energyPos, rows, cols);

            if (dist < nearestDist) {
                nearestDist = dist;
                nearestEnergy = energyPos;
            }
        }

        if (nearestEnergy != null) {
            usedEnergy.add(nearestEnergy.key());
            return computeMoveToward(bot, nearestEnergy, walls, rows, cols);
        }

        return null;
    }

    /**
     * Compute move for exploration
     */
    private Move computeExploreMove(
            VisibleBot bot,
            Set<String> enemyPositions,
            Set<String> walls,
            int rows, int cols
    ) {
        // Move toward center if no other target
        Position center = new Position(rows / 2, cols / 2);
        return computeMoveToward(bot, center, walls, rows, cols);
    }

    /**
     * Compute move toward a target position
     */
    private Move computeMoveToward(VisibleBot bot, Position target, Set<String> walls, int rows, int cols) {
        Direction bestDir = null;
        int bestDist = Integer.MAX_VALUE;

        for (Direction dir : Direction.all()) {
            Position newPos = bot.getPosition().moveToward(dir, rows, cols);

            if (walls.contains(newPos.key())) {
                continue;
            }

            int dist = newPos.distance2(target, rows, cols);
            if (dist < bestDist) {
                bestDist = dist;
                bestDir = dir;
            }
        }

        if (bestDir != null) {
            return new Move(bot.getPosition(), bestDir);
        }

        return null;
    }

    /**
     * Build a set of position keys for O(1) lookup
     */
    private Set<String> buildPositionSet(List<Position> positions) {
        return positions.stream()
                .map(Position::key)
                .collect(Collectors.toSet());
    }
}

/**
 * Tracks enemy position history for movement prediction
 */
class EnemyTracker {
    private Position lastPosition;
    private Position currentPosition;
    private int sightings;

    public void update(Position position, int rows, int cols) {
        lastPosition = currentPosition;
        currentPosition = position;
        sightings++;
    }

    public boolean hasPrediction() {
        return lastPosition != null && currentPosition != null;
    }

    public Position predictNextPosition(int rows, int cols) {
        if (!hasPrediction()) {
            return currentPosition;
        }

        // Simple prediction: continue in same direction
        int dr = currentPosition.getRow() - lastPosition.getRow();
        int dc = currentPosition.getCol() - lastPosition.getCol();

        // Handle wrap
        if (dr > rows / 2) dr -= rows;
        if (dr < -rows / 2) dr += rows;
        if (dc > cols / 2) dc -= cols;
        if (dc < -cols / 2) dc += cols;

        // Predict next position
        int newRow = (currentPosition.getRow() + dr + rows) % rows;
        int newCol = (currentPosition.getCol() + dc + cols) % cols;

        return new Position(newRow, newCol);
    }
}
