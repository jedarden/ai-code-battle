// Package arena — promotion gate.
//
// The gate applies two independent criteria before promoting a candidate:
//
//  1. Nash value (PSRO) ≥ NashThreshold      — sufficient win rate
//  2. MAP-Elites niche fill or improvement   — behavioral novelty
//
// Both must be satisfied. The Wilson-score CI lower bound is an optional
// secondary guard on the overall win rate.
package arena

import (
	"fmt"
	"strings"

	"github.com/aicodebattle/acb/cmd/acb-evolver/internal/mapelites"
)

// GateConfig holds the promotion thresholds.
type GateConfig struct {
	// NashThreshold is the minimum Nash value (worst-case win rate across
	// opponents) required for promotion. Default: 0.50.
	NashThreshold float64

	// WinRateLowerBound is the minimum Wilson-score 95% CI lower bound for
	// the overall win rate. Set ≤ 0 to disable. Default: 0.40.
	WinRateLowerBound float64
}

// DefaultGateConfig returns production-ready promotion thresholds.
func DefaultGateConfig() GateConfig {
	return GateConfig{
		NashThreshold:     0.50,
		WinRateLowerBound: 0.40,
	}
}

// GateResult holds the full promotion decision with supporting evidence.
type GateResult struct {
	// Promoted is true when all criteria are met.
	Promoted bool

	// Nash is the PSRO result for the mini-tournament.
	Nash NashResult

	// WinRate is the overall win rate with 95% Wilson CI.
	WinRate WinRateResult

	// MapElitesPlaced is true when the candidate was written to the MAP-Elites
	// grid (filled an empty cell or outperformed the incumbent).
	MapElitesPlaced bool

	// MapElitesImproved is true when the candidate beat an existing champion
	// (as opposed to simply filling an empty niche).
	MapElitesImproved bool

	// Placement is the 4-D grid cell the candidate occupies.
	Placement mapelites.Placement

	// Reason is a human-readable explanation of the promotion decision.
	Reason string
}

// Gate applies the promotion criteria to mini-tournament results.
type Gate struct {
	cfg  GateConfig
	grid *mapelites.Grid
}

// NewGate creates a Gate backed by the provided MAP-Elites grid.
// The grid is shared across evaluations so niche occupancy persists across
// multiple Evaluate calls within one evolution run.
func NewGate(cfg GateConfig, grid *mapelites.Grid) *Gate {
	return &Gate{cfg: cfg, grid: grid}
}

// Evaluate applies the two-part promotion gate to the arena result.
//
// programID and fitness are the candidate's identifiers in the programs table.
// behaviorVec is [aggression, economy, exploration, formation] ∈ [0,1]⁴;
// defaults to [0.5, 0.5, 0.5, 0.5] when nil or short.
//
// Side effect: g.grid.TryPlace is called — the cell is updated when the
// candidate wins its behavioral niche.
func (g *Gate) Evaluate(result *Result, programID int64, fitness float64, behaviorVec []float64) *GateResult {
	wr := ComputeFromResult(result)
	nash := ComputeNash(result.WinRateVec)

	// Default behavior: all dimensions at 0.5 (center of grid)
	dims := [4]float64{0.5, 0.5, 0.5, 0.5}
	for i := 0; i < len(behaviorVec) && i < 4; i++ {
		dims[i] = behaviorVec[i]
	}
	agg, eco, expl, form := dims[0], dims[1], dims[2], dims[3]

	// Sample the cell state before TryPlace so we can distinguish
	// "fills empty niche" from "beats existing champion".
	cellX, cellY, cellZ, cellW := g.grid.BehaviorToCell(agg, eco, expl, form)
	priorCell := g.grid.Get(cellX, cellY, cellZ, cellW)

	placement, placed := g.grid.TryPlace(programID, fitness, agg, eco, expl, form)

	gr := &GateResult{
		Nash:              nash,
		WinRate:           wr,
		MapElitesPlaced:   placed,
		MapElitesImproved: placed && priorCell.Occupied,
		Placement:         placement,
	}

	nashOK := nash.NashValue >= g.cfg.NashThreshold
	winOK := g.cfg.WinRateLowerBound <= 0 || wr.Lower >= g.cfg.WinRateLowerBound
	mapOK := placed

	if nashOK && winOK && mapOK {
		gr.Promoted = true
		if !priorCell.Occupied {
			gr.Reason = fmt.Sprintf(
				"promoted: Nash=%.3f ≥ %.3f, WR=%.3f (95%% CI %.3f–%.3f), fills new niche [%d,%d,%d,%d]",
				nash.NashValue, g.cfg.NashThreshold,
				wr.Rate, wr.Lower, wr.Upper,
				placement.X, placement.Y, placement.Z, placement.W)
		} else {
			gr.Reason = fmt.Sprintf(
				"promoted: Nash=%.3f ≥ %.3f, WR=%.3f (95%% CI %.3f–%.3f), beats niche [%d,%d,%d,%d] champion (%.3f→%.3f)",
				nash.NashValue, g.cfg.NashThreshold,
				wr.Rate, wr.Lower, wr.Upper,
				placement.X, placement.Y, placement.Z, placement.W, priorCell.Fitness, fitness)
		}
		return gr
	}

	var why []string
	if !nashOK {
		why = append(why, fmt.Sprintf("Nash=%.3f < %.3f", nash.NashValue, g.cfg.NashThreshold))
	}
	if !winOK {
		why = append(why, fmt.Sprintf("WR CI lower=%.3f < %.3f", wr.Lower, g.cfg.WinRateLowerBound))
	}
	if !mapOK {
		why = append(why, fmt.Sprintf("niche [%d,%d,%d,%d] occupied by fitter bot (fitness=%.3f)",
			placement.X, placement.Y, placement.Z, placement.W, priorCell.Fitness))
	}
	gr.Reason = "rejected: " + strings.Join(why, "; ")
	return gr
}
