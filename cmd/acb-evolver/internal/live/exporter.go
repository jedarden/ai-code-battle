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

// IslandStat holds per-island population statistics.
type IslandStat struct {
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

// LiveData is the full evolution dashboard payload written to live.json.
type LiveData struct {
	UpdatedAt     string                `json:"updated_at"`
	TotalPrograms int                   `json:"total_programs"`
	PromotedCount int                   `json:"promoted_count"`
	Islands       map[string]IslandStat `json:"islands"`
	GenerationLog []GenerationEntry     `json:"generation_log"`
	Lineage       []LineageNode         `json:"lineage"`
	MetaSnapshots []MetaSnapshot        `json:"meta_snapshots"`
}

// Export queries the programs database and builds the current evolution state.
func Export(ctx context.Context, db *sql.DB) (*LiveData, error) {
	data := &LiveData{
		UpdatedAt: time.Now().UTC().Format(time.RFC3339),
		Islands:   make(map[string]IslandStat),
	}

	if err := fillIslandStats(ctx, db, data); err != nil {
		return nil, err
	}
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
	rows, err := db.QueryContext(ctx, `
		SELECT island,
		       COUNT(*) AS cnt,
		       COALESCE(AVG(fitness), 0) AS avg_fit,
		       COALESCE(MAX(fitness), 0) AS max_fit,
		       COUNT(DISTINCT language) AS lang_diversity,
		       SUM(CASE WHEN promoted AND bot_id IS NOT NULL THEN 1 ELSE 0 END) AS promoted_cnt
		FROM programs
		GROUP BY island`)
	if err != nil {
		return fmt.Errorf("island stats: %w", err)
	}
	defer rows.Close()

	total := 0
	promoted := 0
	for rows.Next() {
		var island string
		var cnt, langDiv, promotedCnt int
		var avgFit, maxFit float64
		if err := rows.Scan(&island, &cnt, &avgFit, &maxFit, &langDiv, &promotedCnt); err != nil {
			return fmt.Errorf("scan island stats: %w", err)
		}
		// Diversity: language diversity normalized to [0,1], up to 6 languages
		const maxLangs = 6.0
		diversity := float64(langDiv) / maxLangs
		if diversity > 1.0 {
			diversity = 1.0
		}
		data.Islands[island] = IslandStat{
			Count:         cnt,
			BestFitness:   round3(maxFit),
			AvgFitness:    round3(avgFit),
			Diversity:     round3(diversity),
			PromotedCount: promotedCnt,
		}
		total += cnt
		promoted += promotedCnt
	}
	if err := rows.Err(); err != nil {
		return err
	}
	data.TotalPrograms = total
	data.PromotedCount = promoted
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
