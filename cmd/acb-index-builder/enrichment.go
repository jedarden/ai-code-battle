package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"time"
)

// CommentaryEntry is one commentary subtitle in an enriched replay.
type CommentaryEntry struct {
	Turn int    `json:"turn"`
	Text string `json:"text"`
	Type string `json:"type"` // setup, action, reaction, climax, denouement
}

// EnrichedCommentary wraps all AI commentary for a single match.
type EnrichedCommentary struct {
	MatchID   string             `json:"match_id"`
	Generated string            `json:"generated_at"`
	Criteria  []string           `json:"criteria"` // why this match was selected
	Entries   []CommentaryEntry  `json:"entries"`
}

// shouldEnrich returns true and lists criteria if the match qualifies for
// AI commentary enrichment per §13.3.
func shouldEnrich(m MatchData, data *IndexData) ([]string, bool) {
	if len(m.Participants) < 2 || m.WinnerID == "" {
		return nil, false
	}

	var criteria []string

	// 1. Back-and-forth: win_prob crossed 0.5 at least 3 times
	// We can't read the full replay here, so we use a proxy:
	// close score + long match suggests back-and-forth
	scoreDiff := minScoreDiff(m)
	if scoreDiff <= 2 && m.TurnCount >= 200 {
		criteria = append(criteria, "back_and_forth")
	}

	// 2. Upset: lower-rated bot wins by >200 rating points
	if upset := ratingUpsetMagnitude(m); upset > 200 {
		criteria = append(criteria, fmt.Sprintf("upset_%d", upset))
	}

	// 3. Evolution milestone: evolved bot's first top-10 appearance
	for _, p := range m.Participants {
		for _, bot := range data.Bots {
			if bot.ID == p.BotID && bot.Evolved && p.Won {
				rank := getBotRank(bot.ID, data)
				if rank > 0 && rank <= 10 {
					criteria = append(criteria, "evolution_milestone")
				}
			}
		}
	}

	// 4. High interest score as a general qualifier
	if score := interestScore(m); score >= 5.0 {
		criteria = append(criteria, "high_interest")
	}

	return criteria, len(criteria) > 0
}

// enrichReplays selects featured matches from IndexData, downloads their
// replays from B2, generates AI commentary via LLM, and uploads the
// commentary JSON to R2 alongside the replay.
func enrichReplays(ctx context.Context, data *IndexData, cfg *Config, llm *LLMClient) error {
	if llm == nil {
		slog.Debug("No LLM client, skipping replay enrichment")
		return nil
	}

	// Select matches eligible for enrichment
	var candidates []struct {
		match    MatchData
		criteria []string
	}
	for _, m := range data.Matches {
		if crit, ok := shouldEnrich(m, data); ok {
			candidates = append(candidates, struct {
				match    MatchData
				criteria []string
			}{match: m, criteria: crit})
		}
	}

	if len(candidates) == 0 {
		slog.Debug("No matches eligible for replay enrichment")
		return nil
	}

	// Cap at 10 enrichments per cycle to control cost
	if len(candidates) > 10 {
		candidates = candidates[:10]
	}

	slog.Info("Enriching replays with AI commentary", "candidates", len(candidates))

	for _, c := range candidates {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		commentary, err := enrichSingleReplay(ctx, c.match, c.criteria, data, cfg, llm)
		if err != nil {
			slog.Error("Failed to enrich replay", "match_id", c.match.ID, "error", err)
			continue
		}

		// Upload commentary to R2
		if err := uploadCommentaryToR2(ctx, cfg, c.match.ID, commentary); err != nil {
			slog.Error("Failed to upload commentary", "match_id", c.match.ID, "error", err)
			continue
		}

		slog.Info("Enriched replay", "match_id", c.match.ID, "entries", len(commentary.Entries), "criteria", c.criteria)
	}

	return nil
}

// enrichSingleReplay downloads the replay from B2, extracts key moments,
// calls the LLM for commentary, and returns the EnrichedCommentary.
func enrichSingleReplay(ctx context.Context, m MatchData, criteria []string, data *IndexData, cfg *Config, llm *LLMClient) (*EnrichedCommentary, error) {
	// Download replay from B2
	replayJSON, err := downloadReplayFromB2(ctx, cfg, m.ID)
	if err != nil {
		return nil, fmt.Errorf("download replay: %w", err)
	}

	// Parse just enough of the replay for commentary context
	var replay struct {
		WinProb         [][]float64 `json:"win_prob"`
		CriticalMoments []struct {
			Turn        int     `json:"turn"`
			Delta       float64 `json:"delta"`
			Description string  `json:"description"`
		} `json:"critical_moments"`
		Result struct {
			Winner int      `json:"winner"`
			Reason string   `json:"reason"`
			Turns  int      `json:"turns"`
			Scores []int    `json:"scores"`
		} `json:"result"`
		Players []struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
		} `json:"players"`
		Turns []struct {
			Turn    int    `json:"turn"`
			Events  []struct {
				Type    string `json:"type"`
				Turn    int    `json:"turn"`
				Details any    `json:"details"`
			} `json:"events"`
			Scores []int `json:"scores"`
		} `json:"turns"`
	}
	if err := json.Unmarshal(replayJSON, &replay); err != nil {
		return nil, fmt.Errorf("parse replay: %w", err)
	}

	// Refine criteria using actual replay data
	if len(replay.WinProb) > 0 {
		crossings := countWinProbCrossings(replay.WinProb)
		if crossings >= 3 {
			// Add precise back-and-forth criterion if not already present via proxy
			found := false
			for _, c := range criteria {
				if c == "back_and_forth" {
					found = true
					break
				}
			}
			if !found {
				criteria = append(criteria, fmt.Sprintf("back_and_forth_%d_crossings", crossings))
			}
		}
	}

	// Build the prompt with match context
	prompt := buildCommentaryPrompt(m, replay, criteria, data)

	// Call LLM
	response, err := llm.chatCompletion(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("llm commentary: %w", err)
	}

	// Parse the LLM response into commentary entries
	entries := parseCommentaryResponse(response)

	if len(entries) == 0 {
		return nil, fmt.Errorf("no commentary entries generated")
	}

	return &EnrichedCommentary{
		MatchID:   m.ID,
		Generated: time.Now().UTC().Format(time.RFC3339),
		Criteria:  criteria,
		Entries:   entries,
	}, nil
}

// buildCommentaryPrompt creates the LLM prompt for commentary generation.
func buildCommentaryPrompt(m MatchData, replay struct {
	WinProb         [][]float64 `json:"win_prob"`
	CriticalMoments []struct {
		Turn        int     `json:"turn"`
		Delta       float64 `json:"delta"`
		Description string  `json:"description"`
	} `json:"critical_moments"`
	Result struct {
		Winner int      `json:"winner"`
		Reason string   `json:"reason"`
		Turns  int      `json:"turns"`
		Scores []int    `json:"scores"`
	} `json:"result"`
	Players []struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	} `json:"players"`
	Turns []struct {
		Turn    int    `json:"turn"`
		Events  []struct {
			Type    string `json:"type"`
			Turn    int    `json:"turn"`
			Details any    `json:"details"`
		} `json:"events"`
		Scores []int `json:"scores"`
	} `json:"turns"`
}, criteria []string, data *IndexData) string {
	var sb strings.Builder

	sb.WriteString("You are an AI Code Battle commentator. Generate 5-15 lines of play-by-play commentary for this match.\n")
	sb.WriteString("Each line must be exactly: TURN|TYPE|TEXT\n")
	sb.WriteString("Where TYPE is one of: setup, action, reaction, climax, denouement\n")
	sb.WriteString("Only cover key moments. Be dramatic but factual. No emojis. Keep each text under 120 chars.\n\n")

	sb.WriteString("MATCH CONTEXT:\n")
	playerNames := make([]string, len(replay.Players))
	for i, p := range replay.Players {
		playerNames[i] = p.Name
	}
	sb.WriteString(fmt.Sprintf("Players: %s\n", strings.Join(playerNames, " vs ")))

	winnerName := "Draw"
	if replay.Result.Winner >= 0 && replay.Result.Winner < len(replay.Players) {
		winnerName = replay.Players[replay.Result.Winner].Name
	}
	sb.WriteString(fmt.Sprintf("Winner: %s by %s in %d turns\n", winnerName, replay.Result.Reason, replay.Result.Turns))
	sb.WriteString(fmt.Sprintf("Final scores: %v\n", replay.Result.Scores))
	sb.WriteString(fmt.Sprintf("Selection criteria: %s\n", strings.Join(criteria, ", ")))

	// Pre-match ratings
	for _, p := range m.Participants {
		name := getBotName(p.BotID, data)
		sb.WriteString(fmt.Sprintf("  %s: pre-match rating %d (evolved: %v)\n",
			name, int(p.PreMatchRating), isEvolved(p.BotID, data)))
	}

	// Win probability summary
	if len(replay.WinProb) > 0 {
		crossings := countWinProbCrossings(replay.WinProb)
		sb.WriteString(fmt.Sprintf("Win prob crossed 0.5: %d times\n", crossings))

		// Biggest swing
		maxSwing := 0.0
		maxSwingTurn := 0
		for i, wp := range replay.WinProb {
			if len(wp) >= 2 {
				swing := wp[0] - 0.5
				if swing < 0 {
					swing = -swing
				}
				if i > 0 {
					prev := replay.WinProb[i-1]
					if len(prev) >= 2 {
						delta := wp[0] - prev[0]
						if delta < 0 {
							delta = -delta
						}
						if delta > maxSwing {
							maxSwing = delta
							maxSwingTurn = i
						}
					}
				}
			}
		}
		if maxSwing > 0.1 {
			sb.WriteString(fmt.Sprintf("Biggest swing: %.0f%% at turn %d\n", maxSwing*100, maxSwingTurn))
		}
	}

	// Critical moments
	if len(replay.CriticalMoments) > 0 {
		sb.WriteString("Critical moments:\n")
		for _, cm := range replay.CriticalMoments {
			sb.WriteString(fmt.Sprintf("  Turn %d: %s (delta %.0f%%)\n", cm.Turn, cm.Description, cm.Delta*100))
		}
	}

	// Key events (cores captured, mass deaths) - scan at most every 10th turn
	sb.WriteString("Key events:\n")
	step := max(1, len(replay.Turns)/30)
	for i := 0; i < len(replay.Turns); i += step {
		t := replay.Turns[i]
		for _, e := range t.Events {
			if e.Type == "core_captured" || e.Type == "combat_death" {
				sb.WriteString(fmt.Sprintf("  Turn %d: %s\n", t.Turn, e.Type))
			}
		}
	}

	sb.WriteString("\nGenerate commentary now. One entry per line, format: TURN|TYPE|TEXT\n")

	return sb.String()
}

// parseCommentaryResponse converts the LLM text output into CommentaryEntry slice.
func parseCommentaryResponse(response string) []CommentaryEntry {
	var entries []CommentaryEntry
	for _, line := range strings.Split(response, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "//") {
			continue
		}

		parts := strings.SplitN(line, "|", 3)
		if len(parts) != 3 {
			continue
		}

		var turn int
		if _, err := fmt.Sscanf(strings.TrimSpace(parts[0]), "%d", &turn); err != nil {
			continue
		}

		entryType := strings.TrimSpace(parts[1])
		text := strings.TrimSpace(parts[2])

		validTypes := map[string]bool{
			"setup": true, "action": true, "reaction": true,
			"climax": true, "denouement": true,
		}
		if !validTypes[entryType] {
			entryType = "action"
		}

		entries = append(entries, CommentaryEntry{
			Turn: turn,
			Text: text,
			Type: entryType,
		})
	}

	return entries
}

// countWinProbCrossings counts how many times p0's win_prob crosses 0.5.
func countWinProbCrossings(winProb [][]float64) int {
	if len(winProb) < 2 {
		return 0
	}
	crossings := 0
	for i := 1; i < len(winProb); i++ {
		prev := winProb[i-1]
		cur := winProb[i]
		if len(prev) >= 1 && len(cur) >= 1 {
			if (prev[0] < 0.5 && cur[0] >= 0.5) || (prev[0] >= 0.5 && cur[0] < 0.5) {
				crossings++
			}
		}
	}
	return crossings
}

// downloadReplayFromB2 downloads a replay JSON from B2, handling .json.gz.
func downloadReplayFromB2(ctx context.Context, cfg *Config, matchID string) ([]byte, error) {
	b2Client, err := getB2Client(cfg)
	if err != nil {
		return nil, fmt.Errorf("create B2 client: %w", err)
	}

	// Try .json.gz first (standard format)
	key := fmt.Sprintf("replays/%s.json.gz", matchID)
	body, err := b2Client.downloadObject(ctx, key)
	if err != nil {
		// Try uncompressed
		key = fmt.Sprintf("replays/%s.json", matchID)
		body, err = b2Client.downloadObject(ctx, key)
		if err != nil {
			return nil, fmt.Errorf("download replay %s: %w", matchID, err)
		}
	}
	defer body.Close()

	raw, err := io.ReadAll(body)
	if err != nil {
		return nil, fmt.Errorf("read replay body: %w", err)
	}

	// Decompress gzip if needed
	if len(raw) > 2 && raw[0] == 0x1f && raw[1] == 0x8b {
		gzReader, err := gzip.NewReader(bytes.NewReader(raw))
		if err != nil {
			return nil, fmt.Errorf("gzip reader: %w", err)
		}
		defer gzReader.Close()
		decompressed, err := io.ReadAll(gzReader)
		if err != nil {
			return nil, fmt.Errorf("gzip decompress: %w", err)
		}
		return decompressed, nil
	}

	return raw, nil
}

// uploadCommentaryToR2 uploads the enriched commentary JSON to R2.
func uploadCommentaryToR2(ctx context.Context, cfg *Config, matchID string, commentary *EnrichedCommentary) error {
	r2Client, err := getR2Client(cfg)
	if err != nil {
		return fmt.Errorf("create R2 client: %w", err)
	}

	data, err := json.Marshal(commentary)
	if err != nil {
		return fmt.Errorf("marshal commentary: %w", err)
	}

	key := fmt.Sprintf("commentary/%s.json", matchID)
	return r2Client.uploadFile(ctx, key, bytes.NewReader(data), "application/json")
}

// isEvolved checks if a bot is an evolved bot.
func isEvolved(botID string, data *IndexData) bool {
	for _, bot := range data.Bots {
		if bot.ID == botID {
			return bot.Evolved
		}
	}
	return false
}

// generateEnrichedIndex creates data/commentary/index.json listing enriched match IDs.
// The frontend uses this to discover which matches have AI commentary available.
func generateEnrichedIndex(ctx context.Context, data *IndexData, cfg *Config, outputDir string) error {
	commentaryDir := filepath.Join(outputDir, "data", "commentary")

	type enrichedEntry struct {
		MatchID  string   `json:"match_id"`
		Criteria []string `json:"criteria"`
	}

	// Check R2 for existing commentary files
	r2Client, err := getR2Client(cfg)
	if err != nil {
		slog.Debug("Cannot list R2 commentary, skipping enriched index", "error", err)
		return nil
	}

	objects, err := r2Client.listObjects(ctx, "commentary/")
	if err != nil {
		slog.Debug("Failed to list commentary objects", "error", err)
		return nil
	}

	var entries []enrichedEntry
	for _, obj := range objects {
		if !strings.HasSuffix(obj.Key, ".json") {
			continue
		}
		// Extract match_id from commentary/{match_id}.json
		matchID := strings.TrimPrefix(obj.Key, "commentary/")
		matchID = strings.TrimSuffix(matchID, ".json")
		if matchID == "" || matchID == "index" {
			continue
		}

		// Try to read criteria from the commentary file
		criteria := []string{}
		body, err := r2Client.downloadObject(ctx, obj.Key)
		if err == nil {
			var comm EnrichedCommentary
			if json.NewDecoder(body).Decode(&comm) == nil {
				criteria = comm.Criteria
			}
			body.Close()
		}

		entries = append(entries, enrichedEntry{
			MatchID:  matchID,
			Criteria: criteria,
		})
	}

	if len(entries) == 0 {
		return nil
	}

	type enrichedIndex struct {
		UpdatedAt string          `json:"updated_at"`
		Entries   []enrichedEntry `json:"entries"`
	}

	index := enrichedIndex{
		UpdatedAt: data.GeneratedAt.Format(time.RFC3339),
		Entries:   entries,
	}

	return writeJSON(filepath.Join(commentaryDir, "index.json"), index)
}
