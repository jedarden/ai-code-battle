// Package live generates the evolution dashboard live.json snapshot.
package live

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
	"time"
)

// IslandStat holds per-island population statistics (dashboard format).
type IslandStat struct {
	Population  int    `json:"population"`
	BestRating  int    `json:"best_rating"`
	BestBot     string `json:"best_bot"`
	LanguageDiv string `json:"language_div,omitempty"` // dominant language
}

// IslandStatFull holds per-island population statistics (full detail).
type IslandStatFull struct {
	Count         int     `json:"count"`
	BestFitness   float64 `json:"best_fitness"`
	AvgFitness    float64 `json:"avg_fitness"`
	Diversity     float64 `json:"diversity"` // language diversity [0,1]
	PromotedCount int     `json:"promoted_count"`
}

// GenerationEntry is one row in the generation log (island × generation bucket).
type GenerationEntry struct {
	Generation  int     `json:"generation"`
	Island      string  `json:"island"`
	EvaluatedAt string  `json:"evaluated_at"`
	Count       int     `json:"count"`
	Promoted    int     `json:"promoted"`
	BestFitness float64 `json:"best_fitness"`
	AvgFitness  float64 `json:"avg_fitness"`
}

// LineageNode is a single program in the lineage tree.
type LineageNode struct {
	ID         int64   `json:"id"`
	ParentIDs  []int64 `json:"parent_ids"`
	Generation int     `json:"generation"`
	Island     string  `json:"island"`
	Fitness    float64 `json:"fitness"`
	Promoted   bool    `json:"promoted"`
	Language   string  `json:"language"`
	CreatedAt  string  `json:"created_at"`
}

// MetaSnapshot is the island population state at a single generation.
type MetaSnapshot struct {
	Generation        int                `json:"generation"`
	IslandCounts      map[string]int     `json:"island_counts"`
	IslandBestFitness map[string]float64 `json:"island_best_fitness"`
}

// CycleInfo represents the current evolution cycle status.
type CycleInfo struct {
	Generation int        `json:"generation"`
	StartedAt  string     `json:"started_at"`
	Phase      string     `json:"phase"` // generating, validating, evaluating, promoting, idle
	Candidate  *Candidate `json:"candidate,omitempty"`
}

// Candidate represents the current candidate being evaluated.
type Candidate struct {
	ID         string              `json:"id"`         // e.g., "go-847-3"
	Island     string              `json:"island"`
	Language   string              `json:"language"`
	Parents    []ParentInfo        `json:"parents"`
	Validation *ValidationStatus   `json:"validation,omitempty"`
	Evaluation *EvaluationStatus   `json:"evaluation,omitempty"`
}

// ParentInfo holds parent bot information.
type ParentInfo struct {
	ID     string `json:"id"`     // e.g., "go-831-1"
	Rating int    `json:"rating"`
}

// ValidationStatus holds validation stage results.
type ValidationStatus struct {
	Syntax *StageResult `json:"syntax,omitempty"`
	Schema *StageResult `json:"schema,omitempty"`
	Smoke  *StageResult `json:"smoke,omitempty"`
}

// StageResult holds result for a single validation stage.
type StageResult struct {
	Passed bool   `json:"passed"`
	TimeMs int    `json:"time_ms"`
	Error  string `json:"error,omitempty"`
}

// EvaluationStatus holds arena evaluation results.
type EvaluationStatus struct {
	MatchesTotal int           `json:"matches_total"`
	MatchesPlayed int          `json:"matches_played"`
	Results      []MatchResult `json:"results"`
}

// MatchResult is a single evaluation match result.
type MatchResult struct {
	Opponent string `json:"opponent"` // opponent bot name
	Won      bool   `json:"won"`
	Score    string `json:"score"`    // e.g., "5-1"
}

// ActivityEntry is a single event in the recent activity feed.
type ActivityEntry struct {
	Time      string `json:"time"`
	Generation int   `json:"generation"`
	Candidate string `json:"candidate"`
	Island    string `json:"island"`
	Result    string `json:"result"` // promoted, rejected
	Reason    string `json:"reason"`
	Stage     string `json:"stage"`  // validation, promotion, deployment
	BotID     string `json:"bot_id,omitempty"`
	InitialRating int `json:"initial_rating,omitempty"`
}

// Totals holds overall evolution statistics.
type Totals struct {
	GenerationsTotal      int     `json:"generations_total"`
	CandidatesToday       int     `json:"candidates_today"`
	PromotedToday         int     `json:"promoted_today"`
	PromotionRate7d       float64 `json:"promotion_rate_7d"`
	HighestEvolvedRating  int     `json:"highest_evolved_rating"`
	EvolvedInTop10        int     `json:"evolved_in_top_10"`
	MutationsPerHour      float64 `json:"mutations_per_hour"`
}

// LiveData is the full evolution dashboard payload written to live.json (plan §14 format).
type LiveData struct {
	UpdatedAt      string                 `json:"updated_at"`
	Cycle          *CycleInfo             `json:"cycle,omitempty"`
	RecentActivity []ActivityEntry        `json:"recent_activity,omitempty"`
	Islands        map[string]IslandStat  `json:"islands"`
	Totals         Totals                 `json:"totals"`
	// Legacy fields for backward compatibility
	TotalPrograms int                    `json:"total_programs,omitempty"`
	PromotedCount int                    `json:"promoted_count,omitempty"`
	GenerationLog []GenerationEntry      `json:"generation_log,omitempty"`
	Lineage       []LineageNode          `json:"lineage,omitempty"`
	MetaSnapshots []MetaSnapshot         `json:"meta_snapshots,omitempty"`
}

// Export queries the programs database and builds the current evolution state.
func Export(ctx context.Context, db *sql.DB) (*LiveData, error) {
	data := &LiveData{
		UpdatedAt: time.Now().UTC().Format(time.RFC3339),
		Islands:   make(map[string]IslandStat),
		Totals:    Totals{},
	}

	if err := fillIslandStats(ctx, db, data); err != nil {
		return nil, err
	}
	if err := fillTotals(ctx, db, data); err != nil {
		return nil, err
	}
	if err := fillRecentActivity(ctx, db, data); err != nil {
		return nil, err
	}
	// Legacy fields for backward compatibility
	if err := fillGenerationLog(ctx, db, data); err != nil {
		return nil, err
	}
	if err := fillLineage(ctx, db, data); err != nil {
		return nil, err
	}
	if err := fillMetaSnapshots(ctx, db, data); err != nil {
		return nil, err
	}

	return data, nil
}

func fillIslandStats(ctx context.Context, db *sql.DB, data *LiveData) error {
	// Query island stats with bot ratings
	rows, err := db.QueryContext(ctx, `
		SELECT p.island,
		       COUNT(*) AS population,
		       COALESCE(MAX(b.rating_mu - 2*b.rating_phi), 0) AS best_rating,
		       COALESCE(b.bot_id, '') AS best_bot_id
		FROM programs p
		LEFT JOIN bots b ON p.bot_id = b.bot_id
		GROUP BY p.island`)
	if err != nil {
		return fmt.Errorf("island stats: %w", err)
	}
	defer rows.Close()

	total := 0
	for rows.Next() {
		var island string
		var population, bestRating int
		var bestBotID string
		if err := rows.Scan(&island, &population, &bestRating, &bestBotID); err != nil {
			return fmt.Errorf("scan island stats: %w", err)
		}

		data.Islands[island] = IslandStat{
			Population: population,
			BestRating: bestRating,
			BestBot:    bestBotID,
		}
		total += population
	}
	if err := rows.Err(); err != nil {
		return err
	}
	data.TotalPrograms = total
	return nil
}

func fillTotals(ctx context.Context, db *sql.DB, data *LiveData) error {
	// Get max generation
	var maxGen int
	err := db.QueryRowContext(ctx, `SELECT COALESCE(MAX(generation), 0) FROM programs`).Scan(&maxGen)
	if err != nil {
		return fmt.Errorf("max generation: %w", err)
	}

	// Count candidates created today
	var candidatesToday int
	today := time.Now().UTC().Format("2006-01-02")
	err = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM programs WHERE created_at >= $1::date`, today).Scan(&candidatesToday)
	if err != nil {
		return fmt.Errorf("candidates today: %w", err)
	}

	// Count promoted today
	var promotedToday int
	err = db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM programs p
		JOIN bots b ON p.bot_id = b.bot_id
		WHERE p.promoted = TRUE AND b.created_at >= $1::date`, today).Scan(&promotedToday)
	if err != nil {
		return fmt.Errorf("promoted today: %w", err)
	}

	// Calculate 7-day promotion rate
	var promoted7d, total7d int
	err = db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM programs p
		JOIN bots b ON p.bot_id = b.bot_id
		WHERE b.created_at >= NOW() - INTERVAL '7 days'`).Scan(&promoted7d)
	if err != nil {
		promoted7d = 0
	}
	err = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM programs WHERE created_at >= NOW() - INTERVAL '7 days'`).Scan(&total7d)
	if err != nil {
		total7d = 0
	}
	var rate7d float64
	if total7d > 0 {
		rate7d = round3(float64(promoted7d) / float64(total7d))
	}

	// Highest evolved rating
	var highestRating int
	err = db.QueryRowContext(ctx, `
		SELECT COALESCE(MAX(b.rating_mu - 2*b.rating_phi), 0)
		FROM bots b
		WHERE b.owner = 'acb-evolver'`).Scan(&highestRating)
	if err != nil {
		highestRating = 0
	}

	// Count evolved in top 10
	var top10Count int
	err = db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM (
			SELECT b.bot_id, b.rating_mu - 2*b.rating_phi AS display_rating
			FROM bots b
			WHERE b.status = 'active'
			ORDER BY display_rating DESC
			LIMIT 10
		) top10
		JOIN bots b ON top10.bot_id = b.bot_id
		WHERE b.owner = 'acb-evolver'`).Scan(&top10Count)
	if err != nil {
		top10Count = 0
	}

	// Mutations per hour (programs created in the last hour)
	var mutationsLastHour int
	err = db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM programs
		WHERE created_at >= NOW() - INTERVAL '1 hour'`).Scan(&mutationsLastHour)
	if err != nil {
		mutationsLastHour = 0
	}

	data.Totals = Totals{
		GenerationsTotal:     maxGen,
		CandidatesToday:      candidatesToday,
		PromotedToday:        promotedToday,
		PromotionRate7d:      rate7d,
		HighestEvolvedRating: highestRating,
		EvolvedInTop10:       top10Count,
		MutationsPerHour:     round3(float64(mutationsLastHour)),
	}

	return nil
}

func fillRecentActivity(ctx context.Context, db *sql.DB, data *LiveData) error {
	// Get recent promoted bots from bots table (with timestamps)
	rows, err := db.QueryContext(ctx, `
		SELECT
			p.bot_id,
			p.bot_name,
			p.island,
			p.generation,
			p.language,
			b.created_at
		FROM programs p
		JOIN bots b ON p.bot_id = b.bot_id
		WHERE p.promoted = TRUE AND b.owner = 'acb-evolver'
		ORDER BY b.created_at DESC
		LIMIT 10`)
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("recent activity: %w", err)
	}
	defer rows.Close()

	activities := []ActivityEntry{}
	for rows.Next() {
		var botID, botName, island, language string
		var generation int
		var createdAt time.Time
		if err := rows.Scan(&botID, &botName, &island, &generation, &language, &createdAt); err != nil {
			continue
		}
		activities = append(activities, ActivityEntry{
			Time:      createdAt.UTC().Format(time.RFC3339),
			Generation: generation,
			Candidate: botName,
			Island:    island,
			Result:    "promoted",
			Reason:    "Passed promotion gate",
			Stage:     "deployment",
			BotID:     botID,
		})
	}
	data.RecentActivity = activities
	data.PromotedCount = len(activities)

	return nil
}

func fillGenerationLog(ctx context.Context, db *sql.DB, data *LiveData) error {
	rows, err := db.QueryContext(ctx, `
		SELECT generation, island,
		       MAX(created_at) AS latest,
		       COUNT(*) AS cnt,
		       SUM(CASE WHEN promoted AND bot_id IS NOT NULL THEN 1 ELSE 0 END) AS promoted_cnt,
		       COALESCE(MAX(fitness), 0) AS max_fit,
		       COALESCE(AVG(fitness), 0) AS avg_fit
		FROM programs
		GROUP BY generation, island
		ORDER BY generation DESC, island
		LIMIT 100`)
	if err != nil {
		return fmt.Errorf("generation log: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var e GenerationEntry
		var latest time.Time
		if err := rows.Scan(&e.Generation, &e.Island, &latest, &e.Count, &e.Promoted, &e.BestFitness, &e.AvgFitness); err != nil {
			return fmt.Errorf("scan gen log: %w", err)
		}
		e.EvaluatedAt = latest.UTC().Format(time.RFC3339)
		e.BestFitness = round3(e.BestFitness)
		e.AvgFitness = round3(e.AvgFitness)
		data.GenerationLog = append(data.GenerationLog, e)
	}
	return rows.Err()
}

func fillLineage(ctx context.Context, db *sql.DB, data *LiveData) error {
	rows, err := db.QueryContext(ctx, `
		SELECT id, parent_ids, generation, island, fitness, promoted, language, created_at
		FROM programs
		ORDER BY created_at DESC
		LIMIT 200`)
	if err != nil {
		return fmt.Errorf("lineage: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var node LineageNode
		var parentJSON string
		var createdAt time.Time
		if err := rows.Scan(&node.ID, &parentJSON, &node.Generation, &node.Island,
			&node.Fitness, &node.Promoted, &node.Language, &createdAt); err != nil {
			return fmt.Errorf("scan lineage: %w", err)
		}
		if err := json.Unmarshal([]byte(parentJSON), &node.ParentIDs); err != nil {
			node.ParentIDs = []int64{}
		}
		node.Fitness = round3(node.Fitness)
		node.CreatedAt = createdAt.UTC().Format(time.RFC3339)
		data.Lineage = append(data.Lineage, node)
	}
	return rows.Err()
}

func fillMetaSnapshots(ctx context.Context, db *sql.DB, data *LiveData) error {
	rows, err := db.QueryContext(ctx, `
		SELECT generation, island, COUNT(*), COALESCE(MAX(fitness), 0)
		FROM programs
		GROUP BY generation, island
		ORDER BY generation ASC`)
	if err != nil {
		return fmt.Errorf("meta snapshots: %w", err)
	}
	defer rows.Close()

	snapMap := make(map[int]*MetaSnapshot)
	for rows.Next() {
		var gen, cnt int
		var island string
		var maxFit float64
		if err := rows.Scan(&gen, &island, &cnt, &maxFit); err != nil {
			return fmt.Errorf("scan meta snapshots: %w", err)
		}
		if snapMap[gen] == nil {
			snapMap[gen] = &MetaSnapshot{
				Generation:        gen,
				IslandCounts:      make(map[string]int),
				IslandBestFitness: make(map[string]float64),
			}
		}
		snapMap[gen].IslandCounts[island] = cnt
		snapMap[gen].IslandBestFitness[island] = round3(maxFit)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	gens := make([]int, 0, len(snapMap))
	for gen := range snapMap {
		gens = append(gens, gen)
	}
	sort.Ints(gens)
	for _, gen := range gens {
		data.MetaSnapshots = append(data.MetaSnapshots, *snapMap[gen])
	}
	return nil
}

// WriteFile marshals the live data to JSON and writes it to path, creating
// parent directories if needed.
func WriteFile(d *LiveData, path string) error {
	b, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	if err := os.MkdirAll(dirOf(path), 0755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}
	if err := os.WriteFile(path, b, 0644); err != nil {
		return fmt.Errorf("write: %w", err)
	}
	return nil
}

func dirOf(p string) string {
	for i := len(p) - 1; i >= 0; i-- {
		if p[i] == '/' || p[i] == '\\' {
			return p[:i]
		}
	}
	return "."
}

func round3(v float64) float64 {
	return math.Round(v*1000) / 1000
}

// marshal returns indented JSON for the live data.
func marshal(d *LiveData) ([]byte, error) {
	return json.MarshalIndent(d, "", "  ")
}
