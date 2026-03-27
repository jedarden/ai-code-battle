package arena

import "math"

// WinRateResult holds the observed win rate and its 95% Wilson score confidence interval.
type WinRateResult struct {
	Wins   int
	Total  int     // non-error matches only
	Rate   float64 // observed win rate (0–1)
	Lower  float64 // 95% CI lower bound
	Upper  float64 // 95% CI upper bound
}

// WinRate computes the win rate and Wilson score 95% confidence interval
// for wins out of total valid matches.  When total == 0, all values are 0.5.
//
// Wilson score interval:
//
//	center = (p̂ + z²/2n) / (1 + z²/n)
//	margin = z * sqrt(p̂(1-p̂)/n + z²/4n²) / (1 + z²/n)
//	CI = [center − margin, center + margin]
//
// Using z = 1.96 (95% two-tailed confidence).
func WinRate(wins, total int) WinRateResult {
	if total == 0 {
		return WinRateResult{Rate: 0.5, Lower: 0.0, Upper: 1.0}
	}

	const z = 1.96 // 95% CI
	p := float64(wins) / float64(total)
	n := float64(total)
	z2 := z * z

	center := (p + z2/(2*n)) / (1 + z2/n)
	margin := z * math.Sqrt(p*(1-p)/n+z2/(4*n*n)) / (1 + z2/n)

	lower := math.Max(0, center-margin)
	upper := math.Min(1, center+margin)

	return WinRateResult{
		Wins:  wins,
		Total: total,
		Rate:  p,
		Lower: lower,
		Upper: upper,
	}
}

// ComputeFromResult builds a WinRateResult from a tournament Result.
// Only non-error matches are counted; draws count as 0.5 wins.
func ComputeFromResult(r *Result) WinRateResult {
	total := r.Wins + r.Losses + r.Draws
	// Count draws as half-wins for the rate; wins/total integers use integer wins.
	return WinRate(r.Wins, total)
}
