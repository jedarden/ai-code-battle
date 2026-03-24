<?php
/**
 * Game state types for AI Code Battle protocol.
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

    /**
     * Move in a direction with toroidal wrapping
     */
    public function moveToward(string $dir, int $rows, int $cols): Position {
        switch ($dir) {
            case 'N':
                return new Position(($this->row - 1 + $rows) % $rows, $this->col);
            case 'E':
                return new Position($this->row, ($this->col + 1) % $cols);
            case 'S':
                return new Position(($this->row + 1) % $rows, $this->col);
            case 'W':
                return new Position($this->row, ($this->col - 1 + $cols) % $cols);
            default:
                return clone $this;
        }
    }

    /**
     * Calculate squared distance with toroidal wrapping
     */
    public function distance2(Position $other, int $rows, int $cols): int {
        $dr = abs($this->row - $other->row);
        $dc = abs($this->col - $other->col);
        $dr = min($dr, $rows - $dr);
        $dc = min($dc, $cols - $dc);
        return $dr * $dr + $dc * $dc;
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

    public static function fromArray(array $data): self {
        $config = new self();
        $config->rows = $data['rows'];
        $config->cols = $data['cols'];
        $config->maxTurns = $data['max_turns'];
        $config->visionRadius2 = $data['vision_radius2'];
        $config->attackRadius2 = $data['attack_radius2'];
        $config->spawnCost = $data['spawn_cost'];
        $config->energyInterval = $data['energy_interval'];
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
