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
}

// KeyMatch represents a key match for narrative context
type KeyMatch struct {
	MatchID    string `json:"match_id"`
	OpponentID string `json:"opponent_id"`
	OpponentName string `json:"opponent_name"`
	OpponentRating int  `json:"opponent_rating"`
	MapName    string `json:"map_name,omitempty"`
	Score      string `json:"score"`
	TurnCount  int    `json:"turn_count"`
	Won        bool   `json:"won"`
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
	SeasonName  string
	RatingStart int
	RatingEnd   int
	KeyMatches  []KeyMatch
	Archetype   string
	Origin      string
	ParentIDs   []string
	Generation  int
	// Additional context
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

	sb.WriteString("Write a 200-word sports-journalism narrative about this event in the AI Code Battle platform. Be dramatic but factual. Reference specific matches. Write in present tense. Do not use emojis.\n\n")

	switch req.ArcType {
	case ArcRise:
		sb.WriteString(fmt.Sprintf("Arc type: Rise\n"))
		sb.WriteString(fmt.Sprintf("Bot: %s\n", req.BotName))
		sb.WriteString(fmt.Sprintf("Season: %s\n", req.SeasonName))
		sb.WriteString(fmt.Sprintf("Rating: %d → %d over 7 days\n", req.RatingStart, req.RatingEnd))
		if len(req.KeyMatches) > 0 {
			sb.WriteString("Key matches:\n")
			for _, m := range req.KeyMatches {
				outcome := "Lost to"
				if m.Won {
					outcome = "Beat"
				}
				sb.WriteString(fmt.Sprintf("  - %s %s (#%d, %d) on %q — score %s, turn %d\n",
					outcome, m.OpponentName, m.OpponentRating/10, m.OpponentRating, m.MapName, m.Score, m.TurnCount))
			}
		}
		if req.Archetype != "" {
			sb.WriteString(fmt.Sprintf("Archetype: %s\n", req.Archetype))
		}
		if req.Origin != "" {
			sb.WriteString(fmt.Sprintf("Origin: %s\n", req.Origin))
		}

	case ArcFall:
		sb.WriteString(fmt.Sprintf("Arc type: Fall\n"))
		sb.WriteString(fmt.Sprintf("Bot: %s\n", req.BotName))
		sb.WriteString(fmt.Sprintf("Season: %s\n", req.SeasonName))
		sb.WriteString(fmt.Sprintf("Rating: %d → %d over 7 days\n", req.RatingStart, req.RatingEnd))
		if len(req.KeyMatches) > 0 {
			sb.WriteString("Recent losses:\n")
			for _, m := range req.KeyMatches {
				sb.WriteString(fmt.Sprintf("  - Lost to %s (#%d) on %q — score %s, turn %d\n",
					m.OpponentName, m.OpponentRating/10, m.MapName, m.Score, m.TurnCount))
			}
		}

	case ArcRivalry:
		sb.WriteString(fmt.Sprintf("Arc type: Rivalry Intensifies\n"))
		sb.WriteString(fmt.Sprintf("Bots: %s vs %s\n", req.BotName, req.BotBName))
		sb.WriteString(fmt.Sprintf("Season: %s\n", req.SeasonName))
		sb.WriteString(fmt.Sprintf("Head-to-head record: %d-%d (%d matches this week)\n",
			req.BotAWins, req.BotBWins, req.TotalMatches))
		if len(req.KeyMatches) > 0 {
			sb.WriteString("Recent encounters:\n")
			for _, m := range req.KeyMatches {
				outcome := "lost"
				if m.Won {
					outcome = "won"
				}
				sb.WriteString(fmt.Sprintf("  - %s %s against %s (%s)\n",
					req.BotName, outcome, m.OpponentName, m.Score))
			}
		}

	case ArcUpset:
		sb.WriteString(fmt.Sprintf("Arc type: Upset of the Week\n"))
		sb.WriteString(fmt.Sprintf("Underdog: %s (rating %d)\n", req.BotName, req.RatingStart))
		sb.WriteString(fmt.Sprintf("Favorite: %s (rating %d)\n", req.BotBName, req.RatingEnd))
		sb.WriteString(fmt.Sprintf("Season: %s\n", req.SeasonName))
		if len(req.KeyMatches) > 0 {
			m := req.KeyMatches[0]
			sb.WriteString(fmt.Sprintf("Match: Final score %s after %d turns on %q\n",
				m.Score, m.TurnCount, m.MapName))
		}

	case ArcEvolutionMilestone:
		sb.WriteString(fmt.Sprintf("Arc type: Evolution Milestone\n"))
		sb.WriteString(fmt.Sprintf("Bot: %s\n", req.BotName))
		sb.WriteString(fmt.Sprintf("Season: %s\n", req.SeasonName))
		sb.WriteString(fmt.Sprintf("New all-time-high rating: %d\n", req.RatingEnd))
		sb.WriteString(fmt.Sprintf("Origin: %s, generation %d\n", req.Origin, req.Generation))
		if len(req.ParentIDs) > 0 {
			sb.WriteString(fmt.Sprintf("Parents: %s\n", strings.Join(req.ParentIDs, ", ")))
		}
		if req.Archetype != "" {
			sb.WriteString(fmt.Sprintf("Archetype: %s\n", req.Archetype))
		}

	case ArcComeback:
		sb.WriteString(fmt.Sprintf("Arc type: Comeback\n"))
		sb.WriteString(fmt.Sprintf("Bot: %s\n", req.BotName))
		sb.WriteString(fmt.Sprintf("Season: %s\n", req.SeasonName))
		sb.WriteString(fmt.Sprintf("Rating recovery: %d → %d (after declining to %d)\n",
			req.RatingStart, req.RatingEnd, req.RatingStart-150))
		if len(req.KeyMatches) > 0 {
			sb.WriteString("Turning point matches:\n")
			for _, m := range req.KeyMatches {
				sb.WriteString(fmt.Sprintf("  - Beat %s (#%d) — score %s\n",
					m.OpponentName, m.OpponentRating/10, m.Score))
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

func (c *LLMClient) chatCompletion(ctx context.Context, prompt string) (string, error) {
	body, err := json.Marshal(llmChatRequest{
		Model: "GLM-5-Turbo", // Use fast tier for cheap narrative generation
		Messages: []llmChatMessage{
			{Role: "user", Content: prompt},
		},
		MaxTokens: 500, // ~200 words should fit easily
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
				key := fmt.Sprintf("%s-%s", minStr(p1.BotID, p2.BotID), maxStr(p1.BotID, p2.BotID))
				pairMatches[key] = append(pairMatches[key], m)
			}
		}
	}

	// Find pairs with 5+ matches and alternating wins
	for key, matches := range pairMatches {
		if len(matches) < 5 {
			continue
		}

		// Parse bot IDs from key
		parts := strings.Split(key, "-")
		if len(parts) != 2 {
			continue
		}
		botAID, botBID := parts[0], parts[1]

		// Count wins for each bot and check alternation
		botAWins := 0
		botBWins := 0
		alternating := true
		lastWinner := ""

		for _, m := range matches {
			var winnerID string
			for _, p := range m.Participants {
				if p.Won {
					winnerID = p.BotID
					if p.BotID == botAID {
						botAWins++
					} else if p.BotID == botBID {
						botBWins++
					}
					break
				}
			}
			if lastWinner != "" && winnerID == lastWinner {
				alternating = false
			}
			lastWinner = winnerID
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
		gap := loser.PreMatchRating - winner.PreMatchRating
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
			MapName:        m.MapName,
			Score:          fmt.Sprintf("%d-%d", botPart.Score, oppPart.Score),
			TurnCount:      m.TurnCount,
			Won:            botPart.Won,
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
