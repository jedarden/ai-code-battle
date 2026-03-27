package db

import (
	"context"
	"fmt"
	"time"
)

// ValidationLog records the outcome of one validation pipeline run.
// It is written after every candidate evaluation so pass rates can be
// computed per island and per language.
type ValidationLog struct {
	ID        int64
	Island    string    // one of IslandAlpha … IslandDelta
	Language  string    // e.g. "go", "python"
	Stage     string    // last stage attempted: "syntax", "schema", or "sandbox"
	Passed    bool      // true when all stages up to (and including) Stage passed
	ErrorText string    // human-readable failure reason (empty on pass)
	LLMOutput string    // raw LLM response, for retry / learning
	CreatedAt time.Time
}

// RecordValidation inserts one ValidationLog row.
func (s *Store) RecordValidation(ctx context.Context, v *ValidationLog) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO validation_log (island, language, stage, passed, error_text, llm_output)
		VALUES ($1, $2, $3, $4, $5, $6)`,
		v.Island, v.Language, v.Stage, v.Passed, v.ErrorText, v.LLMOutput,
	)
	if err != nil {
		return fmt.Errorf("record validation: %w", err)
	}
	return nil
}

// IslandPassRates returns the validation pass rate (0.0–1.0) keyed by island
// name.  Islands with no records are omitted from the result.
func (s *Store) IslandPassRates(ctx context.Context) (map[string]float64, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT island,
		       SUM(CASE WHEN passed THEN 1 ELSE 0 END)::float
		           / NULLIF(COUNT(*), 0) AS pass_rate
		FROM validation_log
		GROUP BY island
	`)
	if err != nil {
		return nil, fmt.Errorf("island pass rates: %w", err)
	}
	defer rows.Close()

	result := make(map[string]float64)
	for rows.Next() {
		var island string
		var rate float64
		if err := rows.Scan(&island, &rate); err != nil {
			return nil, fmt.Errorf("scan pass rate: %w", err)
		}
		result[island] = rate
	}
	return result, rows.Err()
}

// ValidationStats holds aggregate metrics for one island.
type ValidationStats struct {
	Island     string
	Total      int
	Passed     int
	PassRate   float64
	ByStage    map[string]int // count of runs that FAILED at each stage
}

// IslandValidationStats returns per-island validation statistics including
// breakdown by failure stage.  Islands with no rows are not returned.
func (s *Store) IslandValidationStats(ctx context.Context) ([]ValidationStats, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT island,
		       COUNT(*) AS total,
		       SUM(CASE WHEN passed THEN 1 ELSE 0 END) AS passed_count,
		       SUM(CASE WHEN NOT passed AND stage = 'syntax'  THEN 1 ELSE 0 END) AS fail_syntax,
		       SUM(CASE WHEN NOT passed AND stage = 'schema'  THEN 1 ELSE 0 END) AS fail_schema,
		       SUM(CASE WHEN NOT passed AND stage = 'sandbox' THEN 1 ELSE 0 END) AS fail_sandbox
		FROM validation_log
		GROUP BY island
		ORDER BY island
	`)
	if err != nil {
		return nil, fmt.Errorf("validation stats: %w", err)
	}
	defer rows.Close()

	var out []ValidationStats
	for rows.Next() {
		var v ValidationStats
		var failSyntax, failSchema, failSandbox int
		if err := rows.Scan(
			&v.Island, &v.Total, &v.Passed,
			&failSyntax, &failSchema, &failSandbox,
		); err != nil {
			return nil, fmt.Errorf("scan validation stats: %w", err)
		}
		if v.Total > 0 {
			v.PassRate = float64(v.Passed) / float64(v.Total)
		}
		v.ByStage = map[string]int{
			"syntax":  failSyntax,
			"schema":  failSchema,
			"sandbox": failSandbox,
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
