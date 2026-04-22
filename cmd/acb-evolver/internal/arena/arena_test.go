package arena

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/aicodebattle/acb/cmd/acb-evolver/internal/mapelites"
)

// ── ComputeNash ───────────────────────────────────────────────────────────────

func TestComputeNash_EmptySlice(t *testing.T) {
	r := ComputeNash(nil)
	if r.NashValue != 0.5 {
		t.Errorf("empty: NashValue = %.3f, want 0.5", r.NashValue)
	}
}

func TestComputeNash_SingleOpponent(t *testing.T) {
	r := ComputeNash([]float64{0.7})
	if r.NashValue != 0.7 {
		t.Errorf("single: NashValue = %.3f, want 0.7", r.NashValue)
	}
	if r.OpponentMix[0] != 1.0 {
		t.Errorf("single: mix[0] = %.3f, want 1.0", r.OpponentMix[0])
	}
}

func TestComputeNash_MinimumIsHardestOpponent(t *testing.T) {
	// Column player minimises candidate win rate → Nash value = min(winRates).
	winRates := []float64{0.8, 0.3, 0.6}
	r := ComputeNash(winRates)
	if r.NashValue != 0.3 {
		t.Errorf("NashValue = %.3f, want 0.3", r.NashValue)
	}
	// All weight on opponent index 1 (win rate 0.3).
	for i, w := range r.OpponentMix {
		if i == 1 {
			if w != 1.0 {
				t.Errorf("mix[1] = %.3f, want 1.0", w)
			}
		} else if w != 0.0 {
			t.Errorf("mix[%d] = %.3f, want 0.0", i, w)
		}
	}
}

func TestComputeNash_TiedMinimum(t *testing.T) {
	// Two opponents tied at the minimum: weight is split 50/50.
	winRates := []float64{0.2, 0.8, 0.2}
	r := ComputeNash(winRates)
	if r.NashValue != 0.2 {
		t.Errorf("NashValue = %.3f, want 0.2", r.NashValue)
	}
	if r.OpponentMix[0] != 0.5 || r.OpponentMix[2] != 0.5 {
		t.Errorf("tied mix = %v, want [0.5 0.0 0.5]", r.OpponentMix)
	}
	if r.OpponentMix[1] != 0.0 {
		t.Errorf("mix[1] = %.3f, want 0.0", r.OpponentMix[1])
	}
}

func TestComputeNash_AllEqual(t *testing.T) {
	winRates := []float64{0.5, 0.5, 0.5}
	r := ComputeNash(winRates)
	if r.NashValue != 0.5 {
		t.Errorf("all-equal: NashValue = %.3f, want 0.5", r.NashValue)
	}
	// All opponents get equal weight.
	expected := 1.0 / 3.0
	for i, w := range r.OpponentMix {
		if abs(w-expected) > 1e-9 {
			t.Errorf("mix[%d] = %.6f, want %.6f", i, w, expected)
		}
	}
}

func TestFictitiousPlayNash_MatchesMinimaxForSingleRow(t *testing.T) {
	winRates := []float64{0.8, 0.3, 0.6}
	fp := FictitiousPlayNash(winRates, 10000)
	if abs(fp.NashValue-0.3) > 0.01 {
		t.Errorf("fictitious play: NashValue = %.3f, want ≈0.3", fp.NashValue)
	}
}

// ── WinRate ───────────────────────────────────────────────────────────────────

func TestWinRate_ZeroTotal(t *testing.T) {
	r := WinRate(0, 0)
	if r.Rate != 0.5 {
		t.Errorf("zero total: Rate = %.3f, want 0.5", r.Rate)
	}
}

func TestWinRate_AllWins(t *testing.T) {
	r := WinRate(10, 10)
	if r.Rate != 1.0 {
		t.Errorf("all wins: Rate = %.3f, want 1.0", r.Rate)
	}
	if r.Lower > r.Upper {
		t.Errorf("CI inverted: lower=%.3f upper=%.3f", r.Lower, r.Upper)
	}
}

func TestWinRate_AllLosses(t *testing.T) {
	r := WinRate(0, 10)
	if r.Rate != 0.0 {
		t.Errorf("all losses: Rate = %.3f, want 0.0", r.Rate)
	}
	if r.Lower < 0.0 || r.Upper > 1.0 {
		t.Errorf("CI out of [0,1]: lower=%.3f upper=%.3f", r.Lower, r.Upper)
	}
}

func TestWinRate_FiftyPercent(t *testing.T) {
	r := WinRate(5, 10)
	if abs(r.Rate-0.5) > 1e-9 {
		t.Errorf("50%%: Rate = %.3f, want 0.5", r.Rate)
	}
	if r.Lower >= 0.5 || r.Upper <= 0.5 {
		t.Errorf("50%% CI should straddle 0.5: lower=%.3f upper=%.3f", r.Lower, r.Upper)
	}
}

func TestWinRate_CIBounds(t *testing.T) {
	// CI bounds must always lie in [0, 1].
	for wins := 0; wins <= 10; wins++ {
		r := WinRate(wins, 10)
		if r.Lower < 0.0 || r.Upper > 1.0 {
			t.Errorf("wins=%d: CI [%.3f, %.3f] outside [0,1]", wins, r.Lower, r.Upper)
		}
		if r.Lower > r.Upper {
			t.Errorf("wins=%d: lower (%.3f) > upper (%.3f)", wins, r.Lower, r.Upper)
		}
	}
}

// ── ComputeFromResult ─────────────────────────────────────────────────────────

func TestComputeFromResult_Basic(t *testing.T) {
	r := &Result{Wins: 7, Losses: 2, Draws: 1}
	wr := ComputeFromResult(r)
	if wr.Wins != 7 {
		t.Errorf("Wins = %d, want 7", wr.Wins)
	}
	// 7 wins / 10 total = 0.7 rate
	if abs(wr.Rate-0.7) > 1e-9 {
		t.Errorf("Rate = %.3f, want 0.7", wr.Rate)
	}
}

func TestComputeFromResult_OnlyErrors(t *testing.T) {
	r := &Result{Wins: 0, Losses: 0, Draws: 0, Errors: 5}
	wr := ComputeFromResult(r)
	if wr.Total != 0 {
		t.Errorf("Total = %d, want 0 (errors excluded)", wr.Total)
	}
}

// ── Gate.Evaluate ─────────────────────────────────────────────────────────────

func TestGate_PromotedWhenAllCriteriaMet(t *testing.T) {
	grid := mapelites.New(10)
	gate := NewGate(DefaultGateConfig(), grid)

	result := &Result{
		Wins: 8, Losses: 2, Draws: 0,
		WinRateVec: []float64{0.8, 0.7, 0.9, 0.6, 0.8, 0.7, 0.8, 0.9, 0.7, 0.8},
	}

	gr := gate.Evaluate(result, 1, 0.8, []float64{0.5, 0.5})
	if !gr.Promoted {
		t.Errorf("expected promoted, got rejected: %s", gr.Reason)
	}
	if !gr.MapElitesPlaced {
		t.Error("expected MapElitesPlaced = true for empty grid")
	}
	if gr.MapElitesImproved {
		t.Error("expected MapElitesImproved = false for empty cell")
	}
}

func TestGate_RejectedWhenNashTooLow(t *testing.T) {
	grid := mapelites.New(10)
	cfg := GateConfig{NashThreshold: 0.60, WinRateLowerBound: 0.0}
	gate := NewGate(cfg, grid)

	// WinRateVec has a low value → Nash = min = 0.2, below 0.60
	result := &Result{
		Wins: 7, Losses: 3,
		WinRateVec: []float64{0.9, 0.2, 0.9, 0.9, 0.9},
	}

	gr := gate.Evaluate(result, 2, 0.7, []float64{0.5, 0.5})
	if gr.Promoted {
		t.Errorf("should be rejected (Nash too low), got: %s", gr.Reason)
	}
}

func TestGate_RejectedWhenNicheOccupiedByFitterBot(t *testing.T) {
	grid := mapelites.New(10)

	// Pre-occupy the [5,5] cell with a very fit bot.
	grid.TryPlace(99, 0.99, 0.5, 0.5, 0.5, 0.5)

	cfg := DefaultGateConfig()
	gate := NewGate(cfg, grid)

	// Candidate is in the same niche but has lower fitness.
	result := &Result{
		Wins: 7, Losses: 3,
		WinRateVec: []float64{0.8, 0.7, 0.9, 0.6, 0.8, 0.7, 0.8, 0.9, 0.7, 0.8},
	}

	gr := gate.Evaluate(result, 1, 0.7, []float64{0.5, 0.5})
	if gr.Promoted {
		t.Errorf("should be rejected (niche occupied by fitter bot), got: %s", gr.Reason)
	}
	if gr.MapElitesPlaced {
		t.Error("MapElitesPlaced should be false when existing bot is fitter")
	}
}

func TestGate_PromotedWhenOutperformsNicheChampion(t *testing.T) {
	grid := mapelites.New(10)

	// Pre-occupy with a weaker bot.
	grid.TryPlace(99, 0.4, 0.5, 0.5, 0.5, 0.5)

	cfg := DefaultGateConfig()
	gate := NewGate(cfg, grid)

	// Candidate is fitter than the incumbent.
	result := &Result{
		Wins: 8, Losses: 2,
		WinRateVec: []float64{0.8, 0.7, 0.9, 0.6, 0.8, 0.7, 0.8, 0.9, 0.7, 0.8},
	}

	gr := gate.Evaluate(result, 1, 0.8, []float64{0.5, 0.5})
	if !gr.Promoted {
		t.Errorf("should be promoted (beats incumbent), got: %s", gr.Reason)
	}
	if !gr.MapElitesImproved {
		t.Error("MapElitesImproved should be true when beating existing champion")
	}
}

// ── selectDiverse ─────────────────────────────────────────────────────────────

func TestSelectDiverse_EmptyPool(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	result := selectDiverse(nil, 5, rng)
	if len(result) != 0 {
		t.Errorf("empty pool: got %d opponents, want 0", len(result))
	}
}

func TestSelectDiverse_ExactlyN(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	bots := makeBots(5)
	result := selectDiverse(bots, 5, rng)
	if len(result) != 5 {
		t.Errorf("exact n: got %d opponents, want 5", len(result))
	}
}

func TestSelectDiverse_MoreThanN(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	bots := makeBots(20)
	result := selectDiverse(bots, 10, rng)
	if len(result) != 10 {
		t.Errorf("more than n: got %d opponents, want 10", len(result))
	}
	// Verify spread: should sample across the sorted range, not just top/bottom.
	seen := make(map[string]bool)
	for _, b := range result {
		seen[b.BotID] = true
	}
	if len(seen) != 10 {
		t.Errorf("duplicates in diverse selection: got %d unique, want 10", len(seen))
	}
}

func TestSelectDiverse_FewerThanN(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	bots := makeBots(3)
	// With only 3 bots, need to repeat to fill 10 slots.
	result := selectDiverse(bots, 10, rng)
	if len(result) != 10 {
		t.Errorf("fewer than n: got %d opponents, want 10", len(result))
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

func makeBots(n int) []BotRecord {
	bots := make([]BotRecord, n)
	for i := range bots {
		bots[i] = BotRecord{
			BotID:    fmt.Sprintf("b_%04d", i),
			Name:     fmt.Sprintf("bot-%d", i),
			RatingMu: float64(1000 + i*50),
		}
	}
	return bots
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
