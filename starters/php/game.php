<?php
/**
 * Game state types and grid utilities for AI Code Battle protocol.
 */

/**
 * Position on the grid
 */
class Position {
    public int $row;
    public int $col;

    public function __construct(int $row, int $col) {
        $this->row = $row;
        $this->col = $col;
    }

    public static function fromArray(array $data): self {
        return new self($data['row'], $data['col']);
    }

    public function toArray(): array {
        return ['row' => $this->row, 'col' => $this->col];
    }

    public function equals(Position $other): bool {
        return $this->row === $other->row && $this->col === $other->col;
    }

    public function __toString(): string {
        return "{$this->row},{$this->col}";
    }
}

/**
 * Game configuration
 */
class GameConfig {
    public int $rows;
    public int $cols;
    public int $maxTurns;
    public int $visionRadius2;
    public int $attackRadius2;
    public int $spawnCost;
    public int $energyInterval;
    public ?string $seasonId;
    public ?string $rulesVersion;

    public static function fromArray(array $data): self {
        $config = new self();
        $config->rows = $data['rows'];
        $config->cols = $data['cols'];
        $config->maxTurns = $data['max_turns'];
        $config->visionRadius2 = $data['vision_radius2'];
        $config->attackRadius2 = $data['attack_radius2'];
        $config->spawnCost = $data['spawn_cost'];
        $config->energyInterval = $data['energy_interval'];
        $config->seasonId = $data['season_id'] ?? null;
        $config->rulesVersion = $data['rules_version'] ?? null;
        return $config;
    }
}

/**
 * Player info
 */
class PlayerInfo {
    public int $id;
    public int $energy;
    public int $score;

    public static function fromArray(array $data): self {
        $info = new self();
        $info->id = $data['id'];
        $info->energy = $data['energy'];
        $info->score = $data['score'];
        return $info;
    }
}

/**
 * Visible bot
 */
class VisibleBot {
    public Position $position;
    public int $owner;

    public static function fromArray(array $data): self {
        $bot = new self();
        $bot->position = Position::fromArray($data['position']);
        $bot->owner = $data['owner'];
        return $bot;
    }
}

/**
 * Visible core
 */
class VisibleCore {
    public Position $position;
    public int $owner;
    public bool $active;

    public static function fromArray(array $data): self {
        $core = new self();
        $core->position = Position::fromArray($data['position']);
        $core->owner = $data['owner'];
        $core->active = $data['active'];
        return $core;
    }
}

/**
 * Fog-filtered game state
 */
class GameState {
    public string $matchId;
    public int $turn;
    public GameConfig $config;
    public PlayerInfo $you;
    /** @var VisibleBot[] */
    public array $bots = [];
    /** @var Position[] */
    public array $energy = [];
    /** @var VisibleCore[] */
    public array $cores = [];
    /** @var Position[] */
    public array $walls = [];
    /** @var VisibleBot[] */
    public array $dead = [];

    public static function fromArray(array $data): self {
        $state = new self();
        $state->matchId = $data['match_id'];
        $state->turn = $data['turn'];
        $state->config = GameConfig::fromArray($data['config']);
        $state->you = PlayerInfo::fromArray($data['you']);

        foreach ($data['bots'] ?? [] as $bot) {
            $state->bots[] = VisibleBot::fromArray($bot);
        }

        foreach ($data['energy'] ?? [] as $pos) {
            $state->energy[] = Position::fromArray($pos);
        }

        foreach ($data['cores'] ?? [] as $core) {
            $state->cores[] = VisibleCore::fromArray($core);
        }

        foreach ($data['walls'] ?? [] as $pos) {
            $state->walls[] = Position::fromArray($pos);
        }

        foreach ($data['dead'] ?? [] as $bot) {
            $state->dead[] = VisibleBot::fromArray($bot);
        }

        return $state;
    }
}

/**
 * A single move command
 */
class Move {
    public Position $position;
    public string $direction;

    public function __construct(Position $position, string $direction) {
        $this->position = $position;
        $this->direction = $direction;
    }

    public function toArray(): array {
        return [
            'position' => $this->position->toArray(),
            'direction' => $this->direction
        ];
    }
}

// ============================================================================
// Grid Utilities
// ============================================================================

/**
 * Calculate Manhattan distance with toroidal wrapping
 */
function toroidal_manhattan(Position $a, Position $b, int $rows, int $cols): int {
    $dr = abs($a->row - $b->row);
    $dc = abs($a->col - $b->col);
    $dr = min($dr, $rows - $dr);
    $dc = min($dc, $cols - $dc);
    return $dr + $dc;
}

/**
 * Calculate Chebyshev distance with toroidal wrapping
 */
function toroidal_chebyshev(Position $a, Position $b, int $rows, int $cols): int {
    $dr = abs($a->row - $b->row);
    $dc = abs($a->col - $b->col);
    $dr = min($dr, $rows - $dr);
    $dc = min($dc, $cols - $dc);
    return max($dr, $dc);
}

/**
 * Get 8-directional neighbors with wrap-around
 * @return Position[]
 */
function neighbors(Position $p, int $rows, int $cols): array {
    $offsets = [
        [-1, -1], [-1, 0], [-1, 1],
        [0, -1],           [0, 1],
        [1, -1],  [1, 0],  [1, 1]
    ];
    $result = [];
    foreach ($offsets as [$dr, $dc]) {
        $result[] = new Position(
            ($p->row + $dr + $rows) % $rows,
            ($p->col + $dc + $cols) % $cols
        );
    }
    return $result;
}

/**
 * Get cardinal direction steps with wrap-around
 * @return array Array of ['pos' => Position, 'dir' => string]
 */
function cardinal_steps(Position $p, int $rows, int $cols): array {
    $steps = [
        [-1, 0, 'N'],
        [0, 1, 'E'],
        [1, 0, 'S'],
        [0, -1, 'W']
    ];
    $result = [];
    foreach ($steps as [$dr, $dc, $dir]) {
        $result[] = [
            'pos' => new Position(
                ($p->row + $dr + $rows) % $rows,
                ($p->col + $dc + $cols) % $cols
            ),
            'dir' => $dir
        ];
    }
    return $result;
}

/**
 * BFS pathfinding on a toroidal grid
 *
 * @param Position $start Starting position
 * @param Position $goal Goal position
 * @param callable $passable callable(Position): bool - returns true if cell can be entered
 * @param int $rows Grid rows
 * @param int $cols Grid cols
 * @return Position[]|null Array of positions from start to goal (excluding start), or null if unreachable
 */
function bfs(Position $start, Position $goal, callable $passable, int $rows, int $cols): ?array {
    if ($start->equals($goal)) {
        return [];
    }

    $queue = [[$start, []]];
    $visited = [(string)$start => true];

    while (!empty($queue)) {
        [$current, $path] = array_shift($queue);

        foreach (neighbors($current, $rows, $cols) as $next) {
            if ($next->equals($goal)) {
                return [...$path, $next];
            }

            $key = (string)$next;
            if (!isset($visited[$key]) && $passable($next)) {
                $visited[$key] = true;
                $queue[] = [$next, [...$path, $next]];
            }
        }
    }

    return null;
}
