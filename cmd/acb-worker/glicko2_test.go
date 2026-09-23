package main

import (
	"math"
	"testing"
)

// Expected values in this file were produced by an independent reference
// implementation of http://www.glicko.net/glicko/glicko2.pdf (tau = 0.5,
// epsilon = 1e-6); the paper's own worked example appears as
// TestUpdateSingleRatingPaperExample.

const tol = 1e-2      // rating / RD tolerance in Glicko-1 points
const tolSigma = 5e-5 // volatility tolerance

func assertClose(t *testing.T, name string, got, want, tolerance float64) {
	t.Helper()
	if math.Abs(got-want) > tolerance {
		t.Errorf("%s = %.6f, want %.6f (tolerance %g)", name, got, want, tolerance)
	}
}

func TestToScaleRoundTrip(t *testing.T) {
	cases := [][2]float64{
		{1500, 350},
		{1500, 200},
		{1464.05, 151.52},
		{2020, 149},
		{1000, 60},
	}
	for _, c := range cases {
		mu2, phi2 := toScale(c[0], c[1])
		mu, phi := fromScale(mu2, phi2)
		assertClose(t, "round-trip mu", mu, c[0], 1e-9)
		assertClose(t, "round-trip phi", phi, c[1], 1e-9)
	}

	// The default rating centers to 0; RD is divided by the scale, not centered.
	mu2, phi2 := toScale(glicko2DefaultMu, glicko2DefaultRD)
	assertClose(t, "default mu2", mu2, 0, 1e-12)
	assertClose(t, "default phi2", phi2, glicko2DefaultRD/glicko2Scale, 1e-12)
}

func TestGFunc(t *testing.T) {
	assertClose(t, "g(0)", gFunc(0), 1.0, 1e-12)
	assertClose(t, "g(1)", gFunc(1), 1/math.Sqrt(1+3/(math.Pi*math.Pi)), 1e-12)

	// g is strictly decreasing in phi.
	prev := gFunc(0)
	for _, phi := range []float64{0.25, 0.5, 1, 2, 4} {
		g := gFunc(phi)
		if g >= prev {
			t.Errorf("gFunc(%v) = %v not less than previous %v", phi, g, prev)
		}
		prev = g
	}
}

func TestEFunc(t *testing.T) {
	// Equal ratings always give expectation 0.5.
	if e := eFunc(1.234, 1.234, 0.7); math.Abs(e-0.5) > 1e-12 {
		t.Errorf("eFunc(equal) = %v, want 0.5", e)
	}
	// Monotonically increasing in mu, bounded by (0, 1).
	prev := 0.0
	for _, mu := range []float64{-3, -1, 0, 1, 3} {
		e := eFunc(mu, 0, 0.5)
		if e <= prev {
			t.Errorf("eFunc(%v) = %v not greater than previous %v", mu, e, prev)
		}
		if e <= 0 || e >= 1 {
			t.Errorf("eFunc(%v) = %v outside (0, 1)", mu, e)
		}
		prev = e
	}
}

func TestDisplayRating(t *testing.T) {
	r := Glicko2Rating{Mu: 1500, Phi: 350}
	assertClose(t, "DisplayRating", r.DisplayRating(), 1500-2*350, 1e-9)
}

func TestDefaultRatingValues(t *testing.T) {
	if glicko2DefaultMu != 1500 {
		t.Errorf("default mu = %v, want 1500", glicko2DefaultMu)
	}
	if glicko2DefaultRD != 350 {
		t.Errorf("default RD = %v, want 350", glicko2DefaultRD)
	}
	if glicko2DefaultSigma != 0.06 {
		t.Errorf("default sigma = %v, want 0.06", glicko2DefaultSigma)
	}
}

// TestUpdateSingleRatingPaperExample checks against the worked example in
// glicko2.pdf: a player rated 1500 at RD 200 plays (and beats) a 1500/RD 30
// bot, loses to a 1550/RD 100 bot, and loses to a 1700/RD 300 bot.
// Expected outcome per the paper: rating 1464.06, RD 151.52, volatility 0.05999.
func TestUpdateSingleRatingPaperExample(t *testing.T) {
	player := Glicko2Rating{Mu: 1500, Phi: 200, Sigma: 0.06}
	opps := []opponent{
		{mu2: toScaleI(1400), phi2: 30 / glicko2Scale, score: 1},
		{mu2: toScaleI(1550), phi2: 100 / glicko2Scale, score: 0},
		{mu2: toScaleI(1700), phi2: 300 / glicko2Scale, score: 0},
	}

	got := updateSingleRating(player, opps)

	assertClose(t, "mu", got.Mu, 1464.06, tol)
	assertClose(t, "phi", got.Phi, 151.52, tol)
	assertClose(t, "sigma", got.Sigma, 0.05999, tolSigma)
}

func toScaleI(mu float64) float64 {
	mu2, _ := toScale(mu, 0)
	return mu2
}

// TestInactivityRDGrowth covers the no-opponents branch: RD grows by
// sqrt(phi^2 + sigma^2) each rating period, capped at the default RD, while
// rating and volatility stay put.
func TestInactivityRDGrowth(t *testing.T) {
	// A settled bot (RD 60) idle for one period grows deterministically.
	settled := Glicko2Rating{Mu: 1800, Phi: 60, Sigma: 0.06}
	got := updateSingleRating(settled, nil)

	wantPhi := math.Min(math.Sqrt(60*60+0.06*0.06*glicko2Scale*glicko2Scale), glicko2DefaultRD)
	assertClose(t, "grown phi", got.Phi, wantPhi, 1e-9)
	if got.Phi <= settled.Phi {
		t.Errorf("RD did not grow: %v -> %v", settled.Phi, got.Phi)
	}
	assertClose(t, "mu unchanged", got.Mu, settled.Mu, 1e-12)
	assertClose(t, "sigma unchanged", got.Sigma, settled.Sigma, 1e-12)

	// An idle bot already at the default RD stays capped there.
	loose := Glicko2Rating{Mu: 1500, Phi: glicko2DefaultRD, Sigma: 0.06}
	capped := updateSingleRating(loose, nil)
	assertClose(t, "capped phi", capped.Phi, glicko2DefaultRD, 1e-9)
}

func TestDrawBetweenEqualPlayers(t *testing.T) {
	player := Glicko2Rating{Mu: 1500, Phi: glicko2DefaultRD, Sigma: 0.06}
	opp := []opponent{{mu2: 0, phi2: glicko2DefaultRD / glicko2Scale, score: 0.5}}

	got := updateSingleRating(player, opp)

	// A draw between identical players moves neither rating...
	assertClose(t, "mu", got.Mu, 1500, tol)
	// ...but playing a game is information, so RD tightens: 350 -> 290.32.
	assertClose(t, "phi", got.Phi, 290.32, tol)
}

func TestWinLossSymmetry(t *testing.T) {
	base := Glicko2Rating{Mu: 1500, Phi: glicko2DefaultRD, Sigma: 0.06}
	win := updateSingleRating(base, []opponent{{mu2: 0, phi2: glicko2DefaultRD / glicko2Scale, score: 1}})
	loss := updateSingleRating(base, []opponent{{mu2: 0, phi2: glicko2DefaultRD / glicko2Scale, score: 0}})

	// Equal players trading a win and a loss move by equal and opposite amounts.
	assertClose(t, "winner gain", win.Mu-1500, 162.31, tol)
	assertClose(t, "losser loss", loss.Mu-1500, -162.31, tol)
	if math.Abs((win.Mu-1500)+(loss.Mu-1500)) > tol {
		t.Errorf("win/loss deltas not opposite: %v vs %v", win.Mu-1500, -(loss.Mu - 1500))
	}
	// Both learned something.
	assertClose(t, "win phi", win.Phi, 290.32, tol)
	assertClose(t, "loss phi", loss.Phi, 290.32, tol)
}

// TestUpdateRatingsPairwise covers the multi-player path: scores are compared
// pairwise, so a clear winner / middle / loser ordering on equal defaults
// yields symmetric rating swaps and RD tightening for everyone.
func TestUpdateRatingsPairwise(t *testing.T) {
	ratings := []Glicko2Rating{
		{Mu: 1500, Phi: glicko2DefaultRD, Sigma: 0.06},
		{Mu: 1500, Phi: glicko2DefaultRD, Sigma: 0.06},
		{Mu: 1500, Phi: glicko2DefaultRD, Sigma: 0.06},
	}
	scores := []float64{3.0, 1.0, 0.0} // winner, middle, loser

	got := UpdateRatings(ratings, scores)

	assertClose(t, "winner mu", got[0].Mu, 1747.32, 1.0)
	assertClose(t, "middle mu", got[1].Mu, 1500.0, 1.0)
	assertClose(t, "loser mu", got[2].Mu, 1252.68, 1.0)
	for i, want := range []float64{253.40, 253.40, 253.40} {
		assertClose(t, "player phi", got[i].Phi, want, 1.0)
	}
	if got[0].Mu <= got[1].Mu || got[1].Mu <= got[2].Mu {
		t.Errorf("ordering not preserved by rating: %v %v %v", got[0].Mu, got[1].Mu, got[2].Mu)
	}
}

func TestUpdateRatingsFewerThanTwo(t *testing.T) {
	ratings := []Glicko2Rating{{Mu: 1234, Phi: 200, Sigma: 0.06}}
	got := UpdateRatings(ratings, []float64{1.0})
	if len(got) != 1 || got[0] != ratings[0] {
		t.Errorf("single-player update should be a no-op, got %v", got)
	}
}

func TestComputeRatingUpdatesLengthMismatch(t *testing.T) {
	ids := []string{"a", "b"}
	ratings := []Glicko2Rating{{Mu: 1500, Phi: 350}, {Mu: 1500, Phi: 350}}
	scores := []float64{1.0} // wrong length

	if got := ComputeRatingUpdates(ids, ratings, scores); got != nil {
		t.Errorf("expected nil on length mismatch, got %v", got)
	}
	if got := ComputeRatingUpdates(ids[:1], ratings, scores); got != nil {
		t.Errorf("expected nil on length mismatch, got %v", got)
	}
}

func TestComputeRatingUpdatesFields(t *testing.T) {
	before := []Glicko2Rating{{Mu: 1500, Phi: 350, Sigma: 0.06}, {Mu: 1500, Phi: 350, Sigma: 0.06}}
	updates := ComputeRatingUpdates([]string{"winner", "loser"}, before, []float64{1.0, 0.0})
	if updates == nil {
		t.Fatal("expected updates, got nil")
	}

	winner, loser := updates[0], updates[1]
	if winner.BotID != "winner" || loser.BotID != "loser" {
		t.Fatalf("bot IDs mismatched: %v", updates)
	}
	if winner.DisplayRating != winner.Mu-2*winner.Phi {
		t.Errorf("DisplayRating = %v, want mu-2*phi = %v", winner.DisplayRating, winner.Mu-2*winner.Phi)
	}
	if winner.RatingMuBefore != 1500 || winner.RatingPhiBefore != 350 {
		t.Errorf("before-fields wrong: mu %v phi %v", winner.RatingMuBefore, winner.RatingPhiBefore)
	}
	if winner.RatingDeviationChange != winner.Phi-350 {
		t.Errorf("RatingDeviationChange = %v, want %v", winner.RatingDeviationChange, winner.Phi-350)
	}
	if winner.RatingDeviationChange >= 0 {
		t.Errorf("playing a match should reduce RD, got change %v", winner.RatingDeviationChange)
	}
	if winner.Mu <= 1500 || loser.Mu >= 1500 {
		t.Errorf("winner should gain and loser lose: %v %v", winner.Mu, loser.Mu)
	}
}

// TestVolatilityStaysBounded plays a long winning streak against stronger
// opponents: volatility must stay positive and must not grow — consistent
// results tighten the uncertainty estimate, per the paper.
func TestVolatilityStaysBounded(t *testing.T) {
	player := Glicko2Rating{Mu: 1500, Phi: 350, Sigma: 0.06}
	opps := []opponent{{mu2: toScaleI(1600), phi2: 200 / glicko2Scale, score: 1}}

	for i := 0; i < 10; i++ {
		player = updateSingleRating(player, opps)
		if player.Sigma <= 0 || player.Sigma > 0.1 {
			t.Fatalf("volatility out of bounds after %d wins: %v", i+1, player.Sigma)
		}
	}
	if player.Sigma > 0.06 {
		t.Errorf("volatility grew on a consistent streak: 0.06 -> %v", player.Sigma)
	}
	assertClose(t, "mu after 10 wins", player.Mu, 2020.12, 1.0)
	assertClose(t, "phi after 10 wins", player.Phi, 149.55, 1.0)
}
