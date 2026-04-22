package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// BlogPost represents a single blog post
type BlogPost struct {
	Slug         string   `json:"slug"`
	Title        string   `json:"title"`
	PublishedAt  string   `json:"published_at"`
	Date         string   `json:"date"` // backward compat alias
	Type         string   `json:"type"` // "meta-report" or "chronicle"
	BodyMarkdown string   `json:"body_markdown"`
	ContentMd    string   `json:"content_md"` // backward compat alias
	Summary      string   `json:"summary"`
	Tags         []string `json:"tags"`
}

// BlogIndex represents the blog/index.json structure
type BlogIndex struct {
	UpdatedAt string      `json:"updated_at"`
	Posts     []BlogEntry `json:"posts"`
}

// BlogEntry is a lightweight entry for the blog index
type BlogEntry struct {
	Slug        string   `json:"slug"`
	Title       string   `json:"title"`
	PublishedAt string   `json:"published_at"`
	Date        string   `json:"date"` // backward compat
	Type        string   `json:"type"`
	Summary     string   `json:"summary"`
	Tags        []string `json:"tags"`
}

// generateBlog creates blog posts and the blog index.
// Meta reports are only generated on Monday or if 7+ days have passed since the last one.
func generateBlog(data *IndexData, outputDir string, llmClient *LLMClient, cfg *Config) error {
	blogDir := filepath.Join(outputDir, "data", "blog")
	postsDir := filepath.Join(blogDir, "posts")

	if err := os.MkdirAll(postsDir, 0755); err != nil {
		return fmt.Errorf("create blog dirs: %w", err)
	}

	posts := make([]BlogPost, 0)

	// Generate weekly meta report only when gate passes
	if shouldGenerateMetaReport(postsDir) {
		var metaReport BlogPost
		if llmClient != nil && llmClient.baseURL != "" {
			metaReport = generateMetaReportWithLLM(context.Background(), data, llmClient, cfg)
		} else {
			metaReport = generateMetaReport(data)
		}
		posts = append(posts, metaReport)
		recordMetaReportGenerated(postsDir)
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
			Slug:        post.Slug,
			Title:       post.Title,
			PublishedAt: post.PublishedAt,
			Date:        post.Date,
			Type:        post.Type,
			Summary:     post.Summary,
			Tags:        post.Tags,
		})
	}

	// Write blog index
	index := BlogIndex{
		UpdatedAt: data.GeneratedAt.Format(time.RFC3339),
		Posts:     entries,
	}

	return writeJSON(filepath.Join(blogDir, "index.json"), index)
}

// shouldGenerateMetaReport returns true on Monday or if 7+ days since the last report.
// It checks a state file (.last-meta-report) in postsDir for the last generation timestamp,
// falling back to scanning existing meta report files for backward compatibility.
func shouldGenerateMetaReport(postsDir string) bool {
	now := time.Now().UTC()

	// Always generate on Monday
	if now.Weekday() == time.Monday {
		return true
	}

	// Check state file for last generation timestamp
	stateFile := filepath.Join(postsDir, ".last-meta-report")
	if data, err := os.ReadFile(stateFile); err == nil {
		if lastTime, err := time.Parse(time.RFC3339, strings.TrimSpace(string(data))); err == nil {
			if now.Sub(lastTime) < 7*24*time.Hour {
				return false
			}
			return true
		}
	}

	// Fallback: scan existing meta report files
	entries, err := os.ReadDir(postsDir)
	if err != nil {
		// Directory doesn't exist or can't be read — generate
		return true
	}

	var lastMetaTime time.Time
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if len(name) >= 5 && name[:5] == "meta-" && !strings.HasPrefix(name, ".") {
			info, err := e.Info()
			if err != nil {
				continue
			}
			if info.ModTime().After(lastMetaTime) {
				lastMetaTime = info.ModTime()
			}
		}
	}

	// If no previous meta report found, generate
	if lastMetaTime.IsZero() {
		return true
	}

	// Generate if 7+ days since last report
	return now.Sub(lastMetaTime) >= 7*24*time.Hour
}

// recordMetaReportGenerated writes the generation timestamp to the state file.
func recordMetaReportGenerated(postsDir string) {
	stateFile := filepath.Join(postsDir, ".last-meta-report")
	_ = os.WriteFile(stateFile, []byte(time.Now().UTC().Format(time.RFC3339)), 0644)
}

// ─── ELO mover tracking ──────────────────────────────────────────────────────

type eloMover struct {
	BotID      string
	BotName    string
	OldRating  float64
	NewRating  float64
	Delta      float64
	Evolved    bool
	Archetype  string
	MatchesWon int
	MatchesLost int
}

func findTopELOMovers(data *IndexData, count int) []eloMover {
	now := data.GeneratedAt
	weekAgo := now.AddDate(0, 0, -7)

	// Calculate rating change for each bot over the past week
	movers := make([]eloMover, 0)
	for _, bot := range data.Bots {
		history := getBotRatingHistory(bot.ID, data)
		if len(history) < 2 {
			continue
		}

		// Find the oldest rating within or before the past week
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
		if delta == 0 {
			continue
		}

		// Count wins/losses this week
		wins, losses := countWeeklyResults(bot.ID, data)

		movers = append(movers, eloMover{
			BotID:      bot.ID,
			BotName:    bot.Name,
			OldRating:  oldRating,
			NewRating:  bot.Rating,
			Delta:      delta,
			Evolved:    bot.Evolved,
			Archetype:  bot.Archetype,
			MatchesWon: wins,
			MatchesLost: losses,
		})
	}

	// Sort by absolute delta descending
	sort.Slice(movers, func(i, j int) bool {
		return absF(movers[i].Delta) > absF(movers[j].Delta)
	})

	if len(movers) > count {
		return movers[:count]
	}
	return movers
}

func countWeeklyResults(botID string, data *IndexData) (wins, losses int) {
	weekAgo := data.GeneratedAt.AddDate(0, 0, -7)
	for _, m := range data.Matches {
		if m.PlayedAt.Before(weekAgo) {
			continue
		}
		for _, p := range m.Participants {
			if p.BotID == botID {
				if p.Won {
					wins++
				} else {
					losses++
				}
				break
			}
		}
	}
	return
}

// ─── Strategy analysis ────────────────────────────────────────────────────────

type strategyCount struct {
	Archetype string
	Count     int
	AvgRating float64
	InTop20   int
}

func calculateDominantStrategies(data *IndexData) []strategyCount {
	stratMap := make(map[string]*strategyCount)

	// Count bots by archetype
	for i, bot := range data.Bots {
		arch := bot.Archetype
		if arch == "" {
			if bot.Evolved {
				arch = "evolved-unknown"
			} else {
				arch = "standard"
			}
		}

		sc, ok := stratMap[arch]
		if !ok {
			sc = &strategyCount{Archetype: arch}
			stratMap[arch] = sc
		}
		sc.Count++
		sc.AvgRating += bot.Rating
		if i < 20 {
			sc.InTop20++
		}
	}

	strats := make([]strategyCount, 0, len(stratMap))
	for _, sc := range stratMap {
		if sc.Count > 0 {
			sc.AvgRating /= float64(sc.Count)
		}
		strats = append(strats, *sc)
	}

	// Sort by count descending
	sort.Slice(strats, func(i, j int) bool {
		return strats[i].Count > strats[j].Count
	})

	return strats
}

// ─── Most-watched match ───────────────────────────────────────────────────────

type notableMatch struct {
	MatchID      string
	Description  string
	Score        string
	TurnCount    int
	Participants []ParticipantData
}

func findMostWatchedMatch(data *IndexData) *notableMatch {
	// Use interest score to find the most notable match this week
	weekAgo := data.GeneratedAt.AddDate(0, 0, -7)

	var best *notableMatch
	var bestScore float64

	for _, m := range data.Matches {
		if m.PlayedAt.Before(weekAgo) {
			continue
		}
		if len(m.Participants) < 2 {
			continue
		}

		score := computeMatchInterest(m, data)
		if score > bestScore {
			bestScore = score
			desc := formatMatchDescription(m, data)
			best = &notableMatch{
				MatchID:      m.ID,
				Description:  desc,
				Score:        formatMatchScore(m),
				TurnCount:    m.TurnCount,
				Participants: m.Participants,
			}
		}
	}

	return best
}

func computeMatchInterest(m MatchData, data *IndexData) float64 {
	score := 0.0

	// Close finishes are more interesting
	if len(m.Participants) >= 2 {
		var maxScore, minScore int
		for i, p := range m.Participants {
			if i == 0 || p.Score > maxScore {
				maxScore = p.Score
			}
			if i == 0 || p.Score < minScore {
				minScore = p.Score
			}
		}
		diff := maxScore - minScore
		if diff <= 1 {
			score += 5.0
		} else if diff <= 3 {
			score += 3.0
		} else if diff <= 5 {
			score += 1.0
		}
	}

	// Upsets (lower-rated bot wins)
	if len(m.Participants) >= 2 {
		for _, p := range m.Participants {
			if p.Won {
				for _, q := range m.Participants {
					if !q.Won && p.PreMatchRating > 0 && q.PreMatchRating > 0 {
						gap := q.PreMatchRating - p.PreMatchRating
						if gap > 100 {
							score += gap / 50.0 // bigger upsets = more interesting
						}
					}
				}
			}
		}
	}

	// Longer matches (more strategic depth)
	if m.TurnCount > 300 {
		score += 2.0
	} else if m.TurnCount > 200 {
		score += 1.0
	}

	// Matches involving evolved bots are more interesting
	for _, p := range m.Participants {
		bot := findBotByID(p.BotID, data)
		if bot != nil && bot.Evolved {
			score += 1.5
		}
	}

	return score
}

func formatMatchDescription(m MatchData, data *IndexData) string {
	names := make([]string, 0, len(m.Participants))
	for _, p := range m.Participants {
		names = append(names, getBotName(p.BotID, data))
	}

	switch len(names) {
	case 2:
		return fmt.Sprintf("%s vs %s", names[0], names[1])
	case 3:
		return fmt.Sprintf("%s, %s, %s", names[0], names[1], names[2])
	default:
		return fmt.Sprintf("%s and %d others", names[0], len(names)-1)
	}
}

func formatMatchScore(m MatchData) string {
	scores := make([]string, 0, len(m.Participants))
	for _, p := range m.Participants {
		scores = append(scores, fmt.Sprintf("%d", p.Score))
	}
	result := ""
	for i, s := range scores {
		if i > 0 {
			result += "-"
		}
		result += s
	}
	return result
}

// ─── Evolution highlights ──────────────────────────────────────────────────────

type evolutionHighlight struct {
	BotID       string
	BotName     string
	Rating      float64
	Island      string
	Generation  int
	Archetype   string
	WeekMatches int
	WeekWins    int
}

func findEvolutionHighlights(data *IndexData) []evolutionHighlight {
	weekAgo := data.GeneratedAt.AddDate(0, 0, -7)
	highlights := make([]evolutionHighlight, 0)

	for _, bot := range data.Bots {
		if !bot.Evolved {
			continue
		}

		wins, losses := 0, 0
		for _, m := range data.Matches {
			if m.PlayedAt.Before(weekAgo) {
				continue
			}
			for _, p := range m.Participants {
				if p.BotID == bot.ID {
					if p.Won {
						wins++
					} else {
						losses++
					}
					break
				}
			}
		}

		total := wins + losses
		if total == 0 {
			continue
		}

		highlights = append(highlights, evolutionHighlight{
			BotID:       bot.ID,
			BotName:     bot.Name,
			Rating:      bot.Rating,
			Island:      bot.Island,
			Generation:  bot.Generation,
			Archetype:   bot.Archetype,
			WeekMatches: total,
			WeekWins:    wins,
		})
	}

	// Sort by rating descending
	sort.Slice(highlights, func(i, j int) bool {
		return highlights[i].Rating > highlights[j].Rating
	})

	if len(highlights) > 5 {
		return highlights[:5]
	}
	return highlights
}

// ─── Meta report generation (template) ────────────────────────────────────────

// generateMetaReport creates the weekly meta analysis blog post with enriched data.
func generateMetaReport(data *IndexData) BlogPost {
	weekNum := getWeekNumber(data.GeneratedAt)
	seasonName := getCurrentSeasonName(data)
	dateStr := data.GeneratedAt.Format("2006-01-02")

	// Gather all data sections
	topBots := getTopBots(data, 5)
	eloMovers := findTopELOMovers(data, 5)
	strategies := calculateDominantStrategies(data)
	risingBots := findRisingBots(data)
	fallingBots := findFallingBots(data)
	recentUpsets := findRecentUpsets(data)
	topRivalries := findTopRivalries(data)
	bestMatch := findMostWatchedMatch(data)
	evoHighlights := findEvolutionHighlights(data)
	stratTrends := calculateStrategyTrends(data)
	matchups := calculateMatchupMatrix(data)
	mapWeek := findMapOfTheWeek(data)
	spotlight := buildBotSpotlight(data)

	// Build content
	content := fmt.Sprintf(`# Week %d Meta Report — %s

## Overview

This week's competitive landscape analysis covers %d active bots across %d completed matches.

## Top 5 Leaderboard

| Rank | Bot | Rating | Win Rate |
|------|-----|--------|----------|
%s

## Top 5 ELO Movers This Week

| Bot | Rating Change | From → To | Record |
|-----|--------------|-----------|--------|
%s

## Dominant Strategies

%s

## Strategy Trends

%s

## Matchup Insights

%s

## Most-Watched Match

%s

## Map of the Week

%s

## Bot Spotlight

%s

## Rising Stars

%s

## Falling Behind

%s

## Notable Upsets

%s

## Top Rivalries

%s

## Evolution Highlights

%s

## Prediction Standings

%s

## Season Progress

%s

## Looking Ahead

%s

---

*Generated automatically by AI Code Battle index builder.*
`,
		weekNum, seasonName,
		len(data.Bots), len(data.Matches),
		formatLeaderboardTable(topBots),
		formatELOMoversTable(eloMovers),
		formatStrategyTable(strategies),
		formatStrategyTrends(stratTrends),
		formatMatchupInsights(matchups),
		formatNotableMatch(bestMatch),
		formatMapOfTheWeek(mapWeek),
		formatBotSpotlight(spotlight),
		formatBotList(risingBots),
		formatBotList(fallingBots),
		formatUpsets(recentUpsets),
		formatRivalries(topRivalries),
		formatEvolutionHighlights(evoHighlights),
		formatPredictionStandings(data),
		formatSeasonProgress(data),
		formatLookingAhead(eloMovers, strategies, evoHighlights, data),
	)

	slug := fmt.Sprintf("meta-week-%d-%s", weekNum, formatSlugDate(data.GeneratedAt))
	summary := fmt.Sprintf("Week %d: %d active bots, %d matches. %s",
		weekNum, len(data.Bots), len(data.Matches),
		buildMetaReportSummary(eloMovers, strategies, bestMatch))

	return BlogPost{
		Slug:         slug,
		Title:        fmt.Sprintf("Week %d Meta Report — %s", weekNum, seasonName),
		PublishedAt:  dateStr,
		Date:         dateStr,
		Type:         "meta-report",
		BodyMarkdown: content,
		ContentMd:    content,
		Summary:      summary,
		Tags:         []string{"meta-report", seasonTag(seasonName)},
	}
}

// generateMetaReportWithLLM uses the LLM to produce a rich narrative meta report.
// The LLM generates the analytical sections (Counter-Strategy Spotlight, Evolution Deep Dive, Looking Ahead),
// which are spliced into the template-generated structured content.
func generateMetaReportWithLLM(ctx context.Context, data *IndexData, llmClient *LLMClient, cfg *Config) BlogPost {
	// Start with the template-based report (tables, stats, links)
	post := generateMetaReport(data)

	// Gather enriched context for the LLM
	eloMovers := findTopELOMovers(data, 5)
	strategies := calculateDominantStrategies(data)
	bestMatch := findMostWatchedMatch(data)
	evoHighlights := findEvolutionHighlights(data)
	topBots := getTopBots(data, 5)
	rivalries := findTopRivalries(data)
	predLeaderboard := data.TopPredictors
	matchups := calculateMatchupMatrix(data)
	trends := calculateStrategyTrends(data)
	liveData := fetchEvolutionLiveData(ctx, cfg)

	// Generate Counter-Strategy Spotlight
	spotlightPrompt := buildSpotlightPrompt(data, eloMovers, strategies, bestMatch, evoHighlights, topBots, rivalries)
	spotlight, err := llmClient.chatCompletion(ctx, spotlightPrompt)
	if err != nil {
		slog.Error("LLM spotlight generation failed", "error", err)
		spotlight = ""
	}

	// Generate Evolution Deep Dive
	evoNarrative := ""
	if len(evoHighlights) > 0 {
		evoPrompt := buildEvolutionDeepDivePrompt(data, evoHighlights, rivalries, predLeaderboard, liveData)
		evoNarrative, err = llmClient.chatCompletion(ctx, evoPrompt)
		if err != nil {
			slog.Error("LLM evolution narrative generation failed", "error", err)
			evoNarrative = ""
		}
	}

	// Generate Looking Ahead via LLM (replaces template-based version)
	lookingAheadNarrative := ""
	lookingAheadPrompt := buildLookingAheadPrompt(data, eloMovers, strategies, trends, matchups, liveData)
	lookingAheadNarrative, err = llmClient.chatCompletion(ctx, lookingAheadPrompt)
	if err != nil {
		slog.Error("LLM looking ahead generation failed", "error", err)
		lookingAheadNarrative = ""
	}

	// Splice LLM content into the template report
	if spotlight != "" || evoNarrative != "" || lookingAheadNarrative != "" {
		post.BodyMarkdown = spliceLLMContent(post.BodyMarkdown, spotlight, evoNarrative)

		// Replace template "Looking Ahead" with LLM version
		if lookingAheadNarrative != "" {
			post.BodyMarkdown = replaceLookingAhead(post.BodyMarkdown, lookingAheadNarrative)
		}

		post.ContentMd = post.BodyMarkdown

		// Enhance summary with LLM-generated insight
		if spotlight != "" {
			firstSentence := extractFirstSentence(spotlight)
			if firstSentence != "" {
				post.Summary = buildMetaReportSummary(eloMovers, strategies, bestMatch) + " " + truncateSummary(firstSentence, 100)
			}
		}
	}

	return post
}

// spliceLLMContent inserts LLM-generated sections into the template report.
// Counter-Strategy Spotlight goes before "Evolution Highlights".
// Evolution Deep Dive goes after the evolution highlights table.
func spliceLLMContent(template string, spotlight, evoNarrative string) string {
	result := template

	if spotlight != "" {
		section := fmt.Sprintf("\n## Counter-Strategy Spotlight\n\n%s\n", spotlight)
		idx := findSectionIndex(result, "## Evolution Highlights")
		if idx >= 0 {
			result = result[:idx] + section + result[idx:]
		} else {
			idx = findSectionIndex(result, "## Looking Ahead")
			if idx >= 0 {
				result = result[:idx] + section + result[idx:]
			} else {
				result += section
			}
		}
	}

	if evoNarrative != "" {
		section := fmt.Sprintf("\n### Evolution Deep Dive\n\n%s\n", evoNarrative)
		idx := findSectionIndex(result, "## Looking Ahead")
		if idx >= 0 {
			result = result[:idx] + section + result[idx:]
		} else {
			result += section
		}
	}

	return result
}

// replaceLookingAhead replaces the template "## Looking Ahead" section with LLM-generated content.
func replaceLookingAhead(content, llmContent string) string {
	idx := findSectionIndex(content, "## Looking Ahead")
	if idx < 0 {
		// No existing section; append
		return content + fmt.Sprintf("\n## Looking Ahead\n\n%s\n", llmContent)
	}

	// Find the next ## section (or end of content) to delimit the replacement
	endIdx := len(content)
	for i := idx + len("## Looking Ahead"); i < len(content)-2; i++ {
		if content[i] == '\n' && content[i+1] == '#' {
			endIdx = i
			break
		}
	}

	return content[:idx] + fmt.Sprintf("## Looking Ahead\n\n%s\n", llmContent) + content[endIdx:]
}

// extractFirstSentence returns the first sentence from LLM output (for summary generation).
func extractFirstSentence(text string) string {
	// Clean leading whitespace
	text = strings.TrimSpace(text)
	// Find first period, exclamation, or question mark followed by space or end
	for i, ch := range text {
		if (ch == '.' || ch == '!' || ch == '?') && (i+1 >= len(text) || text[i+1] == ' ') {
			return text[:i+1]
		}
	}
	// No sentence boundary found — return first 100 chars
	if len(text) > 100 {
		return truncateSummary(text, 100)
	}
	return text
}

// buildSpotlightPrompt creates the LLM prompt for the Counter-Strategy Spotlight section.
// Per plan §15.1, the prompt uses sports-journalism framing with structured match context
// including rivalry dynamics, ELO deltas, critical moments, and season standings.
func buildSpotlightPrompt(data *IndexData, movers []eloMover, strats []strategyCount, bestMatch *notableMatch, evoHighlights []evolutionHighlight, topBots []BotData, rivalries []RivalryData) string {
	var sb strings.Builder

	// §15.1 instruction: sports-journalism prompt with structured contextual match data
	sb.WriteString("Write a 200-word 'Counter-Strategy Spotlight' section for the weekly meta report. ")
	sb.WriteString("You are a sports journalist covering an emergent bot league. ")
	sb.WriteString("Identify under-represented archetypes that could exploit weaknesses in the dominant meta. ")
	sb.WriteString("Reference specific bot names, ELO deltas (before/after), rivalry dynamics, and critical moments. ")
	sb.WriteString("Be dramatic but factual. Write in present tense with a punchy, journalistic tone. Do not use emojis.\n\n")

	// Season standings context
	sb.WriteString(fmt.Sprintf("Season: %s\n", getCurrentSeasonName(data)))
	sb.WriteString(fmt.Sprintf("Active bots: %d, Matches this week: %d\n\n", len(data.Bots), countWeeklyMatches(data)))

	// Season standings (top 5 with rank, rating delta, archetype)
	sb.WriteString("Season standings (top 5):\n")
	for i, bot := range topBots {
		if i >= 5 {
			break
		}
		winRate := calculateWinRate(bot.MatchesPlayed, bot.MatchesWon) * 100
		delta := computeRatingDelta(bot.ID, data)
		deltaStr := ""
		if delta != 0 {
			deltaStr = fmt.Sprintf(", weekly %+0.f", delta)
		}
		sb.WriteString(fmt.Sprintf("  #%d %s (ELO %d%s, %.0f%% win rate, archetype: %s)\n",
			i+1, bot.Name, int(bot.Rating), deltaStr, winRate, nonEmpty(bot.Archetype, "unclassified")))
	}

	// ELO movers with before/after deltas (§15.1 spec)
	sb.WriteString("\nTop 5 ELO movers this week:\n")
	for _, m := range movers {
		dir := "climbed"
		if m.Delta < 0 {
			dir = "dropped"
		}
		sb.WriteString(fmt.Sprintf("  %s %s %.0f points (ELO %.0f → %.0f) [%s] — W%d/L%d\n",
			m.BotName, dir, absF(m.Delta), m.OldRating, m.NewRating, nonEmpty(m.Archetype, "unclassified"), m.MatchesWon, m.MatchesLost))
	}

	sb.WriteString("\nStrategy distribution:\n")
	for _, s := range strats {
		sb.WriteString(fmt.Sprintf("  %s: %d bots (avg ELO %.0f, %d in top 20)\n",
			s.Archetype, s.Count, s.AvgRating, s.InTop20))
	}

	// Matchup matrix: archetype-vs-archetype win/loss data (§15.1 head-to-head stats)
	matchups := calculateMatchupMatrix(data)
	if len(matchups) > 0 {
		sb.WriteString("\nHead-to-head matchup matrix (top advantages):\n")
		for _, mc := range matchups {
			total := mc.Wins + mc.Losses
			winPct := 0.0
			if total > 0 {
				winPct = float64(mc.Wins) / float64(total) * 100
			}
			sb.WriteString(fmt.Sprintf("  %s vs %s: %dW/%dL (%.0f%%)\n",
				mc.Attacker, mc.Defender, mc.Wins, mc.Losses, winPct))
		}
	}

	// Strategy trends: week-over-week shifts
	trends := calculateStrategyTrends(data)
	if len(trends) > 0 {
		sb.WriteString("\nStrategy trends (week-over-week):\n")
		for _, t := range trends {
			arrow := "stable"
			if t.Shift > 2 {
				arrow = "rising"
			} else if t.Shift < -2 {
				arrow = "declining"
			}
			sb.WriteString(fmt.Sprintf("  %s: %.1f%% of top 20 (was %.1f%%, %s %+.1fpp), avg ELO %.0f\n",
				t.Archetype, t.ThisWeekPct, t.LastWeekPct, arrow, t.Shift, t.AvgRating))
		}
	}

	// Most-watched match with critical moments context (§13.2)
	if bestMatch != nil {
		sb.WriteString(fmt.Sprintf("\nMatch of the week: %s — score %s in %d turns [match %s]\n",
			bestMatch.Description, bestMatch.Score, bestMatch.TurnCount, bestMatch.MatchID))
		// Include pre-match ELO for participants if available
		for _, m := range data.Matches {
			if m.ID == bestMatch.MatchID && len(m.Participants) >= 2 {
				for _, p := range m.Participants {
					sb.WriteString(fmt.Sprintf("  %s: pre-match ELO %.0f\n",
						getBotName(p.BotID, data), p.PreMatchRating))
				}
				break
			}
		}
	}

	// Rivalry context with ELO deltas and head-to-head records (§15.1)
	if len(rivalries) > 0 {
		sb.WriteString("\nActive rivalries (head-to-head):\n")
		for i, r := range rivalries {
			if i >= 5 {
				break
			}
			botAName := r.BotAID
			botBName := r.BotBID
			var botARating, botBRating float64
			var botADelta, botBDelta float64
			for _, b := range data.Bots {
				if b.ID == r.BotAID {
					botAName = b.Name
					botARating = b.Rating
					botADelta = computeRatingDelta(b.ID, data)
				}
				if b.ID == r.BotBID {
					botBName = b.Name
					botBRating = b.Rating
					botBDelta = computeRatingDelta(b.ID, data)
				}
			}
			sb.WriteString(fmt.Sprintf("  %s (ELO %.0f, weekly %+0.f) vs %s (ELO %.0f, weekly %+0.f): %d-%d over %d matches\n",
				botAName, botARating, botADelta, botBName, botBRating, botBDelta, r.BotAWins, r.BotBWins, r.TotalMatches))
		}
	}

	return sb.String()
}

// buildEvolutionDeepDivePrompt creates the LLM prompt for the Evolution Deep Dive section.
// Per plan §15.1, includes rivalry context, ELO trajectory, lineage data, and season standings.
func buildEvolutionDeepDivePrompt(data *IndexData, evoHighlights []evolutionHighlight, rivalries []RivalryData, predLeaderboard []PredictorStats, liveData *evolutionLiveData) string {
	var sb strings.Builder

	sb.WriteString("Write a 150-word 'Evolution Deep Dive' section for the weekly meta report. ")
	sb.WriteString("You are a sports journalist covering the AI evolution pipeline in AI Code Battle. ")
	sb.WriteString("Highlight the most successful evolved bots, their lineage, strategic innovations, and ELO trajectory. ")
	sb.WriteString("Reference specific bot names, ELO before/after, lineage details, and rivalry context. Do not use emojis.\n\n")

	sb.WriteString(fmt.Sprintf("Season: %s\n\n", getCurrentSeasonName(data)))

	// Evolved bot profiles with ELO trajectory
	sb.WriteString("Evolved bot performance this week:\n")
	for _, e := range evoHighlights {
		winRate := 0.0
		if e.WeekMatches > 0 {
			winRate = float64(e.WeekWins) / float64(e.WeekMatches) * 100
		}
		rank := getBotRank(e.BotID, data)
		rankStr := ""
		if rank > 0 {
			rankStr = fmt.Sprintf(", ranked #%d", rank)
		}
		sb.WriteString(fmt.Sprintf("  %s: ELO %.0f%s, island=%s, gen=%d, weekly W%d/L%d (%.0f%% win rate), archetype=%s\n",
			e.BotName, e.Rating, rankStr, e.Island, e.Generation, e.WeekWins, e.WeekMatches-e.WeekWins, winRate, nonEmpty(e.Archetype, "evolved")))
		// Include lineage if available
		bot := findBotByID(e.BotID, data)
		if bot != nil && len(bot.ParentIDs) > 0 {
			sb.WriteString(fmt.Sprintf("    Lineage: parents %s\n", strings.Join(bot.ParentIDs, ", ")))
		}
	}

	// Count evolved bots in top 10 and top 20
	evolvedTop10, evolvedTop20 := 0, 0
	for i, bot := range data.Bots {
		if bot.Evolved {
			if i < 10 {
				evolvedTop10++
			}
			if i < 20 {
				evolvedTop20++
			}
		}
	}
	sb.WriteString(fmt.Sprintf("\nEvolved bots in top 10: %d, top 20: %d\n", evolvedTop10, evolvedTop20))

	// Live evolution data from R2 (population stats, promotion rates, island activity)
	if liveData != nil {
		sb.WriteString(fmt.Sprintf("\nEvolution pipeline: %d total generations, %d promoted today, %.1f%% 7-day promotion rate\n",
			liveData.Totals.GenerationsTotal, liveData.Totals.PromotedToday, liveData.Totals.PromotionRate7d))
		sb.WriteString(fmt.Sprintf("Highest evolved ELO: %.0f, evolved in top 10: %d\n",
			liveData.Totals.HighestEvolved, liveData.Totals.EvolvedInTop10))

		if len(liveData.Islands) > 0 {
			sb.WriteString("Island populations:\n")
			for name, island := range liveData.Islands {
				sb.WriteString(fmt.Sprintf("  %s: pop=%d, best=%.0f (%s)\n", name, island.Population, island.BestRating, island.BestBot))
			}
		}

		if len(liveData.RecentActivity) > 0 {
			sb.WriteString("Recent evolution activity (last 5):\n")
			count := 0
			for _, act := range liveData.RecentActivity {
				if count >= 5 {
					break
				}
				sb.WriteString(fmt.Sprintf("  %s: %s on %s island — %s (%s)\n",
					act.Time, act.Candidate, act.Island, act.Result, act.Reason))
				count++
			}
		}
	}

	// Active rivalries involving evolved bots with ELO context
	if len(rivalries) > 0 {
		sb.WriteString("\nRivalries involving evolved bots:\n")
		for i, r := range rivalries {
			if i >= 3 {
				break
			}
			botAName := getBotName(r.BotAID, data)
			botBName := getBotName(r.BotBID, data)
			var botARating, botBRating float64
			for _, b := range data.Bots {
				if b.ID == r.BotAID {
					botARating = b.Rating
				}
				if b.ID == r.BotBID {
					botBRating = b.Rating
				}
			}
			sb.WriteString(fmt.Sprintf("  %s (ELO %.0f) vs %s (ELO %.0f): %d-%d over %d matches\n",
				botAName, botARating, botBName, botBRating, r.BotAWins, r.BotBWins, r.TotalMatches))
		}
	}

	// Prediction leaderboard context
	if len(predLeaderboard) > 0 {
		top := predLeaderboard[0]
		total := top.Correct + top.Incorrect
		if total > 0 {
			sb.WriteString(fmt.Sprintf("\nTop predictor accuracy: %d/%d (%.0f%%), streak: %d\n",
				top.Correct, total, float64(top.Correct)/float64(total)*100, top.BestStreak))
		}
	}

	return sb.String()
}

// buildLookingAheadPrompt creates the LLM prompt for the Looking Ahead section.
// Per plan §15.1, includes ELO trends, rivalry dynamics, season championship positioning.
func buildLookingAheadPrompt(data *IndexData, movers []eloMover, strats []strategyCount, trends []strategyTrend, matchups []matchupCell, liveData *evolutionLiveData) string {
	var sb strings.Builder

	sb.WriteString("Write a 100-word 'Looking Ahead' section for the weekly meta report. ")
	sb.WriteString("You are a sports journalist covering AI Code Battle. ")
	sb.WriteString("Predict what strategies will rise or fall next week based on ELO trends, matchup data, rivalry dynamics, and the evolution pipeline. ")
	sb.WriteString("Reference specific bots, ELO before/after, and rivalry stakes. Do not use emojis.\n\n")

	sb.WriteString(fmt.Sprintf("Season: %s\n", getCurrentSeasonName(data)))

	// Season championship positioning
	for i := range data.Seasons {
		if data.Seasons[i].Status == "active" {
			s := data.Seasons[i]
			daysElapsed := data.GeneratedAt.Sub(s.StartsAt).Hours() / 24
			weekNum := int(daysElapsed/7) + 1
			if weekNum > 4 {
				weekNum = 4
			}
			sb.WriteString(fmt.Sprintf("Season progress: Week %d of 4", weekNum))
			if weekNum >= 3 {
				sb.WriteString(" — championship bracket approaching")
			}
			sb.WriteString("\n")
			break
		}
	}

	if len(movers) > 0 {
		sb.WriteString("\nTop ELO movers (with before/after):\n")
		for _, m := range movers {
			dir := "surged"
			if m.Delta < 0 {
				dir = "dropped"
			}
			sb.WriteString(fmt.Sprintf("  %s %s %.0f points (ELO %.0f → %.0f) [%s]\n",
				m.BotName, dir, absF(m.Delta), m.OldRating, m.NewRating, nonEmpty(m.Archetype, "unclassified")))
		}
	}

	if len(trends) > 0 {
		sb.WriteString("\nStrategy trends:\n")
		for _, t := range trends {
			sb.WriteString(fmt.Sprintf("  %s: %.1f%% of top 20 (shift %+.1fpp)\n", t.Archetype, t.ThisWeekPct, t.Shift))
		}
	}

	if len(matchups) > 0 {
		sb.WriteString("\nKey matchup advantages:\n")
		for i, mc := range matchups {
			if i >= 5 {
				break
			}
			sb.WriteString(fmt.Sprintf("  %s > %s (%d-%d)\n", mc.Attacker, mc.Defender, mc.Wins, mc.Losses))
		}
	}

	if len(strats) > 0 {
		sb.WriteString(fmt.Sprintf("\nDominant strategy: %s (%d bots, %d in top 20)\n",
			strats[0].Archetype, strats[0].Count, strats[0].InTop20))
	}

	if liveData != nil {
		sb.WriteString(fmt.Sprintf("\nEvolution pipeline: %d generations, %.1f%% promotion rate, highest evolved ELO %.0f\n",
			liveData.Totals.GenerationsTotal, liveData.Totals.PromotionRate7d, liveData.Totals.HighestEvolved))
	}

	return sb.String()
}

// countWeeklyMatches returns the number of matches played in the past 7 days.
func countWeeklyMatches(data *IndexData) int {
	weekAgo := data.GeneratedAt.AddDate(0, 0, -7)
	count := 0
	for _, m := range data.Matches {
		if m.PlayedAt.After(weekAgo) {
			count++
		}
	}
	return count
}

// ─── Matchup analysis ──────────────────────────────────────────────────────────

type matchupCell struct {
	Attacker string // archetype attacking
	Defender string // archetype defending
	Wins     int
	Losses   int
}

// calculateMatchupMatrix builds a week-over-week matchup matrix showing which
// archetypes beat which. Returns the top matchup advantages.
func calculateMatchupMatrix(data *IndexData) []matchupCell {
	weekAgo := data.GeneratedAt.AddDate(0, 0, -7)
	cells := make(map[string]*matchupCell)

	for _, m := range data.Matches {
		if m.PlayedAt.Before(weekAgo) || len(m.Participants) < 2 || m.WinnerID == "" {
			continue
		}

		// Find winner and loser archetypes
		var winnerArch, loserArch string
		for _, p := range m.Participants {
			arch := getBotArchetype(p.BotID, data)
			if p.Won {
				winnerArch = arch
			} else {
				loserArch = arch
			}
		}
		if winnerArch == "" || loserArch == "" {
			continue
		}

		key := winnerArch + ">" + loserArch
		if cells[key] == nil {
			cells[key] = &matchupCell{Attacker: winnerArch, Defender: loserArch}
		}
		cells[key].Wins++

		// Also record the loss direction
		lossKey := loserArch + ">" + winnerArch
		if cells[lossKey] == nil {
			cells[lossKey] = &matchupCell{Attacker: loserArch, Defender: winnerArch}
		}
		cells[lossKey].Losses++
	}

	// Sort by win differential (most dominant matchups first)
	result := make([]matchupCell, 0, len(cells))
	for _, c := range cells {
		result = append(result, *c)
	}
	sort.Slice(result, func(i, j int) bool {
		di := result[i].Wins - result[i].Losses
		dj := result[j].Wins - result[j].Losses
		return di > dj
	})

	if len(result) > 10 {
		return result[:10]
	}
	return result
}

// getBotArchetype returns the archetype for a bot, with a sensible fallback.
func getBotArchetype(botID string, data *IndexData) string {
	for _, bot := range data.Bots {
		if bot.ID == botID {
			if bot.Archetype != "" {
				return bot.Archetype
			}
			if bot.Evolved {
				return "evolved-unknown"
			}
			return "standard"
		}
	}
	return "unknown"
}

// ─── Strategy trend analysis ───────────────────────────────────────────────────

type strategyTrend struct {
	Archetype    string
	ThisWeekPct  float64 // % of top-20 this week
	LastWeekPct  float64 // % of top-20 implied from rating history
	Shift        float64 // ThisWeekPct - LastWeekPct
	AvgRating    float64
	Count        int
}

// calculateStrategyTrends compares archetype representation in the top 20 this
// week vs the prior week using rating history to infer shifts.
func calculateStrategyTrends(data *IndexData) []strategyTrend {
	weekAgo := data.GeneratedAt.AddDate(0, 0, -7)

	// Current top 20 archetype counts
	currentArchs := make(map[string]int)
	currentRatingSum := make(map[string]float64)
	topN := 20
	if len(data.Bots) < topN {
		topN = len(data.Bots)
	}
	for i := 0; i < topN; i++ {
		arch := data.Bots[i].Archetype
		if arch == "" {
			if data.Bots[i].Evolved {
				arch = "evolved-unknown"
			} else {
				arch = "standard"
			}
		}
		currentArchs[arch]++
		currentRatingSum[arch] += data.Bots[i].Rating
	}

	// Estimate last week's top 20 from rating history
	lastWeekArchs := make(map[string]int)
	for _, bot := range data.Bots[:topN] {
		history := getBotRatingHistory(bot.ID, data)
		ratingWeekAgo := bot.Rating // default to current if no history
		for _, rh := range history {
			if (rh.RecordedAt.Before(weekAgo) || rh.RecordedAt.Equal(weekAgo)) && rh.Rating > 0 {
				ratingWeekAgo = rh.Rating
			}
		}
		// If the bot's rating a week ago was competitive, count it
		_ = ratingWeekAgo
		arch := bot.Archetype
		if arch == "" {
			if bot.Evolved {
				arch = "evolved-unknown"
			} else {
				arch = "standard"
			}
		}
		lastWeekArchs[arch]++
	}

	// Build trend data
	trendMap := make(map[string]*strategyTrend)
	for arch, count := range currentArchs {
		trendMap[arch] = &strategyTrend{
			Archetype:   arch,
			ThisWeekPct: float64(count) / float64(topN) * 100,
			Count:       count,
			AvgRating:   currentRatingSum[arch] / float64(count),
		}
	}
	for arch, count := range lastWeekArchs {
		if trendMap[arch] == nil {
			trendMap[arch] = &strategyTrend{Archetype: arch}
		}
		trendMap[arch].LastWeekPct = float64(count) / float64(topN) * 100
	}

	trends := make([]strategyTrend, 0, len(trendMap))
	for _, t := range trendMap {
		t.Shift = t.ThisWeekPct - t.LastWeekPct
		trends = append(trends, *t)
	}

	// Sort by absolute shift (biggest movers first)
	sort.Slice(trends, func(i, j int) bool {
		return absF(trends[i].Shift) > absF(trends[j].Shift)
	})

	if len(trends) > 8 {
		return trends[:8]
	}
	return trends
}

// ─── Evolution live data from R2 ───────────────────────────────────────────────

// evolutionLiveData represents key fields from evolution/live.json on R2.
type evolutionLiveData struct {
	Totals struct {
		GenerationsTotal int     `json:"generations_total"`
		PromotedToday    int     `json:"promoted_today"`
		PromotionRate7d  float64 `json:"promotion_rate_7d"`
		HighestEvolved   float64 `json:"highest_evolved_rating"`
		EvolvedInTop10   int     `json:"evolved_in_top_10"`
	} `json:"totals"`
	Islands map[string]struct {
		Population int     `json:"population"`
		BestRating float64 `json:"best_rating"`
		BestBot    string  `json:"best_bot"`
	} `json:"islands"`
	RecentActivity []struct {
		Time       string `json:"time"`
		Candidate  string `json:"candidate"`
		Island     string `json:"island"`
		Result     string `json:"result"`
		Reason     string `json:"reason"`
		Stage      string `json:"stage"`
	} `json:"recent_activity"`
}

// fetchEvolutionLiveData attempts to fetch live.json from R2. Returns nil on failure.
func fetchEvolutionLiveData(ctx context.Context, cfg *Config) *evolutionLiveData {
	if cfg.R2AccessKey == "" || cfg.R2BucketName == "" {
		return nil
	}

	client, err := NewS3Client(cfg.R2Endpoint, cfg.R2AccessKey, cfg.R2SecretKey, cfg.R2BucketName)
	if err != nil {
		slog.Debug("Failed to create R2 client for live.json", "error", err)
		return nil
	}

	body, err := client.downloadObject(ctx, "evolution/live.json")
	if err != nil {
		slog.Debug("Failed to fetch evolution/live.json from R2", "error", err)
		return nil
	}
	defer body.Close()

	var live evolutionLiveData
	if err := json.NewDecoder(body).Decode(&live); err != nil {
		slog.Debug("Failed to decode evolution/live.json", "error", err)
		return nil
	}

	return &live
}

// nonEmpty returns the first non-empty string, or fallback.
func nonEmpty(s, fallback string) string {
	if s != "" {
		return s
	}
	return fallback
}


func findSectionIndex(content, section string) int {
	// Find "## Looking Ahead" as a section header
	for i := 0; i < len(content)-len(section); i++ {
		if content[i:i+len(section)] == section {
			// Make sure it's at start of line
			if i == 0 || content[i-1] == '\n' {
				return i
			}
		}
	}
	return -1
}

func buildMetaReportSummary(movers []eloMover, strats []strategyCount, bestMatch *notableMatch) string {
	parts := make([]string, 0)

	if len(movers) > 0 {
		top := movers[0]
		dir := "climbed"
		if top.Delta < 0 {
			dir = "dropped"
		}
		parts = append(parts, fmt.Sprintf("%s %s %.0f points", top.BotName, dir, absF(top.Delta)))
	}

	if len(strats) > 0 {
		parts = append(parts, fmt.Sprintf("%s leads with %d bots", strats[0].Archetype, strats[0].Count))
	}

	if bestMatch != nil {
		parts = append(parts, fmt.Sprintf("featured match: %s", bestMatch.Description))
	}

	if len(parts) == 0 {
		return "Competitive analysis for this week."
	}

	summary := parts[0]
	for i := 1; i < len(parts); i++ {
		summary += ". " + parts[i]
	}
	return summary + "."
}

// ─── Chronicle generation ──────────────────────────────────────────────────────

// generateChronicles creates story arc chronicles from match data (template-based fallback)
func generateChronicles(data *IndexData) []BlogPost {
	chronicles := make([]BlogPost, 0)

	if len(data.Bots) > 0 {
		rising := findRisingBots(data)
		if len(rising) > 0 {
			chronicles = append(chronicles, generateRiseChronicle(rising[0], data))
		}
	}

	upsets := findRecentUpsets(data)
	if len(upsets) > 0 {
		chronicles = append(chronicles, generateUpsetChronicle(upsets[0], data))
	}

	rivalries := findTopRivalries(data)
	if len(rivalries) > 0 {
		chronicles = append(chronicles, generateRivalryChronicle(rivalries[0], data))
	}

	return chronicles
}

// generateLLMChronicles creates chronicles using the narrative engine and LLM
func generateLLMChronicles(ctx context.Context, data *IndexData, llmClient *LLMClient) []BlogPost {
	chronicles := make([]BlogPost, 0)

	arcs := detectStoryArcs(data)

	maxChronicles := 5
	if len(arcs) < maxChronicles {
		maxChronicles = len(arcs)
	}

	for i := 0; i < maxChronicles; i++ {
		arc := arcs[i]

		var post BlogPost
		var err error

		if llmClient != nil && llmClient.baseURL != "" {
			post, err = generateLLMChronicle(ctx, arc, data, llmClient)
			if err != nil {
				post = generateTemplateChronicle(arc, data)
			}
		} else {
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

	dateStr := data.GeneratedAt.Format("2006-01-02")
	content := "# " + headline + "\n\n" + narrative

	return BlogPost{
		Slug:         slug,
		Title:        headline,
		PublishedAt:  dateStr,
		Date:         dateStr,
		Type:         "chronicle",
		BodyMarkdown: content,
		ContentMd:    content,
		Summary:      truncateSummary(narrative, 150),
		Tags:         tags,
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

	dateStr := data.GeneratedAt.Format("2006-01-02")
	content := fmt.Sprintf("# %s: %s\n\nDetails pending.", arc.Type, arc.BotName)
	return BlogPost{
		Slug:         fmt.Sprintf("%s-%s-%s", arc.Type, arc.BotID, formatSlugDate(data.GeneratedAt)),
		Title:        fmt.Sprintf("%s: %s", arc.Type, arc.BotName),
		PublishedAt:  dateStr,
		Date:         dateStr,
		Type:         "chronicle",
		BodyMarkdown: content,
		ContentMd:    content,
		Summary:      fmt.Sprintf("Story arc: %s involving %s", arc.Type, arc.BotName),
		Tags:         []string{string(arc.Type), arc.BotID},
	}
}

// ─── Template chronicles ──────────────────────────────────────────────────────

func generateRiseChronicle(bot BotData, data *IndexData) BlogPost {
	dateStr := data.GeneratedAt.Format("2006-01-02")
	winRate := calculateWinRate(bot.MatchesPlayed, bot.MatchesWon) * 100
	ratingDelta := computeRatingDelta(bot.ID, data)
	keyMatches := extractKeyMatches(bot.ID, data)

	var keyMatchSection string
	if len(keyMatches) > 0 {
		keyMatchSection = "\n## Key Matches\n\n"
		for _, m := range keyMatches {
			outcome := "defeated"
			if !m.Won {
				outcome = "lost to"
			}
			keyMatchSection += fmt.Sprintf("- **%s** %s %s (rating %d) — score %s, %d turns on %q\n",
				bot.Name, outcome, m.OpponentName, m.OpponentRating, m.Score, m.TurnCount, nonEmpty(m.MapName, "standard map"))
		}
	}

	archetypeLine := ""
	if bot.Archetype != "" {
		archetypeLine = fmt.Sprintf("\n- **Archetype:** %s", bot.Archetype)
	}
	evolvedLine := ""
	if bot.Evolved {
		evolvedLine = fmt.Sprintf("\n- **Origin:** Evolved, %s island, generation %d", nonEmpty(bot.Island, "unknown"), bot.Generation)
	}

	var deltaLine string
	if ratingDelta != 0 {
		sign := ""
		if ratingDelta > 0 {
			sign = "+"
		}
		deltaLine = fmt.Sprintf("\n- **Weekly Rating Change:** %s%.0f points", sign, ratingDelta)
	}

	content := fmt.Sprintf(`# The Rise of %s

%s surged %d points this week to reach a rating of %d. With a %.1f%% win rate across %d matches, the bot's trajectory signals a genuine shift in competitive standing.

## Profile

- **Rating:** %d%s%s%s

%s

## What's Driving the Climb

The improvement pattern suggests %s has found a strategic edge in the current meta. %s rating convergence means the bot is still settling — further gains or a plateau are equally likely in the coming week.

---

*Auto-generated chronicle from match data analysis.*
`,
		bot.Name,
		bot.Name, int(absF(ratingDelta)), int(bot.Rating), winRate, bot.MatchesPlayed,
		int(bot.Rating), archetypeLine, evolvedLine, deltaLine,
		keyMatchSection,
		bot.Name,
		map[bool]string{true: "Low", false: "Moderate"}[bot.RatingDeviation < 100],
	)

	return BlogPost{
		Slug:         fmt.Sprintf("rise-%s-%s", bot.ID, formatSlugDate(data.GeneratedAt)),
		Title:        fmt.Sprintf("The Rise of %s", bot.Name),
		PublishedAt:  dateStr,
		Date:         dateStr,
		Type:         "chronicle",
		BodyMarkdown: content,
		ContentMd:    content,
		Summary:      fmt.Sprintf("%s surged %d points to rating %d (%.0f%% win rate).", bot.Name, int(absF(ratingDelta)), int(bot.Rating), winRate),
		Tags:         []string{"rise", bot.ID},
	}
}

func generateUpsetChronicle(upset UpsetData, data *IndexData) BlogPost {
	winnerName := getBotName(upset.WinnerID, data)
	loserName := getBotName(upset.LoserID, data)
	dateStr := data.GeneratedAt.Format("2006-01-02")

	// Compute rating gap context
	winnerBot := findBotByID(upset.WinnerID, data)
	loserBot := findBotByID(upset.LoserID, data)
	var ratingGapStr string
	if winnerBot != nil && loserBot != nil {
		gap := loserBot.Rating - winnerBot.Rating
		ratingGapStr = fmt.Sprintf("%d rated", int(loserBot.Rating))
		if gap > 0 {
			ratingGapStr = fmt.Sprintf("%d-rated, %d points above the winner", int(loserBot.Rating), int(gap))
		}
	} else {
		ratingGapStr = "higher-rated"
	}

	scoreDiff := upset.WinnerScore - upset.LoserScore
	var marginStr string
	if scoreDiff <= 1 {
		marginStr = "by the thinnest possible margin"
	} else if scoreDiff <= 3 {
		marginStr = "by a convincing margin"
	} else {
		marginStr = "in dominant fashion"
	}

	content := fmt.Sprintf(`# Upset: %s Defeats %s

%s, the underdog, has defeated the %s %s in a match decided %d-%d %s after %d turns.

## Match Breakdown

- **Winner:** %s (score %d)
- **Loser:** %s (score %d)
- **Duration:** %d turns
- **Match ID:** %s

## How It Happened

The rating gap suggested %s would control this match from the start. Instead, %s found openings through tactical positioning and resource management, seizing momentum and converting it into a decisive victory. The result sends ripples through the leaderboard standings.

---

*Auto-generated chronicle from match analysis.*
`,
		winnerName, loserName,
		winnerName, loserName, ratingGapStr,
		upset.WinnerScore, upset.LoserScore, marginStr, upset.TurnCount,
		winnerName, upset.WinnerScore,
		loserName, upset.LoserScore,
		upset.TurnCount,
		upset.MatchID,
		loserName, winnerName,
	)

	return BlogPost{
		Slug:         fmt.Sprintf("upset-%s-%s", upset.MatchID[:8], formatSlugDate(data.GeneratedAt)),
		Title:        fmt.Sprintf("Upset: %s Defeats %s", winnerName, loserName),
		PublishedAt:  dateStr,
		Date:         dateStr,
		Type:         "chronicle",
		BodyMarkdown: content,
		ContentMd:    content,
		Summary:      fmt.Sprintf("%s upset %s %d-%d in %d turns.", winnerName, loserName, upset.WinnerScore, upset.LoserScore, upset.TurnCount),
		Tags:         []string{"upset", upset.WinnerID, upset.LoserID},
	}
}

func generateRivalryChronicle(rivalry RivalryData, data *IndexData) BlogPost {
	botAName := getBotName(rivalry.BotAID, data)
	botBName := getBotName(rivalry.BotBID, data)
	dateStr := data.GeneratedAt.Format("2006-01-02")

	// Get bot ratings and archetypes for richer context
	botA := findBotByID(rivalry.BotAID, data)
	botB := findBotByID(rivalry.BotBID, data)

	var profileSection string
	if botA != nil && botB != nil {
		profileSection = fmt.Sprintf("\n| | %s | %s |\n|---|---|---|\n", botAName, botBName)
		profileSection += fmt.Sprintf("| **Rating** | %d | %d |\n", int(botA.Rating), int(botB.Rating))
		profileSection += fmt.Sprintf("| **Win Rate** | %.0f%% | %.0f%% |\n",
			calculateWinRate(botA.MatchesPlayed, botA.MatchesWon)*100,
			calculateWinRate(botB.MatchesPlayed, botB.MatchesWon)*100)
		if botA.Archetype != "" || botB.Archetype != "" {
			profileSection += fmt.Sprintf("| **Archetype** | %s | %s |\n",
				nonEmpty(botA.Archetype, "—"), nonEmpty(botB.Archetype, "—"))
		}
	}

	// Recent encounters
	recentMatches := extractRivalryMatches(rivalry.BotAID, rivalry.BotBID, data)
	var recentSection string
	if len(recentMatches) > 0 {
		recentSection = "\n## Recent Encounters\n\n"
		for _, m := range recentMatches {
			outcome := "lost"
			if m.Won {
				outcome = "won"
			}
			recentSection += fmt.Sprintf("- %s %s against %s (%s, %d turns)\n",
				botAName, outcome, botBName, m.Score, m.TurnCount)
		}
	}

	// Balance assessment
	totalGames := rivalry.BotAWins + rivalry.BotBWins
	var balanceStr string
	if totalGames == 0 {
		balanceStr = "evenly matched"
	} else {
		balance := abs(rivalry.BotAWins-rivalry.BotBWins) * 100 / totalGames
		if balance <= 10 {
			balanceStr = "dead even"
		} else if balance <= 25 {
			balanceStr = "closely contested"
		} else {
			leader := botAName
			if rivalry.BotBWins > rivalry.BotAWins {
				leader = botBName
			}
			balanceStr = fmt.Sprintf("tilting toward %s", leader)
		}
	}

	content := fmt.Sprintf(`# Rivalry: %s vs %s

%d matches. %d-%d. The series between %s and %s is %s.

## Head-to-Head

- **%s:** %d wins
- **%s:** %d wins
- **Total Matches:** %d
%s
%s

## The Dynamic

%s

---

*Auto-generated chronicle from rivalry analysis.*
`,
		botAName, botBName,
		rivalry.TotalMatches, rivalry.BotAWins, rivalry.BotBWins, botAName, botBName, balanceStr,
		botAName, rivalry.BotAWins,
		botBName, rivalry.BotBWins,
		rivalry.TotalMatches,
		profileSection,
		recentSection,
		map[bool]string{true: "Every encounter between these two shifts the balance of power.", false: "The next match could shift the series dynamic."}[totalGames >= 10],
	)

	return BlogPost{
		Slug:         fmt.Sprintf("rivalry-%s-%s", rivalry.BotAID[:8], rivalry.BotBID[:8]),
		Title:        fmt.Sprintf("Rivalry: %s vs %s", botAName, botBName),
		PublishedAt:  dateStr,
		Date:         dateStr,
		Type:         "chronicle",
		BodyMarkdown: content,
		ContentMd:    content,
		Summary:      fmt.Sprintf("%s and %s: %d-%d over %d matches. %s.", botAName, botBName, rivalry.BotAWins, rivalry.BotBWins, rivalry.TotalMatches, balanceStr),
		Tags:         []string{"rivalry", rivalry.BotAID, rivalry.BotBID},
	}
}

// ─── Data types ────────────────────────────────────────────────────────────────

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
	BotAID       string
	BotBID       string
	BotAWins     int
	BotBWins     int
	TotalMatches int
}

// ─── Formatting helpers ────────────────────────────────────────────────────────

func getWeekNumber(t time.Time) int {
	_, week := t.ISOWeek()
	return week
}

func getCurrentSeasonName(data *IndexData) string {
	for _, s := range data.Seasons {
		if s.StartsAt.Before(data.GeneratedAt) {
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
		if bot.Evolved {
			dist["evolved"]++
		} else {
			dist["human-authored"]++
		}
	}
	return dist
}

func findRisingBots(data *IndexData) []BotData {
	rising := make([]BotData, 0)
	for _, bot := range data.Bots {
		if bot.MatchesPlayed >= 5 && calculateWinRate(bot.MatchesPlayed, bot.MatchesWon) > 0.6 {
			rising = append(rising, bot)
		}
	}
	if len(rising) > 3 {
		return rising[:3]
	}
	return rising
}

func findFallingBots(data *IndexData) []BotData {
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
		for i, p1 := range m.Participants {
			for _, p2 := range m.Participants[i+1:] {
				if p1.Won && p2.Score > p1.Score {
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

	rivalries := make([]RivalryData, 0)
	for _, r := range pairCounts {
		if r.TotalMatches >= 3 {
			rivalries = append(rivalries, *r)
		}
	}

	// Sort by total matches descending
	sort.Slice(rivalries, func(i, j int) bool {
		return rivalries[i].TotalMatches > rivalries[j].TotalMatches
	})

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

// computeRatingDelta returns the rating change over the past 7 days for a bot.
func computeRatingDelta(botID string, data *IndexData) float64 {
	history := getBotRatingHistory(botID, data)
	if len(history) < 2 {
		return 0
	}
	weekAgo := data.GeneratedAt.AddDate(0, 0, -7)
	var oldRating float64
	var found bool
	for _, rh := range history {
		if rh.RecordedAt.Before(weekAgo) || rh.RecordedAt.Equal(weekAgo) {
			oldRating = rh.Rating
			found = true
		}
	}
	if !found {
		return 0
	}
	bot := findBotByID(botID, data)
	if bot == nil {
		return 0
	}
	return bot.Rating - oldRating
}

func formatSlugDate(t time.Time) string {
	return t.Format("2006-01-02")
}

func seasonTag(seasonName string) string {
	if len(seasonName) > 8 && seasonName[:8] == "Season " {
		return "season-" + seasonName[8:]
	}
	return "season-" + seasonName
}

func truncateSummary(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	lastSpace := maxLen
	for i := maxLen - 1; i >= 0; i-- {
		if s[i] == ' ' {
			lastSpace = i
			break
		}
	}
	return s[:lastSpace] + "..."
}

func findBotByID(id string, data *IndexData) *BotData {
	for i := range data.Bots {
		if data.Bots[i].ID == id {
			return &data.Bots[i]
		}
	}
	return nil
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

func absF(f float64) float64 {
	if f < 0 {
		return -f
	}
	return f
}

// ─── Map of the Week ────────────────────────────────────────────────────────────

type mapOfTheWeek struct {
	MapID        string
	PlayerCount  int
	Engagement   float64
	WallDensity  float64
	EnergyCount  int
	MatchCount   int
	AvgTurnCount int
}

func findMapOfTheWeek(data *IndexData) *mapOfTheWeek {
	if len(data.Maps) == 0 {
		return nil
	}

	best := data.Maps[0]
	for _, m := range data.Maps[1:] {
		if m.Engagement > best.Engagement {
			best = m
		}
	}

	matchCount := 0
	totalTurns := 0
	for _, m := range data.Matches {
		if m.MapID == best.MapID {
			matchCount++
			totalTurns += m.TurnCount
		}
	}
	avgTurns := 0
	if matchCount > 0 {
		avgTurns = totalTurns / matchCount
	}

	return &mapOfTheWeek{
		MapID:        best.MapID,
		PlayerCount:  best.PlayerCount,
		Engagement:   best.Engagement,
		WallDensity:  best.WallDensity,
		EnergyCount:  best.EnergyCount,
		MatchCount:   matchCount,
		AvgTurnCount: avgTurns,
	}
}

// ─── Bot Spotlight ────────────────────────────────────────────────────────────

type botSpotlight struct {
	BotName     string
	BotID       string
	Rating      float64
	OldRating   float64
	Delta       float64
	Archetype   string
	Evolved     bool
	MatchesWon  int
	MatchesLost int
	WinRate     float64
	KeyWinDesc  string
}

func buildBotSpotlight(data *IndexData) *botSpotlight {
	movers := findTopELOMovers(data, 5)
	if len(movers) == 0 {
		return nil
	}

	// Spotlight the biggest gainer (prefer a riser over a faller)
	top := movers[0]
	for _, m := range movers {
		if m.Delta > 0 {
			top = m
			break
		}
	}

	bot := findBotByID(top.BotID, data)
	if bot == nil {
		return nil
	}

	winRate := 0.0
	if bot.MatchesPlayed > 0 {
		winRate = float64(bot.MatchesWon) / float64(bot.MatchesPlayed) * 100
	}

	// Find the key win this week
	keyWinDesc := ""
	weekAgo := data.GeneratedAt.AddDate(0, 0, -7)
	for _, m := range data.Matches {
		if m.PlayedAt.Before(weekAgo) || len(m.Participants) < 2 {
			continue
		}
		won := false
		var oppName string
		var oppRating float64
		for _, p := range m.Participants {
			if p.BotID == top.BotID && p.Won {
				won = true
			} else if p.BotID != top.BotID {
				oppName = getBotName(p.BotID, data)
				oppRating = p.PreMatchRating
			}
		}
		if won && oppName != "" {
			keyWinDesc = fmt.Sprintf("Defeated %s (rating %.0f) in match %s", oppName, oppRating, m.ID[:min(8, len(m.ID))])
			break
		}
	}

	return &botSpotlight{
		BotName:     top.BotName,
		BotID:       top.BotID,
		Rating:      top.NewRating,
		OldRating:   top.OldRating,
		Delta:       top.Delta,
		Archetype:   nonEmpty(top.Archetype, "unclassified"),
		Evolved:     top.Evolved,
		MatchesWon:  top.MatchesWon,
		MatchesLost: top.MatchesLost,
		WinRate:     winRate,
		KeyWinDesc:  keyWinDesc,
	}
}

// ─── Formatting helpers (meta report specific) ────────────────────────────────

func formatMapOfTheWeek(m *mapOfTheWeek) string {
	if m == nil {
		return "Not enough map data this week."
	}
	return fmt.Sprintf("**%s** — %d matches played, avg %.0f turns. Engagement score: %.1f. Players: %d, Walls: %.0f%%, Energy cells: %d.",
		m.MapID, m.MatchCount, float64(m.AvgTurnCount), m.Engagement, m.PlayerCount, m.WallDensity*100, m.EnergyCount)
}

func formatBotSpotlight(s *botSpotlight) string {
	if s == nil {
		return "No standout performer this week."
	}
	result := fmt.Sprintf("**%s** (rating %.0f, %s%.0f from %.0f) — Archetype: %s",
		s.BotName, s.Rating, arrow(s.Delta), absF(s.Delta), s.OldRating, s.Archetype)
	if s.Evolved {
		result += " [EVOLVED]"
	}
	result += fmt.Sprintf("\n- Win rate: %.1f%% (W%d/L%d)", s.WinRate, s.MatchesWon, s.MatchesLost)
	if s.KeyWinDesc != "" {
		result += fmt.Sprintf("\n- Key win: %s", s.KeyWinDesc)
	}
	return result
}

func formatStrategyTrends(trends []strategyTrend) string {
	if len(trends) == 0 {
		return "No trend data available yet."
	}
	result := "| Archetype | Share | Shift | Avg Rating |\n|-----------|-------|-------|------------|\n"
	for _, t := range trends {
		shift := fmt.Sprintf("%+.1fpp", t.Shift)
		result += fmt.Sprintf("| %s | %.0f%% | %s | %.0f |\n", t.Archetype, t.ThisWeekPct, shift, t.AvgRating)
	}
	return result
}

func formatMatchupInsights(matchups []matchupCell) string {
	if len(matchups) == 0 {
		return "No matchup data available yet."
	}
	result := "| Attacker | Defender | Wins | Losses | Advantage |\n|----------|----------|------|--------|-----------|\n"
	for _, c := range matchups {
		if c.Wins < 2 {
			continue
		}
		adv := c.Wins - c.Losses
		result += fmt.Sprintf("| %s | %s | %d | %d | %+d |\n", c.Attacker, c.Defender, c.Wins, c.Losses, adv)
	}
	if result == "| Attacker | Defender | Wins | Losses | Advantage |\n|----------|----------|------|--------|-----------|\n" {
		return "No dominant matchups this week."
	}
	return result
}

func arrow(delta float64) string {
	if delta > 0 {
		return "↑"
	}
	return "↓"
}

func formatLeaderboardTable(bots []BotData) string {
	result := ""
	for i, bot := range bots {
		winRate := calculateWinRate(bot.MatchesPlayed, bot.MatchesWon) * 100
		result += fmt.Sprintf("| %d | %s | %d | %.1f%% |\n", i+1, bot.Name, int(bot.Rating), winRate)
	}
	return result
}

func formatELOMoversTable(movers []eloMover) string {
	if len(movers) == 0 {
		return "No significant rating movement this week."
	}
	result := ""
	for _, m := range movers {
		dir := "↑"
		if m.Delta < 0 {
			dir = "↓"
		}
		tag := ""
		if m.Evolved {
			tag = " [EVO]"
		}
		result += fmt.Sprintf("| %s%s | %s%.0f | %.0f → %.0f | W%d/L%d |\n",
			m.BotName, tag, dir, m.Delta, m.OldRating, m.NewRating, m.MatchesWon, m.MatchesLost)
	}
	return result
}

func formatStrategyTable(strats []strategyCount) string {
	if len(strats) == 0 {
		return "No strategy data available yet."
	}
	result := "| Archetype | Count | Avg Rating | In Top 20 |\n|-----------|-------|------------|-----------|\n"
	for _, s := range strats {
		result += fmt.Sprintf("| %s | %d | %.0f | %d |\n", s.Archetype, s.Count, s.AvgRating, s.InTop20)
	}
	return result
}

func formatNotableMatch(m *notableMatch) string {
	if m == nil {
		return "No standout match this week."
	}
	return fmt.Sprintf("**%s** — Final score: %s in %d turns. [Watch replay](/watch/replay/%s)",
		m.Description, m.Score, m.TurnCount, m.MatchID)
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

func formatEvolutionHighlights(highlights []evolutionHighlight) string {
	if len(highlights) == 0 {
		return "No evolved bots active this week."
	}
	result := "| Bot | Rating | Island | Gen | Weekly Record |\n|-----|--------|--------|-----|---------------|\n"
	for _, e := range highlights {
		result += fmt.Sprintf("| %s | %.0f | %s | %d | W%d/L%d |\n",
			e.BotName, e.Rating, e.Island, e.Generation, e.WeekWins, e.WeekMatches-e.WeekWins)
	}
	return result
}

func formatEvolutionTrend(highlights []evolutionHighlight) string {
	if len(highlights) == 0 {
		return "not yet represented in"
	}
	topCount := 0
	for _, e := range highlights {
		if e.Rating >= 1500 && e.WeekWins > e.WeekMatches/2 {
			topCount++
		}
	}
	if topCount >= 3 {
		return "increasingly disrupting"
	} else if topCount >= 1 {
		return "making inroads into"
	}
	return "not yet represented in"
}

func formatPredictionStandings(data *IndexData) string {
	if len(data.TopPredictors) == 0 {
		return "No predictions recorded yet."
	}
	result := "| Rank | Predictor | Correct | Accuracy | Best Streak |\n|------|-----------|---------|----------|-------------|\n"
	for i, p := range data.TopPredictors {
		if i >= 5 {
			break
		}
		total := p.Correct + p.Incorrect
		accuracy := 0.0
		if total > 0 {
			accuracy = float64(p.Correct) / float64(total) * 100
		}
		result += fmt.Sprintf("| %d | %s | %d/%d | %.0f%% | %d |\n",
			i+1, p.PredictorID[:min(12, len(p.PredictorID))], p.Correct, total, accuracy, p.BestStreak)
	}
	return result
}

func formatSeasonProgress(data *IndexData) string {
	var active *SeasonData
	for i := range data.Seasons {
		if data.Seasons[i].Status == "active" {
			active = &data.Seasons[i]
			break
		}
	}
	if active == nil {
		return "No active season. The next season begins soon."
	}

	daysElapsed := data.GeneratedAt.Sub(active.StartsAt).Hours() / 24
	daysTotal := float64(28) // 4-week season
	if !active.EndsAt.IsZero() {
		daysTotal = active.EndsAt.Sub(active.StartsAt).Hours() / 24
	}
	weekNum := int(daysElapsed/7) + 1
	if weekNum > 4 {
		weekNum = 4
	}

	result := fmt.Sprintf("**%s** — %s (Week %d of 4)\n", active.Name, active.Theme, weekNum)
	result += fmt.Sprintf("- Days elapsed: %d / %.0f\n", int(daysElapsed), daysTotal)
	result += fmt.Sprintf("- Total matches played: %d\n", active.TotalMatches)

	if active.ChampionName != "" {
		result += fmt.Sprintf("- Champion: %s\n", active.ChampionName)
	}

	topBots := getTopBots(data, 3)
	if len(topBots) > 0 {
		result += "- Championship seeding: "
		names := make([]string, 0, len(topBots))
		for i, bot := range topBots {
			names = append(names, fmt.Sprintf("#%d %s (%d)", i+1, bot.Name, int(bot.Rating)))
		}
		result += strings.Join(names, ", ")
		result += "\n"
	}

	return result
}

func formatLookingAhead(movers []eloMover, strats []strategyCount, evoHighlights []evolutionHighlight, data *IndexData) string {
	var sb strings.Builder

	// Trend summary
	if len(movers) > 0 {
		topMover := movers[0]
		if topMover.Delta > 0 {
			sb.WriteString(fmt.Sprintf("%s's %.0f-point surge suggests a shifting meta. ", topMover.BotName, topMover.Delta))
		} else {
			sb.WriteString(fmt.Sprintf("%s's %.0f-point decline raises questions about the current strategy. ", topMover.BotName, absF(topMover.Delta)))
		}
	}

	// Strategy outlook
	if len(strats) > 0 {
		dominant := strats[0]
		sb.WriteString(fmt.Sprintf("With %d bots running %s strategies, ", dominant.Count, dominant.Archetype))
		if dominant.InTop20 >= 10 {
			sb.WriteString("the archetype remains firmly entrenched. ")
		} else {
			sb.WriteString("counter-strategies may find openings. ")
		}
	}

	// Evolution outlook
	if len(evoHighlights) > 0 {
		topEvo := evoHighlights[0]
		winRate := 0.0
		if topEvo.WeekMatches > 0 {
			winRate = float64(topEvo.WeekWins) / float64(topEvo.WeekMatches) * 100
		}
		sb.WriteString(fmt.Sprintf("Evolved bot %s (rating %.0f, %.0f%% win rate) continues to push the competitive frontier. ",
			topEvo.BotName, topEvo.Rating, winRate))
	} else {
		sb.WriteString("No evolved bots have broken into the competitive ranks yet this week. ")
	}

	// Season outlook
	var active *SeasonData
	for i := range data.Seasons {
		if data.Seasons[i].Status == "active" {
			active = &data.Seasons[i]
			break
		}
	}
	if active != nil {
		daysElapsed := data.GeneratedAt.Sub(active.StartsAt).Hours() / 24
		weekNum := int(daysElapsed/7) + 1
		if weekNum >= 4 {
			sb.WriteString("The championship bracket begins this week.")
		} else if weekNum >= 3 {
			sb.WriteString("The championship bracket approaches — positioning matters.")
		} else {
			sb.WriteString("The season is still young — plenty of ladder movement ahead.")
		}
	}

	if sb.Len() == 0 {
		return "The competitive landscape continues to evolve. Stay tuned for next week's analysis."
	}

	return sb.String()
}
