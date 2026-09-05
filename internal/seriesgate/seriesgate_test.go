package seriesgate

import (
	"strings"
	"testing"
)

// TestBlocking_AnchorsPredicateToCaller tests that the generated predicate
// binds to the caller's own match column and carries every condition the
// ordering rule needs. The worker's job-claim query and the API's open-match
// feed both embed this SQL verbatim, so a dropped condition silently removes
// the in-order guarantee for both of them.
func TestBlocking_AnchorsPredicateToCaller(t *testing.T) {
	tests := []struct {
		name     string
		matchRef string
	}{
		{"worker job claim", "jobs.match_id"},
		{"api open-match feed", "m.match_id"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Blocking(tc.matchRef)

			if !strings.Contains(got, "WHERE mine.match_id = "+tc.matchRef) {
				t.Errorf("predicate must correlate on %q, got:\n%s", tc.matchRef, got)
			}
			// Only games of a series still being played wait.
			if !strings.Contains(got, "waited.status = 'active'") {
				t.Errorf("predicate must ignore games of finished series, got:\n%s", got)
			}
			// The wait is on earlier games only — a later game must never
			// hold up an earlier one.
			if !strings.Contains(got, "earlier.game_num < mine.game_num") {
				t.Errorf("predicate must compare game_num with <, got:\n%s", got)
			}
			// A game with a recorded result (including a draw marker) is resolved.
			if !strings.Contains(got, "earlier.winner_id IS NULL") {
				t.Errorf("predicate must treat a recorded winner as resolved, got:\n%s", got)
			}
			// The caller wraps this in NOT EXISTS, so it must be an EXISTS body:
			// no leading NOT, and it must select from the caller's own series_games.
			if strings.Contains(got, "NOT EXISTS") {
				t.Errorf("Blocking returns an EXISTS body, not a full NOT EXISTS, got:\n%s", got)
			}
			if !strings.Contains(got, "FROM series_games mine") {
				t.Errorf("predicate must drive off the caller's own series_games row, got:\n%s", got)
			}
		})
	}
}

// TestBlocking_DistinctMatchRefs tests that two callers asking for different
// match columns get different SQL — a shared constant would silently correlate
// one caller against the other's table alias.
func TestBlocking_DistinctMatchRefs(t *testing.T) {
	if Blocking("jobs.match_id") == Blocking("m.match_id") {
		t.Error("different match references must produce different predicates")
	}
}
