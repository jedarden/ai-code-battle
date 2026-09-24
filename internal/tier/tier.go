// Package tier holds the one definition of tournament tiers and the per-turn
// response timeout each tier grants (requirements.md "Response Timeout").
//
// The matchmaker resolves a match's tier into a concrete timeout when it
// writes jobs.config_json, and the engine enforces that timeout per turn.
// Because both sides read the mapping from here rather than pasting it, a
// tier's budget can only drift if this file says so.
//
// The default is deliberately the historical 3-second budget: every match
// path that predates tiers (and every unknown tier value) keeps the timing
// the platform has always had.
package tier

import (
	"fmt"
	"time"
)

// Tier is a tournament tier a match can be created under.
type Tier string

const (
	// Casual forgives network and hosting variance (beginner-facing events).
	Casual Tier = "casual"
	// Competitive is the standard budget and the platform default.
	Competitive Tier = "competitive"
	// Speed is for optimized bots on fast connections.
	Speed Tier = "speed"

	// Default is the tier assumed when a match does not name one.
	Default Tier = Competitive
)

// TurnTimeout returns the per-turn response budget for the tier.
// Unknown and empty tiers resolve to the default (competitive) budget so a
// typo in configuration cannot silently zero or inflate match timing.
func (t Tier) TurnTimeout() time.Duration {
	switch t {
	case Casual:
		return 5 * time.Second
	case Competitive:
		return 3 * time.Second
	case Speed:
		return 1 * time.Second
	default:
		return Default.TurnTimeout()
	}
}

// Valid reports whether t is one of the defined tiers.
func (t Tier) Valid() bool {
	switch t {
	case Casual, Competitive, Speed:
		return true
	default:
		return false
	}
}

// Parse resolves a tier name from configuration. Empty resolves to Default;
// anything unrecognized is an error rather than a silent downgrade.
func Parse(s string) (Tier, error) {
	if s == "" {
		return Default, nil
	}
	t := Tier(s)
	if !t.Valid() {
		return Default, fmt.Errorf("unknown tournament tier %q (want casual, competitive, or speed)", s)
	}
	return t, nil
}
