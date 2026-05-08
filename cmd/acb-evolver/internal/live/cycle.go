// Package live provides real-time cycle state tracking for the evolution observatory.
package live

import (
	"fmt"
	"sync"
	"time"
)

// CycleState tracks the current evolution cycle status in real-time.
// This is updated throughout the cycle and exported to live.json.
type CycleState struct {
	mu               sync.RWMutex
	Generation       int
	StartedAt        time.Time
	Phase            string // generating, validating, evaluating, promoting, idle
	CandidateID      string
	CandidateIsland  string
	CandidateLang    string
	ParentIDs        []string
	Validation       *CycleValidation
	Evaluation       *CycleEvaluation
	PromotionReason  string // Set when promoted/rejected
	CommunityHint    string // Community hint that influenced this candidate
}

// CycleValidation tracks validation stage progress.
type CycleValidation struct {
	SyntaxPassed   bool
	SyntaxTimeMs   int
	SchemaPassed   bool
	SchemaTimeMs   int
	SmokePassed    bool
	SmokeTimeMs    int
	LastErrorStage string
	LastError      string
}

// CycleEvaluation tracks arena evaluation progress.
type CycleEvaluation struct {
	MatchesTotal  int
	MatchesPlayed int
	Results       []CycleMatchResult
}

// CycleMatchResult is a single evaluation match result.
type CycleMatchResult struct {
	Opponent string
	Won      bool
	Score    string // e.g. "5-1"
}

// NewCycleState creates a new cycle state tracker.
func NewCycleState() *CycleState {
	return &CycleState{
		Phase: "idle",
	}
}

// SetPhase updates the current phase and timestamp.
func (c *CycleState) SetPhase(phase string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Phase = phase
	if phase == "generating" && c.StartedAt.IsZero() {
		c.StartedAt = time.Now().UTC()
	}
}

// SetGeneration sets the current generation number.
func (c *CycleState) SetGeneration(gen int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Generation = gen
}

// SetCandidate sets the current candidate being evaluated.
func (c *CycleState) SetCandidate(id, island, lang string, parents []string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.CandidateID = id
	c.CandidateIsland = island
	c.CandidateLang = lang
	c.ParentIDs = parents
}

// SetValidationSyntax records the result of syntax validation.
func (c *CycleState) SetValidationSyntax(passed bool, timeMs int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.Validation == nil {
		c.Validation = &CycleValidation{}
	}
	c.Validation.SyntaxPassed = passed
	c.Validation.SyntaxTimeMs = timeMs
	if !passed {
		c.Validation.LastErrorStage = "syntax"
	}
}

// SetValidationSchema records the result of schema validation.
func (c *CycleState) SetValidationSchema(passed bool, timeMs int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.Validation == nil {
		c.Validation = &CycleValidation{}
	}
	c.Validation.SchemaPassed = passed
	c.Validation.SchemaTimeMs = timeMs
	if !passed {
		c.Validation.LastErrorStage = "schema"
	}
}

// SetValidationSmoke records the result of smoke test validation.
func (c *CycleState) SetValidationSmoke(passed bool, timeMs int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.Validation == nil {
		c.Validation = &CycleValidation{}
	}
	c.Validation.SmokePassed = passed
	c.Validation.SmokeTimeMs = timeMs
	if !passed {
		c.Validation.LastErrorStage = "smoke"
	}
}

// SetValidationError records a validation error.
func (c *CycleState) SetValidationError(stage, errMsg string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.Validation == nil {
		c.Validation = &CycleValidation{}
	}
	c.Validation.LastErrorStage = stage
	c.Validation.LastError = errMsg
}

// StartEvaluation initializes the evaluation phase.
func (c *CycleState) StartEvaluation(totalMatches int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Evaluation = &CycleEvaluation{
		MatchesTotal:  totalMatches,
		MatchesPlayed: 0,
		Results:       make([]CycleMatchResult, 0, totalMatches),
	}
}

// AddEvaluationResult adds a match result to the evaluation.
func (c *CycleState) AddEvaluationResult(opponent string, won bool, score string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.Evaluation == nil {
		return
	}
	c.Evaluation.Results = append(c.Evaluation.Results, CycleMatchResult{
		Opponent: opponent,
		Won:      won,
		Score:    score,
	})
	c.Evaluation.MatchesPlayed = len(c.Evaluation.Results)
}

// SetPromotionResult sets the final promotion decision.
func (c *CycleState) SetPromotionResult(reason string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.PromotionReason = reason
}

// SetCommunityHint sets the community hint that influenced this candidate.
func (c *CycleState) SetCommunityHint(hint string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.CommunityHint = hint
}

// SetPhaseInfo sets all candidate info at once (simplified interface).
func (c *CycleState) SetPhaseInfo(phase, candidateID, island, lang string, parents []string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Phase = phase
	c.CandidateID = candidateID
	c.CandidateIsland = island
	c.CandidateLang = lang
	c.ParentIDs = parents
	if phase == "generating" && c.StartedAt.IsZero() {
		c.StartedAt = time.Now().UTC()
	}
}

// SetArenaResult records the final arena result.
func (c *CycleState) SetArenaResult(wins, losses, draws, errors int, winRate float64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.Evaluation == nil {
		c.Evaluation = &CycleEvaluation{
			MatchesTotal:  wins + losses + draws + errors,
			Results:      make([]CycleMatchResult, 0),
		}
	}
	c.Evaluation.MatchesPlayed = c.Evaluation.MatchesTotal
	// Add a summary result
	c.Evaluation.Results = append(c.Evaluation.Results, CycleMatchResult{
		Opponent: "arena",
		Won:      wins > losses,
		Score:    fmt.Sprintf("%d-%d-%d", wins, losses, draws),
	})
}

// Reset clears the cycle state (use between cycles).
func (c *CycleState) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Generation = 0
	c.StartedAt = time.Time{}
	c.Phase = "idle"
	c.CandidateID = ""
	c.CandidateIsland = ""
	c.CandidateLang = ""
	c.ParentIDs = nil
	c.Validation = nil
	c.Evaluation = nil
	c.PromotionReason = ""
	c.CommunityHint = ""
}

// ToCycleInfo converts the cycle state to an exportable CycleInfo.
func (c *CycleState) ToCycleInfo() *CycleInfo {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.Phase == "idle" || c.Generation == 0 {
		return nil
	}

	info := &CycleInfo{
		Generation: c.Generation,
		StartedAt:  c.StartedAt.UTC().Format(time.RFC3339),
		Phase:      c.Phase,
	}

	if c.CandidateID != "" {
		info.Candidate = &Candidate{
			ID:       c.CandidateID,
			Island:   c.CandidateIsland,
			Language: c.CandidateLang,
		}

		// Add parents
		if len(c.ParentIDs) > 0 {
			info.Candidate.Parents = make([]ParentInfo, len(c.ParentIDs))
			for i, pid := range c.ParentIDs {
				info.Candidate.Parents[i] = ParentInfo{ID: pid}
			}
		}

		// Add validation status
		if c.Validation != nil {
			info.Candidate.Validation = &ValidationStatus{
				Syntax: &StageResult{
					Passed:  c.Validation.SyntaxPassed,
					TimeMs:  c.Validation.SyntaxTimeMs,
					Error:   c.Validation.LastError,
				},
				Schema: &StageResult{
					Passed:  c.Validation.SchemaPassed,
					TimeMs:  c.Validation.SchemaTimeMs,
				},
				Smoke: &StageResult{
					Passed:  c.Validation.SmokePassed,
					TimeMs:  c.Validation.SmokeTimeMs,
				},
			}
		}

		// Add evaluation status
		if c.Evaluation != nil {
			info.Candidate.Evaluation = &EvaluationStatus{
				MatchesTotal:  c.Evaluation.MatchesTotal,
				MatchesPlayed: c.Evaluation.MatchesPlayed,
			}
			if len(c.Evaluation.Results) > 0 {
				info.Candidate.Evaluation.Results = make([]MatchResult, len(c.Evaluation.Results))
				for i, r := range c.Evaluation.Results {
					info.Candidate.Evaluation.Results[i] = MatchResult{
						Opponent: r.Opponent,
						Won:      r.Won,
						Score:    r.Score,
					}
				}
			}
		}
	}

	return info
}
