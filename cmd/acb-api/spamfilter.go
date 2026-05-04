package main

import (
	"fmt"
	"strings"
	"unicode"
)

// SpamFilter provides word filtering for feedback submission.
// It normalizes case and strips common unicode substitutions before matching.
type SpamFilter struct {
	blockedTerms map[string]struct{} // normalized blocked terms
	minLength    int                  // minimum content length
}

// Default embedded block-list of common spam/offensive terms.
var defaultBlockList = []string{
	// Profanity and offensive language
	"fuck", "shit", "ass", "bitch", "damn", "crap",
	// Common spam patterns
	"buy now", "click here", "free money", "winner", "congratulations",
	"viagra", "cialis", "porn", "xxx", "casino", "lottery",
	// Scam patterns
	"send bitcoin", "crypto giveaway", "urgent", "act now",
	// All-caps spam (normalized to lowercase)
	"clickbait", "subscribe", "like and subscribe",
}

// unicodeReplacements maps common unicode substitutions to their ASCII equivalents.
var unicodeReplacements = map[rune]rune{
	'0': 'o',
	'1': 'i',
	'3': 'e',
	'4': 'a',
	'5': 's',
	'7': 't',
	'@': 'a',
	'$': 's',
	'+': 't',
	'|': 'i',
	'!': 'i',
	'©': 'c',
	'®': 'r',
}

// NewSpamFilter creates a spam filter with the given block-list and minimum length.
// If blockList is nil, uses the embedded default list.
// If minLength is 0, defaults to 10 characters.
func NewSpamFilter(blockList []string, minLength int) *SpamFilter {
	if minLength == 0 {
		minLength = 10
	}

	sf := &SpamFilter{
		blockedTerms: make(map[string]struct{}),
		minLength:    minLength,
	}

	// Use default list if none provided
	terms := blockList
	if len(terms) == 0 {
		terms = defaultBlockList
	}

	// Normalize and store blocked terms
	for _, term := range terms {
		normalized := sf.normalize(term)
		if normalized != "" {
			sf.blockedTerms[normalized] = struct{}{}
		}
	}

	return sf
}

// normalize converts text to lowercase and strips common unicode substitutions.
func (sf *SpamFilter) normalize(s string) string {
	var result strings.Builder
	result.Grow(len(s))

	for _, r := range s {
		// Skip non-printable characters
		if !unicode.IsPrint(r) && !unicode.IsSpace(r) {
			continue
		}

		// Apply unicode replacements
		if replacement, ok := unicodeReplacements[r]; ok {
			result.WriteRune(replacement)
		} else {
			// Convert to lowercase
			result.WriteRune(unicode.ToLower(r))
		}
	}

	return result.String()
}

// Check validates content against the spam filter.
// Returns an error if content is too short or contains blocked terms.
func (sf *SpamFilter) Check(content string) error {
	// Check minimum length
	if len(content) < sf.minLength {
		return fmt.Errorf("content must be at least %d characters", sf.minLength)
	}

	// Skip empty check after length validation
	if content == "" {
		return fmt.Errorf("content cannot be empty")
	}

	normalized := sf.normalize(content)

	// Check for blocked terms (word-boundary aware)
	for blocked := range sf.blockedTerms {
		if sf.containsWord(normalized, blocked) {
			return fmt.Errorf("content contains blocked term")
		}
	}

	return nil
}

// containsWord checks if text contains the given word as a whole word (not substring).
// It handles word boundaries using non-alphanumeric characters.
func (sf *SpamFilter) containsWord(text, word string) bool {
	wordLen := len(word)
	textLen := len(text)

	for i := 0; i <= textLen-wordLen; i++ {
		// Check if substring matches
		if text[i:i+wordLen] == word {
			// Check word boundary before
			beforeOK := i == 0 || !isAlphanumeric(text[i-1])
			// Check word boundary after
			afterOK := (i+wordLen) >= textLen || !isAlphanumeric(text[i+wordLen])

			if beforeOK && afterOK {
				return true
			}
		}
	}

	return false
}

// isAlphanumeric returns true if the byte is a letter or digit.
func isAlphanumeric(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= '0' && b <= '9')
}

// BlockedCount returns the number of blocked terms in the filter.
func (sf *SpamFilter) BlockedCount() int {
	return len(sf.blockedTerms)
}
