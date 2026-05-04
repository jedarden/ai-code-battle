<?php
/**
 * Strategy implementation for AI Code Battle.
 *
 * This file contains the compute_moves() function which is called each turn.
 * Implement your bot logic here.
 *
 * Available helpers from game.php:
 * - toroidal_manhattan(Position $a, Position $b, int $rows, int $cols): int
 * - toroidal_chebyshev(Position $a, Position $b, int $rows, int $cols): int
 * - bfs(Position $start, Position $goal, callable $passable, int $rows, int $cols): ?array
 * - cardinal_steps(Position $p, int $rows, int $cols): array
 */

/**
 * Compute moves for all owned bots.
 *
 * @param GameState $state The current game state
 * @return Move[] Array of move commands (bots not included stay in place)
 */
function compute_moves(GameState $state): array {
    $myId = $state->you->id;
    $config = $state->config;
    $rows = $config->rows;
    $cols = $config->cols;

    $moves = [];

    foreach ($state->bots as $bot) {
        if ($bot->owner !== $myId) {
            continue;
        }

        // Example: Find nearest energy and move toward it
        if (!empty($state->energy)) {
            $bestDist = PHP_INT_MAX;
            $bestDir = null;

            foreach (cardinal_steps($bot->position, $rows, $cols) as $step) {
                foreach ($state->energy as $energy) {
                    $dist = toroidal_manhattan($step['pos'], $energy, $rows, $cols);
                    if ($dist < $bestDist) {
                        $bestDist = $dist;
                        $bestDir = $step['dir'];
                    }
                }
            }

            if ($bestDir) {
                $moves[] = new Move($bot->position, $bestDir);
                continue;
            }
        }

        // Default: hold position (don't add a move)
        // Or move randomly:
        // $directions = ['N', 'E', 'S', 'W'];
        // $moves[] = new Move($bot->position, $directions[array_rand($directions)]);
    }

    return $moves;
}
