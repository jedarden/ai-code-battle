package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
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
	DebugPublic      bool                   `json:"debug_public"`
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
	Enriched     bool                 `json:"enriched"`
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
func generateAllIndexes(data *IndexData, outputDir string, db *sql.DB, cfg *Config) error {
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
	if err := generateBotProfiles(data, outputDir, cfg); err != nil {
		return fmt.Errorf("bot profiles: %w", err)
	}

	// Generate matches/index.json
	if err := generateMatchIndex(data, outputDir, botNameMap, cfg); err != nil {
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

	// Generate predictions/open.json
	if err := generatePredictionsOpen(data, outputDir); err != nil {
		return fmt.Errorf("predictions open: %w", err)
	}

	// Generate rivalries (data/meta/rivalries.json)
	rivalries := computeRivalries(data, botNameMap)
	if err := generateRivalriesIndex(rivalries, outputDir); err != nil {
		return fmt.Errorf("rivalries index: %w", err)
	}

	// Generate playlists
	if err := generatePlaylists(data, outputDir, botNameMap); err != nil {
		return fmt.Errorf("playlists: %w", err)
	}

	// Persist playlists to DB for incremental queries and R2 pruning exemptions
	if db != nil {
		if err := persistGeneratedPlaylists(context.Background(), db, outputDir); err != nil {
			// Non-fatal: playlists are still written as JSON files
			fmt.Fprintf(os.Stderr, "persist playlists to DB: %v\n", err)
		}
	}

	// Generate maps/index.json and maps/{map_id}.json
	if err := generateMapsIndex(data, outputDir); err != nil {
		return fmt.Errorf("maps index: %w", err)
	}

	// Generate archetypes (data/meta/archetypes.json)
	if err := generateArchetypes(data, outputDir); err != nil {
		return fmt.Errorf("archetypes: %w", err)
	}

	// Generate community hints (data/evolution/community_hints.json)
	if err := generateCommunityHints(data, outputDir); err != nil {
		return fmt.Errorf("community hints: %w", err)
	}

	// Generate per-match feedback (data/matches/{id}/feedback.json)
	if err := generateMatchFeedback(data, outputDir); err != nil {
		return fmt.Errorf("match feedback: %w", err)
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

func generateBotProfiles(data *IndexData, outputDir string, cfg *Config) error {
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
				summary := matchToSummary(m, data, cfg)
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
			DebugPublic:      bot.DebugPublic,
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

func generateMatchIndex(data *IndexData, outputDir string, botNameMap map[string]string, cfg *Config) error {
	summaries := make([]MatchSummary, 0, len(data.Matches))
	for _, m := range data.Matches {
		summaries = append(summaries, matchToSummary(m, data, cfg))
	}

	index := MatchIndex{
		UpdatedAt: data.GeneratedAt.Format(time.RFC3339),
		Matches:   summaries,
	}

	return writeJSON(filepath.Join(outputDir, "data", "matches", "index.json"), index)
}

func matchToSummary(m MatchData, data *IndexData, cfg *Config) MatchSummary {
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

	enriched := isMatchEnriched(m.ID, cfg)

	return MatchSummary{
		ID:           m.ID,
		CompletedAt:  m.CompletedAt.Format(time.RFC3339),
		Participants: participants,
		WinnerID:     m.WinnerID,
		Turns:        m.TurnCount,
		EndReason:    m.EndCondition,
		Enriched:     enriched,
	}
}

// isMatchEnriched checks if a match has AI commentary available on R2.
// Returns true if the commentary file exists in R2.
func isMatchEnriched(matchID string, cfg *Config) bool {
	if cfg == nil || cfg.R2Endpoint == "" || cfg.R2BucketName == "" {
		return false
	}

	r2Client, err := getR2Client(cfg)
	if err != nil {
		return false
	}

	commentaryKey := fmt.Sprintf("commentary/%s.json", matchID)
	exists, err := r2Client.objectExists(context.Background(), commentaryKey)
	if err != nil {
		return false
	}
	return exists
}

func generateSeriesIndex(data *IndexData, outputDir string) error {
	seriesDir := filepath.Join(outputDir, "data", "series")

	for _, s := range data.Series {
		if err := writeJSON(filepath.Join(seriesDir, fmt.Sprintf("%d.json", s.ID)), s); err != nil {
			return err
		}
	}

	type SeriesIndex struct {
		UpdatedAt string       `json:"updated_at"`
		Series    []SeriesData `json:"series"`
	}

	index := SeriesIndex{
		UpdatedAt: data.GeneratedAt.Format(time.RFC3339),
		Series:    data.Series,
	}

	return writeJSON(filepath.Join(seriesDir, "index.json"), index)
}

func generateSeasonsIndex(data *IndexData, outputDir string) error {
	seasonsDir := filepath.Join(outputDir, "data", "seasons")

	for _, s := range data.Seasons {
		if err := writeJSON(filepath.Join(seasonsDir, fmt.Sprintf("%d.json", s.ID)), s); err != nil {
			return err
		}
	}

	var activeSeason *SeasonData
	for i := range data.Seasons {
		if data.Seasons[i].Status == "active" {
			activeSeason = &data.Seasons[i]
			break
		}
	}

	type SeasonsIndex struct {
		UpdatedAt    string       `json:"updated_at"`
		ActiveSeason *SeasonData  `json:"active_season"`
		Seasons      []SeasonData `json:"seasons"`
	}

	index := SeasonsIndex{
		UpdatedAt:    data.GeneratedAt.Format(time.RFC3339),
		ActiveSeason: activeSeason,
		Seasons:      data.Seasons,
	}

	return writeJSON(filepath.Join(seasonsDir, "index.json"), index)
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

// generatePredictionsOpen creates data/predictions/open.json with upcoming
// predictable matches (top-20 vs top-20, rivalry matches, series games,
// evolved bot vs top-10).
func generatePredictionsOpen(data *IndexData, outputDir string) error {
	type OpenMatchEntry struct {
		MatchID           string  `json:"match_id"`
		BotA              string  `json:"bot_a"`
		BotB              string  `json:"bot_b"`
		ARating           int     `json:"a_rating"`
		BRating           int     `json:"b_rating"`
		OpenUntil         string  `json:"open_until"`
		HeadToHeadRecord  *string `json:"head_to_head_record,omitempty"`
	}

	type OpenPredictionsIndex struct {
		UpdatedAt string           `json:"updated_at"`
		Matches   []OpenMatchEntry `json:"matches"`
	}

	entries := make([]OpenMatchEntry, 0, len(data.OpenPredictionMatches))
	for _, m := range data.OpenPredictionMatches {
		// Open until 5 minutes after creation (typical execution time)
		openUntil := m.CreatedAt.Add(5 * time.Minute).Format(time.RFC3339)

		entries = append(entries, OpenMatchEntry{
			MatchID:          m.MatchID,
			BotA:             m.BotAName,
			BotB:             m.BotBName,
			ARating:          int(m.ARating),
			BRating:          int(m.BRating),
			OpenUntil:        openUntil,
			HeadToHeadRecord: m.HeadToHeadRecord,
		})
	}

	index := OpenPredictionsIndex{
		UpdatedAt: data.GeneratedAt.Format(time.RFC3339),
		Matches:   entries,
	}

	return writeJSON(filepath.Join(outputDir, "data", "predictions", "open.json"), index)
}

func generatePlaylists(data *IndexData, outputDir string, botNameMap map[string]string) error {
	playlistsDir := filepath.Join(outputDir, "data", "playlists")

	// Pre-build lookup maps for O(1) playlist curation instead of O(n^2) per match.
	firstMatchPerBot := buildFirstMatchPerBot(data.Matches)
	pairFrequency := buildPairFrequency(data.Matches)

	type playlistDef struct {
		slug        string
		title       string
		description string
		category    string
		filter      func(MatchData) bool
		sort        func([]MatchData)
	}

	defs := []playlistDef{
		{
			slug:        "closest-finishes",
			title:       "Closest Finishes",
			description: "Matches decided by the thinnest margins — nail-biters to the very end",
			category:    "close_games",
			filter: func(m MatchData) bool {
				if len(m.Participants) < 2 || m.WinnerID == "" {
					return false
				}
				return minScoreDiff(m) <= 2
			},
			sort: func(matches []MatchData) {
				sortByScoreDiff(matches)
			},
		},
		{
			slug:        "biggest-upsets",
			title:       "Biggest Upsets",
			description: "Lower-rated bots triumph against higher-rated opponents",
			category:    "upsets",
			filter: func(m MatchData) bool {
				if m.WinnerID == "" || len(m.Participants) < 2 {
					return false
				}
				return ratingUpsetMagnitude(m) >= 100
			},
			sort: func(matches []MatchData) {
				sortByUpsetMagnitude(matches)
			},
		},
		{
			slug:        "best-comebacks",
			title:       "Best Comebacks",
			description: "Bots that were down but never out — dramatic turnarounds and improbable victories",
			category:    "comebacks",
			filter: func(m MatchData) bool {
				return isComeback(m)
			},
			sort: func(matches []MatchData) {
				sortSlice(matches, func(i, j int) bool {
					return turnaroundMagnitude(matches[i]) > turnaroundMagnitude(matches[j])
				})
			},
		},
		{
			slug:        "marathon-matches",
			title:       "Marathon Matches",
			description: "The longest, most grueling matches — endurance-tested battles",
			category:    "long_games",
			filter: func(m MatchData) bool {
				return m.TurnCount >= 300
			},
			sort: func(matches []MatchData) {
				sortByTurnCount(matches)
			},
		},
		{
			slug:        "highest-rated",
			title:       "Clash of Titans",
			description: "Matches between the highest-rated opponents on the ladder",
			category:    "featured",
			filter: func(m MatchData) bool {
				if len(m.Participants) < 2 {
					return false
				}
				return combinedRating(m) >= 3200
			},
			sort: func(matches []MatchData) {
				sortByCombinedRating(matches)
			},
		},
		{
			slug:        "evolution-breakthroughs",
			title:       "Evolution Breakthroughs",
			description: "Evolved bots defeating top-rated opponents — AI strategy milestones",
			category:    "featured",
			filter: func(m MatchData) bool {
				return isEvolutionBreakthrough(m, data)
			},
			sort: func(matches []MatchData) {
				sortByUpsetMagnitude(matches)
			},
		},
		{
			slug:        "rivalry-classics",
			title:       "Rivalry Classics",
			description: "The most closely contested matchups between frequent opponents",
			category:    "rivalry",
			filter: func(m MatchData) bool {
				return isRivalryMatchFast(m, pairFrequency)
			},
			sort: func(matches []MatchData) {
				sortSlice(matches, func(i, j int) bool {
					return minScoreDiff(matches[i]) < minScoreDiff(matches[j])
				})
			},
		},
		{
			slug:        "domination",
			title:       "Total Domination",
			description: "One-sided victories where the winner crushed all opposition",
			category:    "domination",
			filter: func(m MatchData) bool {
				if m.WinnerID == "" || len(m.Participants) < 2 {
					return false
				}
				return maxScoreDiff(m) >= 5
			},
			sort: func(matches []MatchData) {
				sortSlice(matches, func(i, j int) bool {
					return maxScoreDiff(matches[i]) > maxScoreDiff(matches[j])
				})
			},
		},
		{
			slug:        "new-bot-debuts",
			title:       "New Bot Debuts",
			description: "First matches of newly registered bots — watch their opening games",
			category:    "tutorial",
			filter: func(m MatchData) bool {
				return isNewBotDebutFast(m, firstMatchPerBot)
			},
			sort: func(matches []MatchData) {
				// Newest debuts first
				sortSlice(matches, func(i, j int) bool {
					return matches[i].CompletedAt.After(matches[j].CompletedAt)
				})
			},
		},
		{
			slug:        "season-highlights",
			title:       "Season Highlights",
			description: "Top matches from the current season ranked by excitement",
			category:    "season",
			filter: func(m MatchData) bool {
				return isCurrentSeasonMatch(m, data)
			},
			sort: func(matches []MatchData) {
				sortByInterestScore(matches)
			},
		},
		{
			slug:        "featured",
			title:       "Featured Matches",
			description: "Recent highlights from the ladder",
			category:    "featured",
			filter: func(m MatchData) bool {
				return m.WinnerID != ""
			},
			sort: func(matches []MatchData) {
				// Most recent first (already sorted by completed_at DESC from DB)
			},
		},
		{
			slug:        "best-of-week",
			title:       "Best of the Week",
			description: "This week's top matches ranked by excitement: close finishes, upsets, marathon battles, and elite clashes",
			category:    "weekly",
			filter: func(m MatchData) bool {
				weekAgo := data.GeneratedAt.AddDate(0, 0, -7)
				return m.CompletedAt.After(weekAgo) && m.WinnerID != ""
			},
			sort: func(matches []MatchData) {
				sortByInterestScore(matches)
			},
		},
	}

	var summaries []PlaylistSummary

	for _, def := range defs {
		// Special handling for best-of-week: use curated selection with tags
		if def.slug == "best-of-week" {
			weekAgo := data.GeneratedAt.AddDate(0, 0, -7)
			curated := curateWeeklyHighlights(data.Matches, weekAgo)
			curatedMatches := make([]MatchData, 0, len(curated))
			tags := make(map[string]string, len(curated))
			for _, c := range curated {
				curatedMatches = append(curatedMatches, c.Match)
				tags[c.Match.ID] = c.Tag
			}

			if err := writePlaylistWithTags(playlistsDir, def.slug+".json", def.title, def.description, def.category, curatedMatches, tags, data); err != nil {
				return err
			}

			var thumbMatchID string
			if len(curatedMatches) > 0 {
				thumbMatchID = curatedMatches[0].ID
			}
			summaries = append(summaries, PlaylistSummary{
				Slug:            def.slug,
				Title:           def.title,
				Description:     def.description,
				Category:        def.category,
				MatchCount:      len(curatedMatches),
				UpdatedAt:       data.GeneratedAt.Format(time.RFC3339),
				ThumbnailMatchID: thumbMatchID,
			})
			continue
		}

		filtered := filterMatches(data.Matches, def.filter)
		if def.sort != nil {
			def.sort(filtered)
		}
		filtered = filtered[:min(20, len(filtered))]

		if err := writePlaylist(playlistsDir, def.slug+".json", def.title, def.description, def.category, filtered, data); err != nil {
			return err
		}

		var thumbMatchID string
		if len(filtered) > 0 {
			thumbMatchID = filtered[0].ID
		}
		summaries = append(summaries, PlaylistSummary{
			Slug:            def.slug,
			Title:           def.title,
			Description:     def.description,
			Category:        def.category,
			MatchCount:      len(filtered),
			UpdatedAt:       data.GeneratedAt.Format(time.RFC3339),
			ThumbnailMatchID: thumbMatchID,
		})
	}

	index := PlaylistIndex{
		UpdatedAt: data.GeneratedAt.Format(time.RFC3339),
		Playlists: summaries,
	}
	return writeJSON(filepath.Join(playlistsDir, "index.json"), index)
}

type PlaylistIndex struct {
	UpdatedAt string             `json:"updated_at"`
	Playlists []PlaylistSummary  `json:"playlists"`
}

type PlaylistSummary struct {
	Slug             string `json:"slug"`
	Title            string `json:"title"`
	Description      string `json:"description"`
	Category         string `json:"category"`
	MatchCount       int    `json:"match_count"`
	UpdatedAt        string `json:"updated_at"`
	ThumbnailMatchID string `json:"thumbnail_match_id,omitempty"`
}

type Playlist struct {
	Slug        string          `json:"slug"`
	Title       string          `json:"title"`
	Description string          `json:"description"`
	Category    string          `json:"category"`
	MatchCount  int             `json:"match_count"`
	CreatedAt   string          `json:"created_at"`
	UpdatedAt   string          `json:"updated_at"`
	Matches     []PlaylistMatch `json:"matches"`
}

type PlaylistMatch struct {
	MatchID      string                    `json:"match_id"`
	Order        int                       `json:"order"`
	Title        string                    `json:"title,omitempty"`
	ThumbnailURL string                    `json:"thumbnail_url,omitempty"`
	CurationTag  string                    `json:"curation_tag,omitempty"`
	Participants []MatchParticipantSummary `json:"participants,omitempty"`
	Score        string                    `json:"score,omitempty"`
	Turns        int                       `json:"turns,omitempty"`
	EndReason    string                    `json:"end_reason,omitempty"`
	CompletedAt  string                    `json:"completed_at,omitempty"`
}

// curatedWeeklyMatch is a match selected by the weekly curation algorithm
// with a tag explaining why it was selected.
type curatedWeeklyMatch struct {
	Match MatchData
	Tag   string
}

// curateWeeklyHighlights selects the best matches from the past 7 days
// using explicit criteria: upsets, elite clashes, marathon battles, closest finishes.
// It processes specific criteria first so distinctive matches aren't consumed
// by generic tags. It returns deduplicated matches tagged with their selection reason.
func curateWeeklyHighlights(matches []MatchData, cutoff time.Time) []curatedWeeklyMatch {
	seen := make(map[string]string) // match_id -> tag (first selection reason)
	maxPerCriterion := 7

	recent := filterMatches(matches, func(m MatchData) bool {
		return m.CompletedAt.After(cutoff) && m.WinnerID != "" && len(m.Participants) >= 2
	})

	// 1. Biggest upsets first (most distinctive — underdog victories)
	upsetMatches := make([]MatchData, len(recent))
	copy(upsetMatches, recent)
	sortByUpsetMagnitude(upsetMatches)
	for i, m := range upsetMatches {
		if i >= maxPerCriterion {
			break
		}
		mag := ratingUpsetMagnitude(m)
		if mag < 50 {
			continue
		}
		if _, exists := seen[m.ID]; !exists {
			seen[m.ID] = fmt.Sprintf("Upset victory (underdog by %d rating)", mag)
		}
	}

	// 2. Highest-rated opponents (elite clashes)
	ratedMatches := make([]MatchData, len(recent))
	copy(ratedMatches, recent)
	sortByCombinedRating(ratedMatches)
	for i, m := range ratedMatches {
		if i >= maxPerCriterion {
			break
		}
		cr := int(combinedRating(m))
		if cr < 3000 {
			continue
		}
		if _, exists := seen[m.ID]; !exists {
			seen[m.ID] = fmt.Sprintf("Elite clash (combined rating: %d)", cr)
		}
	}

	// 3. Most turns (longest endurance battles)
	longMatches := make([]MatchData, len(recent))
	copy(longMatches, recent)
	sortByTurnCount(longMatches)
	for i, m := range longMatches {
		if i >= maxPerCriterion {
			break
		}
		if m.TurnCount < 300 {
			continue
		}
		if _, exists := seen[m.ID]; !exists {
			seen[m.ID] = fmt.Sprintf("Marathon battle (%d turns)", m.TurnCount)
		}
	}

	// 4. Closest results last (most generic — fills remaining slots)
	closeMatches := make([]MatchData, len(recent))
	copy(closeMatches, recent)
	sortByScoreDiff(closeMatches)
	for i, m := range closeMatches {
		if i >= maxPerCriterion {
			break
		}
		diff := minScoreDiff(m)
		if _, exists := seen[m.ID]; !exists {
			seen[m.ID] = fmt.Sprintf("Closest finish (score diff: %d)", diff)
		}
	}

	// Build result in criterion order: upsets, elite, marathon, closest
	var result []curatedWeeklyMatch
	for _, m := range recent {
		if tag, ok := seen[m.ID]; ok {
			result = append(result, curatedWeeklyMatch{Match: m, Tag: tag})
		}
	}

	if len(result) > 20 {
		result = result[:20]
	}

	return result
}

func writePlaylist(dir, filename, title, description, category string, matches []MatchData, data *IndexData) error {
	slug := filename[:len(filename)-5]
	pm := make([]PlaylistMatch, 0, len(matches))
	for i, m := range matches {
		pm = append(pm, buildPlaylistMatch(m, i, data, ""))
	}

	playlist := Playlist{
		Slug:        slug,
		Title:       title,
		Description: description,
		Category:    category,
		MatchCount:  len(pm),
		CreatedAt:   data.GeneratedAt.Format(time.RFC3339),
		UpdatedAt:   data.GeneratedAt.Format(time.RFC3339),
		Matches:     pm,
	}

	return writeJSON(filepath.Join(dir, filename), playlist)
}

func writePlaylistWithTags(dir, filename, title, description, category string, matches []MatchData, tags map[string]string, data *IndexData) error {
	slug := filename[:len(filename)-5]
	pm := make([]PlaylistMatch, 0, len(matches))
	for i, m := range matches {
		pm = append(pm, buildPlaylistMatch(m, i, data, tags[m.ID]))
	}

	playlist := Playlist{
		Slug:        slug,
		Title:       title,
		Description: description,
		Category:    category,
		MatchCount:  len(pm),
		CreatedAt:   data.GeneratedAt.Format(time.RFC3339),
		UpdatedAt:   data.GeneratedAt.Format(time.RFC3339),
		Matches:     pm,
	}

	return writeJSON(filepath.Join(dir, filename), playlist)
}

func formatMatchTitle(m MatchData, data *IndexData) string {
	names := make([]string, 0, len(m.Participants))
	scores := make([]int, 0, len(m.Participants))
	for _, p := range m.Participants {
		name := "Unknown"
		for _, bot := range data.Bots {
			if bot.ID == p.BotID {
				name = bot.Name
				break
			}
		}
		names = append(names, name)
		scores = append(scores, p.Score)
	}
	if len(names) == 2 {
		return fmt.Sprintf("%s %d – %d %s", names[0], scores[0], scores[1], names[1])
	}
	return fmt.Sprintf("%s (%d players)", m.ID[:min(8, len(m.ID))], len(names))
}

func buildPlaylistMatch(m MatchData, order int, data *IndexData, curationTag string) PlaylistMatch {
	participants := make([]MatchParticipantSummary, 0, len(m.Participants))
	scoreParts := make([]string, 0, len(m.Participants))
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
		scoreParts = append(scoreParts, fmt.Sprintf("%d", p.Score))
	}
	title := formatMatchTitle(m, data)
	completedAt := ""
	if !m.CompletedAt.IsZero() {
		completedAt = m.CompletedAt.Format(time.RFC3339)
	}
	thumbnailURL := fmt.Sprintf("https://r2.aicodebattle.com/thumbnails/%s.png", m.ID)
	return PlaylistMatch{
		MatchID:      m.ID,
		Order:        order,
		Title:        title,
		ThumbnailURL: thumbnailURL,
		CurationTag:  curationTag,
		Participants: participants,
		Score:        strings.Join(scoreParts, "-"),
		Turns:        m.TurnCount,
		EndReason:    m.EndCondition,
		CompletedAt:  completedAt,
	}
}

func ratingUpsetMagnitude(m MatchData) int {
	if m.WinnerID == "" || len(m.Participants) < 2 {
		return 0
	}
	var winnerRating, bestLoserRating float64
	found := false
	for _, p := range m.Participants {
		if p.BotID == m.WinnerID {
			winnerRating = p.PreMatchRating
			found = true
		}
	}
	if !found || winnerRating == 0 {
		return 0
	}
	for _, p := range m.Participants {
		if p.BotID != m.WinnerID && p.PreMatchRating > bestLoserRating {
			bestLoserRating = p.PreMatchRating
		}
	}
	if bestLoserRating == 0 {
		return 0
	}
	return int(bestLoserRating - winnerRating)
}

func combinedRating(m MatchData) float64 {
	total := 0.0
	for _, p := range m.Participants {
		total += p.PreMatchRating
	}
	return total
}

func interestScore(m MatchData) float64 {
	score := 0.0
	// Close finishes are interesting
	if len(m.Participants) >= 2 {
		minDiff := 999
		for i, p1 := range m.Participants {
			for _, p2 := range m.Participants[i+1:] {
				diff := abs(p1.Score - p2.Score)
				if diff < minDiff {
					minDiff = diff
				}
			}
		}
		if minDiff <= 1 {
			score += 3.0
		} else if minDiff <= 2 {
			score += 2.0
		}
	}
	// Upsets are interesting
	upset := ratingUpsetMagnitude(m)
	if upset >= 200 {
		score += 4.0
	} else if upset >= 100 {
		score += 2.0
	}
	// Long matches are interesting
	if m.TurnCount >= 400 {
		score += 2.0
	} else if m.TurnCount >= 300 {
		score += 1.0
	}
	// High-rated opponents
	cr := combinedRating(m)
	if cr >= 3400 {
		score += 2.0
	} else if cr >= 3200 {
		score += 1.0
	}
	return score
}

func sortByScoreDiff(matches []MatchData) {
	sortSlice(matches, func(i, j int) bool {
		return minScoreDiff(matches[i]) < minScoreDiff(matches[j])
	})
}

func sortByUpsetMagnitude(matches []MatchData) {
	sortSlice(matches, func(i, j int) bool {
		return ratingUpsetMagnitude(matches[i]) > ratingUpsetMagnitude(matches[j])
	})
}

func sortByTurnCount(matches []MatchData) {
	sortSlice(matches, func(i, j int) bool {
		return matches[i].TurnCount > matches[j].TurnCount
	})
}

func sortByCombinedRating(matches []MatchData) {
	sortSlice(matches, func(i, j int) bool {
		return combinedRating(matches[i]) > combinedRating(matches[j])
	})
}

func sortByInterestScore(matches []MatchData) {
	sortSlice(matches, func(i, j int) bool {
		return interestScore(matches[i]) > interestScore(matches[j])
	})
}

func minScoreDiff(m MatchData) int {
	minDiff := 999
	for i, p1 := range m.Participants {
		for _, p2 := range m.Participants[i+1:] {
			diff := abs(p1.Score - p2.Score)
			if diff < minDiff {
				minDiff = diff
			}
		}
	}
	return minDiff
}

func sortSlice[T any](s []T, less func(i, j int) bool) {
	for i := 0; i < len(s)-1; i++ {
		for j := i + 1; j < len(s); j++ {
			if less(j, i) {
				s[i], s[j] = s[j], s[i]
			}
		}
	}
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

// maxScoreDiff returns the maximum score difference between winner and any loser
func maxScoreDiff(m MatchData) int {
	if m.WinnerID == "" || len(m.Participants) < 2 {
		return 0
	}
	var winnerScore int
	for _, p := range m.Participants {
		if p.BotID == m.WinnerID {
			winnerScore = p.Score
			break
		}
	}
	maxDiff := 0
	for _, p := range m.Participants {
		if p.BotID != m.WinnerID {
			diff := winnerScore - p.Score
			if diff > maxDiff {
				maxDiff = diff
			}
		}
	}
	return maxDiff
}

// isNewBotDebut detects the first match of each bot by finding the earliest
// completed match for each bot.
func isNewBotDebut(m MatchData, data *IndexData) bool {
	if m.WinnerID == "" {
		return false
	}
	for _, p := range m.Participants {
		earliest := true
		for _, other := range data.Matches {
			if other.ID == m.ID || other.CompletedAt.IsZero() {
				continue
			}
			for _, op := range other.Participants {
				if op.BotID == p.BotID {
					if other.CompletedAt.Before(m.CompletedAt) {
						earliest = false
					}
				}
			}
		}
		if earliest {
			return true
		}
	}
	return false
}

// isCurrentSeasonMatch checks if a match belongs to the current active season.
func isCurrentSeasonMatch(m MatchData, data *IndexData) bool {
	for _, s := range data.Seasons {
		if s.Status != "active" {
			continue
		}
		// Check if match falls within season date range
		if m.CompletedAt.After(s.StartsAt) || m.CompletedAt.Equal(s.StartsAt) {
			if s.EndsAt.IsZero() || m.CompletedAt.Before(s.EndsAt) {
				return m.WinnerID != ""
			}
		}
	}
	return false
}

// isComeback detects matches where the winner was behind on score at some point
// but rallied to win. Uses a heuristic: winner scored more than loser despite
// having a lower pre-match rating (unlikely comeback) or the match had many turns
// (late-game rally) with a close final score.
func isComeback(m MatchData) bool {
	if m.WinnerID == "" || len(m.Participants) < 2 {
		return false
	}
	// An upset with a close score is a comeback
	upset := ratingUpsetMagnitude(m)
	scoreDiff := minScoreDiff(m)
	return upset >= 80 && scoreDiff <= 3
}

// turnaroundMagnitude measures how dramatic a comeback was.
// Higher = more surprising turnaround.
func turnaroundMagnitude(m MatchData) float64 {
	upset := float64(ratingUpsetMagnitude(m))
	closeFactor := 1.0 / float64(max(minScoreDiff(m), 1))
	turnFactor := float64(m.TurnCount) / 500.0
	return upset*closeFactor + turnFactor*50
}

// isEvolutionBreakthrough detects matches where an evolved bot beat a high-rated opponent.
func isEvolutionBreakthrough(m MatchData, data *IndexData) bool {
	if m.WinnerID == "" || len(m.Participants) < 2 {
		return false
	}
	winnerEvolved := false
	for _, bot := range data.Bots {
		if bot.ID == m.WinnerID && bot.Evolved {
			winnerEvolved = true
		}
	}
	if !winnerEvolved {
		return false
	}
	// Winner must have beaten someone rated >= 1600
	for _, p := range m.Participants {
		if p.BotID != m.WinnerID && p.PreMatchRating >= 1600 && !p.Won {
			return true
		}
	}
	return false
}

// buildFirstMatchPerBot returns a map from botID to the matchID of their earliest
// completed match. O(n*p) where n=matches, p=avg participants.
func buildFirstMatchPerBot(matches []MatchData) map[string]string {
	first := make(map[string]string)
	firstTime := make(map[string]time.Time)
	for _, m := range matches {
		if m.CompletedAt.IsZero() || m.WinnerID == "" {
			continue
		}
		for _, p := range m.Participants {
			if t, ok := firstTime[p.BotID]; !ok || m.CompletedAt.Before(t) {
				firstTime[p.BotID] = m.CompletedAt
				first[p.BotID] = m.ID
			}
		}
	}
	return first
}

// isNewBotDebutFast checks if any participant's earliest completed match is this one,
// using a pre-built lookup map.
func isNewBotDebutFast(m MatchData, firstMatchPerBot map[string]string) bool {
	if m.WinnerID == "" {
		return false
	}
	for _, p := range m.Participants {
		if firstMatchPerBot[p.BotID] == m.ID {
			return true
		}
	}
	return false
}

// buildPairFrequency returns a map from "botA:botB" (sorted) to the count of
// 2-player matches between them. O(n) where n=matches.
func buildPairFrequency(matches []MatchData) map[string]int {
	freq := make(map[string]int)
	for _, m := range matches {
		if len(m.Participants) != 2 {
			continue
		}
		a, b := m.Participants[0].BotID, m.Participants[1].BotID
		if a > b {
			a, b = b, a
		}
		freq[a+":"+b]++
	}
	return freq
}

// isRivalryMatchFast checks if a 2-player match is between frequent opponents,
// using a pre-built pair frequency map.
func isRivalryMatchFast(m MatchData, pairFrequency map[string]int) bool {
	if len(m.Participants) != 2 || m.WinnerID == "" {
		return false
	}
	a, b := m.Participants[0].BotID, m.Participants[1].BotID
	if a > b {
		a, b = b, a
	}
	return pairFrequency[a+":"+b] >= 3
}

// isRivalryMatch detects matches between bots that have played each other frequently.
// Builds a frequency map from all matches and checks if this pair qualifies.
func isRivalryMatch(m MatchData, data *IndexData) bool {
	if len(m.Participants) != 2 || m.WinnerID == "" {
		return false
	}
	a, b := m.Participants[0].BotID, m.Participants[1].BotID
	if a > b {
		a, b = b, a
	}
	pairKey := a + ":" + b

	// Count occurrences of this pair across all matches
	count := 0
	for _, other := range data.Matches {
		if len(other.Participants) != 2 {
			continue
		}
		oa, ob := other.Participants[0].BotID, other.Participants[1].BotID
		if oa > ob {
			oa, ob = ob, oa
		}
		if oa+":"+ob == pairKey {
			count++
		}
	}
	return count >= 3
}

// ─── Rivalry Detection (§13.5) ─────────────────────────────────────────────────

const (
	rivalryMinMatches = 10   // minimum h2h matches to qualify
	rivalryTopK       = 20   // max rivalries to emit
	rivalryRecencyDecay = 0.95 // per-day decay for recency weighting
)

// RivalryEntry represents a detected rivalry pair for data/meta/rivalries.json.
type RivalryEntry struct {
	BotA         RivalryBot    `json:"bot_a"`
	BotB         RivalryBot    `json:"bot_b"`
	TotalMatches int           `json:"matches"`
	Record       RivalryRecord `json:"record"`
	ClosestMatch string        `json:"closest_match,omitempty"`
	LongestStreak *RivalryStreak `json:"longest_streak,omitempty"`
	RecentMatches []string     `json:"recent_matches"`
	Narrative    string        `json:"narrative"`
	Score        float64       `json:"score"`
}

type RivalryBot struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type RivalryRecord struct {
	AWins int `json:"a_wins"`
	BWins int `json:"b_wins"`
	Draws int `json:"draws"`
}

type RivalryStreak struct {
	Holder string `json:"holder"`
	Length int    `json:"length"`
}

// RivalriesIndex is the top-level structure for data/meta/rivalries.json.
type RivalriesIndex struct {
	UpdatedAt string         `json:"updated_at"`
	Rivalries []RivalryEntry `json:"rivalries"`
}

// pairKey returns a canonical key for a bot pair (alphabetically ordered).
func pairKey(a, b string) string {
	if a > b {
		a, b = b, a
	}
	return a + ":" + b
}

type h2hRecord struct {
	botAID, botBID string
	aWins, bWins   int
	draws          int
	matchDates     []time.Time
	matchIDs       []string
	scoreDiffs     []int
	winnerSeq      []string // bot_id of winner per match ("draw" for draws)
}

// computeRivalries builds the h2h matrix from all matches, scores each pair
// by win-rate balance × recency × total matches, and returns the top K.
func computeRivalries(data *IndexData, botNameMap map[string]string) []RivalryEntry {
	// Accumulate head-to-head records (only 2-player matches).
	pairs := make(map[string]*h2hRecord)

	for _, m := range data.Matches {
		if len(m.Participants) != 2 {
			continue
		}
		a, b := m.Participants[0].BotID, m.Participants[1].BotID
		key := pairKey(a, b)

		rec, ok := pairs[key]
		if !ok {
			// Canonical order: alphabetically first is bot A.
			if a > b {
				a, b = b, a
			}
			rec = &h2hRecord{botAID: a, botBID: b}
			pairs[key] = rec
		}

		rec.matchIDs = append(rec.matchIDs, m.ID)
		rec.matchDates = append(rec.matchDates, m.PlayedAt)

		// Score diff for closest match detection.
		if len(m.Participants) == 2 {
			rec.scoreDiffs = append(rec.scoreDiffs, absInt(m.Participants[0].Score-m.Participants[1].Score))
		}

		switch {
		case m.WinnerID == "":
			rec.draws++
			rec.winnerSeq = append(rec.winnerSeq, "draw")
		case m.WinnerID == rec.botAID:
			rec.aWins++
			rec.winnerSeq = append(rec.winnerSeq, rec.botAID)
		default:
			rec.bWins++
			rec.winnerSeq = append(rec.winnerSeq, rec.botBID)
		}
	}

	// Score and rank.
	now := data.GeneratedAt
	var candidates []RivalryEntry

	for _, rec := range pairs {
		total := rec.aWins + rec.bWins + rec.draws
		if total < rivalryMinMatches {
			continue
		}

		// Win-rate balance: 1.0 for perfect 50/50, approaches 0 for dominant pairs.
		balance := 1.0 - float64(absInt(rec.aWins-rec.bWins))/float64(total)

		// Recency: weighted sum where recent matches count more.
		var recencyScore float64
		for _, d := range rec.matchDates {
			daysAgo := now.Sub(d).Hours() / 24
			if daysAgo < 0 {
				daysAgo = 0
			}
			recencyScore += math.Pow(rivalryRecencyDecay, daysAgo)
		}
		// Normalise recency to [0, 1] relative to total matches.
		recencyNorm := recencyScore / float64(total)

		// Final score: balance × recency × log(total) for volume weighting.
		score := balance * recencyNorm * math.Log(float64(total))

		// Closest match: smallest score diff.
		closestMatch := ""
		if len(rec.scoreDiffs) > 0 {
			minDiff := rec.scoreDiffs[0]
			minIdx := 0
			for i, d := range rec.scoreDiffs {
				if d < minDiff {
					minDiff = d
					minIdx = i
				}
			}
			closestMatch = rec.matchIDs[minIdx]
		}

		// Longest win streak.
		streak := longestStreak(rec.winnerSeq, rec.botAID, rec.botBID)

		// Recent match IDs (last 10).
		recentCount := 10
		if len(rec.matchIDs) < recentCount {
			recentCount = len(rec.matchIDs)
		}
		recentMatches := make([]string, recentCount)
		for i := 0; i < recentCount; i++ {
			recentMatches[i] = rec.matchIDs[len(rec.matchIDs)-1-i]
		}

		aName := botNameMap[rec.botAID]
		bName := botNameMap[rec.botBID]

		candidates = append(candidates, RivalryEntry{
			BotA: RivalryBot{ID: rec.botAID, Name: aName},
			BotB: RivalryBot{ID: rec.botBID, Name: bName},
			TotalMatches: total,
			Record: RivalryRecord{AWins: rec.aWins, BWins: rec.bWins, Draws: rec.draws},
			ClosestMatch: closestMatch,
			LongestStreak: streak,
			RecentMatches: recentMatches,
			Narrative: buildRivalryNarrative(aName, bName, total, rec.aWins, rec.bWins, rec.draws, streak),
			Score: score,
		})
	}

	// Sort by score descending.
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Score > candidates[j].Score
	})

	if len(candidates) > rivalryTopK {
		candidates = candidates[:rivalryTopK]
	}

	return candidates
}

// longestStreak finds the longest consecutive win streak for either bot in the winner sequence.
func longestStreak(winners []string, botA, botB string) *RivalryStreak {
	if len(winners) == 0 {
		return nil
	}

	var bestHolder string
	var bestLen int
	var curHolder string
	var curLen int

	for _, w := range winners {
		if w == "draw" {
			curLen = 0
			curHolder = ""
			continue
		}
		if w == curHolder {
			curLen++
		} else {
			curHolder = w
			curLen = 1
		}
		if curLen > bestLen {
			bestLen = curLen
			bestHolder = curHolder
		}
	}

	if bestLen < 2 {
		return nil
	}
	return &RivalryStreak{Holder: bestHolder, Length: bestLen}
}

// buildRivalryNarrative generates a template-based narrative from rivalry stats.
func buildRivalryNarrative(aName, bName string, total, aWins, bWins, draws int, streak *RivalryStreak) string {
	leading := aName
	trailing := bName
	leadWins := aWins
	trailWins := bWins
	if bWins > aWins {
		leading, trailing = trailing, leading
		leadWins, trailWins = trailWins, leadWins
	}

	switch {
	case aWins == bWins:
		return fmt.Sprintf("%s and %s have met %d times — the series is dead even at %d-%d%s. Every match shifts the balance.",
			aName, bName, total, aWins, bWins, drawSuffix(draws))
	case streak != nil && streak.Length >= 3:
		return fmt.Sprintf("%s and %s have met %d times with %s holding a %d-%d edge. %s is currently on a %d-match winning streak.",
			aName, bName, total, leading, leadWins, trailWins, streak.Holder, streak.Length)
	default:
		return fmt.Sprintf("%s and %s have met %d times — %s leads the series %d-%d%s. A rivalry defined by closely contested grid battles.",
			aName, bName, total, leading, leadWins, trailWins, drawSuffix(draws))
	}
}

func drawSuffix(draws int) string {
	if draws == 0 {
		return ""
	}
	return fmt.Sprintf(" (%d draw%s)", draws, pluralS(draws))
}

func pluralS(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

func absInt(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// generateRivalriesIndex writes data/meta/rivalries.json.
func generateRivalriesIndex(rivalries []RivalryEntry, outputDir string) error {
	metaDir := filepath.Join(outputDir, "data", "meta")
	if err := os.MkdirAll(metaDir, 0755); err != nil {
		return err
	}

	index := RivalriesIndex{
		UpdatedAt: time.Now().UTC().Format(time.RFC3339),
		Rivalries: rivalries,
	}
	return writeJSON(filepath.Join(metaDir, "rivalries.json"), index)
}

// mapPosition is a grid coordinate used in map geometry output.
type mapPosition struct {
	Row int `json:"row"`
	Col int `json:"col"`
}

// mapCore is a spawn/core point in a map.
type mapCore struct {
	Position mapPosition `json:"position"`
	Owner    int         `json:"owner"`
}

// mapGeometryJSON mirrors the map_json column structure for geometry extraction.
type mapGeometryJSON struct {
	Walls       []mapPosition `json:"walls"`
	Cores       []mapCore     `json:"cores"`
	EnergyNodes []mapPosition `json:"energy_nodes"`
}

// MapIndexEntry is the per-map summary in maps/index.json.
type MapIndexEntry struct {
	MapID       string  `json:"map_id"`
	PlayerCount int     `json:"player_count"`
	Status      string  `json:"status"`
	Engagement  float64 `json:"engagement"`
	WallDensity float64 `json:"wall_density"`
	EnergyCount int     `json:"energy_count"`
	GridWidth   int     `json:"grid_width"`
	GridHeight  int     `json:"grid_height"`
	CreatedAt   string  `json:"created_at"`
}

// MapIndexFile represents maps/index.json.
type MapIndexFile struct {
	UpdatedAt     string                     `json:"updated_at"`
	Maps          []MapIndexEntry            `json:"maps"`
	ByPlayerCount map[string][]MapIndexEntry `json:"by_player_count"`
}

// MapDetail represents maps/{map_id}.json — summary metadata plus full geometry.
type MapDetail struct {
	MapID       string        `json:"map_id"`
	PlayerCount int           `json:"player_count"`
	Status      string        `json:"status"`
	Engagement  float64       `json:"engagement"`
	WallDensity float64       `json:"wall_density"`
	EnergyCount int           `json:"energy_count"`
	GridWidth   int           `json:"grid_width"`
	GridHeight  int           `json:"grid_height"`
	CreatedAt   string        `json:"created_at"`
	Walls       []mapPosition `json:"walls"`
	Cores       []mapCore     `json:"cores"`
	EnergyNodes []mapPosition `json:"energy_nodes"`
}

func generateMapsIndex(data *IndexData, outputDir string) error {
	mapsDir := filepath.Join(outputDir, "maps")
	if err := os.MkdirAll(mapsDir, 0755); err != nil {
		return err
	}

	entries := make([]MapIndexEntry, 0, len(data.Maps))
	byPlayerCount := make(map[string][]MapIndexEntry)

	for _, m := range data.Maps {
		entry := MapIndexEntry{
			MapID:       m.MapID,
			PlayerCount: m.PlayerCount,
			Status:      m.Status,
			Engagement:  m.Engagement,
			WallDensity: m.WallDensity,
			EnergyCount: m.EnergyCount,
			GridWidth:   m.GridWidth,
			GridHeight:  m.GridHeight,
			CreatedAt:   m.CreatedAt.Format(time.RFC3339),
		}
		entries = append(entries, entry)
		key := fmt.Sprintf("%d", m.PlayerCount)
		byPlayerCount[key] = append(byPlayerCount[key], entry)

		var geo mapGeometryJSON
		if len(m.RawJSON) > 0 {
			if err := json.Unmarshal(m.RawJSON, &geo); err != nil {
				return fmt.Errorf("parse map_json for %s: %w", m.MapID, err)
			}
		}

		detail := MapDetail{
			MapID:       m.MapID,
			PlayerCount: m.PlayerCount,
			Status:      m.Status,
			Engagement:  m.Engagement,
			WallDensity: m.WallDensity,
			EnergyCount: m.EnergyCount,
			GridWidth:   m.GridWidth,
			GridHeight:  m.GridHeight,
			CreatedAt:   m.CreatedAt.Format(time.RFC3339),
			Walls:       geo.Walls,
			Cores:       geo.Cores,
			EnergyNodes: geo.EnergyNodes,
		}
		if detail.Walls == nil {
			detail.Walls = []mapPosition{}
		}
		if detail.Cores == nil {
			detail.Cores = []mapCore{}
		}
		if detail.EnergyNodes == nil {
			detail.EnergyNodes = []mapPosition{}
		}

		if err := writeJSON(filepath.Join(mapsDir, m.MapID+".json"), detail); err != nil {
			return fmt.Errorf("write map %s: %w", m.MapID, err)
		}
	}

	index := MapIndexFile{
		UpdatedAt:     data.GeneratedAt.Format(time.RFC3339),
		Maps:          entries,
		ByPlayerCount: byPlayerCount,
	}

	return writeJSON(filepath.Join(mapsDir, "index.json"), index)
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

// persistGeneratedPlaylists reads the generated playlist JSON files from the output
// directory and writes them to the playlists and playlist_matches DB tables.
func persistGeneratedPlaylists(ctx context.Context, db *sql.DB, outputDir string) error {
	playlistsDir := filepath.Join(outputDir, "data", "playlists")

	indexContent, err := os.ReadFile(filepath.Join(playlistsDir, "index.json"))
	if err != nil {
		return fmt.Errorf("read playlist index: %w", err)
	}
	var index PlaylistIndex
	if err := json.Unmarshal(indexContent, &index); err != nil {
		return fmt.Errorf("parse playlist index: %w", err)
	}

	var persisted []persistedPlaylist
	for _, summary := range index.Playlists {
		plContent, err := os.ReadFile(filepath.Join(playlistsDir, summary.Slug+".json"))
		if err != nil {
			continue // skip playlists without files
		}
		var pl Playlist
		if err := json.Unmarshal(plContent, &pl); err != nil {
			continue
		}

		matches := make([]persistedPlaylistMatch, 0, len(pl.Matches))
		for _, m := range pl.Matches {
			matches = append(matches, persistedPlaylistMatch{
				MatchID:     m.MatchID,
				SortOrder:   m.Order,
				CurationTag: m.CurationTag,
			})
		}

		persisted = append(persisted, persistedPlaylist{
			Slug:        pl.Slug,
			Title:       pl.Title,
			Description: pl.Description,
			Category:    pl.Category,
			Matches:     matches,
		})
	}

	return persistPlaylists(ctx, db, persisted)
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

// ─── Archetypes (§15.2) ────────────────────────────────────────────────────────

// ArchetypeBot is a bot entry within an archetype group.
type ArchetypeBot struct {
	ID     string  `json:"id"`
	Name   string  `json:"name"`
	Rating float64 `json:"rating"`
}

// ArchetypeEntry aggregates bots sharing a behavioral archetype.
type ArchetypeEntry struct {
	Name      string         `json:"name"`
	BotCount  int            `json:"bot_count"`
	AvgRating float64        `json:"avg_rating"`
	WinRate   float64        `json:"win_rate"`
	Bots      []ArchetypeBot `json:"bots"`
}

// ArchetypesIndex is the top-level structure for data/meta/archetypes.json.
type ArchetypesIndex struct {
	UpdatedAt  string           `json:"updated_at"`
	Archetypes []ArchetypeEntry `json:"archetypes"`
}

// classifyArchetype infers a behavioral archetype from bot name when the
// archetype field is empty.
func classifyArchetype(bot BotData) string {
	name := strings.ToLower(bot.Name)
	switch {
	case strings.Contains(name, "rush") || strings.Contains(name, "aggress") || strings.Contains(name, "attack") || strings.Contains(name, "blitz"):
		return "aggressive"
	case strings.Contains(name, "defend") || strings.Contains(name, "wall") || strings.Contains(name, "fort") || strings.Contains(name, "guard"):
		return "defensive"
	case strings.Contains(name, "swarm") || strings.Contains(name, "hive") || strings.Contains(name, "colony") || strings.Contains(name, "mass"):
		return "swarm"
	case strings.Contains(name, "hunt") || strings.Contains(name, "chase") || strings.Contains(name, "pursuit") || strings.Contains(name, "stalk"):
		return "hunter"
	case strings.Contains(name, "turtle") || strings.Contains(name, "base") || strings.Contains(name, "camp") || strings.Contains(name, "bunker"):
		return "turtler"
	default:
		return "balanced"
	}
}

// generateArchetypes builds data/meta/archetypes.json from the bot population.
func generateArchetypes(data *IndexData, outputDir string) error {
	type archetypeAccum struct {
		entry       ArchetypeEntry
		totalWins   int
		totalPlayed int
	}
	accum := make(map[string]*archetypeAccum)

	for _, bot := range data.Bots {
		arch := bot.Archetype
		if arch == "" {
			arch = classifyArchetype(bot)
		}

		a, ok := accum[arch]
		if !ok {
			a = &archetypeAccum{entry: ArchetypeEntry{Name: arch}}
			accum[arch] = a
		}
		a.entry.BotCount++
		a.entry.AvgRating += bot.Rating
		a.entry.Bots = append(a.entry.Bots, ArchetypeBot{
			ID:     bot.ID,
			Name:   bot.Name,
			Rating: bot.Rating,
		})
		a.totalWins += bot.MatchesWon
		a.totalPlayed += bot.MatchesPlayed
	}

	archetypes := make([]ArchetypeEntry, 0, len(accum))
	for arch, a := range accum {
		if a.entry.BotCount > 0 {
			a.entry.AvgRating = round1(a.entry.AvgRating / float64(a.entry.BotCount))
		}
		if a.totalPlayed > 0 {
			a.entry.WinRate = round1(float64(a.totalWins) / float64(a.totalPlayed) * 100)
		}
		sort.Slice(a.entry.Bots, func(i, j int) bool {
			return a.entry.Bots[i].Rating > a.entry.Bots[j].Rating
		})
		if len(a.entry.Bots) > 20 {
			a.entry.Bots = a.entry.Bots[:20]
		}
		_ = arch
		archetypes = append(archetypes, a.entry)
	}

	sort.Slice(archetypes, func(i, j int) bool {
		return archetypes[i].BotCount > archetypes[j].BotCount
	})

	metaDir := filepath.Join(outputDir, "data", "meta")
	if err := os.MkdirAll(metaDir, 0755); err != nil {
		return err
	}

	index := ArchetypesIndex{
		UpdatedAt:  data.GeneratedAt.Format(time.RFC3339),
		Archetypes: archetypes,
	}
	return writeJSON(filepath.Join(metaDir, "archetypes.json"), index)
}

// ─── Community Hints (§15.2 / §13.6) ──────────────────────────────────────────

// CommunityHint is a single high-upvote tactical insight from §13.6 feedback.
type CommunityHint struct {
	FeedbackID string `json:"feedback_id"`
	MatchID    string `json:"match_id"`
	Turn       int    `json:"turn"`
	Type       string `json:"type"`
	Body       string `json:"body"`
	Upvotes    int    `json:"upvotes"`
	CreatedAt  string `json:"created_at"`
}

// CommunityHintsFile is the top-level structure for data/evolution/community_hints.json.
type CommunityHintsFile struct {
	GeneratedAt string          `json:"generated_at"`
	Hints       []CommunityHint `json:"hints"`
}

const communityHintMinUpvotes = 3
const communityHintMaxHints = 50

// generateCommunityHints builds data/evolution/community_hints.json from
// high-upvote 'idea' and 'mistake' feedback entries. The evolver reads this
// file to include tactical community insights in LLM prompts.
func generateCommunityHints(data *IndexData, outputDir string) error {
	var hints []CommunityHint
	for _, f := range data.Feedback {
		if f.Type != "idea" && f.Type != "mistake" {
			continue
		}
		if f.Upvotes < communityHintMinUpvotes {
			continue
		}
		hints = append(hints, CommunityHint{
			FeedbackID: f.FeedbackID,
			MatchID:    f.MatchID,
			Turn:       f.Turn,
			Type:       f.Type,
			Body:       f.Body,
			Upvotes:    f.Upvotes,
			CreatedAt:  f.CreatedAt.Format(time.RFC3339),
		})
	}

	// Feedback is already sorted by upvotes DESC from DB; cap at max.
	if len(hints) > communityHintMaxHints {
		hints = hints[:communityHintMaxHints]
	}

	evolDir := filepath.Join(outputDir, "data", "evolution")
	if err := os.MkdirAll(evolDir, 0755); err != nil {
		return err
	}

	file := CommunityHintsFile{
		GeneratedAt: data.GeneratedAt.Format(time.RFC3339),
		Hints:       hints,
	}
	return writeJSON(filepath.Join(evolDir, "community_hints.json"), file)
}

// ─── Per-match Feedback (§15.2) ────────────────────────────────────────────────

// MatchFeedbackFile is the structure for data/matches/{id}/feedback.json.
type MatchFeedbackFile struct {
	MatchID   string          `json:"match_id"`
	UpdatedAt string          `json:"updated_at"`
	Feedback  []FeedbackEntry `json:"feedback"`
}

// generateMatchFeedback creates data/matches/{match_id}/feedback.json for every
// match that has community annotations. The static file mirrors the live API
// response so annotation.ts can fall back to it when the API is unavailable.
func generateMatchFeedback(data *IndexData, outputDir string) error {
	byMatch := make(map[string][]FeedbackEntry)
	for _, f := range data.Feedback {
		byMatch[f.MatchID] = append(byMatch[f.MatchID], f)
	}

	for matchID, entries := range byMatch {
		matchDir := filepath.Join(outputDir, "data", "matches", matchID)
		if err := os.MkdirAll(matchDir, 0755); err != nil {
			return fmt.Errorf("create match dir %s: %w", matchID, err)
		}

		file := MatchFeedbackFile{
			MatchID:   matchID,
			UpdatedAt: data.GeneratedAt.Format(time.RFC3339),
			Feedback:  entries,
		}
		if err := writeJSON(filepath.Join(matchDir, "feedback.json"), file); err != nil {
			return fmt.Errorf("write feedback for match %s: %w", matchID, err)
		}
	}

	return nil
}
