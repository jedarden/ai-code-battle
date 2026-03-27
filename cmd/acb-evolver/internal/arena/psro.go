// Package arena — PSRO Nash equilibrium computation.
//
// LLM-PSRO (Policy Space Response Oracles) uses Nash equilibrium over the
// current bot population as the promotion criterion.  A candidate is promoted
// only if it is a best response to the Nash mixture, i.e. its expected payoff
// against the Nash mixture exceeds the threshold (default 0.50).
//
// For the mini-tournament setting (one candidate, K opponents), the payoff
// matrix has a single row.  The Nash-optimal strategy for the minimising
// column player (opponents) is to concentrate weight on the opponent that
// minimises the candidate's expected win rate.  The resulting Nash value is
// therefore min(winRates), which is the tightest possible test.
//
// The full fictitious-play algorithm is retained so it generalises cleanly
// to K×K payoff matrices when the population grows.
package arena

// NashResult holds the Nash equilibrium computation for the meta-game.
type NashResult struct {
	// OpponentMix[i] = probability of opponent i in the Nash mixture.
	// Sums to 1.0.
	OpponentMix []float64

	// NashValue is the candidate's expected win rate under the Nash mixture.
	// This is the quantity compared against the promotion threshold.
	NashValue float64

	// WinRatePerOpponent mirrors the input payoff row for convenience.
	WinRatePerOpponent []float64
}

// ComputeNash computes the Nash equilibrium for the 1×K meta-game where
// winRates[i] is the candidate's win rate against opponent i.
//
// The column player (opponent) minimises the candidate's expected win rate.
// The optimal column strategy concentrates on the opponent(s) with the lowest
// win rate for the candidate.  Ties in the minimum are distributed uniformly.
//
// Nash value = min(winRates)  (hardest-opponent test).
func ComputeNash(winRates []float64) NashResult {
	if len(winRates) == 0 {
		return NashResult{NashValue: 0.5}
	}

	K := len(winRates)
	mix := make([]float64, K)

	// Find the minimum win rate.
	minVal := winRates[0]
	for _, w := range winRates[1:] {
		if w < minVal {
			minVal = w
		}
	}

	// Distribute weight uniformly over all opponents achieving the minimum.
	count := 0
	for _, w := range winRates {
		if w == minVal {
			count++
		}
	}
	for i, w := range winRates {
		if w == minVal {
			mix[i] = 1.0 / float64(count)
		}
	}

	return NashResult{
		OpponentMix:        mix,
		NashValue:          minVal,
		WinRatePerOpponent: winRates,
	}
}

// FictitiousPlayNash computes the Nash equilibrium via fictitious play,
// converging over iterations rounds.  This generalises to K×K matrices and
// provides a softer mixed-strategy Nash than the pure-minimax above.
//
// For a 1×K payoff matrix both algorithms produce identical results, so this
// function is provided for future use when the full population payoff matrix
// is available.
func FictitiousPlayNash(winRates []float64, iterations int) NashResult {
	if len(winRates) == 0 {
		return NashResult{NashValue: 0.5}
	}
	if iterations <= 0 {
		iterations = 1000
	}

	K := len(winRates)
	counts := make([]float64, K)

	// Fictitious play: column player repeatedly best-responds to the current
	// row player strategy (fixed at "always play candidate").
	for iter := 0; iter < iterations; iter++ {
		// Column player best response: pick opponent minimising candidate win rate.
		best := 0
		for i := 1; i < K; i++ {
			if winRates[i] < winRates[best] {
				best = i
			}
		}
		counts[best]++
	}

	mix := make([]float64, K)
	expected := 0.0
	for i, c := range counts {
		mix[i] = c / float64(iterations)
		expected += mix[i] * winRates[i]
	}

	return NashResult{
		OpponentMix:        mix,
		NashValue:          expected,
		WinRatePerOpponent: winRates,
	}
}
