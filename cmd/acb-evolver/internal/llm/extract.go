// Package llm provides an OpenAI-compatible LLM client and utilities for
// extracting bot code from model responses.
package llm

import (
	"fmt"
	"regexp"
	"strings"
)

// ValidLanguages is the set of language identifiers the game engine supports.
var ValidLanguages = map[string]bool{
	"go":         true,
	"python":     true,
	"rust":       true,
	"typescript": true,
	"java":       true,
	"php":        true,
}

// languageAliases maps common LLM-output labels to canonical names.
var languageAliases = map[string]string{
	"golang":     "go",
	"py":         "python",
	"rs":         "rust",
	"ts":         "typescript",
	"javascript": "typescript",
	"js":         "typescript",
}

// fencedBlock matches ```<lang>\n<code>\n``` in LLM output.
// The language tag is optional (empty string when absent).
var fencedBlock = regexp.MustCompile("(?s)```([a-zA-Z]*)[ \t]*\n(.*?)```")

// Candidate holds a single piece of extracted bot code from an LLM response.
type Candidate struct {
	Code     string
	Language string
}

// ExtractCandidates parses all fenced code blocks from text and returns them
// as Candidates.
//
// If targetLang is non-empty only blocks whose language tag resolves to
// targetLang (after alias expansion) are returned.  Language matching is
// case-insensitive.
//
// Returns an error when no matching blocks are found.
func ExtractCandidates(text, targetLang string) ([]Candidate, error) {
	matches := fencedBlock.FindAllStringSubmatch(text, -1)
	if len(matches) == 0 {
		return nil, fmt.Errorf("no fenced code blocks found in LLM response")
	}

	target := strings.ToLower(strings.TrimSpace(targetLang))

	var out []Candidate
	for _, m := range matches {
		rawLang := strings.ToLower(strings.TrimSpace(m[1]))
		code := strings.TrimSpace(m[2])
		if code == "" {
			continue
		}

		// Resolve alias to canonical name.
		lang := rawLang
		if alias, ok := languageAliases[rawLang]; ok {
			lang = alias
		}

		// Filter by target language when one is specified.
		if target != "" && lang != target {
			continue
		}

		// Skip unlabelled blocks that look like prose, not code.
		if lang == "" && !looksLikeCode(code) {
			continue
		}

		out = append(out, Candidate{Code: code, Language: lang})
	}

	if len(out) == 0 {
		if target != "" {
			return nil, fmt.Errorf("no %q code blocks found in LLM response", target)
		}
		return nil, fmt.Errorf("no usable code blocks found in LLM response")
	}
	return out, nil
}

// ExtractBestCandidate returns the longest code block matching targetLang.
// It is a convenience wrapper around ExtractCandidates.
func ExtractBestCandidate(text, targetLang string) (*Candidate, error) {
	candidates, err := ExtractCandidates(text, targetLang)
	if err != nil {
		return nil, err
	}
	best := &candidates[0]
	for i := 1; i < len(candidates); i++ {
		if len(candidates[i].Code) > len(best.Code) {
			best = &candidates[i]
		}
	}
	return best, nil
}

// looksLikeCode returns true when s contains at least one character that
// commonly appears in source code but rarely in plain prose.
func looksLikeCode(s string) bool {
	for _, ch := range s {
		switch ch {
		case '{', '}', '(', ')', ';', '=', '<', '>', '/', '*':
			return true
		}
	}
	return false
}
