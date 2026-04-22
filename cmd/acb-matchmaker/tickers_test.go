package main

import (
	"fmt"
	"math/rand"
	"testing"
	"time"
)

func seedBot(id string, mu float64) candidateBot {
	return candidateBot{ID: id, Mu: mu, Phi: 350}
}

func TestBestCandidate_NeverPairedPreferred(t *testing.T) {
	paired := candidateBot{ID: "paired", Mu: 1500, LastPairedAt: time.Now(), Games24h: 0}
	never := candidateBot{ID: "never", Mu: 1500, LastPairedAt: time.Time{}, Games24h: 10}

	got := bestCandidate([]candidateBot{paired, never})
	if got.ID != "never" {
		t.Errorf("never-paired bot should be preferred, got %s", got.ID)
	}

	// Reverse order should still pick never.
	got = bestCandidate([]candidateBot{never, paired})
	if got.ID != "never" {
		t.Errorf("never-paired bot should be preferred (reverse), got %s", got.ID)
	}
}

func TestBestCandidate_OldestPairingPreferred(t *testing.T) {
	old := candidateBot{ID: "old", Mu: 1500, LastPairedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), Games24h: 10}
	recent := candidateBot{ID: "recent", Mu: 1500, LastPairedAt: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC), Games24h: 0}

	got := bestCandidate([]candidateBot{old, recent})
	if got.ID != "old" {
		t.Errorf("oldest pairing should be preferred, got %s", got.ID)
	}

	got = bestCandidate([]candidateBot{recent, old})
	if got.ID != "old" {
		t.Errorf("oldest pairing should be preferred (reverse), got %s", got.ID)
	}
}

func TestBestCandidate_GameCountBreaksTie(t *testing.T) {
	pairTime := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)
	fewer := candidateBot{ID: "fewer", Mu: 1500, LastPairedAt: pairTime, Games24h: 3}
	more := candidateBot{ID: "more", Mu: 1500, LastPairedAt: pairTime, Games24h: 8}

	got := bestCandidate([]candidateBot{fewer, more})
	if got.ID != "fewer" {
		t.Errorf("fewer 24h games should win tie, got %s", got.ID)
	}

	got = bestCandidate([]candidateBot{more, fewer})
	if got.ID != "fewer" {
		t.Errorf("fewer 24h games should win tie (reverse), got %s", got.ID)
	}
}

func TestBestCandidate_GameCountNeverPairedTie(t *testing.T) {
	a := candidateBot{ID: "a", Mu: 1500, LastPairedAt: time.Time{}, Games24h: 5}
	b := candidateBot{ID: "b", Mu: 1500, LastPairedAt: time.Time{}, Games24h: 2}

	got := bestCandidate([]candidateBot{a, b})
	if got.ID != "b" {
		t.Errorf("fewer games should break tie among never-paired, got %s", got.ID)
	}
}

func TestBestCandidate_PairingRecencyBeatsGameCount(t *testing.T) {
	// Regression test: game count must NOT override pairing recency.
	// "old" was paired long ago but has many games; "recent" was paired
	// recently but has few games. Pairing recency must win.
	old := candidateBot{ID: "old", Mu: 1500, LastPairedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), Games24h: 15}
	recent := candidateBot{ID: "recent", Mu: 1500, LastPairedAt: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC), Games24h: 1}

	got := bestCandidate([]candidateBot{old, recent})
	if got.ID != "old" {
		t.Errorf("pairing recency must beat game count, got %s", got.ID)
	}
}

func TestBestCandidate_NeverPairedBeatsOldPairingEvenWithManyGames(t *testing.T) {
	never := candidateBot{ID: "never", Mu: 1500, LastPairedAt: time.Time{}, Games24h: 50}
	paired := candidateBot{ID: "paired", Mu: 1500, LastPairedAt: time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC), Games24h: 0}

	got := bestCandidate([]candidateBot{never, paired})
	if got.ID != "never" {
		t.Errorf("never-paired must beat any previously-paired bot regardless of games, got %s", got.ID)
	}
}

func TestSelectOpponents_ParetoDistribution(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	seedMu := 1500.0

	// Build pool: 20 bots spread from 1400 to 1600.
	pool := make([]candidateBot, 20)
	for i := range pool {
		pool[i] = candidateBot{
			ID:   fmt.Sprintf("bot_%02d", i),
			Mu:   1400 + float64(i)*10,
			LastPairedAt: time.Time{},
			Games24h:     0,
		}
	}

	// Run many selections and check that the chosen opponents cluster near seed.
	totalDist := 0.0
	trials := 1000
	for range trials {
		poolCopy := make([]candidateBot, len(pool))
		copy(poolCopy, pool)
		selected := selectOpponents(rng, seedMu, poolCopy, 1)
		if len(selected) != 1 {
			t.Fatalf("expected 1 opponent, got %d", len(selected))
		}
		totalDist += abs(selected[0].Mu - seedMu)
	}
	avgDist := totalDist / float64(trials)

	// With 80% Pareto within 16 closest (~80 rating range) and 20% from all
	// (~100 rating range), average distance should be well under 50.
	if avgDist > 50 {
		t.Errorf("Pareto not concentrating opponents near seed: avg distance = %.1f", avgDist)
	}
}

func TestSelectOpponents_SelectsMultiple(t *testing.T) {
	rng := rand.New(rand.NewSource(99))
	seedMu := 1500.0

	pool := make([]candidateBot, 10)
	for i := range pool {
		pool[i] = candidateBot{
			ID:           fmt.Sprintf("bot_%d", i),
			Mu:           1500 + float64(i-5)*20,
			LastPairedAt: time.Time{},
			Games24h:     0,
		}
	}

	selected := selectOpponents(rng, seedMu, pool, 3)
	if len(selected) != 3 {
		t.Fatalf("expected 3 opponents, got %d", len(selected))
	}

	// No duplicates.
	seen := map[string]bool{}
	for _, s := range selected {
		if seen[s.ID] {
			t.Errorf("duplicate opponent: %s", s.ID)
		}
		seen[s.ID] = true
	}
}

func TestSelectOpponents_RespectsRecency(t *testing.T) {
	rng := rand.New(rand.NewSource(7))
	seedMu := 1500.0

	// Two bots at same distance from seed, same games, but different pairing times.
	fresh := candidateBot{ID: "fresh", Mu: 1510, LastPairedAt: time.Now(), Games24h: 3}
	stale := candidateBot{ID: "stale", Mu: 1490, LastPairedAt: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC), Games24h: 3}

	staleWins := 0
	trials := 100
	for range trials {
		pool := []candidateBot{fresh, stale}
		selected := selectOpponents(rng, seedMu, pool, 1)
		if selected[0].ID == "stale" {
			staleWins++
		}
	}

	// Since both are within the Pareto window and stale has older pairing,
	// it should always win (no randomness in bestCandidate when criteria are clear).
	if staleWins != trials {
		t.Errorf("stale should always win over fresh when both in Pareto window, won %d/%d", staleWins, trials)
	}
}

func TestGridForPlayers(t *testing.T) {
	tests := []struct {
		players int
		minArea int
		maxArea int
	}{
		{2, 3000, 4200},   // 60x60 = 3600
		{3, 4000, 6000},
		{4, 5000, 8500},
		{6, 7000, 12000},
	}
	for _, tt := range tests {
		r, c := gridForPlayers(tt.players)
		area := r * c
		if area < tt.minArea || area > tt.maxArea {
			t.Errorf("gridForPlayers(%d) = %dx%d (area=%d), want area in [%d,%d]",
				tt.players, r, c, area, tt.minArea, tt.maxArea)
		}
	}
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
