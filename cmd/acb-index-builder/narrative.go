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
	ArcRise            StoryArcType = "rise"
	ArcFall            StoryArcType = "fall"
	ArcRivalry         StoryArcType = "rivalry"
	ArcUpset           StoryArcType = "upset"
	ArcEvolutionMilestone StoryArcType = "evolution"
	ArcComeback        StoryArcType = "comeback"
	ArcSeasonRecap     StoryArcType = "season-recap"
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
	KeyMatches   []KeyMatch `json:"key_matches,omitempty"`
	Archetype    string     `json:"archetype,omitempty"`
	Origin       string     `json:"origin,omitempty"`
	ParentIDs    []string   `json:"parent_ids,omitempty"`
	Generation   int        `json:"generation,omitempty"`
	CommunityHint string    `json:"community_hint,omitempty"`

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

func buildNarrativePrompt(req NarrativeRequest) string {
	var sb strings.Builder

	// §15.5 instruction: sports-journalism narrative with structured contextual match data
	sb.WriteString("Write a 200-word sports-journalism narrative about this event in the AI Code Battle platform. ")
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
		sb.WriteString(fmt.Sprintf("Arc type: Rise\n"))
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
			}
		}
		if len(req.HeadToHead) > 0 {
			sb.WriteString("Head-to-head records (season):\n")
			for _, h := range req.HeadToHead {
				rankStr := ""
				if h.OpponentRank > 0 {
					rankStr = fmt.Sprintf(" (#%d)", h.OpponentRank)
				}
				sb.WriteString(fmt.Sprintf("  - vs %s%s: %dW-%dL (%d matches)\n",
					h.OpponentName, rankStr, h.Wins, h.Losses, h.TotalMatches))
			}
		}
		if req.CommunityHint != "" {
			sb.WriteString(fmt.Sprintf("Community tactical insight that may have contributed: \"%s\"\n", req.CommunityHint))
		}

	case ArcFall:
		sb.WriteString(fmt.Sprintf("Arc type: Fall\n"))
		sb.WriteString(fmt.Sprintf("Bot: %s\n", req.BotName))
		sb.WriteString(fmt.Sprintf("Season: %s\n", seasonLabel))
		if req.BotRank > 0 {
			sb.WriteString(fmt.Sprintf("Current rank: #%d\n", req.BotRank))
		}
		delta := req.RatingStart - req.RatingEnd
		sb.WriteString(fmt.Sprintf("ELO: %d → %d (delta -%d) over 7 days\n", req.RatingStart, req.RatingEnd, delta))
		if req.Archetype != "" {
			sb.WriteString(fmt.Sprintf("Archetype: %s\n", req.Archetype))
		}
		if len(req.KeyMatches) > 0 {
			sb.WriteString("Critical losses (turning points in the decline):\n")
			for _, m := range req.KeyMatches {
				rankStr := ""
				if m.OpponentRank > 0 {
					rankStr = fmt.Sprintf(", #%d", m.OpponentRank)
				}
				condStr := ""
				if m.EndCondition != "" {
					condStr = fmt.Sprintf(" [%s]", m.EndCondition)
				}
				sb.WriteString(fmt.Sprintf("  - Lost to %s (ELO %d%s) on \"%s\" — score %s, %d turns%s. Match ID: %s\n",
					m.OpponentName, m.OpponentRating, rankStr, nonEmpty(m.MapName, "standard map"), m.Score, m.TurnCount, condStr, m.MatchID))
			}
		}
		if len(req.HeadToHead) > 0 {
			sb.WriteString("Head-to-head records (season):\n")
			for _, h := range req.HeadToHead {
				rankStr := ""
				if h.OpponentRank > 0 {
					rankStr = fmt.Sprintf(" (#%d)", h.OpponentRank)
				}
				sb.WriteString(fmt.Sprintf("  - vs %s%s: %dW-%dL (%d matches)\n",
					h.OpponentName, rankStr, h.Wins, h.Losses, h.TotalMatches))
			}
		}

	case ArcRivalry:
		sb.WriteString(fmt.Sprintf("Arc type: Rivalry Intensifies\n"))
		sb.WriteString(fmt.Sprintf("Bots: %s vs %s\n", req.BotName, req.BotBName))
		sb.WriteString(fmt.Sprintf("Season: %s\n", seasonLabel))
		sb.WriteString(fmt.Sprintf("Head-to-head record this week: %d-%d %s vs %s (%d total matches)\n",
			req.BotAWins, req.BotBWins, req.BotName, req.BotBName, req.TotalMatches))
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
				sb.WriteString(fmt.Sprintf("  - %s won on \"%s\" (%d turns, score %s, opponent ELO %d)%s. Match ID: %s\n",
					winner, nonEmpty(m.MapName, "standard map"), m.TurnCount, m.Score, m.OpponentRating, condStr, m.MatchID))
			}
		}
		if len(req.HeadToHead) > 0 {
			sb.WriteString("All-time head-to-head:\n")
			for _, h := range req.HeadToHead {
				sb.WriteString(fmt.Sprintf("  - vs %s: %dW-%dL (%d matches)\n",
					h.OpponentName, h.Wins, h.Losses, h.TotalMatches))
			}
		}

	case ArcUpset:
		sb.WriteString(fmt.Sprintf("Arc type: Upset of the Week\n"))
		sb.WriteString(fmt.Sprintf("Underdog: %s (ELO %d)\n", req.BotName, req.RatingStart))
		sb.WriteString(fmt.Sprintf("Favorite: %s (ELO %d)\n", req.BotBName, req.RatingEnd))
		gap := req.RatingEnd - req.RatingStart
		sb.WriteString(fmt.Sprintf("ELO gap: %d points\n", gap))
		sb.WriteString(fmt.Sprintf("Season: %s\n", seasonLabel))
		if len(req.KeyMatches) > 0 {
			m := req.KeyMatches[0]
			condStr := ""
			if m.EndCondition != "" {
				condStr = fmt.Sprintf(" [%s]", m.EndCondition)
			}
			sb.WriteString(fmt.Sprintf("Match: %s stunned %s with a %s scoreline after %d turns on \"%s\"%s. Match ID: %s\n",
				req.BotName, req.BotBName, m.Score, m.TurnCount, nonEmpty(m.MapName, "standard map"), condStr, m.MatchID))
		}
		if len(req.HeadToHead) > 0 {
			sb.WriteString("Prior head-to-head:\n")
			for _, h := range req.HeadToHead {
				sb.WriteString(fmt.Sprintf("  - vs %s: %dW-%dL (%d matches)\n",
					h.OpponentName, h.Wins, h.Losses, h.TotalMatches))
			}
		}

	case ArcEvolutionMilestone:
		sb.WriteString(fmt.Sprintf("Arc type: Evolution Milestone\n"))
		sb.WriteString(fmt.Sprintf("Bot: %s\n", req.BotName))
		sb.WriteString(fmt.Sprintf("Season: %s\n", seasonLabel))
		if req.BotRank > 0 {
			sb.WriteString(fmt.Sprintf("Current rank: #%d\n", req.BotRank))
		}
		sb.WriteString(fmt.Sprintf("ELO: new all-time high of %d\n", req.RatingEnd))
		sb.WriteString(fmt.Sprintf("Origin: %s, generation %d\n", req.Origin, req.Generation))
		if len(req.ParentIDs) > 0 {
			sb.WriteString(fmt.Sprintf("Lineage (parent bots): %s\n", strings.Join(req.ParentIDs, ", ")))
		}
		if req.CommunityHint != "" {
			sb.WriteString(fmt.Sprintf("Community tactical insight that influenced this bot: \"%s\"\n", req.CommunityHint))
		}
		if req.Archetype != "" {
			sb.WriteString(fmt.Sprintf("Archetype: %s\n", req.Archetype))
		}
		if len(req.KeyMatches) > 0 {
			sb.WriteString("Key matches driving the milestone:\n")
			for _, m := range req.KeyMatches {
				outcome := "lost to"
				if m.Won {
					outcome = "defeated"
				}
				rankStr := ""
				if m.OpponentRank > 0 {
					rankStr = fmt.Sprintf(", #%d", m.OpponentRank)
				}
				sb.WriteString(fmt.Sprintf("  - %s %s (ELO %d%s) — score %s, %d turns. Match ID: %s\n",
					req.BotName, outcome, m.OpponentRating, rankStr, m.Score, m.TurnCount, m.MatchID))
			}
		}
		if len(req.HeadToHead) > 0 {
			sb.WriteString("Head-to-head vs top opponents:\n")
			for _, h := range req.HeadToHead {
				rankStr := ""
				if h.OpponentRank > 0 {
					rankStr = fmt.Sprintf(" (#%d)", h.OpponentRank)
				}
				sb.WriteString(fmt.Sprintf("  - vs %s%s: %dW-%dL\n",
					h.OpponentName, rankStr, h.Wins, h.Losses))
			}
		}

	case ArcComeback:
		sb.WriteString(fmt.Sprintf("Arc type: Comeback\n"))
		sb.WriteString(fmt.Sprintf("Bot: %s\n", req.BotName))
		sb.WriteString(fmt.Sprintf("Season: %s\n", seasonLabel))
		if req.BotRank > 0 {
			sb.WriteString(fmt.Sprintf("Current rank: #%d\n", req.BotRank))
		}
		sb.WriteString(fmt.Sprintf("ELO recovery: %d → %d (after declining to %d, climbed back %+d)\n",
			req.RatingStart, req.RatingEnd, req.RatingStart-150, req.RatingEnd-(req.RatingStart-150)))
		if req.Archetype != "" {
			sb.WriteString(fmt.Sprintf("Archetype: %s\n", req.Archetype))
		}
		if len(req.KeyMatches) > 0 {
			sb.WriteString("Turning point matches:\n")
			for _, m := range req.KeyMatches {
				rankStr := ""
				if m.OpponentRank > 0 {
					rankStr = fmt.Sprintf(" (#%d)", m.OpponentRank)
				}
				sb.WriteString(fmt.Sprintf("  - Defeated %s (ELO %d%s) on \"%s\" — score %s, %d turns. Match ID: %s\n",
					m.OpponentName, m.OpponentRating, rankStr, nonEmpty(m.MapName, "standard map"), m.Score, m.TurnCount, m.MatchID))
			}
		}
		if len(req.HeadToHead) > 0 {
			sb.WriteString("Head-to-head during comeback:\n")
			for _, h := range req.HeadToHead {
				sb.WriteString(fmt.Sprintf("  - vs %s: %dW-%dL (%d matches)\n",
					h.OpponentName, h.Wins, h.Losses, h.TotalMatches))
			}
		}
	}

	return sb.String()
}

type llmChatRequest struct {
	Model    string          `json:"model"`
	Messages []llmChatMessage `json:"messages"`
	MaxTokens int            `json:"max_tokens,omitempty"`
}

type llmChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type llmChatResponse struct {
	Choices []struct {
		Message llmChatMessage `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// systemPromptSportsJournalist is the system prompt framing the LLM as a
// sports journalist covering AI Code Battle — per plan §15.1 and §15.5.
const systemPromptSportsJournalist = `You are a sports journalist covering an emergent bot league called AI Code Battle, where autonomous programs compete in grid-based strategy matches. Write with the energy and narrative instinct of esports journalism — dramatic but factual, specific but accessible.

Your coverage style:
- Reference bots by name, cite ELO ratings with before/after deltas, and describe strategic turning points the way a play-by-play commentator would.
- Weave in rivalry context, head-to-head records, season standings, and critical moments from match data.
- Describe ELO shifts the way a power rankings columnist describes team movement — "surged 200 points" not "increased."
- Use present tense. Keep paragraphs tight and punchy. Do not use emojis.
- When lineage or evolution data is provided, frame it like a scouting report — origin story, parent strategies, behavioral archetype.
- Always ground narrative in the specific match data, scores, and ratings provided — never fabricate match details.`

func (c *LLMClient) chatCompletion(ctx context.Context, prompt string) (string, error) {
	body, err := json.Marshal(llmChatRequest{
		Model: "GLM-5-Turbo",
		Messages: []llmChatMessage{
			{Role: "system", Content: systemPromptSportsJournalist},
			{Role: "user", Content: prompt},
		},
		MaxTokens: 500,
	})
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	url := c.baseURL + "/v1/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(string(body)))
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	var cr llmChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&cr); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}

	if cr.Error != nil {
		return "", fmt.Errorf("llm api error: %s", cr.Error.Message)
	}
	if len(cr.Choices) == 0 {
		return "", fmt.Errorf("llm api returned no choices")
	}

	return cr.Choices[0].Message.Content, nil
}

// detectStoryArcs scans data for narrative arcs per plan §15.5
func detectStoryArcs(data *IndexData) []StoryArc {
	arcs := make([]StoryArc, 0)

	// Rise: Bot gained >=200 rating in last 7 days
	arcs = append(arcs, detectRiseArcs(data)...)

	// Fall: Bot lost >=200 rating in last 7 days
	arcs = append(arcs, detectFallArcs(data)...)

	// Rivalry Intensifies: 5+ matches this week with alternating wins
	arcs = append(arcs, detectRivalryArcs(data)...)

	// Upset of the Week: Biggest rating gap where underdog won
	arcs = append(arcs, detectUpsetArcs(data)...)

	// Evolution Milestone: Evolved bot reached new ATH or entered top 5
	arcs = append(arcs, detectEvolutionArcs(data)...)

	// Comeback: Bot recovered >=150 rating after decline
	arcs = append(arcs, detectComebackArcs(data)...)

	// Enrich arcs with community tactical hints where available.
	arcs = attachCommunityHints(arcs, data)

	return arcs
}

// attachCommunityHints enriches detected story arcs with the highest-upvote
// community tactical hint associated with the primary bot in each arc.
// Feedback is expected to be sorted by upvotes DESC (as fetched from the DB).
func attachCommunityHints(arcs []StoryArc, data *IndexData) []StoryArc {
	if len(data.Feedback) == 0 {
		return arcs
	}

	// Build matchID → participant botIDs map.
	matchBots := make(map[string][]string, len(data.Matches))
	for _, m := range data.Matches {
		ids := make([]string, 0, len(m.Participants))
		for _, p := range m.Participants {
			ids = append(ids, p.BotID)
		}
		matchBots[m.ID] = ids
	}

	// Assign the first (highest-upvote) eligible hint per bot.
	const minHintUpvotes = 3
	botHint := make(map[string]string)
	for _, f := range data.Feedback {
		if f.Type != "idea" && f.Type != "mistake" {
			continue
		}
		if f.Upvotes < minHintUpvotes {
			break // Sorted DESC; no higher-upvote entries remain.
		}
		for _, botID := range matchBots[f.MatchID] {
			if _, seen := botHint[botID]; !seen {
				botHint[botID] = f.Body
			}
		}
	}

	for i := range arcs {
		if arcs[i].CommunityHint != "" {
			continue
		}
		if hint, ok := botHint[arcs[i].BotID]; ok {
			arcs[i].CommunityHint = hint
		}
	}
	return arcs
}

func detectRiseArcs(data *IndexData) []StoryArc {
	arcs := make([]StoryArc, 0)

	for _, bot := range data.Bots {
		// Check if bot has rating history showing >=200 point gain
		if len(getBotRatingHistory(bot.ID, data)) < 2 {
			continue
		}

		// Find rating from 7 days ago
		now := data.GeneratedAt
		sevenDaysAgo := now.AddDate(0, 0, -7)

		var oldRating float64
		var foundOld bool
		for _, rh := range getBotRatingHistory(bot.ID, data) {
			if rh.RecordedAt.Before(sevenDaysAgo) || rh.RecordedAt.Equal(sevenDaysAgo) {
				oldRating = rh.Rating
				foundOld = true
			}
		}

		if !foundOld {
			continue
		}

		currentRating := bot.Rating
		ratingGain := currentRating - oldRating

		if ratingGain >= 200 {
			arcs = append(arcs, StoryArc{
				Type:        ArcRise,
				BotID:       bot.ID,
				BotName:     bot.Name,
				RatingStart: int(oldRating),
				RatingEnd:   int(currentRating),
				KeyMatches:  extractKeyMatches(bot.ID, data),
				Archetype:   bot.Archetype,
			})
		}
	}

	return arcs
}

func detectFallArcs(data *IndexData) []StoryArc {
	arcs := make([]StoryArc, 0)

	for _, bot := range data.Bots {
		if len(getBotRatingHistory(bot.ID, data)) < 2 {
			continue
		}

		now := data.GeneratedAt
		sevenDaysAgo := now.AddDate(0, 0, -7)

		var oldRating float64
		var foundOld bool
		for _, rh := range getBotRatingHistory(bot.ID, data) {
			if rh.RecordedAt.Before(sevenDaysAgo) || rh.RecordedAt.Equal(sevenDaysAgo) {
				oldRating = rh.Rating
				foundOld = true
			}
		}

		if !foundOld {
			continue
		}

		currentRating := bot.Rating
		ratingLoss := oldRating - currentRating

		if ratingLoss >= 200 {
			arcs = append(arcs, StoryArc{
				Type:        ArcFall,
				BotID:       bot.ID,
				BotName:     bot.Name,
				RatingStart: int(oldRating),
				RatingEnd:   int(currentRating),
				KeyMatches:  extractKeyMatches(bot.ID, data),
			})
		}
	}

	return arcs
}

func detectRivalryArcs(data *IndexData) []StoryArc {
	arcs := make([]StoryArc, 0)

	// Count matches between bot pairs this week
	pairMatches := make(map[string][]MatchData)

	now := data.GeneratedAt
	weekAgo := now.AddDate(0, 0, -7)

	for _, m := range data.Matches {
		if m.PlayedAt.Before(weekAgo) {
			continue
		}
		if len(m.Participants) < 2 {
			continue
		}

		for i, p1 := range m.Participants {
			for _, p2 := range m.Participants[i+1:] {
				key := fmt.Sprintf("%s|%s", minStr(p1.BotID, p2.BotID), maxStr(p1.BotID, p2.BotID))
				pairMatches[key] = append(pairMatches[key], m)
			}
		}
	}

	// Find pairs with 5+ matches and alternating wins
	for key, matches := range pairMatches {
		if len(matches) < 5 {
			continue
		}

		// Parse bot IDs from key (separator is "|" to avoid conflicts with UUID hyphens).
		parts := strings.SplitN(key, "|", 2)
		if len(parts) != 2 {
			continue
		}
		botAID, botBID := parts[0], parts[1]

		// Count wins for each bot
		botAWins := 0
		botBWins := 0

		for _, m := range matches {
			for _, p := range m.Participants {
				if p.Won {
					if p.BotID == botAID {
						botAWins++
					} else if p.BotID == botBID {
						botBWins++
					}
					break
				}
			}
		}

		// Only include if wins are reasonably close (not one-sided)
		if botAWins >= 2 && botBWins >= 2 {
			arcs = append(arcs, StoryArc{
				Type:         ArcRivalry,
				BotID:        botAID,
				BotName:      getBotName(botAID, data),
				BotBID:       botBID,
				BotBName:     getBotName(botBID, data),
				BotAWins:     botAWins,
				BotBWins:     botBWins,
				TotalMatches: len(matches),
				KeyMatches:   extractRivalryMatches(botAID, botBID, data),
			})
		}
	}

	return arcs
}

func detectUpsetArcs(data *IndexData) []StoryArc {
	arcs := make([]StoryArc, 0)

	var biggestUpset *StoryArc
	var biggestGap int

	for _, m := range data.Matches {
		if len(m.Participants) < 2 {
			continue
		}

		// Find winner and loser
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

		// Check if underdog won (winner had lower rating)
		gap := int(loser.PreMatchRating - winner.PreMatchRating)
		if gap > biggestGap {
			biggestGap = gap
			biggestUpset = &StoryArc{
				Type:        ArcUpset,
				BotID:       winner.BotID,
				BotName:     getBotName(winner.BotID, data),
				BotBID:      loser.BotID,
				BotBName:    getBotName(loser.BotID, data),
				RatingStart: int(winner.PreMatchRating),
				RatingEnd:   int(loser.PreMatchRating),
				MatchID:     m.ID,
				KeyMatches: []KeyMatch{{
					MatchID:       m.ID,
					OpponentID:    loser.BotID,
					OpponentName:  getBotName(loser.BotID, data),
					OpponentRating: int(loser.PreMatchRating),
					MapName:       m.MapName,
					Score:         fmt.Sprintf("%d-%d", winner.Score, loser.Score),
					TurnCount:     m.TurnCount,
					Won:           true,
				}},
			}
		}
	}

	if biggestUpset != nil && biggestGap >= 100 { // Minimum 100 rating gap to count as upset
		arcs = append(arcs, *biggestUpset)
	}

	return arcs
}

func detectEvolutionArcs(data *IndexData) []StoryArc {
	arcs := make([]StoryArc, 0)

	for _, bot := range data.Bots {
		if !bot.Evolved {
			continue
		}

		// Check if bot reached new all-time-high rating
		var previousATH float64
		for _, rh := range getBotRatingHistory(bot.ID, data) {
			if rh.Rating > previousATH && rh.RecordedAt.Before(data.GeneratedAt.AddDate(0, 0, -1)) {
				previousATH = rh.Rating
			}
		}

		// Current rating exceeds previous ATH by significant margin
		if bot.Rating > previousATH+50 {
			arcs = append(arcs, StoryArc{
				Type:        ArcEvolutionMilestone,
				BotID:       bot.ID,
				BotName:     bot.Name,
				RatingEnd:   int(bot.Rating),
				Origin:      fmt.Sprintf("evolved, %s island", bot.Island),
				Generation:  bot.Generation,
				ParentIDs:   bot.ParentIDs,
				Archetype:   bot.Archetype,
				KeyMatches:  extractKeyMatches(bot.ID, data),
			})
		}

		// Check if bot entered top 5
		rank := getBotRank(bot.ID, data)
		if rank > 0 && rank <= 5 {
			arcs = append(arcs, StoryArc{
				Type:        ArcEvolutionMilestone,
				BotID:       bot.ID,
				BotName:     bot.Name,
				RatingEnd:   int(bot.Rating),
				Origin:      fmt.Sprintf("evolved, %s island, generation %d", bot.Island, bot.Generation),
				Generation:  bot.Generation,
				ParentIDs:   bot.ParentIDs,
				Archetype:   bot.Archetype,
			})
		}
	}

	return arcs
}

func detectComebackArcs(data *IndexData) []StoryArc {
	arcs := make([]StoryArc, 0)

	for _, bot := range data.Bots {
		if len(getBotRatingHistory(bot.ID, data)) < 3 {
			continue
		}

		// Find a decline followed by recovery
		currentRating := bot.Rating
		var peakRating, troughRating float64
		var foundDecline, foundRecovery bool

		// Walk through history to find decline and recovery pattern
		for i, rh := range getBotRatingHistory(bot.ID, data) {
			if rh.Rating > peakRating {
				peakRating = rh.Rating
			}
			if i > 0 && rh.Rating < getBotRatingHistory(bot.ID, data)[i-1].Rating {
				if rh.Rating < troughRating || troughRating == 0 {
					troughRating = rh.Rating
					foundDecline = true
				}
			}
		}

		// Check if current rating represents recovery of >=150 from trough
		if foundDecline && currentRating >= troughRating+150 {
			foundRecovery = true
		}

		if foundRecovery {
			arcs = append(arcs, StoryArc{
				Type:        ArcComeback,
				BotID:       bot.ID,
				BotName:     bot.Name,
				RatingStart: int(peakRating),
				RatingEnd:   int(currentRating),
				KeyMatches:  extractKeyMatches(bot.ID, data),
			})
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
		})

		if len(matches) >= 5 {
			break
		}
	}

	return matches
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
