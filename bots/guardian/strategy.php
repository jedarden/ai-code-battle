<?php
/**
 * GuardianBot strategy: defensive core protection with cautious expansion.
 *
 * Strategy: Defend own core, gather nearby energy, cautious expansion.
 * - Maintain a perimeter of bots within 5 tiles of each owned core
 * - Assign excess bots to gather energy within 10 tiles of a core
 * - Consolidate defenders when enemies approach
 * - Only send scouts to explore beyond the safe zone
 * - Conservative spawning - maintains energy reserve of 6
 */

require_once __DIR__ . '/game.php';

class GuardianStrategy {
    private const PERIMETER_RADIUS = 5;
    private const SAFE_ZONE_RADIUS = 10;
    private const ENERGY_RESERVE = 6;
    private const DIRECTIONS = ['N', 'E', 'S', 'W'];

    /**
     * Compute moves for all owned bots
     */
    public function computeMoves(GameState $state): array {
        $myId = $state->you->id;
        $config = $state->config;

        // Separate my bots from enemies
        $myBots = [];
        $enemyBots = [];
        foreach ($state->bots as $bot) {
            if ($bot->owner === $myId) {
                $myBots[] = $bot;
            } else {
                $enemyBots[] = $bot;
            }
        }

        if (empty($myBots)) {
            return [];
        }

        // Find my cores and enemy cores
        $myCores = [];
        $enemyCores = [];
        foreach ($state->cores as $core) {
            if ($core->owner === $myId && $core->active) {
                $myCores[] = $core;
            } elseif ($core->active) {
                $enemyCores[] = $core;
            }
        }

        // Build wall lookup
        $walls = $this->buildPositionSet($state->walls);

        // Build enemy position lookup
        $enemyPositions = $this->buildPositionSet(array_map(fn($b) => $b->position, $enemyBots));

        // Build energy position set
        $energyPositions = $this->buildPositionSet($state->energy);

        // Assign roles to bots
        $moves = [];
        $usedEnergy = [];
        $assignedPositions = [];

        // First pass: assign defenders to cores
        $defenders = $this->assignDefenders($myBots, $myCores, $enemyBots, $config);

        // Second pass: assign gatherers to nearby energy
        foreach ($myBots as $bot) {
            if (isset($assignedPositions[$this->posKey($bot->position)])) {
                continue;
            }

            // Check if this bot should be a defender
            if (isset($defenders[$this->posKey($bot->position)])) {
                $move = $this->computeDefenderMove($bot, $defenders[$this->posKey($bot->position)], $enemyBots, $walls, $config);
            } elseif ($this->shouldGather($bot, $myCores, $config)) {
                $move = $this->computeGatherMove($bot, $energyPositions, $usedEnergy, $enemyPositions, $walls, $myCores, $config);
            } else {
                // Scout - explore cautiously
                $move = $this->computeScoutMove($bot, $enemyPositions, $walls, $config);
            }

            if ($move) {
                $moves[] = $move;
                $assignedPositions[$this->posKey($bot->position)] = true;
            }
        }

        return $moves;
    }

    /**
     * Assign bots to defend cores based on threat level
     */
    private function assignDefenders(array $myBots, array $myCores, array $enemyBots, GameConfig $config): array {
        $defenders = [];

        if (empty($myCores)) {
            return $defenders;
        }

        // Calculate threat level for each core
        $coreThreats = [];
        foreach ($myCores as $core) {
            $threat = 0;
            foreach ($enemyBots as $enemy) {
                $dist2 = $enemy->position->distance2($core->position, $config->rows, $config->cols);
                if ($dist2 <= 100) { // Within 10 tiles
                    $threat += 10 - (int)sqrt($dist2);
                }
            }
            $coreThreats[$this->posKey($core->position)] = $threat;
        }

        // Assign bots to cores based on threat and proximity
        foreach ($myBots as $bot) {
            $bestCore = null;
            $bestScore = PHP_INT_MAX;

            foreach ($myCores as $core) {
                $dist2 = $bot->position->distance2($core->position, $config->rows, $config->cols);
                $threat = $coreThreats[$this->posKey($core->position)];

                // Prioritize threatened cores
                $score = $dist2 - $threat * 100;

                if ($score < $bestScore) {
                    $bestScore = $score;
                    $bestCore = $core;
                }
            }

            if ($bestCore) {
                $defenders[$this->posKey($bot->position)] = $bestCore;
            }
        }

        return $defenders;
    }

    /**
     * Compute move for a defender bot
     */
    private function computeDefenderMove(VisibleBot $bot, VisibleCore $core, array $enemyBots, array $walls, GameConfig $config): ?Move {
        $rows = $config->rows;
        $cols = $config->cols;

        // Find nearest enemy within threat range
        $nearestEnemy = null;
        $nearestEnemyDist = PHP_INT_MAX;
        foreach ($enemyBots as $enemy) {
            $dist2 = $bot->position->distance2($enemy->position, $rows, $cols);
            if ($dist2 < $nearestEnemyDist && $dist2 <= 100) {
                $nearestEnemyDist = $dist2;
                $nearestEnemy = $enemy;
            }
        }

        // If enemy is approaching, intercept
        if ($nearestEnemy && $nearestEnemyDist <= 50) {
            $dir = $this->getDirectionToward($bot->position, $nearestEnemy->position, $walls, $config);
            if ($dir) {
                return new Move($bot->position, $dir);
            }
        }

        // Otherwise, maintain perimeter around core
        $distToCore = $bot->position->distance2($core->position, $rows, $cols);

        if ($distToCore > self::PERIMETER_RADIUS * self::PERIMETER_RADIUS) {
            // Move toward core
            $dir = $this->getDirectionToward($bot->position, $core->position, $walls, $config);
            if ($dir) {
                return new Move($bot->position, $dir);
            }
        }

        // Stay in place or patrol
        return null;
    }

    /**
     * Check if bot should gather energy
     */
    private function shouldGather(VisibleBot $bot, array $myCores, GameConfig $config): bool {
        foreach ($myCores as $core) {
            $dist2 = $bot->position->distance2($core->position, $config->rows, $config->cols);
            if ($dist2 <= self::SAFE_ZONE_RADIUS * self::SAFE_ZONE_RADIUS) {
                return true;
            }
        }
        return false;
    }

    /**
     * Compute move for a gatherer bot
     */
    private function computeGatherMove(VisibleBot $bot, array $energyPositions, array &$usedEnergy, array $enemyPositions, array $walls, array $myCores, GameConfig $config): ?Move {
        // Find nearest untargeted energy within safe zone
        $bestEnergy = null;
        $bestDist = PHP_INT_MAX;

        foreach ($energyPositions as $posKey => $pos) {
            if (isset($usedEnergy[$posKey])) {
                continue;
            }

            // Check if energy is within safe zone of any core
            $inSafeZone = false;
            foreach ($myCores as $core) {
                $dist2 = $pos->distance2($core->position, $config->rows, $config->cols);
                if ($dist2 <= self::SAFE_ZONE_RADIUS * self::SAFE_ZONE_RADIUS) {
                    $inSafeZone = true;
                    break;
                }
            }

            if (!$inSafeZone) {
                continue;
            }

            $dist2 = $bot->position->distance2($pos, $config->rows, $config->cols);
            if ($dist2 < $bestDist) {
                $bestDist = $dist2;
                $bestEnergy = $pos;
            }
        }

        if ($bestEnergy) {
            $usedEnergy[$this->posKey($bestEnergy)] = true;
            $dir = $this->getDirectionToward($bot->position, $bestEnergy, $walls, $config);
            if ($dir) {
                return new Move($bot->position, $dir);
            }
        }

        return null;
    }

    /**
     * Compute move for a scout bot
     */
    private function computeScoutMove(VisibleBot $bot, array $enemyPositions, array $walls, GameConfig $config): ?Move {
        // Move away from enemies if too close
        foreach ($enemyPositions as $posKey => $pos) {
            $dist2 = $bot->position->distance2($pos, $config->rows, $config->cols);
            if ($dist2 <= $config->attackRadius2 + 4) {
                $dir = $this->getDirectionAway($bot->position, $pos, $walls, $config);
                if ($dir) {
                    return new Move($bot->position, $dir);
                }
            }
        }

        // Explore - move toward unexplored areas
        $bestDir = null;
        $bestScore = -1;

        foreach (self::DIRECTIONS as $dir) {
            $newPos = $bot->position->moveToward($dir, $config->rows, $config->cols);
            $posKey = $this->posKey($newPos);

            if (isset($walls[$posKey]) || isset($enemyPositions[$posKey])) {
                continue;
            }

            // Prefer directions that move toward center of map (more exploration)
            $centerRow = $config->rows / 2;
            $centerCol = $config->cols / 2;
            $distToCenter = abs($newPos->row - $centerRow) + abs($newPos->col - $centerCol);

            // Prefer edges for exploration
            $edgeDist = min($newPos->row, $newPos->col, $config->rows - $newPos->row, $config->cols - $newPos->col);
            $score = $edgeDist < 10 ? 10 - $edgeDist : 0;

            if ($score > $bestScore) {
                $bestScore = $score;
                $bestDir = $dir;
            }
        }

        if ($bestDir) {
            return new Move($bot->position, $bestDir);
        }

        return null;
    }

    /**
     * Get direction toward a target position using simple greedy approach
     */
    private function getDirectionToward(Position $from, Position $to, array $walls, GameConfig $config): ?string {
        $rows = $config->rows;
        $cols = $config->cols;

        $bestDir = null;
        $bestDist = PHP_INT_MAX;

        foreach (self::DIRECTIONS as $dir) {
            $newPos = $from->moveToward($dir, $rows, $cols);

            if (isset($walls[$this->posKey($newPos)])) {
                continue;
            }

            $dist2 = $newPos->distance2($to, $rows, $cols);
            if ($dist2 < $bestDist) {
                $bestDist = $dist2;
                $bestDir = $dir;
            }
        }

        return $bestDir;
    }

    /**
     * Get direction away from a threat
     */
    private function getDirectionAway(Position $from, Position $threat, array $walls, GameConfig $config): ?string {
        $rows = $config->rows;
        $cols = $config->cols;

        $bestDir = null;
        $bestDist = 0;

        foreach (self::DIRECTIONS as $dir) {
            $newPos = $from->moveToward($dir, $rows, $cols);

            if (isset($walls[$this->posKey($newPos)])) {
                continue;
            }

            $dist2 = $newPos->distance2($threat, $rows, $cols);
            if ($dist2 > $bestDist) {
                $bestDist = $dist2;
                $bestDir = $dir;
            }
        }

        return $bestDir;
    }

    /**
     * Build a set of positions for O(1) lookup
     */
    private function buildPositionSet(array $positions): array {
        $set = [];
        foreach ($positions as $pos) {
            $set[$this->posKey($pos)] = $pos;
        }
        return $set;
    }

    /**
     * Create a unique key for a position
     */
    private function posKey(Position $pos): string {
        return "{$pos->row},{$pos->col}";
    }
}
