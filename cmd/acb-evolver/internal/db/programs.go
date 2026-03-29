package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/lib/pq"
)

// Island names for the 4 independent populations.
const (
	IslandAlpha = "alpha" // core-rushing strategies
	IslandBeta  = "beta"  // energy-focused strategies
	IslandGamma = "gamma" // defensive strategies
	IslandDelta = "delta" // mixed / experimental
)

// AllIslands is the ordered list of the 4 island names.
var AllIslands = []string{IslandAlpha, IslandBeta, IslandGamma, IslandDelta}

// Program represents an evolved strategy program stored in the database.
// BehaviorVector is a 2-element slice: [aggression, economy], each in [0, 1].
type Program struct {
	ID             int64
	Code           string
	Language       string
	Island         string
	Generation     int
	ParentIDs      []int64
	BehaviorVector []float64
	Fitness        float64
	Promoted       bool
	CreatedAt      time.Time
}

// Store provides CRUD operations for programs.
type Store struct {
	db *sql.DB
}

// NewStore creates a Store backed by the given database connection.
func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// Create inserts a new program and returns its assigned ID.
func (s *Store) Create(ctx context.Context, p *Program) (int64, error) {
	parentJSON, err := json.Marshal(p.ParentIDs)
	if err != nil {
		return 0, fmt.Errorf("marshal parent_ids: %w", err)
	}

	var id int64
	err = s.db.QueryRowContext(ctx, `
		INSERT INTO programs (code, language, island, generation, parent_ids, behavior_vector, fitness, promoted)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id`,
		p.Code,
		p.Language,
		p.Island,
		p.Generation,
		string(parentJSON),
		pq.Array(p.BehaviorVector),
		p.Fitness,
		p.Promoted,
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("insert program: %w", err)
	}
	return id, nil
}

// Get retrieves a program by ID. Returns (nil, nil) if not found.
func (s *Store) Get(ctx context.Context, id int64) (*Program, error) {
	p := &Program{}
	var parentJSON string
	err := s.db.QueryRowContext(ctx, `
		SELECT id, code, language, island, generation, parent_ids,
		       behavior_vector, fitness, promoted, created_at
		FROM programs WHERE id = $1`, id).Scan(
		&p.ID, &p.Code, &p.Language, &p.Island, &p.Generation,
		&parentJSON, pq.Array(&p.BehaviorVector), &p.Fitness, &p.Promoted, &p.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get program %d: %w", id, err)
	}
	if err := json.Unmarshal([]byte(parentJSON), &p.ParentIDs); err != nil {
		return nil, fmt.Errorf("unmarshal parent_ids: %w", err)
	}
	return p, nil
}

// ListByIsland returns all programs on the given island ordered by fitness desc.
func (s *Store) ListByIsland(ctx context.Context, island string) ([]*Program, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, code, language, island, generation, parent_ids,
		       behavior_vector, fitness, promoted, created_at
		FROM programs WHERE island = $1
		ORDER BY fitness DESC`, island)
	if err != nil {
		return nil, fmt.Errorf("list programs on %s: %w", island, err)
	}
	defer rows.Close()

	var programs []*Program
	for rows.Next() {
		p := &Program{}
		var parentJSON string
		if err := rows.Scan(
			&p.ID, &p.Code, &p.Language, &p.Island, &p.Generation,
			&parentJSON, pq.Array(&p.BehaviorVector), &p.Fitness, &p.Promoted, &p.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan program: %w", err)
		}
		if err := json.Unmarshal([]byte(parentJSON), &p.ParentIDs); err != nil {
			return nil, fmt.Errorf("unmarshal parent_ids: %w", err)
		}
		programs = append(programs, p)
	}
	return programs, rows.Err()
}

// UpdateFitness updates the fitness score and behavior vector of a program.
func (s *Store) UpdateFitness(ctx context.Context, id int64, fitness float64, behaviorVec []float64) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE programs SET fitness = $1, behavior_vector = $2 WHERE id = $3`,
		fitness, pq.Array(behaviorVec), id,
	)
	if err != nil {
		return fmt.Errorf("update fitness for program %d: %w", id, err)
	}
	return nil
}

// SetPromoted marks a program as promoted to the live bot fleet.
func (s *Store) SetPromoted(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `UPDATE programs SET promoted = TRUE WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("set promoted for program %d: %w", id, err)
	}
	return nil
}

// CountByIsland returns the number of programs on each island.
func (s *Store) CountByIsland(ctx context.Context) (map[string]int, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT island, COUNT(*) FROM programs GROUP BY island`)
	if err != nil {
		return nil, fmt.Errorf("count by island: %w", err)
	}
	defer rows.Close()

	counts := make(map[string]int)
	for rows.Next() {
		var island string
		var count int
		if err := rows.Scan(&island, &count); err != nil {
			return nil, fmt.Errorf("scan island count: %w", err)
		}
		counts[island] = count
	}
	return counts, rows.Err()
}

// TotalCount returns the total number of programs across all islands.
func (s *Store) TotalCount(ctx context.Context) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM programs`).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("total count: %w", err)
	}
	return n, nil
}

// SetBotID records the deployed bot_id for a promoted program.
func (s *Store) SetBotID(ctx context.Context, id int64, botID string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE programs SET bot_id = $1 WHERE id = $2`, botID, id)
	if err != nil {
		return fmt.Errorf("set bot_id for program %d: %w", id, err)
	}
	return nil
}

// PromotedProgram holds a promoted program linked to its live bot.
type PromotedProgram struct {
	ProgramID      int64
	BotID          string
	BotName        string // K8s/API name, e.g. "acb-evo-42"
	BotSecret      string // plaintext secret stored for retirement operations
	Island         string
	BehaviorVector []float64
	Fitness        float64
}

// ListPromoted returns all programs that have been promoted (bot_id is set).
func (s *Store) ListPromoted(ctx context.Context) ([]*PromotedProgram, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, bot_id, COALESCE(bot_name, ''), COALESCE(bot_secret, ''),
		       island, behavior_vector, fitness
		FROM programs
		WHERE promoted = TRUE AND bot_id IS NOT NULL
		ORDER BY fitness DESC`)
	if err != nil {
		return nil, fmt.Errorf("list promoted programs: %w", err)
	}
	defer rows.Close()

	var out []*PromotedProgram
	for rows.Next() {
		p := &PromotedProgram{}
		if err := rows.Scan(&p.ProgramID, &p.BotID, &p.BotName, &p.BotSecret,
			&p.Island, pq.Array(&p.BehaviorVector), &p.Fitness); err != nil {
			return nil, fmt.Errorf("scan promoted program: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// SetBotNameAndSecret records the K8s bot name and plaintext shared secret for
// a promoted program.  These are stored so the retirement path can locate and
// clean up the bot without requiring an extra API call.
func (s *Store) SetBotNameAndSecret(ctx context.Context, id int64, botName, botSecret string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE programs SET bot_name = $1, bot_secret = $2 WHERE id = $3`,
		botName, botSecret, id)
	if err != nil {
		return fmt.Errorf("set bot name/secret for program %d: %w", id, err)
	}
	return nil
}

// PromotedCount returns the number of currently promoted (deployed) programs.
func (s *Store) PromotedCount(ctx context.Context) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM programs WHERE promoted = TRUE AND bot_id IS NOT NULL`).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("promoted count: %w", err)
	}
	return n, nil
}

// UnsetPromoted clears the promoted flag and bot_id for a retired program.
func (s *Store) UnsetPromoted(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE programs SET promoted = FALSE, bot_id = NULL WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("unset promoted for program %d: %w", id, err)
	}
	return nil
}

// GetByBotID returns the program associated with a deployed bot ID, or nil.
func (s *Store) GetByBotID(ctx context.Context, botID string) (*Program, error) {
	p := &Program{}
	var parentJSON string
	err := s.db.QueryRowContext(ctx, `
		SELECT id, code, language, island, generation, parent_ids,
		       behavior_vector, fitness, promoted, created_at
		FROM programs WHERE bot_id = $1`, botID).Scan(
		&p.ID, &p.Code, &p.Language, &p.Island, &p.Generation,
		&parentJSON, pq.Array(&p.BehaviorVector), &p.Fitness, &p.Promoted, &p.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get program by bot_id %s: %w", botID, err)
	}
	if err := json.Unmarshal([]byte(parentJSON), &p.ParentIDs); err != nil {
		return nil, fmt.Errorf("unmarshal parent_ids: %w", err)
	}
	return p, nil
}

// Delete removes a program by ID. Returns error if deletion fails.
func (s *Store) Delete(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM programs WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete program %d: %w", id, err)
	}
	return nil
}

// List returns all programs ordered by creation date descending.
func (s *Store) List(ctx context.Context, limit, offset int) ([]*Program, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, code, language, island, generation, parent_ids,
		       behavior_vector, fitness, promoted, created_at
		FROM programs
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list programs: %w", err)
	}
	defer rows.Close()

	var programs []*Program
	for rows.Next() {
		p := &Program{}
		var parentJSON string
		if err := rows.Scan(
			&p.ID, &p.Code, &p.Language, &p.Island, &p.Generation,
			&parentJSON, pq.Array(&p.BehaviorVector), &p.Fitness, &p.Promoted, &p.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan program: %w", err)
		}
		if err := json.Unmarshal([]byte(parentJSON), &p.ParentIDs); err != nil {
			return nil, fmt.Errorf("unmarshal parent_ids: %w", err)
		}
		programs = append(programs, p)
	}
	return programs, rows.Err()
}

// ListTopByIsland returns the top N programs on the given island by fitness.
func (s *Store) ListTopByIsland(ctx context.Context, island string, limit int) ([]*Program, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, code, language, island, generation, parent_ids,
		       behavior_vector, fitness, promoted, created_at
		FROM programs WHERE island = $1
		ORDER BY fitness DESC
		LIMIT $2`, island, limit)
	if err != nil {
		return nil, fmt.Errorf("list top programs on %s: %w", island, err)
	}
	defer rows.Close()

	var programs []*Program
	for rows.Next() {
		p := &Program{}
		var parentJSON string
		if err := rows.Scan(
			&p.ID, &p.Code, &p.Language, &p.Island, &p.Generation,
			&parentJSON, pq.Array(&p.BehaviorVector), &p.Fitness, &p.Promoted, &p.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan program: %w", err)
		}
		if err := json.Unmarshal([]byte(parentJSON), &p.ParentIDs); err != nil {
			return nil, fmt.Errorf("unmarshal parent_ids: %w", err)
		}
		programs = append(programs, p)
	}
	return programs, rows.Err()
}

// GetLineage returns all ancestor program IDs for a given program by
// traversing the parent_ids chain recursively.
func (s *Store) GetLineage(ctx context.Context, id int64) ([]int64, error) {
	visited := make(map[int64]bool)
	var lineage []int64

	var traverse func(programID int64) error
	traverse = func(programID int64) error {
		if visited[programID] {
			return nil
		}
		visited[programID] = true

		p, err := s.Get(ctx, programID)
		if err != nil {
			return err
		}
		if p == nil {
			return nil
		}

		for _, parentID := range p.ParentIDs {
			if err := traverse(parentID); err != nil {
				return err
			}
		}
		lineage = append(lineage, programID)
		return nil
	}

	if err := traverse(id); err != nil {
		return nil, fmt.Errorf("get lineage for %d: %w", id, err)
	}
	return lineage, nil
}
