package validator

import (
	"fmt"
	"regexp"
)

// endpointSpec holds the regex patterns used to detect the two required
// HTTP endpoints in a bot's source code.
type endpointSpec struct {
	// healthRe matches the /health endpoint route registration.
	healthRe *regexp.Regexp
	// turnRe matches the /turn endpoint route registration.
	turnRe *regexp.Regexp
}

// specs maps canonical language names to their detection patterns.
// The patterns are intentionally broad to accommodate the various HTTP
// frameworks an LLM might choose (net/http, Flask, Express, Actix, etc.).
var specs = map[string]endpointSpec{
	"go": {
		// Matches string literals "/health" and "/turn" (double-quote or backtick).
		healthRe: regexp.MustCompile("[\"`]/health[\"`]"),
		turnRe:   regexp.MustCompile("[\"`]/turn[\"`]"),
	},
	"python": {
		healthRe: regexp.MustCompile(`['"]/health['"]`),
		turnRe:   regexp.MustCompile(`['"]/turn['"]`),
	},
	"rust": {
		healthRe: regexp.MustCompile(`['"]/health['"]`),
		turnRe:   regexp.MustCompile(`['"]/turn['"]`),
	},
	"typescript": {
		healthRe: regexp.MustCompile(`['"]/health['"]`),
		turnRe:   regexp.MustCompile(`['"]/turn['"]`),
	},
	"java": {
		healthRe: regexp.MustCompile(`['"]/health['"]`),
		turnRe:   regexp.MustCompile(`['"]/turn['"]`),
	},
	"php": {
		healthRe: regexp.MustCompile(`['"]/health['"]`),
		turnRe:   regexp.MustCompile(`['"]/turn['"]`),
	},
}

// movesRe detects whether the source references a JSON "moves" field, which
// is required in the POST /turn response.
var movesRe = regexp.MustCompile(`(?i)["']moves["']|\bmoves\b`)

// CheckSchema performs static analysis on code to confirm it exposes the two
// required HTTP endpoints and returns a moves response:
//
//   - GET  /health → must return HTTP 200
//   - POST /turn   → must accept JSON game state and return {"moves":[...]}
//
// The check is language-aware but uses lightweight regex matching — it does
// not perform full AST analysis.  False negatives (a syntactically unusual
// but correct bot) are possible; they will be caught by the sandbox stage.
func CheckSchema(code, language string) error {
	spec, ok := specs[language]
	if !ok {
		return fmt.Errorf("unsupported language: %s", language)
	}

	if !spec.healthRe.MatchString(code) {
		return fmt.Errorf("schema: /health endpoint not found — bot must implement GET /health")
	}
	if !spec.turnRe.MatchString(code) {
		return fmt.Errorf("schema: /turn endpoint not found — bot must implement POST /turn")
	}
	if !movesRe.MatchString(code) {
		return fmt.Errorf(`schema: no "moves" field detected — bot must return JSON {"moves":[...]}`)
	}

	return nil
}
