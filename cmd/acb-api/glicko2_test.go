package main

import (
	"math"
	"testing"
)

func TestGFunc(t *testing.T) {
	// g(0) should be 1
	if g := gFunc(0); math.Abs(g-1.0) > 1e-10 {
		t.Errorf("g(0) = %f, want 1.0", g)
	}

	// g(phi) should be between 0 and 1 for positive phi
	if g := gFunc(1.0); g <= 0 || g >= 1.0 {
		t.Errorf("g(1.0) = %f, want in (0, 1)", g)
	}

	// g should decrease as phi increases
	g1 := gFunc(0.5)
	g2 := gFunc(1.5)
	if g1 <= g2 {
		t.Errorf("g should decrease: g(0.5)=%f, g(1.5)=%f", g1, g2)
	}
}

func TestEFunc(t *testing.T) {
	// E(mu, mu, phi) should be 0.5 (equal ratings)
	e := eFunc(0, 0, 1.0)
	if math.Abs(e-0.5) > 1e-10 {
		t.Errorf("E(0, 0, 1.0) = %f, want 0.5", e)
	}

	// Higher mu should give higher expected score
	eHigh := eFunc(1.0, 0, 1.0)
	eLow := eFunc(-1.0, 0, 1.0)
	if eHigh <= 0.5 || eLow >= 0.5 {
		t.Errorf("expected eHigh>0.5, eLow<0.5, got %f, %f", eHigh, eLow)
	}
}

func TestUpdateRatings_TwoPlayers(t *testing.T) {
	// Two equally rated players, player 0 wins
	r := []Glicko2Rating{
		{Mu: 1500, Phi: 350, Sigma: 0.06},
		{Mu: 1500, Phi: 350, Sigma: 0.06},
	}
	scores := []float64{10, 5} // player 0 wins

	newR := updateRatings(r, scores)

	// Winner should gain rating
	if newR[0].Mu <= r[0].Mu {
		t.Errorf("winner mu should increase: %f -> %f", r[0].Mu, newR[0].Mu)
	}
	// Loser should lose rating
	if newR[1].Mu >= r[1].Mu {
		t.Errorf("loser mu should decrease: %f -> %f", r[1].Mu, newR[1].Mu)
	}

	// RD should decrease for both (more information)
	if newR[0].Phi >= r[0].Phi {
		t.Errorf("winner phi should decrease: %f -> %f", r[0].Phi, newR[0].Phi)
	}
	if newR[1].Phi >= r[1].Phi {
		t.Errorf("loser phi should decrease: %f -> %f", r[1].Phi, newR[1].Phi)
	}
}

func TestUpdateRatings_Draw(t *testing.T) {
	r := []Glicko2Rating{
		{Mu: 1500, Phi: 350, Sigma: 0.06},
		{Mu: 1500, Phi: 350, Sigma: 0.06},
	}
	scores := []float64{5, 5} // draw

	newR := updateRatings(r, scores)

	// Equal ratings + draw = negligible mu change
	diff := math.Abs(newR[0].Mu - newR[1].Mu)
	if diff > 1.0 {
		t.Errorf("draw should keep ratings close: %f vs %f (diff=%f)", newR[0].Mu, newR[1].Mu, diff)
	}

	// Both should be close to original
	if math.Abs(newR[0].Mu-1500) > 5.0 {
		t.Errorf("draw between equals should barely change rating: %f", newR[0].Mu)
	}
}

func TestUpdateRatings_UpsetGivesLargerGain(t *testing.T) {
	// Lower-rated player beats higher-rated player
	r := []Glicko2Rating{
		{Mu: 1300, Phi: 100, Sigma: 0.06}, // underdog
		{Mu: 1700, Phi: 100, Sigma: 0.06}, // favorite
	}
	scores := []float64{10, 5} // underdog wins

	newR := updateRatings(r, scores)

	underdogGain := newR[0].Mu - r[0].Mu
	favoriteGain := r[1].Mu - newR[1].Mu

	if underdogGain <= 0 {
		t.Errorf("underdog should gain rating: %f", underdogGain)
	}
	if favoriteGain <= 0 {
		t.Errorf("favorite should lose rating: %f", favoriteGain)
	}

	// Now test expected win: higher-rated player beats lower
	r2 := []Glicko2Rating{
		{Mu: 1700, Phi: 100, Sigma: 0.06}, // favorite
		{Mu: 1300, Phi: 100, Sigma: 0.06}, // underdog
	}
	scores2 := []float64{10, 5}
	newR2 := updateRatings(r2, scores2)
	expectedGain := newR2[0].Mu - r2[0].Mu

	// Upset should give larger rating change than expected result
	if underdogGain <= expectedGain {
		t.Errorf("upset gain (%f) should exceed expected win gain (%f)", underdogGain, expectedGain)
	}
}

func TestUpdateRatings_MultiPlayer(t *testing.T) {
	// 4-player match
	r := []Glicko2Rating{
		{Mu: 1500, Phi: 200, Sigma: 0.06},
		{Mu: 1500, Phi: 200, Sigma: 0.06},
		{Mu: 1500, Phi: 200, Sigma: 0.06},
		{Mu: 1500, Phi: 200, Sigma: 0.06},
	}
	scores := []float64{20, 15, 10, 5}

	newR := updateRatings(r, scores)

	// Ratings should be ordered by score
	for i := 0; i < len(newR)-1; i++ {
		if newR[i].Mu <= newR[i+1].Mu {
			t.Errorf("player %d (score=%0.f, mu=%f) should be rated above player %d (score=%0.f, mu=%f)",
				i, scores[i], newR[i].Mu, i+1, scores[i+1], newR[i+1].Mu)
		}
	}
}

func TestUpdateRatings_LowRDPlayersChangeMore(t *testing.T) {
	// Lower RD (more certain) means the system gives more weight to the result
	highRD := Glicko2Rating{Mu: 1500, Phi: 300, Sigma: 0.06}
	lowRD := Glicko2Rating{Mu: 1500, Phi: 100, Sigma: 0.06}

	// Both beat a 1500-rated opponent
	opp := Glicko2Rating{Mu: 1500, Phi: 200, Sigma: 0.06}

	r1 := updateRatings([]Glicko2Rating{highRD, opp}, []float64{10, 5})
	r2 := updateRatings([]Glicko2Rating{lowRD, opp}, []float64{10, 5})

	highRDGain := r1[0].Mu - highRD.Mu
	lowRDGain := r2[0].Mu - lowRD.Mu

	// High RD player should change more (less certainty = more adjustable)
	if highRDGain <= lowRDGain {
		t.Errorf("high RD player should gain more: %f vs %f", highRDGain, lowRDGain)
	}
}

func TestUpdateRatings_Determinism(t *testing.T) {
	r := []Glicko2Rating{
		{Mu: 1600, Phi: 150, Sigma: 0.06},
		{Mu: 1400, Phi: 250, Sigma: 0.06},
	}
	scores := []float64{8, 12}

	r1 := updateRatings(r, scores)
	r2 := updateRatings(r, scores)

	for i := range r1 {
		if r1[i].Mu != r2[i].Mu || r1[i].Phi != r2[i].Phi || r1[i].Sigma != r2[i].Sigma {
			t.Errorf("ratings not deterministic at index %d: %+v vs %+v", i, r1[i], r2[i])
		}
	}
}

func TestDisplayRating(t *testing.T) {
	r := Glicko2Rating{Mu: 1500, Phi: 350, Sigma: 0.06}
	display := r.DisplayRating()
	expected := 1500.0 - 2*350.0
	if math.Abs(display-expected) > 1e-10 {
		t.Errorf("DisplayRating() = %f, want %f", display, expected)
	}
}

func TestComputeVolatility_Convergence(t *testing.T) {
	// Should not panic or infinite loop
	sigma := computeVolatility(0.06, 1.0, 10.0, 5.0)
	if sigma <= 0 || math.IsNaN(sigma) || math.IsInf(sigma, 0) {
		t.Errorf("volatility should be positive finite: %f", sigma)
	}
}
