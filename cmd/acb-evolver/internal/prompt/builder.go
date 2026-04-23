// Package prompt assembles evolution prompts for the LLM ensemble.
//
// A prompt is built from three sources:
//   - Parent programs: high-fitness individuals sampled from the island
//     population (typically via tournament selection).
//   - Replay analysis: key moments, strategies, and weaknesses extracted
//     from recent match replays.
//   - Meta description: a snapshot of the current leaderboard and dominant
//     strategies, giving the LLM competitive context.
package prompt

import (
	"fmt"
	"strings"

	evolverdb "github.com/aicodebattle/acb/cmd/acb-evolver/internal/db"
)

// MatchSummary captures the salient facts from a completed match replay.
type MatchSummary struct {
	// MatchID is the unique identifier of the match in the database.
	MatchID string
	// WinnerName is the name of the winning bot (empty for draws).
	WinnerName string
	// LoserName is the name of the losing bot (empty for draws).
	LoserName string
	// Condition is one of "elimination", "dominance", "turns", or "draw".
	Condition string
	// TurnCount is the number of turns played.
	TurnCount int
	// Scores holds the final score for each player slot.
	Scores []int
	// KeyMoments are natural-language sentences describing notable events.
	KeyMoments []string
	// Strategies lists the key tactics observed in the winning side.
	Strategies []string
	// Weaknesses lists exploitable patterns observed in the losing side.
	Weaknesses []string
}

// BotSummary is a brief leaderboard entry.
type BotSummary struct {
	Name    string
	Rating  float64
	Island  string
	Evolved bool
}

// IslandStat summarises a single island's population.
type IslandStat struct {
	Count      int
	AvgFitness float64
	TopFitness float64
}

// MetaDescription captures the current state of the competitive meta.
type MetaDescription struct {
	// TotalBots is the number of registered bots.
	TotalBots int
	// TopBots lists the highest-rated bots in descending order.
	TopBots []BotSummary
	// DominantStrategy is a narrative description of the current meta.
	DominantStrategy string
	// NashMixture describes the current Nash equilibrium mixture over the
	// population (per plan §10.3 PSRO). When non-empty, the candidate should
	// beat this mixture, not just one opponent. Example:
	//   "40% swarm, 30% hunter, 30% gatherer"
	NashMixture string
	// MetaWeaknesses lists known exploitable gaps in the current population.
	MetaWeaknesses []string
	// IslandStats summarises each island's population and fitness.
	IslandStats map[string]IslandStat
}

// CommunityHint is a high-upvote tactical observation from the replay viewer
// (§13.6), consumed by the evolver to ground prompts in player feedback.
type CommunityHint struct {
	MatchID string
	Turn    int
	Type    string // "idea" or "mistake"
	Body    string
	Upvotes int
}

// Request bundles everything the prompt builder needs to produce a prompt.
type Request struct {
	// Parents are the programs selected as evolutionary parents.
	Parents []*evolverdb.Program
	// Replays is the recent match history used for strategy analysis.
	Replays []MatchSummary
	// Meta describes the current competitive landscape.
	Meta MetaDescription
	// CommunityHints are high-upvote tactical insights from §13.6 feedback.
	CommunityHints []CommunityHint
	// Island is the island this candidate will compete on.
	Island string
	// TargetLang is the programming language for the evolved bot
	// (e.g. "go", "python", "rust", "typescript", "java", "php").
	TargetLang string
	// Generation is the current evolution generation number.
	Generation int
	// TaskOverride replaces the default task section when set (used for retry prompts).
	TaskOverride string
}

// Assemble builds the full LLM prompt from a Request.
// The returned string is ready to be sent as the user message to the LLM.
func Assemble(r Request) string {
	var sb strings.Builder

	writeSystemContext(&sb, r.TargetLang)
	writeIslandContext(&sb, r.Island, r.Generation)
	writeMetaSection(&sb, r.Meta)
	writeReplaySection(&sb, r.Replays)
	writeCommunityHintsSection(&sb, r.CommunityHints)
	writeParentSection(&sb, r.Parents)
	if r.TaskOverride != "" {
		sb.WriteString("## Task\n")
		sb.WriteString(r.TaskOverride)
	} else {
		writeTaskSection(&sb, r.TargetLang)
	}

	return sb.String()
}

func writeCommunityHintsSection(sb *strings.Builder, hints []CommunityHint) {
	if len(hints) == 0 {
		return
	}
	sb.WriteString("## Community Tactical Insights (from replay annotations)\n\n")
	for _, h := range hints {
		fmt.Fprintf(sb, "Replay %s, Turn %d (%d upvotes):\n", h.MatchID, h.Turn, h.Upvotes)
		fmt.Fprintf(sb, "%q\n\n", h.Body)
	}
}

func writeSystemContext(sb *strings.Builder, targetLang string) {
	sb.WriteString("You are an AI bot evolution engine for a competitive grid strategy game.\n")
	sb.WriteString("Your task is to write an improved bot strategy in ")
	sb.WriteString(langDisplayName(targetLang))
	sb.WriteString(" based on the parents and match analysis provided.\n\n")

	sb.WriteString("## Game Rules\n")
	sb.WriteString("- Grid: toroidal (wraps horizontally and vertically) with walls, energy pickups, and player cores\n")
	sb.WriteString("- Bots spawn from your core (costs 3 energy). Spawn whenever energy ≥ 3.\n")
	sb.WriteString("- Each turn: move each bot one step (N/E/S/W) or stay. Submit all moves as JSON.\n")
	sb.WriteString("- Combat: focus-fire algorithm. A bot dies if ANY enemy within attack radius²=5 has ≤ its own enemy count.\n")
	sb.WriteString("  - 2v1: the lone bot dies, pair survives. 1v1: both die. Tight formations are defensive.\n")
	sb.WriteString("- Collect energy tiles (uncontested adjacent bots only) to gain energy.\n")
	sb.WriteString("- Win by: sole survivor, dominance (≥80% bots for 100 turns), or highest score at turn 500.\n")
	sb.WriteString("- Vision radius²=49 (~7 tiles). Fog of war: you only see tiles within vision of your bots.\n\n")
	sb.WriteString("## HTTP Protocol\n")
	sb.WriteString("- Your bot is an HTTP server listening on port 8080.\n")
	sb.WriteString("- Engine POSTs game state (JSON) to /turn each turn. You have 3 seconds to respond.\n")
	sb.WriteString("- Response: {\"moves\": [{\"row\":10,\"col\":15,\"direction\":\"N\"}], \"debug\": {...}}\n")
	sb.WriteString("- Headers include HMAC-SHA256 signature: X-ACB-Signature, X-ACB-Match-Id, X-ACB-Turn.\n")
	sb.WriteString("- 10 consecutive failures → bot marked crashed (units hold position for rest of match).\n\n")
}

func writeIslandContext(sb *strings.Builder, island string, generation int) {
	sb.WriteString("## Island Context\n")
	fmt.Fprintf(sb, "Evolving on island **%s** (generation %d).\n", island, generation)
	switch island {
	case evolverdb.IslandAlpha:
		sb.WriteString("Island Alpha favors aggressive, core-rushing strategies.\n")
	case evolverdb.IslandBeta:
		sb.WriteString("Island Beta favors energy-focused, economic strategies.\n")
	case evolverdb.IslandGamma:
		sb.WriteString("Island Gamma favors defensive, adaptive strategies.\n")
	case evolverdb.IslandDelta:
		sb.WriteString("Island Delta is experimental — any novel strategy is welcome.\n")
	}
	sb.WriteString("\n")
}

func writeMetaSection(sb *strings.Builder, meta MetaDescription) {
	if meta.TotalBots == 0 && len(meta.TopBots) == 0 {
		return
	}
	sb.WriteString("## Current Meta\n")
	if meta.TotalBots > 0 {
		fmt.Fprintf(sb, "Total active bots: %d\n", meta.TotalBots)
	}
	if meta.DominantStrategy != "" {
		fmt.Fprintf(sb, "Dominant strategy: %s\n", meta.DominantStrategy)
	}
	// Nash equilibrium mixture per plan §10.3 ("Beat this mix").
	if meta.NashMixture != "" {
		fmt.Fprintf(sb, "Nash equilibrium mixture: %s\n", meta.NashMixture)
		sb.WriteString("Your candidate must beat this mixture, not just one opponent.\n")
	}
	if len(meta.TopBots) > 0 {
		sb.WriteString("\nTop-rated bots:\n")
		for i, bot := range meta.TopBots {
			line := fmt.Sprintf("  %d. %s (rating %.0f", i+1, bot.Name, bot.Rating)
			if bot.Island != "" {
				line += fmt.Sprintf(", island: %s", bot.Island)
			}
			if bot.Evolved {
				line += ", evolved"
			}
			line += ")\n"
			sb.WriteString(line)
		}
	}
	if len(meta.MetaWeaknesses) > 0 {
		sb.WriteString("\nKnown weaknesses in current population:\n")
		for _, w := range meta.MetaWeaknesses {
			fmt.Fprintf(sb, "  - %s\n", w)
		}
	}
	if len(meta.IslandStats) > 0 {
		sb.WriteString("\nIsland population stats:\n")
		for _, island := range evolverdb.AllIslands {
			if stat, ok := meta.IslandStats[island]; ok {
				fmt.Fprintf(sb, "  %s: %d programs, avg fitness %.3f, top fitness %.3f\n",
					island, stat.Count, stat.AvgFitness, stat.TopFitness)
			}
		}
	}
	sb.WriteString("\n")
}

func writeReplaySection(sb *strings.Builder, replays []MatchSummary) {
	if len(replays) == 0 {
		return
	}
	sb.WriteString("## Recent Match Analysis\n")
	for i, m := range replays {
		fmt.Fprintf(sb, "\n### Match %d (ID: %s)\n", i+1, m.MatchID)
		if m.Condition == "draw" || m.WinnerName == "" {
			fmt.Fprintf(sb, "Result: Draw (%d turns)\n", m.TurnCount)
		} else {
			fmt.Fprintf(sb, "Result: %s defeated %s (%s, %d turns)\n",
				m.WinnerName, m.LoserName, m.Condition, m.TurnCount)
		}
		if len(m.Scores) > 0 {
			fmt.Fprintf(sb, "Scores: %v\n", m.Scores)
		}
		if len(m.Strategies) > 0 {
			fmt.Fprintf(sb, "Winning strategies: %s\n", strings.Join(m.Strategies, ", "))
		}
		if len(m.Weaknesses) > 0 {
			fmt.Fprintf(sb, "Exploited weaknesses: %s\n", strings.Join(m.Weaknesses, ", "))
		}
		if len(m.KeyMoments) > 0 {
			sb.WriteString("Key moments:\n")
			for _, moment := range m.KeyMoments {
				sb.WriteString("  - " + moment + "\n")
			}
		}
	}
	sb.WriteString("\n")
}

func writeParentSection(sb *strings.Builder, parents []*evolverdb.Program) {
	if len(parents) == 0 {
		return
	}
	sb.WriteString("## Parent Programs\n")
	sb.WriteString("Study these parents and improve upon them:\n\n")
	for i, p := range parents {
		fmt.Fprintf(sb, "### Parent %d (ID: %d, fitness: %.3f, language: %s)\n",
			i+1, p.ID, p.Fitness, p.Language)
		if len(p.BehaviorVector) >= 4 {
			fmt.Fprintf(sb, "Behavior: aggression=%.2f economy=%.2f exploration=%.2f formation=%.2f\n",
				p.BehaviorVector[0], p.BehaviorVector[1], p.BehaviorVector[2], p.BehaviorVector[3])
		} else if len(p.BehaviorVector) >= 2 {
			fmt.Fprintf(sb, "Behavior: aggression=%.2f economy=%.2f\n",
				p.BehaviorVector[0], p.BehaviorVector[1])
		}
		sb.WriteString("\n```" + p.Language + "\n")
		sb.WriteString(p.Code)
		if !strings.HasSuffix(p.Code, "\n") {
			sb.WriteByte('\n')
		}
		sb.WriteString("```\n\n")
	}
}

func writeTaskSection(sb *strings.Builder, targetLang string) {
	sb.WriteString("## Task\n")
	fmt.Fprintf(sb, "Write an **improved** bot strategy in **%s** that:\n", langDisplayName(targetLang))
	sb.WriteString("1. Addresses the weaknesses and counter-strategies identified in the match analysis.\n")
	sb.WriteString("2. Builds on the best tactical patterns from the parent programs.\n")
	sb.WriteString("3. Can beat the Nash mixture described above (not just one opponent).\n")
	sb.WriteString("4. Is complete and self-contained (define all required game types inline).\n")
	sb.WriteString("5. Fits in a single file under 10 KB.\n\n")
	sb.WriteString("Return **only** the complete bot code in a single fenced code block with no additional explanation:\n")
	sb.WriteString("```" + targetLang + "\n")
	sb.WriteString("// your complete bot code here\n")
	sb.WriteString("```\n")
}

// langDisplayName returns a human-readable name for a language identifier.
func langDisplayName(lang string) string {
	switch lang {
	case "go":
		return "Go"
	case "python":
		return "Python"
	case "rust":
		return "Rust"
	case "typescript":
		return "TypeScript"
	case "java":
		return "Java"
	case "php":
		return "PHP"
	default:
		return lang
	}
}
