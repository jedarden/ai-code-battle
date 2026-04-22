package com.acb.raider;

import java.util.*;
import java.util.stream.Collectors;

/**
 * RaiderBot strategy: hit-and-run harassment.
 *
 * - Scouts for lone enemy bots (no allies within 2 cells)
 * - Attacks isolated targets from flank
 * - After 1-2 attack turns, retreats regardless of outcome
 * - Never attacks groups of >=3 enemies
 * - Defends own core if under pressure
 */
public class RaiderStrategy {

    private static final int GROUP_AVOID_THRESHOLD = 3;
    private static final int MAX_ENGAGEMENT_TURNS = 2;
    private static final int ISOLATION_MANHATTAN = 4; // 2 cells squared-distance-ish radius

    // Per-bot engagement tracking
    private final Map<String, EngagementTracker> engagementTrackers = new HashMap<>();

    public List<Move> computeMoves(GameState state) {
        int myId = state.getYou().getId();
        GameConfig config = state.getConfig();
        int rows = config.getRows();
        int cols = config.getCols();
        int attackR2 = config.getAttackRadius2();

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

        Set<String> walls = buildPositionSet(state.getWalls());
        Set<String> enemyPositions = buildPositionSet(
                enemyBots.stream().map(VisibleBot::getPosition).collect(Collectors.toList())
        );
        Set<String> energyPositions = buildPositionSet(state.getEnergy());

        // Identify own active cores
        List<Position> myCores = new ArrayList<>();
        for (VisibleCore core : state.getCores()) {
            if (core.getOwner() == myId && core.isActive()) {
                myCores.add(core.getPosition());
            }
        }

        // Check if own core is under pressure
        boolean coreUnderPressure = isCoreUnderPressure(myCores, enemyBots, rows, cols);

        // Find lone enemy bots
        List<VisibleBot> loneEnemies = findLoneEnemies(enemyBots, rows, cols);

        // Build assignments
        List<Move> moves = new ArrayList<>();
        Set<String> assignedBots = new HashSet<>();
        Set<String> claimedDests = new HashSet<>();

        if (coreUnderPressure && !myCores.isEmpty()) {
            // Core defense: bots near core intercept enemies, others continue raiding
            List<VisibleBot> defenders = new ArrayList<>();
            List<VisibleBot> raiders = new ArrayList<>();

            for (VisibleBot bot : myBots) {
                int nearestCoreDist = minDistToAny(bot.getPosition(), myCores, rows, cols);
                if (nearestCoreDist <= 25) { // within ~5 tiles of a core
                    defenders.add(bot);
                } else {
                    raiders.add(bot);
                }
            }

            // Defenders move toward nearest enemy near core
            for (VisibleBot defender : defenders) {
                Position target = nearestEnemyToCore(myCores, enemyBots, rows, cols);
                Move move = computeDefendMove(defender, target, walls, enemyPositions,
                        claimedDests, rows, cols);
                if (move != null) {
                    assignedBots.add(defender.getPosition().key());
                    Position dest = defender.getPosition().moveToward(move.getDirection(), rows, cols);
                    claimedDests.add(dest.key());
                    moves.add(move);
                }
            }

            // Raiders continue hit-and-run
            myBots = raiders;
        }

        // Assign raiders to lone enemy targets
        Map<VisibleBot, VisibleBot> raidAssignments = assignRaiders(myBots, loneEnemies,
                assignedBots, rows, cols);

        for (Map.Entry<VisibleBot, VisibleBot> entry : raidAssignments.entrySet()) {
            VisibleBot raider = entry.getKey();
            VisibleBot target = entry.getValue();
            String trackerKey = raider.getPosition().key() + "->" + target.getPosition().key();
            EngagementTracker tracker = engagementTrackers.computeIfAbsent(
                    trackerKey, k -> new EngagementTracker());

            Move move;
            if (tracker.engagementTurns >= MAX_ENGAGEMENT_TURNS) {
                // Retreat after max engagement turns
                move = computeRetreatMove(raider, target.getPosition(), walls, enemyPositions,
                        claimedDests, rows, cols);
                if (tracker.engagementTurns >= MAX_ENGAGEMENT_TURNS + 2) {
                    tracker.engagementTurns = 0; // reset after sufficient retreat
                }
            } else {
                // Attack the lone target
                move = computeAttackMove(raider, target.getPosition(), walls, enemyPositions,
                        claimedDests, rows, cols, attackR2);
            }

            if (move != null) {
                assignedBots.add(raider.getPosition().key());
                Position dest = raider.getPosition().moveToward(move.getDirection(), rows, cols);
                claimedDests.add(dest.key());
                moves.add(move);
                tracker.engagementTurns++;
            }
        }

        // Remaining bots: gather energy or explore
        for (VisibleBot bot : myBots) {
            if (assignedBots.contains(bot.getPosition().key())) continue;

            Move move;
            if (!state.getEnergy().isEmpty()) {
                move = computeGatherMove(bot, energyPositions, walls, enemyPositions,
                        claimedDests, rows, cols);
            } else {
                move = computeExploreMove(bot, walls, claimedDests, rows, cols);
            }

            if (move != null) {
                Position dest = bot.getPosition().moveToward(move.getDirection(), rows, cols);
                if (!claimedDests.contains(dest.key())) {
                    claimedDests.add(dest.key());
                    moves.add(move);
                }
            }
        }

        return moves;
    }

    /**
     * Find lone enemies: no allied bots within ~2 manhattan cells
     */
    private List<VisibleBot> findLoneEnemies(List<VisibleBot> enemyBots, int rows, int cols) {
        List<VisibleBot> loneEnemies = new ArrayList<>();

        for (VisibleBot bot : enemyBots) {
            int nearbyAllies = 0;

            // Count enemies within attack_radius2 (same owner = allies of this enemy)
            for (VisibleBot other : enemyBots) {
                if (bot == other) continue;
                int dist2 = bot.getPosition().distance2(other.getPosition(), rows, cols);
                if (dist2 <= 4) { // ~2 cells squared
                    nearbyAllies++;
                }
            }

            // Lone if no nearby allies and check group size
            int localGroupSize = countLocalGroup(bot, enemyBots, rows, cols);
            if (nearbyAllies == 0 && localGroupSize < GROUP_AVOID_THRESHOLD) {
                loneEnemies.add(bot);
            }
        }

        return loneEnemies;
    }

    /**
     * Count how many enemy bots are clustered near this bot (within ~4 manhattan tiles)
     */
    private int countLocalGroup(VisibleBot bot, List<VisibleBot> enemyBots, int rows, int cols) {
        int count = 1; // include self
        for (VisibleBot other : enemyBots) {
            if (bot == other) continue;
            int dist = bot.getPosition().manhattanDistance(other.getPosition(), rows, cols);
            if (dist <= ISOLATION_MANHATTAN) {
                count++;
            }
        }
        return count;
    }

    /**
     * Check if any enemy is close to our core
     */
    private boolean isCoreUnderPressure(List<Position> myCores, List<VisibleBot> enemyBots,
                                         int rows, int cols) {
        for (Position core : myCores) {
            for (VisibleBot enemy : enemyBots) {
                if (core.distance2(enemy.getPosition(), rows, cols) <= 36) { // ~6 tiles
                    return true;
                }
            }
        }
        return false;
    }

    /**
     * Find nearest enemy to any of our cores
     */
    private Position nearestEnemyToCore(List<Position> myCores, List<VisibleBot> enemyBots,
                                         int rows, int cols) {
        Position nearest = null;
        int bestDist = Integer.MAX_VALUE;

        for (Position core : myCores) {
            for (VisibleBot enemy : enemyBots) {
                int dist = core.distance2(enemy.getPosition(), rows, cols);
                if (dist < bestDist) {
                    bestDist = dist;
                    nearest = enemy.getPosition();
                }
            }
        }
        return nearest;
    }

    /**
     * Assign raiders to lone targets using greedy closest-first matching
     */
    private Map<VisibleBot, VisibleBot> assignRaiders(List<VisibleBot> myBots,
                                                       List<VisibleBot> loneEnemies,
                                                       Set<String> alreadyAssigned,
                                                       int rows, int cols) {
        Map<VisibleBot, VisibleBot> assignments = new HashMap<>();
        if (loneEnemies.isEmpty()) return assignments;

        List<VisibleBot> available = myBots.stream()
                .filter(b -> !alreadyAssigned.contains(b.getPosition().key()))
                .collect(Collectors.toList());

        // Sort targets by isolation (most isolated first — easier prey)
        loneEnemies.sort((a, b) -> {
            int groupA = countLocalGroup(a, Collections.emptyList(), rows, cols); // 1 always
            int groupB = countLocalGroup(b, Collections.emptyList(), rows, cols);
            return Integer.compare(groupA, groupB);
        });

        for (VisibleBot target : loneEnemies) {
            if (available.isEmpty()) break;

            // Assign 1-2 raiders per target
            available.sort((a, b) -> {
                int distA = a.getPosition().distance2(target.getPosition(), rows, cols);
                int distB = b.getPosition().distance2(target.getPosition(), rows, cols);
                return Integer.compare(distA, distB);
            });

            int assigned = 0;
            Iterator<VisibleBot> iter = available.iterator();
            while (iter.hasNext() && assigned < 2) {
                VisibleBot raider = iter.next();
                assignments.put(raider, target);
                iter.remove();
                assigned++;
            }
        }

        return assignments;
    }

    /**
     * Compute attack move toward a target, using flanking when possible.
     * Approach from an offset angle rather than head-on.
     */
    private Move computeAttackMove(VisibleBot bot, Position target, Set<String> walls,
                                    Set<String> enemyPositions, Set<String> claimedDests,
                                    int rows, int cols, int attackR2) {
        Direction bestDir = null;
        int bestScore = Integer.MIN_VALUE;

        for (Direction dir : Direction.all()) {
            Position newPos = bot.getPosition().moveToward(dir, rows, cols);
            String key = newPos.key();

            if (walls.contains(key)) continue;
            if (claimedDests.contains(key)) continue;

            int score = 0;
            int distToTarget = newPos.distance2(target, rows, cols);
            int currentDist = bot.getPosition().distance2(target, rows, cols);

            // Reward getting closer
            score += (currentDist - distToTarget) * 10;

            // Big bonus for entering attack range
            if (distToTarget <= attackR2) {
                score += 50;
            }

            // Flanking: prefer approaching from sides (perpendicular offset)
            // rather than directly head-on
            int dr = newPos.getRow() - target.getRow();
            int dc = newPos.getCol() - target.getCol();
            // Wrap differences
            if (Math.abs(dr) > rows / 2) dr = dr > 0 ? dr - rows : dr + rows;
            if (Math.abs(dc) > cols / 2) dc = dc > 0 ? dc - cols : dc + cols;
            // Diagonal approach is flanking
            if (dr != 0 && dc != 0) {
                score += 5; // prefer diagonal/flanking approach
            }

            // Penalty for nearby enemies (more than the target)
            int nearbyEnemies = 0;
            for (String epk : enemyPositions) {
                if (epk.equals(target.key())) continue;
                String[] parts = epk.split(",");
                Position ep = new Position(Integer.parseInt(parts[0]), Integer.parseInt(parts[1]));
                if (newPos.distance2(ep, rows, cols) <= attackR2) {
                    nearbyEnemies++;
                }
            }
            score -= nearbyEnemies * 30; // heavy penalty for enemy reinforcements

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
     * Compute retreat move away from a target
     */
    private Move computeRetreatMove(VisibleBot bot, Position target, Set<String> walls,
                                     Set<String> enemyPositions, Set<String> claimedDests,
                                     int rows, int cols) {
        Direction bestDir = null;
        int bestScore = Integer.MIN_VALUE;

        for (Direction dir : Direction.all()) {
            Position newPos = bot.getPosition().moveToward(dir, rows, cols);
            String key = newPos.key();

            if (walls.contains(key)) continue;
            if (claimedDests.contains(key)) continue;

            int distFromTarget = newPos.distance2(target, rows, cols);

            // Maximize distance from target
            int score = distFromTarget;

            // Penalty for being near any other enemy
            for (String epk : enemyPositions) {
                String[] parts = epk.split(",");
                Position ep = new Position(Integer.parseInt(parts[0]), Integer.parseInt(parts[1]));
                if (newPos.distance2(ep, rows, cols) <= 5) {
                    score -= 20;
                }
            }

            // Reward moving toward nearest energy (opportunistic while retreating)
            // (energy positions not passed here for simplicity)

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
     * Compute defensive move: intercept enemy approaching core
     */
    private Move computeDefendMove(VisibleBot bot, Position target, Set<String> walls,
                                    Set<String> enemyPositions, Set<String> claimedDests,
                                    int rows, int cols) {
        if (target == null) return null;

        Direction bestDir = null;
        int bestDist = Integer.MAX_VALUE;

        for (Direction dir : Direction.all()) {
            Position newPos = bot.getPosition().moveToward(dir, rows, cols);
            String key = newPos.key();

            if (walls.contains(key)) continue;
            if (claimedDests.contains(key)) continue;

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
     * Compute energy-gathering move toward nearest energy
     */
    private Move computeGatherMove(VisibleBot bot, Set<String> energyPositions,
                                    Set<String> walls, Set<String> enemyPositions,
                                    Set<String> claimedDests, int rows, int cols) {
        Position nearestEnergy = null;
        int nearestDist = Integer.MAX_VALUE;

        for (String ek : energyPositions) {
            String[] parts = ek.split(",");
            Position ep = new Position(Integer.parseInt(parts[0]), Integer.parseInt(parts[1]));
            int dist = bot.getPosition().distance2(ep, rows, cols);
            if (dist < nearestDist) {
                nearestDist = dist;
                nearestEnergy = ep;
            }
        }

        if (nearestEnergy != null) {
            Direction bestDir = null;
            int bestDist = Integer.MAX_VALUE;

            for (Direction dir : Direction.all()) {
                Position newPos = bot.getPosition().moveToward(dir, rows, cols);
                String key = newPos.key();

                if (walls.contains(key)) continue;
                if (claimedDests.contains(key)) continue;

                int dist = newPos.distance2(nearestEnergy, rows, cols);

                // Avoid enemies while gathering
                for (String epk : enemyPositions) {
                    String[] parts = epk.split(",");
                    Position ep = new Position(Integer.parseInt(parts[0]), Integer.parseInt(parts[1]));
                    if (newPos.distance2(ep, rows, cols) <= 5) {
                        dist += 20; // penalty
                    }
                }

                if (dist < bestDist) {
                    bestDist = dist;
                    bestDir = dir;
                }
            }

            if (bestDir != null) {
                return new Move(bot.getPosition(), bestDir);
            }
        }
        return null;
    }

    /**
     * Explore: move toward grid center or away from friendly bots
     */
    private Move computeExploreMove(VisibleBot bot, Set<String> walls,
                                     Set<String> claimedDests, int rows, int cols) {
        Position center = new Position(rows / 2, cols / 2);
        Direction bestDir = null;
        int bestDist = Integer.MAX_VALUE;

        for (Direction dir : Direction.all()) {
            Position newPos = bot.getPosition().moveToward(dir, rows, cols);
            String key = newPos.key();

            if (walls.contains(key)) continue;
            if (claimedDests.contains(key)) continue;

            int dist = newPos.distance2(center, rows, cols);
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

    private int minDistToAny(Position pos, List<Position> targets, int rows, int cols) {
        int minDist = Integer.MAX_VALUE;
        for (Position t : targets) {
            minDist = Math.min(minDist, pos.distance2(t, rows, cols));
        }
        return minDist;
    }

    private Set<String> buildPositionSet(List<Position> positions) {
        return positions.stream()
                .map(Position::key)
                .collect(Collectors.toSet());
    }

    /**
     * Tracks how long a raider has been engaged with a target
     */
    private static class EngagementTracker {
        int engagementTurns = 0;
    }
}
