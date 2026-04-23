package prompt

import (
	"testing"

	evolverdb "github.com/aicodebattle/acb/cmd/acb-evolver/internal/db"
	"github.com/aicodebattle/acb/cmd/acb-evolver/internal/meta"
	"github.com/aicodebattle/acb/cmd/acb-evolver/internal/replay"
)

func TestFromReplayAnalysis_Nil(t *testing.T) {
	got := FromReplayAnalysis(nil)
	if got.MatchID != "" || got.WinnerName != "" {
		t.Errorf("expected empty MatchSummary for nil input, got %+v", got)
	}
}

func TestFromReplayAnalysis_Full(t *testing.T) {
	analysis := &replay.Analysis{
		MatchID:    "match-123",
		WinnerName: "Winner",
		LoserName:  "Loser",
		Condition:  "elimination",
		TurnCount:  100,
		Scores:     []int{50, 20},
		KeyMoments: []string{"moment1", "moment2"},
		Strategies: []string{"strategy1"},
		Weaknesses: []string{"weakness1"},
	}

	got := FromReplayAnalysis(analysis)

	if got.MatchID != "match-123" {
		t.Errorf("expected MatchID 'match-123', got %q", got.MatchID)
	}
	if got.WinnerName != "Winner" {
		t.Errorf("expected WinnerName 'Winner', got %q", got.WinnerName)
	}
	if got.LoserName != "Loser" {
		t.Errorf("expected LoserName 'Loser', got %q", got.LoserName)
	}
	if got.Condition != "elimination" {
		t.Errorf("expected Condition 'elimination', got %q", got.Condition)
	}
	if got.TurnCount != 100 {
		t.Errorf("expected TurnCount 100, got %d", got.TurnCount)
	}
	if len(got.Scores) != 2 || got.Scores[0] != 50 || got.Scores[1] != 20 {
		t.Errorf("expected Scores [50, 20], got %v", got.Scores)
	}
	if len(got.KeyMoments) != 2 {
		t.Errorf("expected 2 KeyMoments, got %d", len(got.KeyMoments))
	}
	if len(got.Strategies) != 1 {
		t.Errorf("expected 1 Strategy, got %d", len(got.Strategies))
	}
	if len(got.Weaknesses) != 1 {
		t.Errorf("expected 1 Weakness, got %d", len(got.Weaknesses))
	}
}

func TestFromReplayAnalysis_SliceCopy(t *testing.T) {
	analysis := &replay.Analysis{
		Scores:     []int{1, 2, 3},
		KeyMoments: []string{"a", "b"},
	}

	got := FromReplayAnalysis(analysis)

	// Modify original slices to ensure we have a copy
	analysis.Scores[0] = 999
	analysis.KeyMoments[0] = "modified"

	if got.Scores[0] == 999 {
		t.Error("expected Scores to be a copy, not a reference")
	}
	if got.KeyMoments[0] == "modified" {
		t.Error("expected KeyMoments to be a copy, not a reference")
	}
}

func TestFromReplayAnalyses_Nil(t *testing.T) {
	got := FromReplayAnalyses(nil)
	if got != nil {
		t.Errorf("expected nil for nil input, got %v", got)
	}
}

func TestFromReplayAnalyses_Empty(t *testing.T) {
	got := FromReplayAnalyses([]*replay.Analysis{})
	if got != nil {
		t.Errorf("expected nil for empty input, got %v", got)
	}
}

func TestFromReplayAnalyses_Multiple(t *testing.T) {
	analyses := []*replay.Analysis{
		{MatchID: "m1", WinnerName: "w1"},
		{MatchID: "m2", WinnerName: "w2"},
	}

	got := FromReplayAnalyses(analyses)

	if len(got) != 2 {
		t.Fatalf("expected 2 summaries, got %d", len(got))
	}
	if got[0].MatchID != "m1" || got[1].MatchID != "m2" {
		t.Errorf("expected match IDs m1, m2, got %v", got)
	}
}

func TestFromMetaDescription_Nil(t *testing.T) {
	got := FromMetaDescription(nil)
	if got.TotalBots != 0 || got.DominantStrategy != "" {
		t.Errorf("expected empty MetaDescription for nil input, got %+v", got)
	}
}

func TestFromMetaDescription_Full(t *testing.T) {
	desc := &meta.Description{
		TotalBots:        42,
		DominantStrategy: "aggressive",
		NashMixture:      "60% aggressive, 40% economy",
		MetaWeaknesses:   []string{"No bots exploring defense", "Low diversity in alpha"},
		TopBots: []meta.BotInfo{
			{Name: "bot1", Rating: 1600, Island: "alpha", Evolved: true},
			{Name: "bot2", Rating: 1500, Island: "beta", Evolved: false},
		},
		IslandStats: map[string]meta.IslandStats{
			"alpha": {Count: 10, AvgFitness: 0.5, TopFitness: 0.9, Diversity: 0.8},
		},
	}

	got := FromMetaDescription(desc)

	if got.TotalBots != 42 {
		t.Errorf("expected TotalBots 42, got %d", got.TotalBots)
	}
	if got.DominantStrategy != "aggressive" {
		t.Errorf("expected DominantStrategy 'aggressive', got %q", got.DominantStrategy)
	}
	if got.NashMixture != "60% aggressive, 40% economy" {
		t.Errorf("expected NashMixture, got %q", got.NashMixture)
	}
	if len(got.MetaWeaknesses) != 2 || got.MetaWeaknesses[0] != "No bots exploring defense" {
		t.Errorf("expected 2 MetaWeaknesses, got %v", got.MetaWeaknesses)
	}
	if len(got.TopBots) != 2 {
		t.Errorf("expected 2 TopBots, got %d", len(got.TopBots))
	}
	if got.TopBots[0].Name != "bot1" || got.TopBots[0].Rating != 1600 {
		t.Errorf("expected bot1 with rating 1600, got %+v", got.TopBots[0])
	}
	if len(got.IslandStats) != 1 {
		t.Errorf("expected 1 IslandStats entry, got %d", len(got.IslandStats))
	}
	if stat, ok := got.IslandStats["alpha"]; !ok {
		t.Error("expected alpha in IslandStats")
	} else if stat.Count != 10 || stat.AvgFitness != 0.5 {
		t.Errorf("expected Count=10, AvgFitness=0.5, got %+v", stat)
	}
}

func TestFromBotInfos_Nil(t *testing.T) {
	got := FromBotInfos(nil)
	if got != nil {
		t.Errorf("expected nil for nil input, got %v", got)
	}
}

func TestFromBotInfos_Empty(t *testing.T) {
	got := FromBotInfos([]meta.BotInfo{})
	if got != nil {
		t.Errorf("expected nil for empty input, got %v", got)
	}
}

func TestFromBotInfos_Multiple(t *testing.T) {
	bots := []meta.BotInfo{
		{Name: "a", Rating: 100, Island: "x", Evolved: true},
		{Name: "b", Rating: 200, Island: "y", Evolved: false},
	}

	got := FromBotInfos(bots)

	if len(got) != 2 {
		t.Fatalf("expected 2 bots, got %d", len(got))
	}
	if got[0].Name != "a" || got[0].Rating != 100 {
		t.Errorf("expected a/100, got %+v", got[0])
	}
	if got[1].Name != "b" || got[1].Rating != 200 {
		t.Errorf("expected b/200, got %+v", got[1])
	}
}

func TestFromIslandStatsMap_Nil(t *testing.T) {
	got := FromIslandStatsMap(nil)
	if got != nil {
		t.Errorf("expected nil for nil input, got %v", got)
	}
}

func TestFromIslandStatsMap_Multiple(t *testing.T) {
	stats := map[string]meta.IslandStats{
		"alpha": {Count: 5, AvgFitness: 0.6, TopFitness: 0.95},
		"beta":  {Count: 3, AvgFitness: 0.4, TopFitness: 0.8},
	}

	got := FromIslandStatsMap(stats)

	if len(got) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(got))
	}
	if got["alpha"].Count != 5 {
		t.Errorf("expected alpha.Count=5, got %d", got["alpha"].Count)
	}
	if got["beta"].AvgFitness != 0.4 {
		t.Errorf("expected beta.AvgFitness=0.4, got %f", got["beta"].AvgFitness)
	}
}

func TestBuildRequest_Full(t *testing.T) {
	parents := []*evolverdb.Program{
		{ID: 1, Code: "code1", Language: "go", Fitness: 0.8},
	}
	analyses := []*replay.Analysis{
		{MatchID: "m1", WinnerName: "w1"},
	}
	metaDesc := &meta.Description{
		TotalBots:        10,
		DominantStrategy: "aggressive",
	}

	req := BuildRequest(parents, analyses, metaDesc, "alpha", "go", 5)

	if len(req.Parents) != 1 {
		t.Errorf("expected 1 parent, got %d", len(req.Parents))
	}
	if len(req.Replays) != 1 {
		t.Errorf("expected 1 replay, got %d", len(req.Replays))
	}
	if req.Meta.TotalBots != 10 {
		t.Errorf("expected Meta.TotalBots=10, got %d", req.Meta.TotalBots)
	}
	if req.Island != "alpha" {
		t.Errorf("expected Island 'alpha', got %q", req.Island)
	}
	if req.TargetLang != "go" {
		t.Errorf("expected TargetLang 'go', got %q", req.TargetLang)
	}
	if req.Generation != 5 {
		t.Errorf("expected Generation 5, got %d", req.Generation)
	}
}

func TestBuildRequest_NilInputs(t *testing.T) {
	req := BuildRequest(nil, nil, nil, "beta", "python", 0)

	if req.Parents != nil {
		t.Errorf("expected nil Parents, got %v", req.Parents)
	}
	if req.Replays != nil {
		t.Errorf("expected nil Replays, got %v", req.Replays)
	}
	if req.Meta.TotalBots != 0 {
		t.Errorf("expected empty Meta, got %+v", req.Meta)
	}
}
