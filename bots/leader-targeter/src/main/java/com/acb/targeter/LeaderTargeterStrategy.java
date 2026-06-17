package com.acb.targeter;

import java.util.*;
import java.util.stream.Collectors;

/**
 * LeaderTargeterStrategy - Multi-player score leader targeting bot.
 *
 * Strategy: In N>2 games, always direct all units toward the current score leader.
 * - Identify all visible opponents and their scores (cores count as proxy: each active core ≈ +2 score)
 * - Pick primary target: opponent with highest inferred score (tiebreak: nearest)
 * - Send all bots toward primary target's centroid (mean of target's visible bots + cores)
 * - Exception: if own core is under direct threat (enemy bot within 6 tiles), detach 2 bots to defend
 * - In 2-player games: fall back to straight aggressor (target the only opponent)
 *
 * This creates a natural kingmaker dynamic that prevents any single bot from running away with the game.
 */
public class LeaderTargeterStrategy {
    private static final int CORE_DEFENSE_THRESHOLD = 36; // 6 tiles squared distance
    private static final int DEFENDERS_COUNT = 2; // Number of bots to detach for core defense
    private static final int SCORE_PER_ACTIVE_CORE = 2; // Approximate score contribution per active core

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

        // Calculate opponent scores
        Map<Integer, Integer> opponentScores = calculateOpponentScores(enemyBots, state.getCores());

        // Find all unique opponent IDs
        Set<Integer> opponentIds = new HashSet<>();
        for (VisibleBot bot : enemyBots) {
            opponentIds.add(bot.getOwner());
        }
        for (VisibleCore core : state.getCores()) {
            if (core.getOwner() != myId) {
                opponentIds.add(core.getOwner());
            }
        }

        // Build position lookups
        Set<String> walls = buildPositionSet(state.getWalls());
        Set<String> enemyPositions = buildPositionSet(
                enemyBots.stream().map(VisibleBot::getPosition).collect(Collectors.toList())
        );
        Set<String> myBotPositions = buildPositionSet(
                myBots.stream().map(VisibleBot::getPosition).collect(Collectors.toList())
        );

        // Check if own core is under threat
        Position myCorePosition = findOwnCore(state.getCores(), myId);
        boolean coreUnderThreat = false;
        if (myCorePosition != null && !enemyBots.isEmpty()) {
            for (VisibleBot enemyBot : enemyBots) {
                if (enemyBot.getPosition().distance2(myCorePosition, rows, cols) <= CORE_DEFENSE_THRESHOLD) {
                    coreUnderThreat = true;
                    break;
                }
            }
        }

        // Select target based on game type
        Position targetPosition;
        int targetOwnerId;

        if (opponentIds.size() <= 1) {
            // 2-player game: target the only opponent
            targetOwnerId = opponentIds.isEmpty() ? -1 : opponentIds.iterator().next();
            targetPosition = findOpponentCentroid(enemyBots, state.getCores(), targetOwnerId, rows, cols);
            System.out.println("2-player game: targeting opponent " + targetOwnerId);
        } else {
            // Multi-player game: target the score leader
            int scoreLeaderId = findScoreLeader(opponentScores, opponentIds, rows, cols,
                    enemyBots, state.getCores(), myBots.get(0).getPosition());
            targetOwnerId = scoreLeaderId;
            targetPosition = findOpponentCentroid(enemyBots, state.getCores(), scoreLeaderId, rows, cols);
            System.out.println("Multi-player game: score leader is opponent " + scoreLeaderId +
                    " with score ~" + opponentScores.getOrDefault(scoreLeaderId, 0));
        }

        if (targetPosition == null) {
            // No valid target, move toward center
            targetPosition = new Position(rows / 2, cols / 2);
            System.out.println("No valid target, moving toward center");
        }

        // Compute moves
        List<Move> moves = new ArrayList<>();
        Set<String> assignedBots = new HashSet<>();

        // If core is under threat, assign defenders first
        if (coreUnderThreat && myCorePosition != null) {
            List<VisibleBot> defenders = findNearestBots(myBots, myCorePosition, DEFENDERS_COUNT, rows, cols);
            for (VisibleBot defender : defenders) {
                Move move = computeDefensiveMove(defender, myCorePosition, enemyPositions, walls,
                        myBotPositions, rows, cols);
                if (move != null) {
                    moves.add(move);
                    assignedBots.add(defender.getPosition().key());
                    myBotPositions.remove(defender.getPosition().key());
                }
            }
            System.out.println("Core under threat! Assigned " + defenders.size() + " defenders");
        }

        // Assign remaining bots to attack the primary target
        for (VisibleBot bot : myBots) {
            if (assignedBots.contains(bot.getPosition().key())) {
                continue; // Already assigned as defender
            }

            Move move = computeAttackMove(bot, targetPosition, enemyPositions, walls, myBotPositions, rows, cols);
            if (move != null) {
                moves.add(move);
                myBotPositions.remove(bot.getPosition().key());
            }
        }

        System.out.println("Computed " + moves.size() + " moves for " + myBots.size() + " bots");

        return moves;
    }

    /**
     * Calculate approximate scores for all opponents based on visible bots and cores
     */
    private Map<Integer, Integer> calculateOpponentScores(List<VisibleBot> enemyBots, List<VisibleCore> cores) {
        Map<Integer, Integer> scores = new HashMap<>();

        // Count bots per opponent
        Map<Integer, Integer> botCounts = new HashMap<>();
        for (VisibleBot bot : enemyBots) {
            botCounts.merge(bot.getOwner(), 1, Integer::sum);
        }

        // Count active cores per opponent
        Map<Integer, Integer> coreCounts = new HashMap<>();
        for (VisibleCore core : cores) {
            if (core.isActive()) {
                coreCounts.merge(core.getOwner(), 1, Integer::sum);
            }
        }

        // Calculate approximate scores: botCount * 10 + activeCoreCount * SCORE_PER_ACTIVE_CORE
        // (Each bot is worth ~10 points based on spawn cost)
        for (Integer ownerId : botCounts.keySet()) {
            int botCount = botCounts.get(ownerId);
            int coreCount = coreCounts.getOrDefault(ownerId, 0);
            int estimatedScore = botCount * 10 + coreCount * SCORE_PER_ACTIVE_CORE;
            scores.put(ownerId, estimatedScore);
        }

        // Include opponents with only cores visible
        for (Integer ownerId : coreCounts.keySet()) {
            if (!scores.containsKey(ownerId)) {
                int coreCount = coreCounts.get(ownerId);
                scores.put(ownerId, coreCount * SCORE_PER_ACTIVE_CORE);
            }
        }

        return scores;
    }

    /**
     * Find the score leader among opponents
     */
    private int findScoreLeader(Map<Integer, Integer> opponentScores, Set<Integer> opponentIds,
                                int rows, int cols, List<VisibleBot> enemyBots,
                                List<VisibleCore> cores, Position referencePosition) {
        int leaderId = -1;
        int maxScore = Integer.MIN_VALUE;
        int minDistance = Integer.MAX_VALUE;

        for (Integer ownerId : opponentIds) {
            int score = opponentScores.getOrDefault(ownerId, 0);

            // Find this opponent's centroid for distance calculation
            Position centroid = findOpponentCentroid(enemyBots, cores, ownerId, rows, cols);
            int distance = centroid != null ? referencePosition.distance2(centroid, rows, cols) : Integer.MAX_VALUE;

            // Prefer higher score, tiebreak by nearest distance
            if (score > maxScore || (score == maxScore && distance < minDistance)) {
                maxScore = score;
                minDistance = distance;
                leaderId = ownerId;
            }
        }

        return leaderId != -1 ? leaderId : opponentIds.iterator().next();
    }

    /**
     * Find the centroid (average position) of an opponent's visible assets
     */
    private Position findOpponentCentroid(List<VisibleBot> enemyBots, List<VisibleCore> cores,
                                          int ownerId, int rows, int cols) {
        List<Position> positions = new ArrayList<>();

        // Add bot positions
        for (VisibleBot bot : enemyBots) {
            if (bot.getOwner() == ownerId) {
                positions.add(bot.getPosition());
            }
        }

        // Add core positions
        for (VisibleCore core : cores) {
            if (core.getOwner() == ownerId) {
                positions.add(core.getPosition());
            }
        }

        if (positions.isEmpty()) {
            return null;
        }

        // Calculate average position
        double avgRow = 0;
        double avgCol = 0;
        for (Position pos : positions) {
            avgRow += pos.getRow();
            avgCol += pos.getCol();
        }
        avgRow /= positions.size();
        avgCol /= positions.size();

        return new Position((int) Math.round(avgRow), (int) Math.round(avgCol));
    }

    /**
     * Find own core position
     */
    private Position findOwnCore(List<VisibleCore> cores, int myId) {
        for (VisibleCore core : cores) {
            if (core.getOwner() == myId) {
                return core.getPosition();
            }
        }
        return null;
    }

    /**
     * Find the N nearest bots to a target position
     */
    private List<VisibleBot> findNearestBots(List<VisibleBot> bots, Position target,
                                              int count, int rows, int cols) {
        List<VisibleBot> sorted = new ArrayList<>(bots);
        sorted.sort((a, b) -> {
            int distA = a.getPosition().distance2(target, rows, cols);
            int distB = b.getPosition().distance2(target, rows, cols);
            return Integer.compare(distA, distB);
        });

        return sorted.stream().limit(count).collect(Collectors.toList());
    }

    /**
     * Compute a defensive move to protect own core
     */
    private Move computeDefensiveMove(VisibleBot bot, Position corePosition,
                                      Set<String> enemyPositions, Set<String> walls,
                                      Set<String> myBotPositions, int rows, int cols) {
        // Move to position core to intercept enemies
        // Prefer positions that are between core and nearest enemy
        Direction bestDir = null;
        int bestScore = Integer.MIN_VALUE;

        for (Direction dir : Direction.all()) {
            Position newPos = bot.getPosition().moveToward(dir, rows, cols);
            String newPosKey = newPos.key();

            if (walls.contains(newPosKey)) {
                continue;
            }

            if (myBotPositions.contains(newPosKey)) {
                continue;
            }

            // Score: prefer being close to core but not on it
            int distToCore = newPos.distance2(corePosition, rows, cols);
            int score = -distToCore;

            // Bonus for being between core and potential enemies
            if (distToCore > 0 && distToCore <= 9) { // 2-3 tiles from core
                score += 50;
            }

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
     * Compute an attack move toward the target position
     */
    private Move computeAttackMove(VisibleBot bot, Position target,
                                   Set<String> enemyPositions, Set<String> walls,
                                   Set<String> myBotPositions, int rows, int cols) {
        Direction bestDir = null;
        int bestScore = Integer.MIN_VALUE;

        for (Direction dir : Direction.all()) {
            Position newPos = bot.getPosition().moveToward(dir, rows, cols);
            String newPosKey = newPos.key();

            if (walls.contains(newPosKey)) {
                continue;
            }

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

            // Small penalty for moving adjacent to multiple enemies (but less strict than hunter)
            int adjacentEnemies = 0;
            for (String enemyPosKey : enemyPositions) {
                String[] parts = enemyPosKey.split(",");
                Position enemyPos = new Position(Integer.parseInt(parts[0]), Integer.parseInt(parts[1]));
                if (newPos.distance2(enemyPos, rows, cols) <= 2) {
                    adjacentEnemies++;
                }
            }
            score -= adjacentEnemies * 5; // Lower penalty than hunter, we're aggressive

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
     * Build a set of position keys for O(1) lookup
     */
    private Set<String> buildPositionSet(List<Position> positions) {
        return positions.stream()
                .map(Position::key)
                .collect(Collectors.toSet());
    }
}
