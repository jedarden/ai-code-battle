package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// StoryArcType represents the type of narrative arc
type StoryArcType string

const (
	ArcRise               StoryArcType = "rise"
	ArcFall               StoryArcType = "fall"
	ArcRivalry            StoryArcType = "rivalry"
	ArcUpset              StoryArcType = "upset"
	ArcEvolutionMilestone StoryArcType = "evolution"
	ArcComeback           StoryArcType = "comeback"
	ArcSeasonRecap        StoryArcType = "season-recap"
)

// StoryArc represents a detected narrative arc
type StoryArc struct {
	Type        StoryArcType `json:"type"`
	BotID       string       `json:"bot_id,omitempty"`
	BotName     string       `json:"bot_name,omitempty"`
	BotBID      string       `json:"bot_b_id,omitempty"`
	BotBName    string       `json:"bot_b_name,omitempty"`
	RatingStart int          `json:"rating_start,omitempty"`
	RatingEnd   int          `json:"rating_end,omitempty"`
	MatchID     string       `json:"match_id,omitempty"`
	SeasonName  string       `json:"season_name,omitempty"`

	// Context for LLM prompt
	KeyMatches    []KeyMatch `json:"key_matches,omitempty"`
	Archetype     string     `json:"archetype,omitempty"`
	Origin        string     `json:"origin,omitempty"`
	ParentIDs     []string   `json:"parent_ids,omitempty"`
	Generation    int        `json:"generation,omitempty"`
	CommunityHint string     `json:"community_hint,omitempty"`

	// Rivalry-specific fields
	BotAWins     int `json:"bot_a_wins,omitempty"`
	BotBWins     int `json:"bot_b_wins,omitempty"`
	TotalMatches int `json:"total_matches,omitempty"`
}

// KeyMatch represents a key match for narrative context
type KeyMatch struct {
	MatchID        string `json:"match_id"`
	OpponentID     string `json:"opponent_id"`
	OpponentName   string `json:"opponent_name"`
	OpponentRating int    `json:"opponent_rating"`
	OpponentRank   int    `json:"opponent_rank,omitempty"`
	MapName        string `json:"map_name,omitempty"`
	Score          string `json:"score"`
	TurnCount      int    `json:"turn_count"`
	Won            bool   `json:"won"`
	EndCondition   string `json:"end_condition,omitempty"`
	CriticalMoment string `json:"critical_moment,omitempty"` // §13.2 turning point summary
}

// HeadToHeadRecord represents the head-to-head record between two bots
type HeadToHeadRecord struct {
	OpponentName string `json:"opponent_name"`
	OpponentRank int    `json:"opponent_rank,omitempty"`
	Wins         int    `json:"wins"`
	Losses       int    `json:"losses"`
	TotalMatches int    `json:"total_matches"`
}

// LLMClient handles narrative generation via LLM
type LLMClient struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// NewLLMClient creates a new LLM client for narrative generation
func NewLLMClient(baseURL, apiKey string) *LLMClient {
	return &LLMClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// NarrativeRequest contains context for generating a narrative
type NarrativeRequest struct {
	ArcType     StoryArcType
	BotName     string
	BotID       string
	SeasonName  string
	SeasonTheme string
	RatingStart int
	RatingEnd   int
	KeyMatches  []KeyMatch
	Archetype   string
	Origin      string
	ParentIDs   []string
	Generation  int
	// Enriched context per §15.5
	BotRank       int
	CommunityHint string
	HeadToHead    []HeadToHeadRecord
	// Rivalry-specific fields
	BotBName     string
	BotAWins     int
	BotBWins     int
	TotalMatches int
}

// GenerateNarrative generates a 200-word sports-journalism narrative
func (c *LLMClient) GenerateNarrative(ctx context.Context, req NarrativeRequest) (headline, narrative string, err error) {
	prompt := buildNarrativePrompt(req)

	response, err := c.chatCompletion(ctx, prompt)
	if err != nil {
		return "", "", fmt.Errorf("llm request: %w", err)
	}

	// Parse response - first line is headline, rest is narrative
	lines := strings.Split(strings.TrimSpace(response), "\n")
	if len(lines) < 2 {
		return "AI Code Battle Chronicle", response, nil
	}

	headline = strings.TrimPrefix(lines[0], "# ")
	headline = strings.TrimSpace(headline)
	narrative = strings.Join(lines[1:], "\n")
	narrative = strings.TrimSpace(narrative)

	return headline, narrative, nil
}

// buildNarrativePrompt constructs a sports-journalism prompt per plan §15.5,
// injecting rivalry context, ELO before/after, critical moments from §13.2,
// season standings, and head-to-head stats.
func buildNarrativePrompt(req NarrativeRequest) string {
	var sb strings.Builder

	// §15.5 instruction: sports-journalism narrative with structured contextual match data
	sb.WriteString("Write a 200-word sports-journalism narrative about this event in the AI Code Battle platform. ")
	sb.WriteString("You are a sports journalist covering an emergent bot league — write with the energy and specificity of esports commentary. ")
	sb.WriteString("Be dramatic but factual. Reference specific matches by ID, ELO before/after deltas, rivalry context, head-to-head records, critical turning points, and season standings. ")
	sb.WriteString("Weave the data into a compelling story — quote scores, cite map names, describe the strategic moments that defined the outcome. ")
	sb.WriteString("Write in present tense with a punchy, journalistic tone. Do not use emojis.\n\n")

	// Season and standings context
	seasonLabel := req.SeasonName
	if req.SeasonTheme != "" {
		seasonLabel = fmt.Sprintf("%s (%s)", req.SeasonName, req.SeasonTheme)
	}

	switch req.ArcType {
	case ArcRise:
		sb.WriteString("Arc type: Rise\n")
		sb.WriteString(fmt.Sprintf("Bot: %s\n", req.BotName))
		sb.WriteString(fmt.Sprintf("Season: %s\n", seasonLabel))
		if req.BotRank > 0 {
			sb.WriteString(fmt.Sprintf("Current rank: #%d\n", req.BotRank))
		}
		delta := req.RatingEnd - req.RatingStart
		sb.WriteString(fmt.Sprintf("ELO: %d → %d (delta %+d) over 7 days\n", req.RatingStart, req.RatingEnd, delta))
		if req.Archetype != "" {
			sb.WriteString(fmt.Sprintf("Archetype: %s\n", req.Archetype))
		}
		if req.Origin != "" {
			sb.WriteString(fmt.Sprintf("Origin: %s\n", req.Origin))
		}
		if req.Generation > 0 && len(req.ParentIDs) > 0 {
			sb.WriteString(fmt.Sprintf("Lineage: generation %d, parents: %s\n", req.Generation, strings.Join(req.ParentIDs, ", ")))
		}
		if req.CommunityHint != "" {
			sb.WriteString(fmt.Sprintf("Community tactical hint: %s\n", req.CommunityHint))
		}
		if len(req.KeyMatches) > 0 {
			sb.WriteString("Critical moments (turning points in the climb):\n")
			for _, m := range req.KeyMatches {
				outcome := "Lost to"
				if m.Won {
					outcome = "Beat"
				}
				rankStr := ""
				if m.OpponentRank > 0 {
					rankStr = fmt.Sprintf(", #%d", m.OpponentRank)
				}
				condStr := ""
				if m.EndCondition != "" {
					condStr = fmt.Sprintf(" [%s]", m.EndCondition)
				}
				sb.WriteString(fmt.Sprintf("  - %s %s (ELO %d%s) on \"%s\" — score %s, %d turns%s. Match ID: %s\n",
					outcome, m.OpponentName, m.OpponentRating, rankStr, nonEmpty(m.MapName, "standard map"), m.Score, m.TurnCount, condStr, m.MatchID))
				if m.CriticalMoment != "" {
					sb.WriteString(fmt.Sprintf("    Turning point: %s\n", m.CriticalMoment))
				}
			}
		}
		if len(req.HeadToHead) > 0 {
			sb.WriteString("Head-to-head records (season context):\n")
			for _, h := range req.HeadToHead {
				rankStr := ""
				if h.OpponentRank > 0 {
					rankStr = fmt.Sprintf(", ranked #%d", h.OpponentRank)
				}
				sb.WriteString(fmt.Sprintf("  vs %s%s: %dW-%dL (%d matches)\n", h.OpponentName, rankStr, h.Wins, h.Losses, h.TotalMatches))
			}
		}

	case ArcFall:
		sb.WriteString("Arc type: Fall\n")
		sb.WriteString(fmt.Sprintf("Bot: %s\n", req.BotName))
		sb.WriteString(fmt.Sprintf("Season: %s\n", seasonLabel))
		if req.BotRank > 0 {
			sb.WriteString(fmt.Sprintf("Current rank: #%d\n", req.BotRank))
		}
		delta := req.RatingStart - req.RatingEnd
		sb.WriteString(fmt.Sprintf("ELO: %d → %d (dropped %d points) over 7 days\n", req.RatingStart, req.RatingEnd, delta))
		if req.Archetype != "" {
			sb.WriteString(fmt.Sprintf("Archetype: %s\n", req.Archetype))
		}
		if len(req.KeyMatches) > 0 {
			sb.WriteString("Critical losses (turning points in the decline):\n")
			for _, m := range req.KeyMatches {
				outcome := "Lost to"
				if m.Won {
					outcome = "Beat"
				}
				rankStr := ""
				if m.OpponentRank > 0 {
					rankStr = fmt.Sprintf(", #%d", m.OpponentRank)
				}
				condStr := ""
				if m.EndCondition != "" {
					condStr = fmt.Sprintf(" [%s]", m.EndCondition)
				}
				sb.WriteString(fmt.Sprintf("  - %s %s (ELO %d%s) on \"%s\" — score %s, %d turns%s. Match ID: %s\n",
					outcome, m.OpponentName, m.OpponentRating, rankStr, nonEmpty(m.MapName, "standard map"), m.Score, m.TurnCount, condStr, m.MatchID))
				if m.CriticalMoment != "" {
					sb.WriteString(fmt.Sprintf("    Turning point: %s\n", m.CriticalMoment))
				}
			}
		}
		if len(req.HeadToHead) > 0 {
			sb.WriteString("Head-to-head records (season context):\n")
			for _, h := range req.HeadToHead {
				rankStr := ""
				if h.OpponentRank > 0 {
					rankStr = fmt.Sprintf(", ranked #%d", h.OpponentRank)
				}
				sb.WriteString(fmt.Sprintf("  vs %s%s: %dW-%dL (%d matches)\n", h.OpponentName, rankStr, h.Wins, h.Losses, h.TotalMatches))
			}
		}

	case ArcRivalry:
		sb.WriteString("Arc type: Rivalry Intensifies\n")
		sb.WriteString(fmt.Sprintf("Bot A: %s\n", req.BotName))
		sb.WriteString(fmt.Sprintf("Bot B: %s\n", req.BotBName))
		sb.WriteString(fmt.Sprintf("Season: %s\n", seasonLabel))
		sb.WriteString(fmt.Sprintf("Head-to-head: %d-%d over %d matches\n", req.BotAWins, req.BotBWins, req.TotalMatches))
		if req.RatingStart > 0 || req.RatingEnd > 0 {
			sb.WriteString(fmt.Sprintf("ELO context: %s at %d, %s at %d\n", req.BotName, req.RatingStart, req.BotBName, req.RatingEnd))
		}
		if len(req.KeyMatches) > 0 {
			sb.WriteString("Recent encounters (critical moments):\n")
			for _, m := range req.KeyMatches {
				winner := req.BotBName
				if m.Won {
					winner = req.BotName
				}
				condStr := ""
				if m.EndCondition != "" {
					condStr = fmt.Sprintf(" [%s]", m.EndCondition)
				}
				sb.WriteString(fmt.Sprintf("  - %s won on \"%s\" — score %s, %d turns, opponent ELO %d%s. Match ID: %s\n",
					winner, nonEmpty(m.MapName, "standard map"), m.Score, m.TurnCount, m.OpponentRating, condStr, m.MatchID))
				if m.CriticalMoment != "" {
					sb.WriteString(fmt.Sprintf("    Turning point: %s\n", m.CriticalMoment))
				}
			}
		}
		if len(req.HeadToHead) > 0 {
			sb.WriteString("All-time head-to-head:\n")
			for _, h := range req.HeadToHead {
				sb.WriteString(fmt.Sprintf("  vs %s: %dW-%dL (%d matches)\n",
					h.OpponentName, h.Wins, h.Losses, h.TotalMatches))
			}
		}

	case ArcUpset:
		sb.WriteString("Arc type: Upset of the Week\n")
		sb.WriteString(fmt.Sprintf("Underdog: %s\n", req.BotName))
		sb.WriteString(fmt.Sprintf("Favorite: %s\n", req.BotBName))
		sb.WriteString(fmt.Sprintf("Season: %s\n", seasonLabel))
		eloDelta := req.RatingEnd - req.RatingStart
		sb.WriteString(fmt.Sprintf("ELO gap: %d (underdog %d vs favorite %d)\n", eloDelta, req.RatingStart, req.RatingEnd))
		if len(req.KeyMatches) > 0 {
			m := req.KeyMatches[0]
			sb.WriteString(fmt.Sprintf("Match: %s upset %s on \"%s\" — score %s, %d turns. Match ID: %s\n",
				req.BotName, req.BotBName, nonEmpty(m.MapName, "standard map"), m.Score, m.TurnCount, m.MatchID))
			if m.CriticalMoment != "" {
				sb.WriteString(fmt.Sprintf("  Turning point: %s\n", m.CriticalMoment))
			}
		}

	case ArcEvolutionMilestone:
		sb.WriteString("Arc type: Evolution Milestone\n")
		sb.WriteString(fmt.Sprintf("Bot: %s\n", req.BotName))
		sb.WriteString(fmt.Sprintf("Season: %s\n", seasonLabel))
		if req.BotRank > 0 {
			sb.WriteString(fmt.Sprintf("Current rank: #%d\n", req.BotRank))
		}
		sb.WriteString(fmt.Sprintf("ELO: %d\n", req.RatingEnd))
		if req.Archetype != "" {
			sb.WriteString(fmt.Sprintf("Archetype: %s\n", req.Archetype))
		}
		if req.Origin != "" {
			sb.WriteString(fmt.Sprintf("Origin: %s\n", req.Origin))
		}
		if req.Generation > 0 {
			sb.WriteString(fmt.Sprintf("generation %d\n", req.Generation))
		}
		if len(req.ParentIDs) > 0 {
			sb.WriteString(fmt.Sprintf("Parents: %s\n", strings.Join(req.ParentIDs, ", ")))
		}
		if req.CommunityHint != "" {
			sb.WriteString(fmt.Sprintf("Community tactical hint that influenced it: %s\n", req.CommunityHint))
		}
		if len(req.KeyMatches) > 0 {
			sb.WriteString("Key matches in the breakthrough:\n")
			for _, m := range req.KeyMatches {
				outcome := "Lost to"
				if m.Won {
					outcome = "Beat"
				}
				sb.WriteString(fmt.Sprintf("  - %s %s (ELO %d) — score %s, %d turns. Match ID: %s\n",
					outcome, m.OpponentName, m.OpponentRating, m.Score, m.TurnCount, m.MatchID))
				if m.CriticalMoment != "" {
					sb.WriteString(fmt.Sprintf("    Turning point: %s\n", m.CriticalMoment))
				}
			}
		}

	case ArcComeback:
		sb.WriteString("Arc type: Comeback\n")
		sb.WriteString(fmt.Sprintf("Bot: %s\n", req.BotName))
		sb.WriteString(fmt.Sprintf("Season: %s\n", seasonLabel))
		if req.BotRank > 0 {
			sb.WriteString(fmt.Sprintf("Current rank: #%d\n", req.BotRank))
		}
		sb.WriteString("Story: Climbed from bottom 25%% of leaderboard to top 25%%\n")
		sb.WriteString(fmt.Sprintf("Current ELO: %d\n", req.RatingEnd))
		if req.Archetype != "" {
			sb.WriteString(fmt.Sprintf("Archetype: %s\n", req.Archetype))
		}
		if len(req.KeyMatches) > 0 {
			sb.WriteString("Key matches in the comeback:\n")
			for _, m := range req.KeyMatches {
				outcome := "Lost to"
				if m.Won {
					outcome = "Beat"
				}
				rankStr := ""
				if m.OpponentRank > 0 {
					rankStr = fmt.Sprintf(", #%d", m.OpponentRank)
				}
				sb.WriteString(fmt.Sprintf("  - %s %s (ELO %d%s) — score %s, %d turns. Match ID: %s\n",
					outcome, m.OpponentName, m.OpponentRating, rankStr, m.Score, m.TurnCount, m.MatchID))
				if m.CriticalMoment != "" {
					sb.WriteString(fmt.Sprintf("    Turning point: %s\n", m.CriticalMoment))
				}
			}
		}
		if len(req.HeadToHead) > 0 {
			sb.WriteString("Head-to-head records (season context):\n")
			for _, h := range req.HeadToHead {
				rankStr := ""
				if h.OpponentRank > 0 {
					rankStr = fmt.Sprintf(", ranked #%d", h.OpponentRank)
				}
				sb.WriteString(fmt.Sprintf("  vs %s%s: %dW-%dL (%d matches)\n", h.OpponentName, rankStr, h.Wins, h.Losses, h.TotalMatches))
			}
		}

	case ArcSeasonRecap:
		sb.WriteString("Arc type: Season Narrative\n")
		sb.WriteString(fmt.Sprintf("Season: %s\n", seasonLabel))
		if req.BotName != "" {
			sb.WriteString(fmt.Sprintf("Champion: %s\n", req.BotName))
		}
	}

	return sb.String()
}

// systemPromptSportsJournalist frames the LLM as a sports journalist covering AI Code Battle.
// Per plan §15.1 and §15.5, this produces sports-journalism-style output with structured
// contextual match data including rivalry context, ELO deltas, critical moments, season stakes.
const systemPromptSportsJournalist = `You are a sports journalist covering an emergent bot league called AI Code Battle, where autonomous programs compete in grid-based strategy matches. Write with the energy and narrative instinct of esports journalism — dramatic but factual, specific but accessible.

Your coverage style:
- Reference bots by name, cite ELO ratings with before/after deltas, and describe strategic turning points the way a play-by-play commentator would.
- Weave in rivalry context, head-to-head records, season standings, and critical moments from match data.
- Describe ELO shifts the way a power rankings columnist describes team movement — "surged 200 points" not "increased."
- Use present tense. Keep paragraphs tight and punchy. Do not use emojis.
- When lineage or evolution data is provided, frame it like a scouting report — origin story, parent strategies, behavioral archetype.
- Always ground narrative in the specific match data, scores, and ratings provided — never fabricate match details.
- Keep narratives to 200 words.`

// chatCompletion sends a prompt to the LLM API and returns the completion text.
// Per §15.1/§15.5, uses systemPromptSportsJournalist to frame the output.
func (c *LLMClient) chatCompletion(ctx context.Context, prompt string) (string, error) {
	reqBody := struct {
		Model    string `json:"model"`
		Messages []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
		MaxTokens   int     `json:"max_tokens"`
		Temperature float64 `json:"temperature"`
	}{
		Model: "gpt-4o-mini",
		Messages: []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		}{
			{Role: "system", Content: systemPromptSportsJournalist},
			{Role: "user", Content: prompt},
		},
		MaxTokens:   1024,
		Temperature: 0.7,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/v1/chat/completions", strings.NewReader(string(bodyBytes)))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("llm request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("llm api returned status %d", resp.StatusCode)
	}

	var chatResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}
	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("no choices in response")
	}
	return strings.TrimSpace(chatResp.Choices[0].Message.Content), nil
}

// detectStoryArcs scans index data for active story arcs per §15.5.
func detectStoryArcs(data *IndexData) []StoryArc {
	arcs := make([]StoryArc, 0)
	arcs = append(arcs, detectRiseArcs(data)...)
	arcs = append(arcs, detectFallArcs(data)...)
	arcs = append(arcs, detectRivalryArcs(data)...)
	arcs = append(arcs, detectUpsetArcs(data)...)
	arcs = append(arcs, detectEvolutionArcs(data)...)
	arcs = append(arcs, detectComebackArcs(data)...)
	return arcs
}

func detectRiseArcs(data *IndexData) []StoryArc {
	arcs := make([]StoryArc, 0)

	for _, bot := range data.Bots {
		history := getBotRatingHistory(bot.ID, data)
		if len(history) < 2 {
			continue
		}

		weekAgo := data.GeneratedAt.AddDate(0, 0, -7)
		var oldRating float64
		var foundOld bool
		for _, rh := range history {
			if rh.RecordedAt.Before(weekAgo) || rh.RecordedAt.Equal(weekAgo) {
				oldRating = rh.Rating
				foundOld = true
			}
		}
		if !foundOld {
			continue
		}

		delta := bot.Rating - oldRating
		if delta >= 200 {
			arcs = append(arcs, StoryArc{
				Type:        ArcRise,
				BotID:       bot.ID,
				BotName:     bot.Name,
				RatingStart: int(oldRating),
				RatingEnd:   int(bot.Rating),
				Archetype:   bot.Archetype,
				Origin:      buildOriginString(bot),
				ParentIDs:   bot.ParentIDs,
				Generation:  bot.Generation,
				KeyMatches:  extractKeyMatches(bot.ID, data),
			})
		}
	}

	return arcs
}

func detectFallArcs(data *IndexData) []StoryArc {
	arcs := make([]StoryArc, 0)

	for _, bot := range data.Bots {
		history := getBotRatingHistory(bot.ID, data)
		if len(history) < 2 {
			continue
		}

		weekAgo := data.GeneratedAt.AddDate(0, 0, -7)
		var oldRating float64
		var foundOld bool
		for _, rh := range history {
			if rh.RecordedAt.Before(weekAgo) || rh.RecordedAt.Equal(weekAgo) {
				oldRating = rh.Rating
				foundOld = true
			}
		}
		if !foundOld {
			continue
		}

		delta := oldRating - bot.Rating
		if delta >= 200 {
			arcs = append(arcs, StoryArc{
				Type:        ArcFall,
				BotID:       bot.ID,
				BotName:     bot.Name,
				RatingStart: int(oldRating),
				RatingEnd:   int(bot.Rating),
				Archetype:   bot.Archetype,
				KeyMatches:  extractKeyMatches(bot.ID, data),
			})
		}
	}

	return arcs
}

func detectRivalryArcs(data *IndexData) []StoryArc {
	arcs := make([]StoryArc, 0)

	pairData := make(map[string]*struct {
		botAID, botBID string
		aWins, bWins   int
		total          int
	})

	for _, m := range data.Matches {
		if len(m.Participants) < 2 {
			continue
		}
		for i, p1 := range m.Participants {
			for _, p2 := range m.Participants[i+1:] {
				key := minStr(p1.BotID, p2.BotID) + "-" + maxStr(p1.BotID, p2.BotID)
				aID := minStr(p1.BotID, p2.BotID)
				bID := maxStr(p1.BotID, p2.BotID)

				if pairData[key] == nil {
					pairData[key] = &struct {
						botAID, botBID string
						aWins, bWins   int
						total          int
					}{botAID: aID, botBID: bID}
				}
				pairData[key].total++
				if p1.Won {
					if p1.BotID == aID {
						pairData[key].aWins++
					} else {
						pairData[key].bWins++
					}
				} else if p2.Won {
					if p2.BotID == aID {
						pairData[key].aWins++
					} else {
						pairData[key].bWins++
					}
				}
			}
		}
	}

	for _, pd := range pairData {
		// Grudge matches: 10+ total meetings
		if pd.total >= 10 && pd.aWins >= 2 && pd.bWins >= 2 {
			arcs = append(arcs, StoryArc{
				Type:         ArcRivalry,
				BotID:        pd.botAID,
				BotName:      getBotName(pd.botAID, data),
				BotBID:       pd.botBID,
				BotBName:     getBotName(pd.botBID, data),
				BotAWins:     pd.aWins,
				BotBWins:     pd.bWins,
				TotalMatches: pd.total,
				KeyMatches:   extractRivalryMatches(pd.botAID, pd.botBID, data),
			})
		}
	}

	return arcs
}

func detectUpsetArcs(data *IndexData) []StoryArc {
	arcs := make([]StoryArc, 0)

	if len(data.Bots) < 10 {
		return arcs
	}

	// Get current top 10 and bottom 10 bot IDs
	top10 := make(map[string]bool, 10)
	bottom10 := make(map[string]bool, 10)

	for i := 0; i < 10 && i < len(data.Bots); i++ {
		top10[data.Bots[i].ID] = true
	}
	for i := len(data.Bots) - 10; i < len(data.Bots); i++ {
		if i >= 0 {
			bottom10[data.Bots[i].ID] = true
		}
	}

	// Find matches where bottom-10 beat top-10
	for _, m := range data.Matches {
		if len(m.Participants) < 2 {
			continue
		}

		var winner, loser *ParticipantData
		for i := range m.Participants {
			if m.Participants[i].Won {
				winner = &m.Participants[i]
			} else {
				loser = &m.Participants[i]
			}
		}

		if winner == nil || loser == nil {
			continue
		}

		// Check if winner was bottom-10 and loser was top-10
		if bottom10[winner.BotID] && top10[loser.BotID] {
			arcs = append(arcs, StoryArc{
				Type:        ArcUpset,
				BotID:       winner.BotID,
				BotName:     getBotName(winner.BotID, data),
				BotBID:      loser.BotID,
				BotBName:    getBotName(loser.BotID, data),
				RatingStart: int(winner.PreMatchRating),
				RatingEnd:   int(loser.PreMatchRating),
				MatchID:     m.ID,
				KeyMatches: []KeyMatch{{
					MatchID:        m.ID,
					OpponentID:     loser.BotID,
					OpponentName:   getBotName(loser.BotID, data),
					OpponentRating: int(loser.PreMatchRating),
					OpponentRank:   getBotRank(loser.BotID, data),
					MapName:        m.MapName,
					Score:          fmt.Sprintf("%d-%d", winner.Score, loser.Score),
					TurnCount:      m.TurnCount,
					Won:            true,
					EndCondition:   m.EndCondition,
					CriticalMoment: summarizeCriticalMoment(m, winner, loser),
				}},
			})
		}
	}

	return arcs
}

func detectEvolutionArcs(data *IndexData) []StoryArc {
	arcs := make([]StoryArc, 0)

	for _, bot := range data.Bots {
		if !bot.Evolved {
			continue
		}

		var previousATH float64
		for _, rh := range getBotRatingHistory(bot.ID, data) {
			if rh.Rating > previousATH && rh.RecordedAt.Before(data.GeneratedAt.AddDate(0, 0, -1)) {
				previousATH = rh.Rating
			}
		}

		if bot.Rating > previousATH+50 {
			arcs = append(arcs, StoryArc{
				Type:       ArcEvolutionMilestone,
				BotID:      bot.ID,
				BotName:    bot.Name,
				RatingEnd:  int(bot.Rating),
				Origin:     fmt.Sprintf("evolved, %s island", bot.Island),
				Generation: bot.Generation,
				ParentIDs:  bot.ParentIDs,
				Archetype:  bot.Archetype,
				KeyMatches: extractKeyMatches(bot.ID, data),
			})
		}

		rank := getBotRank(bot.ID, data)
		if rank > 0 && rank <= 5 {
			arcs = append(arcs, StoryArc{
				Type:       ArcEvolutionMilestone,
				BotID:      bot.ID,
				BotName:    bot.Name,
				RatingEnd:  int(bot.Rating),
				Origin:     fmt.Sprintf("evolved, %s island, generation %d", bot.Island, bot.Generation),
				Generation: bot.Generation,
				ParentIDs:  bot.ParentIDs,
				Archetype:  bot.Archetype,
			})
		}
	}

	return arcs
}

func detectComebackArcs(data *IndexData) []StoryArc {
	arcs := make([]StoryArc, 0)

	if len(data.Bots) < 4 {
		return arcs
	}

	// Calculate quartile thresholds
	bottomQuartileCutoff := len(data.Bots) / 4
	topQuartileStart := len(data.Bots) - (len(data.Bots) / 4)

	for _, bot := range data.Bots {
		currentRank := getBotRank(bot.ID, data)
		if currentRank == 0 {
			continue
		}

		// Check if bot is currently in top 25%
		if currentRank < topQuartileStart {
			// Look at rating history to see if bot was ever in bottom 25%
			history := getBotRatingHistory(bot.ID, data)
			if len(history) < 2 {
				continue
			}

			// Check if bot was in bottom 25% at some point in the past 30 days
			monthAgo := data.GeneratedAt.AddDate(0, 0, -30)
			var wasInBottomQuartile bool

			for _, rh := range history {
				if rh.RecordedAt.Before(monthAgo) {
					continue
				}
				// Estimate historical rank by comparing rating to others at that time
				// This is an approximation - we count how many current bots had higher ratings then
				lowerCount := 0
				for _, otherBot := range data.Bots {
					if otherBot.ID == bot.ID {
						continue
					}
					otherHistory := getBotRatingHistory(otherBot.ID, data)
					var otherRatingAtTime float64
					for _, orh := range otherHistory {
						if orh.RecordedAt.Before(rh.RecordedAt) || orh.RecordedAt.Equal(rh.RecordedAt) {
							otherRatingAtTime = orh.Rating
						}
					}
					if otherRatingAtTime > rh.Rating {
						lowerCount++
					}
				}
				historicalRank := lowerCount + 1
				if historicalRank > bottomQuartileCutoff {
					wasInBottomQuartile = true
					break
				}
			}

			if wasInBottomQuartile {
				arcs = append(arcs, StoryArc{
					Type:        ArcComeback,
					BotID:       bot.ID,
					BotName:     bot.Name,
					RatingStart: 0, // Was in bottom quartile
					RatingEnd:   int(bot.Rating),
					KeyMatches:  extractKeyMatches(bot.ID, data),
				})
			}
		}
	}

	return arcs
}

func extractKeyMatches(botID string, data *IndexData) []KeyMatch {
	matches := make([]KeyMatch, 0, 3)

	for _, m := range data.Matches {
		var botPart *ParticipantData
		var oppPart *ParticipantData

		for i := range m.Participants {
			if m.Participants[i].BotID == botID {
				botPart = &m.Participants[i]
			} else {
				oppPart = &m.Participants[i]
			}
		}

		if botPart == nil || oppPart == nil {
			continue
		}

		matches = append(matches, KeyMatch{
			MatchID:        m.ID,
			OpponentID:     oppPart.BotID,
			OpponentName:   getBotName(oppPart.BotID, data),
			OpponentRating: int(oppPart.PreMatchRating),
			OpponentRank:   getBotRank(oppPart.BotID, data),
			MapName:        m.MapName,
			Score:          fmt.Sprintf("%d-%d", botPart.Score, oppPart.Score),
			TurnCount:      m.TurnCount,
			Won:            botPart.Won,
			EndCondition:   m.EndCondition,
			CriticalMoment: summarizeCriticalMoment(m, botPart, oppPart),
		})

		if len(matches) >= 3 {
			break
		}
	}

	return matches
}

func extractRivalryMatches(botAID, botBID string, data *IndexData) []KeyMatch {
	matches := make([]KeyMatch, 0, 5)

	for _, m := range data.Matches {
		var botAPart, botBPart *ParticipantData

		for i := range m.Participants {
			if m.Participants[i].BotID == botAID {
				botAPart = &m.Participants[i]
			} else if m.Participants[i].BotID == botBID {
				botBPart = &m.Participants[i]
			}
		}

		if botAPart == nil || botBPart == nil {
			continue
		}

		matches = append(matches, KeyMatch{
			MatchID:        m.ID,
			OpponentID:     botBID,
			OpponentName:   getBotName(botBID, data),
			OpponentRating: int(botBPart.PreMatchRating),
			MapName:        m.MapName,
			Score:          fmt.Sprintf("%d-%d", botAPart.Score, botBPart.Score),
			TurnCount:      m.TurnCount,
			Won:            botAPart.Won,
			CriticalMoment: summarizeCriticalMoment(m, botAPart, botBPart),
		})

		if len(matches) >= 5 {
			break
		}
	}

	return matches
}

// summarizeCriticalMoment generates a brief turning-point description from
// match data per plan §13.2.
func summarizeCriticalMoment(m MatchData, winner, loser *ParticipantData) string {
	scoreDelta := winner.Score - loser.Score
	if scoreDelta < 0 {
		scoreDelta = -scoreDelta
	}

	parts := make([]string, 0, 3)

	if scoreDelta <= 1 {
		parts = append(parts, "decided by a single point")
	}

	if winner.PreMatchRating > 0 && loser.PreMatchRating > 0 {
		eloDelta := loser.PreMatchRating - winner.PreMatchRating
		if eloDelta >= 150 {
			parts = append(parts, fmt.Sprintf("upset by %.0f ELO points", eloDelta))
		}
	}

	if m.EndCondition != "" && m.EndCondition != "turn_limit" {
		parts = append(parts, m.EndCondition)
	}

	if m.TurnCount >= 400 {
		parts = append(parts, "marathon match")
	}

	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, ", ")
}

func getBotRank(botID string, data *IndexData) int {
	for i, bot := range data.Bots {
		if bot.ID == botID {
			return i + 1
		}
	}
	return 0
}

func buildHeadToHeadFromArc(arc StoryArc, data *IndexData) []HeadToHeadRecord {
	if arc.BotID == "" {
		return nil
	}

	type wl struct{ wins, losses int }
	recordMap := make(map[string]*wl)

	for _, m := range data.Matches {
		var botIn, opponentIn bool
		var opponentID string
		for _, p := range m.Participants {
			if p.BotID == arc.BotID {
				botIn = true
			} else {
				opponentIn = true
				opponentID = p.BotID
			}
		}
		if !botIn || !opponentIn || opponentID == "" {
			continue
		}
		r, ok := recordMap[opponentID]
		if !ok {
			r = &wl{}
			recordMap[opponentID] = r
		}
		if m.WinnerID == arc.BotID {
			r.wins++
		} else if m.WinnerID == opponentID {
			r.losses++
		}
	}

	var records []HeadToHeadRecord
	for oppID, r := range recordMap {
		name := oppID
		for _, b := range data.Bots {
			if b.ID == oppID {
				name = b.Name
				break
			}
		}
		records = append(records, HeadToHeadRecord{
			OpponentName: name,
			OpponentRank: getBotRank(oppID, data),
			Wins:         r.wins,
			Losses:       r.losses,
			TotalMatches: r.wins + r.losses,
		})
	}
	return records
}

func buildOriginString(bot BotData) string {
	if !bot.Evolved {
		return ""
	}
	return fmt.Sprintf("evolved, %s island", nonEmpty(bot.Island, "unknown"))
}

// getBotRatingHistory returns rating history entries for a specific bot
func getBotRatingHistory(botID string, data *IndexData) []RatingHistoryEntry {
	entries := make([]RatingHistoryEntry, 0)
	for _, rh := range data.RatingHistory {
		if rh.BotID == botID {
			entries = append(entries, rh)
		}
	}
	return entries
}

// ─── Weekly Chronicles Generation ────────────────────────────────────────────────

// GenerateWeeklyChronicles creates a ~500-word aggregated narrative for the week
func (c *LLMClient) GenerateWeeklyChronicles(ctx context.Context, req WeeklyChroniclesRequest) (string, error) {
	prompt := buildWeeklyChroniclesPrompt(req)
	return c.chatCompletion(ctx, prompt)
}

// buildWeeklyChroniclesPrompt constructs a prompt for weekly aggregated narrative
func buildWeeklyChroniclesPrompt(req WeeklyChroniclesRequest) string {
	var sb strings.Builder

	sb.WriteString("Write a 500-word weekly chronicle for AI Code Battle. ")
	sb.WriteString("You are a sports journalist covering an emergent bot league — write with the energy and narrative instinct of esports journalism. ")
	sb.WriteString("Be dramatic but factual. Reference specific bots, ELO before/after deltas, rivalry context, head-to-head records, critical turning points, and season standings. ")
	sb.WriteString("Weave the data into a compelling story — quote scores, cite match IDs, describe strategic moments. ")
	sb.WriteString("Write in present tense with a punchy, journalistic tone. Do not use emojis.\n\n")

	sb.WriteString(fmt.Sprintf("Week: %d of %d\n", req.WeekNumber, req.Year))
	sb.WriteString(fmt.Sprintf("Season: %s\n", req.SeasonName))
	sb.WriteString(fmt.Sprintf("Competitive landscape: %d active bots, %d matches this week\n", req.BotCount, req.MatchCount))

	// Top bot and championship race
	if req.TopBotName != "" {
		sb.WriteString(fmt.Sprintf("\nCurrent #1: %s (ELO %.0f)\n", req.TopBotName, req.TopBotRating))
	}

	// Top ELO movers
	if len(req.TopMovers) > 0 {
		sb.WriteString("\nTop ELO movers this week:\n")
		for _, m := range req.TopMovers {
			dir := "surged"
			if m.Delta < 0 {
				dir = "dropped"
			}
			arch := nonEmpty(m.Archetype, "unclassified")
			sb.WriteString(fmt.Sprintf("  - %s %s %.0f points (%.0f → %.0f) [%s], W%d/L%d\n",
				m.BotName, dir, absF(m.Delta), m.OldRating, m.NewRating, arch, m.MatchesWon, m.MatchesLost))
		}
	}

	// Dominant strategy
	if req.DominantStrat != "" {
		sb.WriteString(fmt.Sprintf("\nDominant archetype: %s\n", req.DominantStrat))
	}

	// Story arcs by type
	sb.WriteString("\nStory arcs this week:\n")

	riseArcs := filterArcsByType(req.StoryArcs, ArcRise)
	fallArcs := filterArcsByType(req.StoryArcs, ArcFall)
	rivalryArcs := filterArcsByType(req.StoryArcs, ArcRivalry)
	upsetArcs := filterArcsByType(req.StoryArcs, ArcUpset)
	evoArcs := filterArcsByType(req.StoryArcs, ArcEvolutionMilestone)
	comebackArcs := filterArcsByType(req.StoryArcs, ArcComeback)

	if len(riseArcs) > 0 {
		sb.WriteString("\n### Rising Stars\n")
		for _, arc := range riseArcs {
			delta := arc.RatingEnd - arc.RatingStart
			sb.WriteString(fmt.Sprintf("  - %s: climbed %d points (%d → %d)", arc.BotName, delta, arc.RatingStart, arc.RatingEnd))
			if arc.Archetype != "" {
				sb.WriteString(fmt.Sprintf(" [%s]", arc.Archetype))
			}
			if arc.Origin != "" {
				sb.WriteString(fmt.Sprintf(" (%s)", arc.Origin))
			}
			sb.WriteString("\n")
			// Add key match context
			if len(arc.KeyMatches) > 0 {
				m := arc.KeyMatches[0]
				outcome := "Beat"
				if !m.Won {
					outcome = "Lost to"
				}
				sb.WriteString(fmt.Sprintf("    Key match: %s %s (rating %d) — score %s, %d turns. Match ID: %s\n",
					outcome, m.OpponentName, m.OpponentRating, m.Score, m.TurnCount, m.MatchID))
			}
		}
	}

	if len(fallArcs) > 0 {
		sb.WriteString("\n### Falling Behind\n")
		for _, arc := range fallArcs {
			delta := arc.RatingStart - arc.RatingEnd
			sb.WriteString(fmt.Sprintf("  - %s: dropped %d points (%d → %d)", arc.BotName, delta, arc.RatingStart, arc.RatingEnd))
			if arc.Archetype != "" {
				sb.WriteString(fmt.Sprintf(" [%s]", arc.Archetype))
			}
			sb.WriteString("\n")
		}
	}

	if len(rivalryArcs) > 0 {
		sb.WriteString("\n### Intensifying Rivalries\n")
		for _, arc := range rivalryArcs {
			sb.WriteString(fmt.Sprintf("  - %s vs %s: %d-%d over %d matches\n",
				arc.BotName, arc.BotBName, arc.BotAWins, arc.BotBWins, arc.TotalMatches))
			// Add recent encounters
			if len(arc.KeyMatches) > 0 {
				sb.WriteString("    Recent matches: ")
				for i, m := range arc.KeyMatches {
					if i > 0 {
						sb.WriteString(", ")
					}
					winner := arc.BotBName
					if m.Won {
						winner = arc.BotName
					}
					sb.WriteString(fmt.Sprintf("%s won %d-%d", winner, m.TurnCount, m.TurnCount))
				}
				sb.WriteString("\n")
			}
		}
	}

	if len(upsetArcs) > 0 {
		sb.WriteString("\n### Upsets of the Week\n")
		for _, arc := range upsetArcs {
			gap := arc.RatingEnd - arc.RatingStart
			sb.WriteString(fmt.Sprintf("  - %s upset %s: %d-point ELO gap", arc.BotName, arc.BotBName, gap))
			if len(arc.KeyMatches) > 0 {
				m := arc.KeyMatches[0]
				sb.WriteString(fmt.Sprintf(", score %s in %d turns. Match ID: %s\n", m.Score, m.TurnCount, m.MatchID))
			} else {
				sb.WriteString("\n")
			}
		}
	}

	if len(evoArcs) > 0 {
		sb.WriteString("\n### Evolution Milestones\n")
		for _, arc := range evoArcs {
			sb.WriteString(fmt.Sprintf("  - %s: generation %d evolved bot", arc.BotName, arc.Generation))
			if arc.Archetype != "" {
				sb.WriteString(fmt.Sprintf(" [%s]", arc.Archetype))
			}
			if arc.Origin != "" {
				sb.WriteString(fmt.Sprintf(" (%s)", arc.Origin))
			}
			sb.WriteString(fmt.Sprintf(", rating %.0f\n", float64(arc.RatingEnd)))
		}
	}

	if len(comebackArcs) > 0 {
		sb.WriteString("\n### Comebacks\n")
		for _, arc := range comebackArcs {
			sb.WriteString(fmt.Sprintf("  - %s: climbed from bottom 25%% to top 25%% (current ELO: %d)\n",
				arc.BotName, arc.RatingEnd))
		}
	}

	// Match of the week
	if req.BestMatch != nil {
		sb.WriteString(fmt.Sprintf("\n### Match of the Week\n"))
		sb.WriteString(fmt.Sprintf("%s — score %s in %d turns. Match ID: %s\n",
			req.BestMatch.Description, req.BestMatch.Score, req.BestMatch.TurnCount, req.BestMatch.MatchID))
	}

	sb.WriteString("\nWrite a compelling 500-word weekly chronicle that weaves these story arcs together. ")
	sb.WriteString("Focus on the most dramatic narratives — the rise, the fall, the rivalries, the upsets. ")
	sb.WriteString("Make it feel like a sports recap that makes readers want to watch the replays.")

	return sb.String()
}

// filterArcsByType returns arcs of a specific type
func filterArcsByType(arcs []StoryArc, arcType StoryArcType) []StoryArc {
	filtered := make([]StoryArc, 0)
	for _, arc := range arcs {
		if arc.Type == arcType {
			filtered = append(filtered, arc)
		}
	}
	return filtered
}
