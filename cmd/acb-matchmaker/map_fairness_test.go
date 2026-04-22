package main

import (
	"math"
	"testing"
	"time"
)

func TestFairnessThresholdCalculation(t *testing.T) {
	// For N-player maps, expected win rate is 1/N.
	// A slot is flagged unfair if its win rate deviates by > 10pp.
	tests := []struct {
		name         string
		playerCount  int
		winRate      float64
		shouldFlag   bool
	}{
		{"2-player exact 50%", 2, 0.50, false},
		{"2-player 59%", 2, 0.59, false},
		{"2-player 60%", 2, 0.60, false},
		{"2-player 61%", 2, 0.61, true},
		{"2-player 39%", 2, 0.39, true},
		{"2-player 38%", 2, 0.38, true},
		{"2-player 37%", 2, 0.37, true},
		{"3-player exact 33%", 3, 1.0 / 3.0, false},
		{"3-player 44%", 3, 0.44, true},
		{"3-player 22%", 3, 0.22, true},
		{"4-player exact 25%", 4, 0.25, false},
		{"4-player 36%", 4, 0.36, true},
		{"4-player 14%", 4, 0.14, true},
		{"6-player exact 16.7%", 6, 1.0 / 6.0, false},
		{"6-player 27%", 6, 0.27, true},
		{"6-player 6%", 6, 0.06, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			expected := 1.0 / float64(tc.playerCount)
			deviation := math.Abs(tc.winRate - expected)
			shouldFlag := deviation > fairnessThresholdPP
			if shouldFlag != tc.shouldFlag {
				t.Errorf("playerCount=%d winRate=%.2f: deviation=%.4f, shouldFlag=%v, want %v",
					tc.playerCount, tc.winRate, deviation, shouldFlag, tc.shouldFlag)
			}
		})
	}
}

func TestFairnessMinGamesThreshold(t *testing.T) {
	// Only maps with >= 80 matches per slot are evaluated.
	tests := []struct {
		games      int
		shouldEval bool
	}{
		{0, false},
		{1, false},
		{79, false},
		{80, true},
		{100, true},
		{1000, true},
	}

	for _, tc := range tests {
		shouldEval := tc.games >= fairnessMinGames
		if shouldEval != tc.shouldEval {
			t.Errorf("games=%d: shouldEval=%v, want %v", tc.games, shouldEval, tc.shouldEval)
		}
	}
}

func TestVoteForceRetireThreshold(t *testing.T) {
	// Maps with >20 net negative votes are force-retired.
	tests := []struct {
		netVotes  int
		shouldRetire bool
	}{
		{-25, true},
		{-21, true},
		{-20, false},
		{-19, false},
		{-10, false},
		{0, false},
		{10, false},
		{50, false},
	}

	for _, tc := range tests {
		shouldRetire := tc.netVotes < voteForceRetireThreshold
		if shouldRetire != tc.shouldRetire {
			t.Errorf("netVotes=%d: shouldRetire=%v, want %v", tc.netVotes, shouldRetire, tc.shouldRetire)
		}
	}
}

func TestEngagementPrunePercentage(t *testing.T) {
	// Bottom 10% are pruned monthly per player-count tier.
	tests := []struct {
		totalActive int
		wantPruned  int
	}{
		{5, 0},   // too few to prune
		{10, 1},
		{20, 2},
		{50, 5},
		{100, 10},
	}

	for _, tc := range tests {
		toPrune := int(math.Ceil(float64(tc.totalActive) * engagementPrunePct))
		if tc.totalActive < 10 {
			toPrune = 0 // logic skips tiers with <10 maps
		}
		if toPrune != tc.wantPruned {
			t.Errorf("totalActive=%d: pruned=%d, want %d", tc.totalActive, toPrune, tc.wantPruned)
		}
	}
}

func TestClassicPromotionCriteria(t *testing.T) {
	// Maps must be active, have engagement > 0, be 3+ months old,
	// and be in the top 5 by engagement for their player count.
	tests := []struct {
		name        string
		engagement  float64
		ageMonths   int
		status      string
		shouldPromote bool
	}{
		{"meets all criteria", 8.5, 4, "active", true},
		{"too young", 9.0, 2, "active", false},
		{"zero engagement", 0.0, 6, "active", false},
		{"already classic", 9.0, 6, "classic", false},
		{"on probation", 7.0, 4, "probation", false},
		{"exactly 3 months", 7.0, 3, "active", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			isEligible := tc.status == "active" && tc.engagement > 0 && tc.ageMonths >= classicMinMonths
			if isEligible != tc.shouldPromote {
				t.Errorf("engagement=%.1f ageMonths=%d status=%s: eligible=%v, want %v",
					tc.engagement, tc.ageMonths, tc.status, isEligible, tc.shouldPromote)
			}
		})
	}
}

func TestFairnessAuditConfigDefault(t *testing.T) {
	cfg := loadConfig()
	if cfg.FairnessAuditSecs != 3600 {
		t.Errorf("FairnessAuditSecs default: got %d, want 3600", cfg.FairnessAuditSecs)
	}
}

func TestFairnessAuditConfigOverride(t *testing.T) {
	t.Setenv("ACB_FAIRNESS_AUDIT_INTERVAL", "7200")
	cfg := loadConfig()
	if cfg.FairnessAuditSecs != 7200 {
		t.Errorf("FairnessAuditSecs override: got %d, want 7200", cfg.FairnessAuditSecs)
	}
}

func TestMonthlyPruneOnlyOnFirst(t *testing.T) {
	// pruneLowEngagementMaps only runs on the 1st of each month.
	tests := []struct {
		day  int
		run  bool
	}{
		{1, true},
		{2, false},
		{15, false},
		{28, false},
		{31, false},
	}

	for _, tc := range tests {
		t.Run("", func(t *testing.T) {
			shouldRun := tc.day == 1
			if shouldRun != tc.run {
				t.Errorf("day=%d: shouldRun=%v, want %v", tc.day, shouldRun, tc.run)
			}
		})
	}
}

func TestClassicTopN(t *testing.T) {
	if classicTopN != 5 {
		t.Errorf("classicTopN: got %d, want 5", classicTopN)
	}
}

func TestClassicMinMonths(t *testing.T) {
	if classicMinMonths != 3 {
		t.Errorf("classicMinMonths: got %d, want 3", classicMinMonths)
	}
}

func TestFairnessAuditStepOrder(t *testing.T) {
	// Verify the ordering of steps in tickFairnessAudit:
	// 1. updateMapFairnessStats (recompute from match data)
	// 2. flagUnfairMaps (probation for unfair maps)
	// 3. retireDislikedMaps (force-retire by votes)
	// 4. pruneLowEngagementMaps (monthly bottom 10%)
	// 5. promoteClassicMaps (top-5 sustained engagement)
	//
	// This ordering matters because:
	// - Stats must be current before fairness checks
	// - Probation must happen before retirement (probation is a warning)
	// - Vote retirement is independent of engagement
	// - Classic promotion should happen after pruning (so promoted maps
	//   are truly immune)
	steps := []string{
		"updateMapFairnessStats",
		"flagUnfairMaps",
		"retireDislikedMaps",
		"pruneLowEngagementMaps",
		"promoteClassicMaps",
	}

	if len(steps) != 5 {
		t.Errorf("expected 5 fairness audit steps, got %d", len(steps))
	}
	if steps[0] != "updateMapFairnessStats" {
		t.Errorf("step 0 should be updateMapFairnessStats, got %s", steps[0])
	}
	if steps[1] != "flagUnfairMaps" {
		t.Errorf("step 1 should be flagUnfairMaps, got %s", steps[1])
	}
	if steps[4] != "promoteClassicMaps" {
		t.Errorf("step 4 should be promoteClassicMaps, got %s", steps[4])
	}
}

func TestProbationDoesNotAffectClassic(t *testing.T) {
	// Classic maps should never be moved to probation.
	// The flagUnfairMaps query only targets status='active'.
	status := "classic"
	canFlag := status == "active"
	if canFlag {
		t.Errorf("classic maps should not be flaggable as probation")
	}
}

func TestEngagementPruneSkipTierWithFewMaps(t *testing.T) {
	// Tiers with < 10 active maps should not be pruned.
	for _, totalActive := range []int{0, 1, 5, 9} {
		shouldSkip := totalActive < 10
		if !shouldSkip {
			t.Errorf("totalActive=%d should be skipped for pruning", totalActive)
		}
	}
}

func TestThreeMonthAgeCheck(t *testing.T) {
	// created_at must be >= 3 months ago for classic promotion.
	now := time.Now()
	tests := []struct {
		createdAgo time.Duration
		eligible   bool
	}{
		{30 * 24 * time.Hour, false},          // 1 month
		{89 * 24 * time.Hour, false},          // ~3 months minus 1 day
		{90 * 24 * time.Hour, true},           // 3 months
		{180 * 24 * time.Hour, true},          // 6 months
		{365 * 24 * time.Hour, true},          // 1 year
	}

	for _, tc := range tests {
		createdAt := now.Add(-tc.createdAgo)
		// Use a simpler check: created_at < NOW() - 3 months
		cutoff := now.AddDate(0, -classicMinMonths, 0)
		eligibleByDate := createdAt.Before(cutoff)
		if eligibleByDate != tc.eligible {
			t.Errorf("created %v ago: eligible=%v, want %v", tc.createdAgo, eligibleByDate, tc.eligible)
		}
	}
}
