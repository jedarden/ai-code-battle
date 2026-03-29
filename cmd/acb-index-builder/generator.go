package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// LeaderboardIndex represents the leaderboard.json structure
type LeaderboardIndex struct {
	UpdatedAt string            `json:"updated_at"`
	Entries   []LeaderboardEntry `json:"entries"`
}

// LeaderboardEntry represents a single bot on the leaderboard
type LeaderboardEntry struct {
	Rank           int     `json:"rank"`
	BotID          string  `json:"bot_id"`
	Name           string  `json:"name"`
	OwnerID        string  `json:"owner_id"`
	Rating         int     `json:"rating"`
	RatingDeviation float64 `json:"rating_deviation"`
	MatchesPlayed  int     `json:"matches_played"`
	MatchesWon     int     `json:"matches_won"`
	WinRate        float64 `json:"win_rate"`
	HealthStatus   string  `json:"health_status"`
}

// BotDirectory represents bots/index.json
type BotDirectory struct {
	UpdatedAt string              `json:"updated_at"`
	Bots      []BotDirectoryEntry `json:"bots"`
}

// BotDirectoryEntry represents a bot in the directory
type BotDirectoryEntry struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	Rating        int     `json:"rating"`
	MatchesPlayed int     `json:"matches_played"`
	WinRate       float64 `json:"win_rate"`
}

// BotProfile represents data/bots/{bot_id}.json
type BotProfile struct {
	ID               string                 `json:"id"`
	Name             string                 `json:"name"`
	OwnerID          string                 `json:"owner_id"`
	Description      string                 `json:"description,omitempty"`
	Rating           int                    `json:"rating"`
	RatingDeviation  float64                `json:"rating_deviation"`
	RatingVolatility float64                `json:"rating_volatility"`
	MatchesPlayed    int                    `json:"matches_played"`
	MatchesWon       int                    `json:"matches_won"`
	WinRate          float64                `json:"win_rate"`
	HealthStatus     string                 `json:"health_status"`
	Evolved          bool                   `json:"evolved"`
	Island           string                 `json:"island,omitempty"`
	Generation       int                    `json:"generation,omitempty"`
	CreatedAt        string                 `json:"created_at"`
	RatingHistory    []RatingHistoryEntry   `json:"rating_history"`
	RecentMatches    []MatchSummary         `json:"recent_matches"`
}

// MatchSummary represents a match in listings
type MatchSummary struct {
	ID           string               `json:"id"`
	CompletedAt  string               `json:"completed_at"`
	Participants []MatchParticipantSummary `json:"participants"`
	WinnerID     string               `json:"winner_id,omitempty"`
	Turns        int                  `json:"turns"`
	EndReason    string               `json:"end_reason"`
}

// MatchParticipantSummary represents a bot in a match summary
type MatchParticipantSummary struct {
	BotID string `json:"bot_id"`
	Name  string `json:"name"`
	Score int    `json:"score"`
	Won   bool   `json:"won"`
}

// MatchIndex represents matches/index.json
type MatchIndex struct {
	UpdatedAt string         `json:"updated_at"`
	Matches   []MatchSummary `json:"matches"`
}

// generateAllIndexes creates all JSON index files
func generateAllIndexes(data *IndexData, outputDir string) error {
	botNameMap := make(map[string]string)
	for _, bot := range data.Bots {
		botNameMap[bot.ID] = bot.Name
	}

	// Generate leaderboard.json
	if err := generateLeaderboard(data, outputDir); err != nil {
		return fmt.Errorf("leaderboard: %w", err)
	}

	// Generate bots/index.json
	if err := generateBotDirectory(data, outputDir); err != nil {
		return fmt.Errorf("bot directory: %w", err)
	}

	// Generate individual bot profiles
	if err := generateBotProfiles(data, outputDir); err != nil {
		return fmt.Errorf("bot profiles: %w", err)
	}

	// Generate matches/index.json
	if err := generateMatchIndex(data, outputDir, botNameMap); err != nil {
		return fmt.Errorf("match index: %w", err)
	}

	// Generate series/index.json
	if err := generateSeriesIndex(data, outputDir); err != nil {
		return fmt.Errorf("series index: %w", err)
	}

	// Generate seasons/index.json
	if err := generateSeasonsIndex(data, outputDir); err != nil {
		return fmt.Errorf("seasons index: %w", err)
	}

	// Generate predictions/leaderboard.json
	if err := generatePredictionsIndex(data, outputDir); err != nil {
		return fmt.Errorf("predictions index: %w", err)
	}

	// Generate playlists
	if err := generatePlaylists(data, outputDir, botNameMap); err != nil {
		return fmt.Errorf("playlists: %w", err)
	}

	return nil
}

func generateLeaderboard(data *IndexData, outputDir string) error {
	entries := make([]LeaderboardEntry, 0, len(data.Bots))
	for i, bot := range data.Bots {
		if bot.MatchesPlayed == 0 {
			continue
		}
		winRate := 0.0
		if bot.MatchesPlayed > 0 {
			winRate = float64(bot.MatchesWon) / float64(bot.MatchesPlayed) * 100
		}
		entries = append(entries, LeaderboardEntry{
			Rank:            i + 1,
			BotID:           bot.ID,
			Name:            bot.Name,
			OwnerID:         bot.OwnerID,
			Rating:          int(bot.Rating),
			RatingDeviation: bot.RatingDeviation,
			MatchesPlayed:   bot.MatchesPlayed,
			MatchesWon:      bot.MatchesWon,
			WinRate:         round1(winRate),
			HealthStatus:    bot.HealthStatus,
		})
	}

	leaderboard := LeaderboardIndex{
		UpdatedAt: data.GeneratedAt.Format(time.RFC3339),
		Entries:   entries,
	}

	return writeJSON(filepath.Join(outputDir, "data", "leaderboard.json"), leaderboard)
}

func generateBotDirectory(data *IndexData, outputDir string) error {
	entries := make([]BotDirectoryEntry, 0, len(data.Bots))
	for _, bot := range data.Bots {
		winRate := 0.0
		if bot.MatchesPlayed > 0 {
			winRate = float64(bot.MatchesWon) / float64(bot.MatchesPlayed) * 100
		}
		entries = append(entries, BotDirectoryEntry{
			ID:            bot.ID,
			Name:          bot.Name,
			Rating:        int(bot.Rating),
			MatchesPlayed: bot.MatchesPlayed,
			WinRate:       round1(winRate),
		})
	}

	dir := BotDirectory{
		UpdatedAt: data.GeneratedAt.Format(time.RFC3339),
		Bots:      entries,
	}

	return writeJSON(filepath.Join(outputDir, "data", "bots", "index.json"), dir)
}

func generateBotProfiles(data *IndexData, outputDir string) error {
	botsDir := filepath.Join(outputDir, "data", "bots")

	for _, bot := range data.Bots {
		winRate := 0.0
		if bot.MatchesPlayed > 0 {
			winRate = float64(bot.MatchesWon) / float64(bot.MatchesPlayed) * 100
		}

		// Get rating history for this bot
		history := make([]RatingHistoryEntry, 0)
		for _, h := range data.RatingHistory {
			if h.BotID == bot.ID {
				history = append(history, h)
			}
		}

		// Get recent matches for this bot (last 20)
		recentMatches := make([]MatchSummary, 0)
		for _, m := range data.Matches {
			participated := false
			for _, p := range m.Participants {
				if p.BotID == bot.ID {
					participated = true
					break
				}
			}
			if participated {
				summary := matchToSummary(m, data)
				recentMatches = append(recentMatches, summary)
				if len(recentMatches) >= 20 {
					break
				}
			}
		}

		profile := BotProfile{
			ID:               bot.ID,
			Name:             bot.Name,
			OwnerID:          bot.OwnerID,
			Description:      bot.Description,
			Rating:           int(bot.Rating),
			RatingDeviation:  bot.RatingDeviation,
			RatingVolatility: bot.RatingVolatility,
			MatchesPlayed:    bot.MatchesPlayed,
			MatchesWon:       bot.MatchesWon,
			WinRate:          round1(winRate),
			HealthStatus:     bot.HealthStatus,
			Evolved:          bot.Evolved,
			Island:           bot.Island,
			Generation:       bot.Generation,
			CreatedAt:        bot.CreatedAt.Format(time.RFC3339),
			RatingHistory:    history,
			RecentMatches:    recentMatches,
		}

		if err := writeJSON(filepath.Join(botsDir, bot.ID+".json"), profile); err != nil {
			return err
		}
	}

	return nil
}

func generateMatchIndex(data *IndexData, outputDir string, botNameMap map[string]string) error {
	summaries := make([]MatchSummary, 0, len(data.Matches))
	for _, m := range data.Matches {
		summaries = append(summaries, matchToSummary(m, data))
	}

	index := MatchIndex{
		UpdatedAt: data.GeneratedAt.Format(time.RFC3339),
		Matches:   summaries,
	}

	return writeJSON(filepath.Join(outputDir, "data", "matches", "index.json"), index)
}

func matchToSummary(m MatchData, data *IndexData) MatchSummary {
	participants := make([]MatchParticipantSummary, 0, len(m.Participants))
	for _, p := range m.Participants {
		name := "Unknown"
		for _, bot := range data.Bots {
			if bot.ID == p.BotID {
				name = bot.Name
				break
			}
		}
		participants = append(participants, MatchParticipantSummary{
			BotID: p.BotID,
			Name:  name,
			Score: p.Score,
			Won:   p.BotID == m.WinnerID,
		})
	}

	return MatchSummary{
		ID:           m.ID,
		CompletedAt:  m.CompletedAt.Format(time.RFC3339),
		Participants: participants,
		WinnerID:     m.WinnerID,
		Turns:        m.TurnCount,
		EndReason:    m.EndCondition,
	}
}

func generateSeriesIndex(data *IndexData, outputDir string) error {
	type SeriesIndex struct {
		UpdatedAt string       `json:"updated_at"`
		Series    []SeriesData `json:"series"`
	}

	index := SeriesIndex{
		UpdatedAt: data.GeneratedAt.Format(time.RFC3339),
		Series:    data.Series,
	}

	return writeJSON(filepath.Join(outputDir, "data", "series", "index.json"), index)
}

func generateSeasonsIndex(data *IndexData, outputDir string) error {
	type SeasonsIndex struct {
		UpdatedAt string       `json:"updated_at"`
		Seasons   []SeasonData `json:"seasons"`
	}

	index := SeasonsIndex{
		UpdatedAt: data.GeneratedAt.Format(time.RFC3339),
		Seasons:   data.Seasons,
	}

	return writeJSON(filepath.Join(outputDir, "data", "seasons", "index.json"), index)
}

func generatePredictionsIndex(data *IndexData, outputDir string) error {
	type PredictionsLeaderboard struct {
		UpdatedAt string           `json:"updated_at"`
		Entries   []PredictorStats `json:"entries"`
	}

	index := PredictionsLeaderboard{
		UpdatedAt: data.GeneratedAt.Format(time.RFC3339),
		Entries:   data.TopPredictors,
	}

	return writeJSON(filepath.Join(outputDir, "data", "predictions", "leaderboard.json"), index)
}

func generatePlaylists(data *IndexData, outputDir string, botNameMap map[string]string) error {
	playlistsDir := filepath.Join(outputDir, "data", "playlists")

	// Closest finishes: matches with smallest score differential
	closest := filterMatches(data.Matches, func(m MatchData) bool {
		if len(m.Participants) < 2 {
			return false
		}
		// Check if score difference is small (1-2 points)
		minDiff := 999
		for i, p1 := range m.Participants {
			for _, p2 := range m.Participants[i+1:] {
				diff := abs(p1.Score - p2.Score)
				if diff < minDiff {
					minDiff = diff
				}
			}
		}
		return minDiff <= 2
	})
	if err := writePlaylist(playlistsDir, "closest-finishes.json", "Closest Finishes", closest, data); err != nil {
		return err
	}

	// Biggest upsets: lower-rated bot won
	// This would need rating data at match time, simplified here
	upsets := filterMatches(data.Matches, func(m MatchData) bool {
		// Simplified: check if winner had fewer wins overall
		if m.WinnerID == "" {
			return false
		}
		return true // Placeholder - would need actual rating delta
	})
	if err := writePlaylist(playlistsDir, "biggest-upsets.json", "Biggest Upsets", upsets[:min(20, len(upsets))], data); err != nil {
		return err
	}

	// Best comebacks: winner had low win probability at some point
	// Would need win probability data - placeholder
	comebacks := filterMatches(data.Matches, func(m MatchData) bool {
		return false // Placeholder - needs win_prob data
	})
	if err := writePlaylist(playlistsDir, "best-comebacks.json", "Best Comebacks", comebacks, data); err != nil {
		return err
	}

	// Featured: recent high-profile matches
	featured := data.Matches[:min(20, len(data.Matches))]
	if err := writePlaylist(playlistsDir, "featured.json", "Featured Matches", featured, data); err != nil {
		return err
	}

	return nil
}

type Playlist struct {
	Slug        string         `json:"slug"`
	Title       string         `json:"title"`
	Description string         `json:"description"`
	UpdatedAt   string         `json:"updated_at"`
	Matches     []MatchSummary `json:"matches"`
}

func writePlaylist(dir, filename, title string, matches []MatchData, data *IndexData) error {
	summaries := make([]MatchSummary, 0, len(matches))
	for _, m := range matches {
		summaries = append(summaries, matchToSummary(m, data))
	}

	playlist := Playlist{
		Slug:        filename[:len(filename)-5], // remove .json
		Title:       title,
		Description: fmt.Sprintf("Auto-curated playlist: %s", title),
		UpdatedAt:   data.GeneratedAt.Format(time.RFC3339),
		Matches:     summaries,
	}

	return writeJSON(filepath.Join(dir, filename), playlist)
}

func filterMatches(matches []MatchData, pred func(MatchData) bool) []MatchData {
	result := make([]MatchData, 0)
	for _, m := range matches {
		if pred(m) {
			result = append(result, m)
		}
	}
	return result
}

func writeJSON(path string, data interface{}) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	return enc.Encode(data)
}

func round1(v float64) float64 {
	return float64(int(v*10+0.5)) / 10
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
