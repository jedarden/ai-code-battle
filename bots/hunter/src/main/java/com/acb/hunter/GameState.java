package com.acb.hunter;

import com.fasterxml.jackson.annotation.JsonProperty;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.DeserializationFeature;

import java.util.List;
import java.util.Collections;

/**
 * Game state types for AI Code Battle protocol.
 */
public class GameState {
    private static final ObjectMapper MAPPER = new ObjectMapper()
            .configure(DeserializationFeature.FAIL_ON_UNKNOWN_PROPERTIES, false);

    @JsonProperty("match_id")
    private String matchId;

    private int turn;
    private GameConfig config;
    private PlayerInfo you;

    private List<VisibleBot> bots = Collections.emptyList();
    private List<Position> energy = Collections.emptyList();
    private List<VisibleCore> cores = Collections.emptyList();
    private List<Position> walls = Collections.emptyList();
    private List<VisibleBot> dead = Collections.emptyList();

    // Getters
    public String getMatchId() { return matchId; }
    public int getTurn() { return turn; }
    public GameConfig getConfig() { return config; }
    public PlayerInfo getYou() { return you; }
    public List<VisibleBot> getBots() { return bots; }
    public List<Position> getEnergy() { return energy; }
    public List<VisibleCore> getCores() { return cores; }
    public List<Position> getWalls() { return walls; }
    public List<VisibleBot> getDead() { return dead; }

    public static GameState fromJson(String json) throws Exception {
        return MAPPER.readValue(json, GameState.class);
    }
}

class GameConfig {
    private int rows;
    private int cols;

    @JsonProperty("max_turns")
    private int maxTurns;

    @JsonProperty("vision_radius2")
    private int visionRadius2;

    @JsonProperty("attack_radius2")
    private int attackRadius2;

    @JsonProperty("spawn_cost")
    private int spawnCost;

    @JsonProperty("energy_interval")
    private int energyInterval;

    // Getters
    public int getRows() { return rows; }
    public int getCols() { return cols; }
    public int getMaxTurns() { return maxTurns; }
    public int getVisionRadius2() { return visionRadius2; }
    public int getAttackRadius2() { return attackRadius2; }
    public int getSpawnCost() { return spawnCost; }
    public int getEnergyInterval() { return energyInterval; }
}

class PlayerInfo {
    private int id;
    private int energy;
    private int score;

    // Getters
    public int getId() { return id; }
    public int getEnergy() { return energy; }
    public int getScore() { return score; }
}

class Position {
    private int row;
    private int col;

    // Default constructor for Jackson
    public Position() {}

    public Position(int row, int col) {
        this.row = row;
        this.col = col;
    }

    public int getRow() { return row; }
    public int getCol() { return col; }

    /**
     * Move in a direction with toroidal wrapping
     */
    public Position moveToward(Direction dir, int rows, int cols) {
        return switch (dir) {
            case N -> new Position((row - 1 + rows) % rows, col);
            case E -> new Position(row, (col + 1) % cols);
            case S -> new Position((row + 1) % rows, col);
            case W -> new Position(row, (col - 1 + cols) % cols);
        };
    }

    /**
     * Calculate squared distance with toroidal wrapping
     */
    public int distance2(Position other, int rows, int cols) {
        int dr = Math.abs(row - other.row);
        int dc = Math.abs(col - other.col);
        dr = Math.min(dr, rows - dr);
        dc = Math.min(dc, cols - dc);
        return dr * dr + dc * dc;
    }

    /**
     * Manhattan distance with toroidal wrapping
     */
    public int manhattanDistance(Position other, int rows, int cols) {
        int dr = Math.abs(row - other.row);
        int dc = Math.abs(col - other.col);
        dr = Math.min(dr, rows - dr);
        dc = Math.min(dc, cols - dc);
        return dr + dc;
    }

    public String key() {
        return row + "," + col;
    }

    @Override
    public boolean equals(Object o) {
        if (this == o) return true;
        if (o == null || getClass() != o.getClass()) return false;
        Position position = (Position) o;
        return row == position.row && col == position.col;
    }

    @Override
    public int hashCode() {
        return 31 * row + col;
    }
}

class VisibleBot {
    private Position position;
    private int owner;

    public Position getPosition() { return position; }
    public int getOwner() { return owner; }
}

class VisibleCore {
    private Position position;
    private int owner;
    private boolean active;

    public Position getPosition() { return position; }
    public int getOwner() { return owner; }
    public boolean isActive() { return active; }
}

enum Direction {
    N, E, S, W;

    public static Direction[] all() {
        return values();
    }
}

class Move {
    private final Position position;
    private final Direction direction;

    public Move(Position position, Direction direction) {
        this.position = position;
        this.direction = direction;
    }

    public Position getPosition() { return position; }
    public Direction getDirection() { return direction; }
}

class MoveResponse {
    private static final ObjectMapper MAPPER = new ObjectMapper();

    private final List<Move> moves;

    public MoveResponse(List<Move> moves) {
        this.moves = moves;
    }

    public List<Move> getMoves() { return moves; }

    public static String toJson(List<Move> moves) {
        try {
            var response = new MoveResponse(moves);
            return MAPPER.writeValueAsString(response);
        } catch (Exception e) {
            throw new RuntimeException("Failed to serialize moves", e);
        }
    }
}
