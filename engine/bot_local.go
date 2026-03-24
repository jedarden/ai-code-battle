package engine

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"os"
)

// LocalBot is a bot that communicates via stdin/stdout (Phase 1).
// This is used for local development and testing.
type LocalBot struct {
	stdin  io.Reader
	stdout io.Writer
}

// NewLocalBot creates a new local bot using stdin/stdout.
func NewLocalBot() *LocalBot {
	return &LocalBot{
		stdin:  os.Stdin,
		stdout: os.Stdout,
	}
}

// NewLocalBotWithIO creates a local bot with custom IO (for testing).
func NewLocalBotWithIO(stdin io.Reader, stdout io.Writer) *LocalBot {
	return &LocalBot{
		stdin:  stdin,
		stdout: stdout,
	}
}

// GetMoves reads game state from stdin and writes moves to stdout.
func (b *LocalBot) GetMoves(state *VisibleState) ([]Move, error) {
	// Write state to stdout as JSON
	encoder := json.NewEncoder(b.stdout)
	if err := encoder.Encode(state); err != nil {
		return nil, fmt.Errorf("failed to encode state: %w", err)
	}

	// Read moves from stdin
	scanner := bufio.NewScanner(b.stdin)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return nil, fmt.Errorf("failed to read moves: %w", err)
		}
		return nil, fmt.Errorf("EOF reading moves")
	}

	var moves []Move
	if err := json.Unmarshal(scanner.Bytes(), &moves); err != nil {
		return nil, fmt.Errorf("failed to decode moves: %w", err)
	}

	return moves, nil
}

// RandomBot is a simple bot that makes random moves.
type RandomBot struct {
	rng *rand.Rand
}

// NewRandomBot creates a new random bot.
func NewRandomBot(seed int64) *RandomBot {
	return &RandomBot{
		rng: rand.New(rand.NewSource(seed)),
	}
}

// GetMoves returns random moves for all visible bots.
func (b *RandomBot) GetMoves(state *VisibleState) ([]Move, error) {
	moves := make([]Move, 0)
	directions := []Direction{DirN, DirE, DirS, DirW}

	for _, bot := range state.Bots {
		if bot.Owner == state.You.ID {
			moves = append(moves, Move{
				Position:  bot.Position,
				Direction: directions[b.rng.Intn(len(directions))],
			})
		}
	}

	return moves, nil
}

// IdleBot is a bot that never moves.
type IdleBot struct{}

// NewIdleBot creates a new idle bot.
func NewIdleBot() *IdleBot {
	return &IdleBot{}
}

// GetMoves returns no moves (bot stays in place).
func (b *IdleBot) GetMoves(state *VisibleState) ([]Move, error) {
	return []Move{}, nil
}
