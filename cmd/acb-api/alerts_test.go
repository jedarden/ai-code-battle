package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestAlerterEnabled(t *testing.T) {
	tests := []struct {
		name    string
		discord string
		slack   string
		want    bool
	}{
		{"both empty", "", "", false},
		{"discord only", "http://discord.example.com", "", true},
		{"slack only", "", "http://slack.example.com", true},
		{"both set", "http://discord.example.com", "http://slack.example.com", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := NewAlerter(tt.discord, tt.slack)
			if got := a.Enabled(); got != tt.want {
				t.Errorf("Enabled() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAlerterSendNoOp(t *testing.T) {
	// With no webhook URLs, Send should be a no-op (no panic, no error).
	a := NewAlerter("", "")
	a.Send(context.Background(), AlertError, "Test", "message", "key")
}

func TestAlerterSendDiscord(t *testing.T) {
	var received discordPayload
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("content-type = %s, want application/json", ct)
		}
		json.NewDecoder(r.Body).Decode(&received)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	a := NewAlerter(ts.URL, "")
	a.Send(context.Background(), AlertError, "Test Alert", "Something broke", "")

	if len(received.Embeds) != 1 {
		t.Fatalf("embeds count = %d, want 1", len(received.Embeds))
	}
	embed := received.Embeds[0]
	if embed.Title != "[ACB] Test Alert" {
		t.Errorf("title = %q, want %q", embed.Title, "[ACB] Test Alert")
	}
	if embed.Description != "Something broke" {
		t.Errorf("description = %q, want %q", embed.Description, "Something broke")
	}
	if embed.Color != 0xe74c3c {
		t.Errorf("color = %#x, want %#x (red/error)", embed.Color, 0xe74c3c)
	}
	if embed.Timestamp == "" {
		t.Error("timestamp should not be empty")
	}
}

func TestAlerterSendSlack(t *testing.T) {
	var received slackPayload
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&received)
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	a := NewAlerter("", ts.URL)
	a.Send(context.Background(), AlertWarning, "Warning", "Watch out", "")

	if len(received.Attachments) != 1 {
		t.Fatalf("attachments count = %d, want 1", len(received.Attachments))
	}
	att := received.Attachments[0]
	if att.Title != "[ACB] Warning" {
		t.Errorf("title = %q, want %q", att.Title, "[ACB] Warning")
	}
	if att.Text != "Watch out" {
		t.Errorf("text = %q, want %q", att.Text, "Watch out")
	}
	if att.Color != "#f39c12" {
		t.Errorf("color = %q, want %q (warning)", att.Color, "#f39c12")
	}
	if att.Footer != "AI Code Battle" {
		t.Errorf("footer = %q, want %q", att.Footer, "AI Code Battle")
	}
}

func TestAlerterColorCodes(t *testing.T) {
	tests := []struct {
		level        AlertLevel
		wantDiscord  int
		wantSlack    string
	}{
		{AlertInfo, 0x3498db, "#3498db"},
		{AlertWarning, 0xf39c12, "#f39c12"},
		{AlertError, 0xe74c3c, "#e74c3c"},
	}

	for _, tt := range tests {
		var discordReceived discordPayload
		var slackReceived slackPayload

		discordSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			json.NewDecoder(r.Body).Decode(&discordReceived)
			w.WriteHeader(http.StatusNoContent)
		}))
		slackSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			json.NewDecoder(r.Body).Decode(&slackReceived)
			w.WriteHeader(http.StatusOK)
		}))

		a := NewAlerter(discordSrv.URL, slackSrv.URL)
		a.Send(context.Background(), tt.level, "Test", "msg", "")

		if len(discordReceived.Embeds) > 0 && discordReceived.Embeds[0].Color != tt.wantDiscord {
			t.Errorf("level %d: discord color = %#x, want %#x", tt.level, discordReceived.Embeds[0].Color, tt.wantDiscord)
		}
		if len(slackReceived.Attachments) > 0 && slackReceived.Attachments[0].Color != tt.wantSlack {
			t.Errorf("level %d: slack color = %q, want %q", tt.level, slackReceived.Attachments[0].Color, tt.wantSlack)
		}

		discordSrv.Close()
		slackSrv.Close()
	}
}

func TestAlerterRateLimiting(t *testing.T) {
	var callCount int
	var mu sync.Mutex

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		callCount++
		mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	a := NewAlerter(ts.URL, "")
	a.cooldown = 1 * time.Hour // long cooldown for test

	ctx := context.Background()

	// First send should go through
	a.Send(ctx, AlertInfo, "Test", "msg1", "same-key")
	mu.Lock()
	if callCount != 1 {
		t.Errorf("after first send: count = %d, want 1", callCount)
	}
	mu.Unlock()

	// Second send with same key should be suppressed
	a.Send(ctx, AlertInfo, "Test", "msg2", "same-key")
	mu.Lock()
	if callCount != 1 {
		t.Errorf("after duplicate send: count = %d, want 1 (suppressed)", callCount)
	}
	mu.Unlock()

	// Different key should go through
	a.Send(ctx, AlertInfo, "Test", "msg3", "other-key")
	mu.Lock()
	if callCount != 2 {
		t.Errorf("after different key: count = %d, want 2", callCount)
	}
	mu.Unlock()
}

func TestAlerterNoDedupKeyAlwaysSends(t *testing.T) {
	var callCount int
	var mu sync.Mutex

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		callCount++
		mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	a := NewAlerter(ts.URL, "")
	ctx := context.Background()

	// Empty dedup key should always send
	a.Send(ctx, AlertInfo, "Test", "msg1", "")
	a.Send(ctx, AlertInfo, "Test", "msg2", "")
	a.Send(ctx, AlertInfo, "Test", "msg3", "")

	mu.Lock()
	if callCount != 3 {
		t.Errorf("without dedup key: count = %d, want 3", callCount)
	}
	mu.Unlock()
}

func TestAlerterRateLimitExpiry(t *testing.T) {
	var callCount int
	var mu sync.Mutex

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		callCount++
		mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	a := NewAlerter(ts.URL, "")
	a.cooldown = 10 * time.Millisecond // very short for test

	ctx := context.Background()

	a.Send(ctx, AlertInfo, "Test", "msg1", "expire-key")
	mu.Lock()
	c1 := callCount
	mu.Unlock()

	time.Sleep(20 * time.Millisecond)

	// After cooldown, same key should send again
	a.Send(ctx, AlertInfo, "Test", "msg2", "expire-key")
	mu.Lock()
	c2 := callCount
	mu.Unlock()

	if c1 != 1 || c2 != 2 {
		t.Errorf("rate limit expiry: counts = (%d, %d), want (1, 2)", c1, c2)
	}
}

func TestAlerterWebhookError(t *testing.T) {
	// Server that returns 500 — should not panic
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	a := NewAlerter(ts.URL, ts.URL)
	// Should log errors but not panic
	a.Send(context.Background(), AlertError, "Test", "msg", "")
}

func TestAlerterBothWebhooks(t *testing.T) {
	var discordCalls, slackCalls int
	var mu sync.Mutex

	discordSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		discordCalls++
		mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	}))
	defer discordSrv.Close()

	slackSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		slackCalls++
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer slackSrv.Close()

	a := NewAlerter(discordSrv.URL, slackSrv.URL)
	a.Send(context.Background(), AlertInfo, "Test", "msg", "")

	mu.Lock()
	defer mu.Unlock()
	if discordCalls != 1 {
		t.Errorf("discord calls = %d, want 1", discordCalls)
	}
	if slackCalls != 1 {
		t.Errorf("slack calls = %d, want 1", slackCalls)
	}
}

func TestHelperBotMarkedInactive(t *testing.T) {
	var received discordPayload
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&received)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	a := NewAlerter(ts.URL, "")
	a.BotMarkedInactive(context.Background(), "bot_abc123", 3)

	if len(received.Embeds) != 1 {
		t.Fatal("expected 1 embed")
	}
	if received.Embeds[0].Color != 0xf39c12 {
		t.Errorf("bot inactive should be warning (yellow), got %#x", received.Embeds[0].Color)
	}
}

func TestHelperBotRecovered(t *testing.T) {
	var received discordPayload
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&received)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	a := NewAlerter(ts.URL, "")
	a.BotRecovered(context.Background(), "bot_abc123")

	if len(received.Embeds) != 1 {
		t.Fatal("expected 1 embed")
	}
	if received.Embeds[0].Color != 0x3498db {
		t.Errorf("bot recovered should be info (blue), got %#x", received.Embeds[0].Color)
	}
}

func TestHelperStaleJobsReaped(t *testing.T) {
	var received discordPayload
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&received)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	a := NewAlerter(ts.URL, "")
	a.StaleJobsReaped(context.Background(), []string{"j_001", "j_002"})

	if len(received.Embeds) != 1 {
		t.Fatal("expected 1 embed")
	}
	if received.Embeds[0].Color != 0xf39c12 {
		t.Errorf("stale jobs should be warning (yellow), got %#x", received.Embeds[0].Color)
	}
}

func TestHelperMatchError(t *testing.T) {
	var received discordPayload
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&received)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	a := NewAlerter(ts.URL, "")
	a.MatchError(context.Background(), "m_12345678", "bot timeout")

	if len(received.Embeds) != 1 {
		t.Fatal("expected 1 embed")
	}
	if received.Embeds[0].Color != 0xe74c3c {
		t.Errorf("match error should be error (red), got %#x", received.Embeds[0].Color)
	}
}

func TestShouldSendGarbageCollects(t *testing.T) {
	a := NewAlerter("http://unused", "")
	a.cooldown = 1 * time.Millisecond

	// Fill beyond 100 entries to trigger GC
	for i := 0; i < 110; i++ {
		a.sent[string(rune('a'+i%26))+string(rune('0'+i/26))] = time.Now().Add(-time.Hour)
	}

	// This should trigger cleanup and succeed
	got := a.shouldSend("new-key")
	if !got {
		t.Error("shouldSend returned false for new key after GC should have cleaned expired entries")
	}

	// Old expired entries should have been cleaned
	a.mu.Lock()
	count := len(a.sent)
	a.mu.Unlock()
	// Should only have "new-key" left (all others expired)
	if count > 10 {
		t.Errorf("after GC: %d entries remain, expected most to be cleaned", count)
	}
}
