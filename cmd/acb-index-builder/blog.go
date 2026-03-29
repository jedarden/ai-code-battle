package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// BlogPost represents a single blog post
type BlogPost struct {
	Slug      string   `json:"slug"`
	Title     string   `json:"title"`
	Date      string   `json:"date"`
	Type      string   `json:"type"` // "meta-report" or "chronicle"
	ContentMd string   `json:"content_md"`
	Summary   string   `json:"summary"`
	Tags      []string `json:"tags"`
}

// BlogIndex represents the blog/index.json structure
type BlogIndex struct {
	UpdatedAt string      `json:"updated_at"`
	Posts     []BlogEntry `json:"posts"`
}

// BlogEntry is a lightweight entry for the blog index
type BlogEntry struct {
	Slug    string   `json:"slug"`
	Title   string   `json:"title"`
	Date    string   `json:"date"`
	Type    string   `json:"type"`
	Summary string   `json:"summary"`
	Tags    []string `json:"tags"`
}

// generateBlog creates blog posts and the blog index
func generateBlog(data *IndexData, outputDir string, llmClient *LLMClient) error {
	blogDir := filepath.Join(outputDir, "data", "blog")
	postsDir := filepath.Join(blogDir, "posts")

	if err := os.MkdirAll(postsDir, 0755); err != nil {
		return fmt.Errorf("create blog dirs: %w", err)
	}

	posts := make([]BlogPost, 0)

	// Generate weekly meta report (only on Mondays or for testing)
	if time.Now().Weekday() == time.Monday || len(data.Matches) > 0 {
		metaReport := generateMetaReport(data)
		posts = append(posts, metaReport)
	}

	// Generate story arc chronicles using narrative engine
	chronicles := generateLLMChronicles(context.Background(), data, llmClient)
	posts = append(posts, chronicles...)

	// Write individual post files
	entries := make([]BlogEntry, 0, len(posts))
	for _, post := range posts {
		postPath := filepath.Join(postsDir, post.Slug+".json")
		if err := writeJSON(postPath, post); err != nil {
			return fmt.Errorf("write post %s: %w", post.Slug, err)
		}
		entries = append(entries, BlogEntry{
			Slug:    post.Slug,
			Title:   post.Title,
			Date:    post.Date,
			Type:    post.Type,
			Summary: post.Summary,
			Tags:    post.Tags,
		})
	}

	// Write blog index
	index := BlogIndex{
		UpdatedAt: data.GeneratedAt.Format(time.RFC3339),
		Posts:     entries,
	}

	return writeJSON(filepath.Join(blogDir, "index.json"), index)
}

// generateMetaReport creates the weekly meta analysis blog post
func generateMetaReport(data *IndexData) BlogPost {
	weekNum := getWeekNumber(data.GeneratedAt)
	seasonName := getCurrentSeasonName(data)

	// Calculate meta statistics
	topBots := getTopBots(data, 5)
	strategyDistribution := calculateStrategyDistribution(data)
	risingBots := findRisingBots(data)
	fallingBots := findFallingBots(data)
	recentUpsets := findRecentUpsets(data)
	topRivalries := findTopRivalries(data)

	// Build content
	content := fmt.Sprintf(`# Week %d Meta Report — %s

## Overview

This week's competitive landscape analysis covers %d active bots across %d completed matches.

## Top 5 Leaderboard

| Rank | Bot | Rating | Win Rate |
|------|-----|--------|----------|
%s

## Strategy Distribution

%s

## Rising Stars

%s

## Falling Behind

%s

## Notable Upsets

%s

## Top Rivalries

%s

## Looking Ahead

The meta continues to evolve as bots adapt their strategies. Key trends to watch:
- Formation-based play continues to dominate
- Energy control remains crucial in early game
- Adaptation to map layouts shows clear skill differentials

---

*Generated automatically by AI Code Battle index builder.*
`,
		weekNum, seasonName,
		len(data.Bots), len(data.Matches),
		formatLeaderboardTable(topBots),
		formatStrategyDistribution(strategyDistribution),
		formatBotList(risingBots),
		formatBotList(fallingBots),
		formatUpsets(recentUpsets),
		formatRivalries(topRivalries),
	)

	slug := fmt.Sprintf("meta-week-%d-%s", weekNum, formatSlugDate(data.GeneratedAt))

	return BlogPost{
		Slug:      slug,
		Title:     fmt.Sprintf("Week %d Meta Report — %s", weekNum, seasonName),
		Date:      data.GeneratedAt.Format("2006-01-02"),
		Type:      "meta-report",
		ContentMd: content,
		Summary:   fmt.Sprintf("Weekly competitive analysis: %d bots, top strategies, rising stars, and key rivalries.", len(data.Bots)),
		Tags:      []string{"meta-report", seasonTag(seasonName)},
	}
}

// generateChronicles creates story arc chronicles from match data (template-based fallback)
func generateChronicles(data *IndexData) []BlogPost {
	chronicles := make([]BlogPost, 0)

	// Find rising star stories
	if len(data.Bots) > 0 {
		rising := findRisingBots(data)
		if len(rising) > 0 {
			chronicles = append(chronicles, generateRiseChronicle(rising[0], data))
		}
	}

	// Find upset stories
	upsets := findRecentUpsets(data)
	if len(upsets) > 0 {
		chronicles = append(chronicles, generateUpsetChronicle(upsets[0], data))
	}

	// Find rivalry stories
	rivalries := findTopRivalries(data)
	if len(rivalries) > 0 {
		chronicles = append(chronicles, generateRivalryChronicle(rivalries[0], data))
	}

	return chronicles
}

// generateLLMChronicles creates chronicles using the narrative engine and LLM
func generateLLMChronicles(ctx context.Context, data *IndexData, llmClient *LLMClient) []BlogPost {
	chronicles := make([]BlogPost, 0)

	// Detect story arcs from data
	arcs := detectStoryArcs(data)

	// Limit to 3-5 chronicles per week
	maxChronicles := 5
	if len(arcs) < maxChronicles {
		maxChronicles = len(arcs)
	}

	for i := 0; i < maxChronicles; i++ {
		arc := arcs[i]

		var post BlogPost
		var err error

		// Try to generate LLM narrative
		if llmClient != nil && llmClient.baseURL != "" {
			post, err = generateLLMChronicle(ctx, arc, data, llmClient)
			if err != nil {
				// Fall back to template-based chronicle
				post = generateTemplateChronicle(arc, data)
			}
		} else {
			// No LLM client, use template
			post = generateTemplateChronicle(arc, data)
		}

		chronicles = append(chronicles, post)
	}

	return chronicles
}

// generateLLMChronicle creates a chronicle using LLM narrative generation
func generateLLMChronicle(ctx context.Context, arc StoryArc, data *IndexData, llmClient *LLMClient) (BlogPost, error) {
	seasonName := getCurrentSeasonName(data)

	req := NarrativeRequest{
		ArcType:     arc.Type,
		BotName:     arc.BotName,
		SeasonName:  seasonName,
		RatingStart: arc.RatingStart,
		RatingEnd:   arc.RatingEnd,
		KeyMatches:  arc.KeyMatches,
		Archetype:   arc.Archetype,
		Origin:      arc.Origin,
		ParentIDs:   arc.ParentIDs,
		Generation:  arc.Generation,
		BotBName:    arc.BotBName,
	}

	// Get rivalry-specific data
	if arc.Type == ArcRivalry {
		req.BotAWins = arc.BotAWins
		req.BotBWins = arc.BotBWins
		req.TotalMatches = arc.TotalMatches
	}

	headline, narrative, err := llmClient.GenerateNarrative(ctx, req)
	if err != nil {
		return BlogPost{}, err
	}

	slug := fmt.Sprintf("%s-%s-%s", arc.Type, arc.BotID, formatSlugDate(data.GeneratedAt))
	if arc.Type == ArcRivalry {
		slug = fmt.Sprintf("rivalry-%s-%s", arc.BotID[:8], arc.BotBID[:8])
	} else if arc.Type == ArcUpset {
		slug = fmt.Sprintf("upset-%s-%s", arc.MatchID[:8], formatSlugDate(data.GeneratedAt))
	}

	tags := []string{string(arc.Type)}
	if arc.BotID != "" {
		tags = append(tags, arc.BotID)
	}
	if arc.BotBID != "" {
		tags = append(tags, arc.BotBID)
	}

	return BlogPost{
		Slug:      slug,
		Title:     headline,
		Date:      data.GeneratedAt.Format("2006-01-02"),
		Type:      "chronicle",
		ContentMd: "# " + headline + "\n\n" + narrative,
		Summary:   truncateSummary(narrative, 150),
		Tags:      tags,
	}, nil
}

// generateTemplateChronicle creates a chronicle using templates (fallback)
func generateTemplateChronicle(arc StoryArc, data *IndexData) BlogPost {
	switch arc.Type {
	case ArcRise:
		bot := findBotByID(arc.BotID, data)
		if bot != nil {
			return generateRiseChronicle(*bot, data)
		}
	case ArcUpset:
		upset := UpsetData{
			MatchID:     arc.MatchID,
			WinnerID:    arc.BotID,
			LoserID:     arc.BotBID,
			WinnerScore: arc.RatingStart,
			LoserScore:  arc.RatingEnd,
		}
		return generateUpsetChronicle(upset, data)
	case ArcRivalry:
		rivalry := RivalryData{
			BotAID:       arc.BotID,
			BotBID:       arc.BotBID,
			BotAWins:     arc.BotAWins,
			BotBWins:     arc.BotBWins,
			TotalMatches: arc.TotalMatches,
		}
		return generateRivalryChronicle(rivalry, data)
	}

	// Generic fallback
	return BlogPost{
		Slug:      fmt.Sprintf("%s-%s-%s", arc.Type, arc.BotID, formatSlugDate(data.GeneratedAt)),
		Title:     fmt.Sprintf("%s: %s", arc.Type, arc.BotName),
		Date:      data.GeneratedAt.Format("2006-01-02"),
		Type:      "chronicle",
		ContentMd: fmt.Sprintf("# %s: %s\n\nDetails pending.", arc.Type, arc.BotName),
		Summary:   fmt.Sprintf("Story arc: %s involving %s", arc.Type, arc.BotName),
		Tags:      []string{string(arc.Type), arc.BotID},
	}
}

// truncateSummary truncates a string to maxLen characters
func truncateSummary(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	// Find last space before maxLen
	lastSpace := maxLen
	for i := maxLen - 1; i >= 0; i-- {
		if s[i] == ' ' {
			lastSpace = i
			break
		}
	}
	return s[:lastSpace] + "..."
}

// findBotByID finds a bot by ID in the data
func findBotByID(id string, data *IndexData) *BotData {
	for i := range data.Bots {
		if data.Bots[i].ID == id {
			return &data.Bots[i]
		}
	}
	return nil
}

// generateRiseChronicle creates a "rising star" story
func generateRiseChronicle(bot BotData, data *IndexData) BlogPost {
	content := fmt.Sprintf(`# The Rise of %s

%s has been climbing the leaderboard with impressive momentum. With a current rating of %d and a %.1f%% win rate, this bot is making waves in the competitive scene.

## Key Statistics

- **Rating:** %d
- **Matches Played:** %d
- **Win Rate:** %.1f%%

## Analysis

%s's recent performance shows consistent improvement. The bot's strategy execution has been notably strong in energy collection and unit positioning.

## What's Next

As %s continues to climb, it faces tougher competition. The coming weeks will test whether this ascent can be sustained against top-tier opponents.

---

*Auto-generated chronicle from match data analysis.*
`,
		bot.Name,
		bot.Name, int(bot.Rating), calculateWinRate(bot.MatchesPlayed, bot.MatchesWon)*100,
		int(bot.Rating), bot.MatchesPlayed, calculateWinRate(bot.MatchesPlayed, bot.MatchesWon)*100,
		bot.Name,
		bot.Name,
	)

	return BlogPost{
		Slug:      fmt.Sprintf("rise-%s-%s", bot.ID, formatSlugDate(data.GeneratedAt)),
		Title:     fmt.Sprintf("The Rise of %s", bot.Name),
		Date:      data.GeneratedAt.Format("2006-01-02"),
		Type:      "chronicle",
		ContentMd: content,
		Summary:   fmt.Sprintf("%s climbs the leaderboard with a %d rating and %.0f%% win rate.", bot.Name, int(bot.Rating), calculateWinRate(bot.MatchesPlayed, bot.MatchesWon)*100),
		Tags:      []string{"rise", bot.ID},
	}
}

// generateUpsetChronicle creates an upset story
func generateUpsetChronicle(upset UpsetData, data *IndexData) BlogPost {
	winnerName := getBotName(upset.WinnerID, data)
	loserName := getBotName(upset.LoserID, data)

	content := fmt.Sprintf(`# Shocking Upset: %s Defeats %s

In a stunning turn of events, %s has defeated the heavily favored %s in a match that will be remembered.

## Match Details

- **Winner:** %s
- **Score:** %d - %d
- **Turns:** %d

## How It Happened

The match started with %s taking an early lead, but %s found an opening. Through careful resource management and tactical positioning, the underdog seized control and never looked back.

## Community Reaction

This upset shakes up the leaderboard and proves that in AI Code Battle, anything can happen when bots execute their strategies flawlessly.

---

*Auto-generated chronicle from match analysis.*
`,
		winnerName, loserName,
		winnerName, loserName,
		winnerName,
		upset.WinnerScore, upset.LoserScore, upset.TurnCount,
		loserName, winnerName,
	)

	return BlogPost{
		Slug:      fmt.Sprintf("upset-%s-%s", upset.MatchID[:8], formatSlugDate(data.GeneratedAt)),
		Title:     fmt.Sprintf("Upset: %s Defeats %s", winnerName, loserName),
		Date:      data.GeneratedAt.Format("2006-01-02"),
		Type:      "chronicle",
		ContentMd: content,
		Summary:   fmt.Sprintf("%s pulled off a stunning upset against %s.", winnerName, loserName),
		Tags:      []string{"upset", upset.WinnerID, upset.LoserID},
	}
}

// generateRivalryChronicle creates a rivalry story
func generateRivalryChronicle(rivalry RivalryData, data *IndexData) BlogPost {
	botAName := getBotName(rivalry.BotAID, data)
	botBName := getBotName(rivalry.BotBID, data)

	content := fmt.Sprintf(`# Rivalry: %s vs %s

One of the most compelling rivalries in AI Code Battle continues to develop between %s and %s.

## Head-to-Head Record

- **%s:** %d wins
- **%s:** %d wins
- **Total Matches:** %d

## The Story So Far

These two bots have developed a fierce competitive relationship. Each match brings new tactical adjustments as they learn from previous encounters.

## What Makes This Rivalry Special

The contrasting strategies of these two competitors create must-watch matches. When they face off, the outcome is never certain until the final turn.

## Next Chapter

As both bots continue to evolve, their rivalry promises more excitement. The next encounter could shift the balance of power.

---

*Auto-generated chronicle from rivalry analysis.*
`,
		botAName, botBName,
		botAName, botBName,
		botAName, rivalry.BotAWins,
		botBName, rivalry.BotBWins,
		rivalry.TotalMatches,
	)

	return BlogPost{
		Slug:      fmt.Sprintf("rivalry-%s-%s", rivalry.BotAID[:8], rivalry.BotBID[:8]),
		Title:     fmt.Sprintf("Rivalry: %s vs %s", botAName, botBName),
		Date:      data.GeneratedAt.Format("2006-01-02"),
		Type:      "chronicle",
		ContentMd: content,
		Summary:   fmt.Sprintf("%s and %s have played %d matches. Current record: %d-%d.", botAName, botBName, rivalry.TotalMatches, rivalry.BotAWins, rivalry.BotBWins),
		Tags:      []string{"rivalry", rivalry.BotAID, rivalry.BotBID},
	}
}

// UpsetData represents an upset match
type UpsetData struct {
	MatchID     string
	WinnerID    string
	LoserID     string
	WinnerScore int
	LoserScore  int
	TurnCount   int
}

// RivalryData represents a rivalry between two bots
type RivalryData struct {
	BotAID      string
	BotBID      string
	BotAWins    int
	BotBWins    int
	TotalMatches int
}

// Helper functions

func getWeekNumber(t time.Time) int {
	_, week := t.ISOWeek()
	return week
}

func getCurrentSeasonName(data *IndexData) string {
	for _, s := range data.Seasons {
		if s.StartsAt.Before(data.GeneratedAt) {
			// Check if season is still active (no end date or end date is in future)
			if s.EndsAt.IsZero() || s.EndsAt.After(data.GeneratedAt) {
				return s.Name
			}
		}
	}
	return "Season 1"
}

func getTopBots(data *IndexData, count int) []BotData {
	if len(data.Bots) < count {
		return data.Bots
	}
	return data.Bots[:count]
}

func calculateStrategyDistribution(data *IndexData) map[string]int {
	dist := make(map[string]int)
	for _, bot := range data.Bots {
		// Classify by evolved status
		if bot.Evolved {
			dist["evolved"]++
		} else {
			dist["human-authored"]++
		}
	}
	return dist
}

func findRisingBots(data *IndexData) []BotData {
	// Simple heuristic: bots with high win rates and reasonable match counts
	rising := make([]BotData, 0)
	for _, bot := range data.Bots {
		if bot.MatchesPlayed >= 5 && calculateWinRate(bot.MatchesPlayed, bot.MatchesWon) > 0.6 {
			rising = append(rising, bot)
		}
	}
	// Sort by rating ascending (lower rated bots that are winning are "rising")
	// For simplicity, just return top performers
	if len(rising) > 3 {
		return rising[:3]
	}
	return rising
}

func findFallingBots(data *IndexData) []BotData {
	// Simple heuristic: bots with low win rates
	falling := make([]BotData, 0)
	for _, bot := range data.Bots {
		if bot.MatchesPlayed >= 5 && calculateWinRate(bot.MatchesPlayed, bot.MatchesWon) < 0.4 {
			falling = append(falling, bot)
		}
	}
	if len(falling) > 3 {
		return falling[:3]
	}
	return falling
}

func findRecentUpsets(data *IndexData) []UpsetData {
	upsets := make([]UpsetData, 0)
	for _, m := range data.Matches {
		if len(m.Participants) < 2 {
			continue
		}
		// Look for close matches or unexpected winners
		for i, p1 := range m.Participants {
			for _, p2 := range m.Participants[i+1:] {
				if p1.Won && p2.Score > p1.Score {
					// Winner had lower score - unlikely upset scenario
					upsets = append(upsets, UpsetData{
						MatchID:     m.ID,
						WinnerID:    p1.BotID,
						LoserID:     p2.BotID,
						WinnerScore: p1.Score,
						LoserScore:  p2.Score,
						TurnCount:   m.TurnCount,
					})
				}
			}
		}
	}
	if len(upsets) > 3 {
		return upsets[:3]
	}
	return upsets
}

func findTopRivalries(data *IndexData) []RivalryData {
	// Count matches between bot pairs
	pairCounts := make(map[string]*RivalryData)

	for _, m := range data.Matches {
		if len(m.Participants) < 2 {
			continue
		}
		for i, p1 := range m.Participants {
			for _, p2 := range m.Participants[i+1:] {
				key := fmt.Sprintf("%s-%s", minStr(p1.BotID, p2.BotID), maxStr(p1.BotID, p2.BotID))
				if pairCounts[key] == nil {
					pairCounts[key] = &RivalryData{
						BotAID: minStr(p1.BotID, p2.BotID),
						BotBID: maxStr(p1.BotID, p2.BotID),
					}
				}
				pairCounts[key].TotalMatches++
				if p1.Won {
					if p1.BotID == pairCounts[key].BotAID {
						pairCounts[key].BotAWins++
					} else {
						pairCounts[key].BotBWins++
					}
				} else if p2.Won {
					if p2.BotID == pairCounts[key].BotAID {
						pairCounts[key].BotAWins++
					} else {
						pairCounts[key].BotBWins++
					}
				}
			}
		}
	}

	// Find pairs with most matches
	rivalries := make([]RivalryData, 0)
	for _, r := range pairCounts {
		if r.TotalMatches >= 3 {
			rivalries = append(rivalries, *r)
		}
	}

	// Sort by total matches (simplified - just return first few)
	if len(rivalries) > 3 {
		return rivalries[:3]
	}
	return rivalries
}

func calculateWinRate(played, won int) float64 {
	if played == 0 {
		return 0
	}
	return float64(won) / float64(played)
}

func getBotName(botID string, data *IndexData) string {
	for _, bot := range data.Bots {
		if bot.ID == botID {
			return bot.Name
		}
	}
	return botID
}

func formatSlugDate(t time.Time) string {
	return t.Format("2006-01-02")
}

func seasonTag(seasonName string) string {
	// Convert "Season 4" to "season-4"
	return "season-" + seasonName[len("Season "):]
}

func formatLeaderboardTable(bots []BotData) string {
	result := ""
	for i, bot := range bots {
		winRate := calculateWinRate(bot.MatchesPlayed, bot.MatchesWon) * 100
		result += fmt.Sprintf("| %d | %s | %d | %.1f%% |\n", i+1, bot.Name, int(bot.Rating), winRate)
	}
	return result
}

func formatStrategyDistribution(dist map[string]int) string {
	result := ""
	for strategy, count := range dist {
		result += fmt.Sprintf("- **%s:** %d bots\n", strategy, count)
	}
	return result
}

func formatBotList(bots []BotData) string {
	if len(bots) == 0 {
		return "No significant movement this week."
	}
	result := ""
	for _, bot := range bots {
		winRate := calculateWinRate(bot.MatchesPlayed, bot.MatchesWon) * 100
		result += fmt.Sprintf("- **%s** (Rating: %d, Win Rate: %.1f%%)\n", bot.Name, int(bot.Rating), winRate)
	}
	return result
}

func formatUpsets(upsets []UpsetData) string {
	if len(upsets) == 0 {
		return "No major upsets this week."
	}
	result := ""
	for _, u := range upsets {
		result += fmt.Sprintf("- Match %s: Close contest with score %d-%d\n", u.MatchID[:8], u.WinnerScore, u.LoserScore)
	}
	return result
}

func formatRivalries(rivalries []RivalryData) string {
	if len(rivalries) == 0 {
		return "No emerging rivalries this week."
	}
	result := ""
	for _, r := range rivalries {
		result += fmt.Sprintf("- %s vs %s: %d-%d record\n", r.BotAID[:8], r.BotBID[:8], r.BotAWins, r.BotBWins)
	}
	return result
}

func minStr(a, b string) string {
	if a < b {
		return a
	}
	return b
}

func maxStr(a, b string) string {
	if a > b {
		return a
	}
	return b
}
