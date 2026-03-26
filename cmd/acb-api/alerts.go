package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

// AlertLevel indicates severity for color-coding in webhook messages.
type AlertLevel int

const (
	AlertInfo    AlertLevel = iota // blue / informational
	AlertWarning                   // yellow / warning
	AlertError                     // red / error
)

// Alerter sends notifications to configured Discord and/or Slack webhooks.
type Alerter struct {
	discordURL string
	slackURL   string
	client     *http.Client

	// Rate limiting: max 1 alert per key per cooldown period.
	mu       sync.Mutex
	cooldown time.Duration
	sent     map[string]time.Time
}

// NewAlerter creates an Alerter. If both URLs are empty, Send is a no-op.
func NewAlerter(discordURL, slackURL string) *Alerter {
	return &Alerter{
		discordURL: discordURL,
		slackURL:   slackURL,
		client:     &http.Client{Timeout: 10 * time.Second},
		cooldown:   5 * time.Minute,
		sent:       make(map[string]time.Time),
	}
}

// Enabled returns true if at least one webhook URL is configured.
func (a *Alerter) Enabled() bool {
	return a.discordURL != "" || a.slackURL != ""
}

// Send dispatches an alert to all configured webhooks. The dedupKey is used
// for rate limiting — identical keys within the cooldown window are suppressed.
func (a *Alerter) Send(ctx context.Context, level AlertLevel, title, message, dedupKey string) {
	if !a.Enabled() {
		return
	}

	if dedupKey != "" && !a.shouldSend(dedupKey) {
		return
	}

	if a.discordURL != "" {
		if err := a.sendDiscord(ctx, level, title, message); err != nil {
			log.Printf("alert: discord send error: %v", err)
		}
	}

	if a.slackURL != "" {
		if err := a.sendSlack(ctx, level, title, message); err != nil {
			log.Printf("alert: slack send error: %v", err)
		}
	}
}

// shouldSend checks rate limiting. Returns true if the alert should be sent.
func (a *Alerter) shouldSend(key string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()

	now := time.Now()

	// Garbage collect expired entries periodically
	if len(a.sent) > 100 {
		for k, t := range a.sent {
			if now.Sub(t) > a.cooldown {
				delete(a.sent, k)
			}
		}
	}

	if last, ok := a.sent[key]; ok && now.Sub(last) < a.cooldown {
		return false
	}
	a.sent[key] = now
	return true
}

// discordPayload is the Discord webhook message format.
type discordPayload struct {
	Embeds []discordEmbed `json:"embeds"`
}

type discordEmbed struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Color       int    `json:"color"`
	Timestamp   string `json:"timestamp"`
}

func (a *Alerter) sendDiscord(ctx context.Context, level AlertLevel, title, message string) error {
	color := 0x3498db // blue
	switch level {
	case AlertWarning:
		color = 0xf39c12 // yellow/orange
	case AlertError:
		color = 0xe74c3c // red
	}

	payload := discordPayload{
		Embeds: []discordEmbed{{
			Title:       fmt.Sprintf("[ACB] %s", title),
			Description: message,
			Color:       color,
			Timestamp:   time.Now().UTC().Format(time.RFC3339),
		}},
	}

	return a.postJSON(ctx, a.discordURL, payload)
}

// slackPayload is the Slack incoming webhook format.
type slackPayload struct {
	Attachments []slackAttachment `json:"attachments"`
}

type slackAttachment struct {
	Color    string `json:"color"`
	Title    string `json:"title"`
	Text     string `json:"text"`
	Footer   string `json:"footer"`
	Ts       int64  `json:"ts"`
}

func (a *Alerter) sendSlack(ctx context.Context, level AlertLevel, title, message string) error {
	color := "#3498db"
	switch level {
	case AlertWarning:
		color = "#f39c12"
	case AlertError:
		color = "#e74c3c"
	}

	payload := slackPayload{
		Attachments: []slackAttachment{{
			Color:  color,
			Title:  fmt.Sprintf("[ACB] %s", title),
			Text:   message,
			Footer: "AI Code Battle",
			Ts:     time.Now().Unix(),
		}},
	}

	return a.postJSON(ctx, a.slackURL, payload)
}

func (a *Alerter) postJSON(ctx context.Context, url string, payload any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.client.Do(req)
	if err != nil {
		return fmt.Errorf("send webhook: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}
	return nil
}

// Alert helper methods for common events.

func (a *Alerter) BotMarkedInactive(ctx context.Context, botID string, failCount int) {
	a.Send(ctx, AlertWarning,
		"Bot Marked Inactive",
		fmt.Sprintf("Bot `%s` marked inactive after %d consecutive health check failures.", botID, failCount),
		"bot-inactive:"+botID,
	)
}

func (a *Alerter) BotRecovered(ctx context.Context, botID string) {
	a.Send(ctx, AlertInfo,
		"Bot Recovered",
		fmt.Sprintf("Bot `%s` is back online and marked active.", botID),
		"bot-recovered:"+botID,
	)
}

func (a *Alerter) StaleJobsReaped(ctx context.Context, jobIDs []string) {
	a.Send(ctx, AlertWarning,
		"Stale Jobs Re-enqueued",
		fmt.Sprintf("%d stale job(s) re-enqueued: %s", len(jobIDs), strings.Join(jobIDs, ", ")),
		"stale-jobs",
	)
}

func (a *Alerter) MatchError(ctx context.Context, matchID, reason string) {
	a.Send(ctx, AlertError,
		"Match Error",
		fmt.Sprintf("Match `%s` failed: %s", matchID, reason),
		"match-error:"+matchID,
	)
}
