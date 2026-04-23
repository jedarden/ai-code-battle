// Package prompt assembles evolution prompts for the LLM ensemble.
package prompt

import (
	evolverdb "github.com/aicodebattle/acb/cmd/acb-evolver/internal/db"
	"github.com/aicodebattle/acb/cmd/acb-evolver/internal/meta"
	"github.com/aicodebattle/acb/cmd/acb-evolver/internal/replay"
)

// FromReplayAnalysis converts a replay.Analysis to a prompt.MatchSummary.
// This allows the prompt builder to consume output from the replay analyzer.
func FromReplayAnalysis(a *replay.Analysis) MatchSummary {
	if a == nil {
		return MatchSummary{}
	}
	return MatchSummary{
		MatchID:    a.MatchID,
		WinnerName: a.WinnerName,
		LoserName:  a.LoserName,
		Condition:  a.Condition,
		TurnCount:  a.TurnCount,
		Scores:     append([]int(nil), a.Scores...), // copy to avoid aliasing
		KeyMoments: append([]string(nil), a.KeyMoments...),
		Strategies: append([]string(nil), a.Strategies...),
		Weaknesses: append([]string(nil), a.Weaknesses...),
	}
}

// FromReplayAnalyses converts multiple replay analyses to match summaries.
func FromReplayAnalyses(analyses []*replay.Analysis) []MatchSummary {
	if len(analyses) == 0 {
		return nil
	}
	summaries := make([]MatchSummary, len(analyses))
	for i, a := range analyses {
		summaries[i] = FromReplayAnalysis(a)
	}
	return summaries
}

// FromMetaDescription converts a meta.Description to a prompt.MetaDescription.
// This allows the prompt builder to consume output from the meta builder.
func FromMetaDescription(d *meta.Description) MetaDescription {
	if d == nil {
		return MetaDescription{}
	}
	return MetaDescription{
		TotalBots:        d.TotalBots,
		DominantStrategy: d.DominantStrategy,
		NashMixture:      d.NashMixture,
		MetaWeaknesses:   append([]string(nil), d.MetaWeaknesses...),
		TopBots:          FromBotInfos(d.TopBots),
		IslandStats:      FromIslandStatsMap(d.IslandStats),
	}
}

// FromBotInfos converts meta.BotInfo slices to prompt.BotSummary slices.
func FromBotInfos(bots []meta.BotInfo) []BotSummary {
	if len(bots) == 0 {
		return nil
	}
	result := make([]BotSummary, len(bots))
	for i, b := range bots {
		result[i] = BotSummary{
			Name:    b.Name,
			Rating:  b.Rating,
			Island:  b.Island,
			Evolved: b.Evolved,
		}
	}
	return result
}

// FromIslandStatsMap converts meta island stats to prompt island stats.
func FromIslandStatsMap(stats map[string]meta.IslandStats) map[string]IslandStat {
	if stats == nil {
		return nil
	}
	result := make(map[string]IslandStat, len(stats))
	for k, v := range stats {
		result[k] = IslandStat{
			Count:      v.Count,
			AvgFitness: v.AvgFitness,
			TopFitness: v.TopFitness,
		}
	}
	return result
}

// BuildRequest is a convenience function that assembles a prompt.Request
// from the standard evolution pipeline components.
//
// Parameters:
//   - parents: programs selected as evolutionary parents (from tournament selection)
//   - analyses: replay analyses from recent matches
//   - metaDesc: meta-game description from the meta builder
//   - island: target island for the new candidate
//   - targetLang: programming language for the evolved bot
//   - generation: current evolution generation number
func BuildRequest(
	parents []*evolverdb.Program,
	analyses []*replay.Analysis,
	metaDesc *meta.Description,
	island string,
	targetLang string,
	generation int,
) Request {
	return Request{
		Parents:     parents,
		Replays:     FromReplayAnalyses(analyses),
		Meta:        FromMetaDescription(metaDesc),
		Island:      island,
		TargetLang:  targetLang,
		Generation:  generation,
	}
}
