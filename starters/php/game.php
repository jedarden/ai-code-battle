<?php
/**
 * Game state types and grid utilities for AI Code Battle protocol.
 */

function acb_is_json_object($value): bool {
    return $value instanceof stdClass;
}

function acb_has_only_fields($object, array $allowed): bool {
    foreach ($object as $key => $_) {
        if (!in_array($key, $allowed, true)) {
            return false;
        }
    }
    return true;
}

function acb_valid_position($value): bool {
    return acb_is_json_object($value)
        && acb_has_only_fields($value, ['row', 'col'])
        && property_exists($value, 'row') && is_int($value->row)
        && property_exists($value, 'col') && is_int($value->col);
}

function acb_valid_element($value, string $shape): bool {
    if (!acb_is_json_object($value)) {
        return false;
    }
    if ($shape === 'point') {
        return acb_valid_position($value);
    }
    $allowed = $shape === 'core' ? ['position', 'owner', 'active'] : ['position', 'owner'];
    if (!acb_has_only_fields($value, $allowed)
        || !property_exists($value, 'position') || !acb_valid_position($value->position)
        || !property_exists($value, 'owner') || !is_int($value->owner)) {
        return false;
    }
    return $shape !== 'core' || (property_exists($value, 'active') && is_bool($value->active));
}

function validate_request_schema($state): bool {
    $required = ['match_id', 'turn', 'config', 'you', 'bots', 'energy', 'cores', 'walls', 'dead'];
    if (!acb_is_json_object($state) || !acb_has_only_fields($state, array_merge($required, ['zone']))) {
        return false;
    }
    foreach ($required as $key) {
        if (!property_exists($state, $key)) {
            return false;
        }
    }
    if (!is_string($state->match_id) || $state->match_id === '' || !is_int($state->turn)) {
        return false;
    }

    $config = $state->config;
    $configRequired = ['rows', 'cols', 'max_turns', 'vision_radius2', 'attack_radius2', 'spawn_cost', 'energy_interval', 'cores_per_player', 'zone_enabled', 'zone_start_turn', 'zone_shrink_interval', 'zone_shrink_step', 'zone_min_radius', 'kill_score'];
    $configAllowed = array_merge($configRequired, ['map_id', 'season_id', 'rules_version', 'turn_timeout']);
    if (!acb_is_json_object($config) || !acb_has_only_fields($config, $configAllowed)) {
        return false;
    }
    foreach ($configRequired as $key) {
        if (!property_exists($config, $key)) {
            return false;
        }
        if ($key === 'zone_enabled' ? !is_bool($config->$key) : !is_int($config->$key)) {
            return false;
        }
    }
    foreach (['map_id', 'season_id', 'rules_version'] as $key) {
        if (property_exists($config, $key) && !is_string($config->$key)) {
            return false;
        }
    }
    if (property_exists($config, 'turn_timeout') && !is_int($config->turn_timeout)) {
        return false;
    }

    $you = $state->you;
    if (!acb_is_json_object($you) || !acb_has_only_fields($you, ['id', 'energy', 'score'])) {
        return false;
    }
    foreach (['id', 'energy', 'score'] as $key) {
        if (!property_exists($you, $key) || !is_int($you->$key)) {
            return false;
        }
    }

    foreach (['bots' => 'bot', 'dead' => 'bot', 'energy' => 'point', 'walls' => 'point', 'cores' => 'core'] as $key => $shape) {
        if (!is_array($state->$key)) {
            return false;
        }
        foreach ($state->$key as $element) {
            if (!acb_valid_element($element, $shape)) {
                return false;
            }
        }
    }

    if (property_exists($state, 'zone') && $state->zone !== null) {
        $zone = $state->zone;
        if (!acb_is_json_object($zone) || !acb_has_only_fields($zone, ['center', 'radius', 'active'])
            || !property_exists($zone, 'center') || !acb_valid_position($zone->center)
            || !property_exists($zone, 'radius') || !is_int($zone->radius)
            || !property_exists($zone, 'active') || !is_bool($zone->active)) {
            return false;
        }
    }
    return true;
}

function acb_json_value_to_array($value) {
    if ($value instanceof stdClass) {
        $result = [];
        foreach ($value as $key => $child) {
            $result[$key] = acb_json_value_to_array($child);
        }
        return $result;
    }
    if (is_array($value)) {
        return array_map('acb_json_value_to_array', $value);
    }
    return $value;
}

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
