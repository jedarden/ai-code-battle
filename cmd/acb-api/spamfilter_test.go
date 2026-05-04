package main

import (
	"testing"
)

func TestSpamFilter_MinLength(t *testing.T) {
	sf := NewSpamFilter(nil, 10)

	tests := []struct {
		name    string
		content string
		wantErr bool
	}{
		{"empty", "", true},
		{"too short", "hi", true},
		{"exactly min", "1234567890", false},
		{"above min", "this is valid feedback", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := sf.Check(tt.content)
			if (err != nil) != tt.wantErr {
				t.Errorf("Check() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSpamFilter_BlockedTerms(t *testing.T) {
	customList := []string{"spam", "scam", "viagra"}
	sf := NewSpamFilter(customList, 5)

	tests := []struct {
		name    string
		content string
		wantErr bool
	}{
		{"clean content", "this is good feedback", false},
		{"exact blocked", "spam here", true},
		{"blocked at start", "scam alert", true},
		{"blocked at end", "buy viagra", true},
		{"blocked in middle", "this is a scam attempt", true},
		{"case insensitive", "SPAM everywhere", true},
		{"mixed case", "VIAGRA pills", true},
		{"substring not blocked", "spamming is okay", false}, // "spamming" != "spam"
		{"partial word not blocked", "this is spammy", false}, // "spammy" != "spam"
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := sf.Check(tt.content)
			if (err != nil) != tt.wantErr {
				t.Errorf("Check() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSpamFilter_UnicodeNormalization(t *testing.T) {
	customList := []string{"viagra", "casino"}
	sf := NewSpamFilter(customList, 5)

	tests := []struct {
		name    string
		content string
		wantErr bool
	}{
		{"leetspeak 0", "v1agra pills", true},
		{"leetspeak 1", "v1@gra pills", true},
		{"leetspeak 3", "v1agr@ is bad", true},
		{"leetspeak 4", "c@s1n0 royale", true},
		{"leetspeak 5", "c451n0 royale", true},
		{"leetspeak 7", "c@5in0 royale", true},
		{"mixed leetspeak", "v1@gr@ and c@s1n0", true},
		{"clean content", "this is okay", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := sf.Check(tt.content)
			if (err != nil) != tt.wantErr {
				t.Errorf("Check() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSpamFilter_DefaultBlockList(t *testing.T) {
	sf := NewSpamFilter(nil, 5)

	// Check that default block-list has terms
	if sf.BlockedCount() == 0 {
		t.Error("default block list is empty")
	}

	// Test some terms from the default list
	tests := []struct {
		content string
		wantErr bool
	}{
		{"this is good feedback", false},
		{"buy now click here", true},
		{"free money winner", true},
		{"send bitcoin scam", true},
	}

	for _, tt := range tests {
		t.Run(tt.content, func(t *testing.T) {
			err := sf.Check(tt.content)
			if (err != nil) != tt.wantErr {
				t.Errorf("Check(%q) error = %v, wantErr %v", tt.content, err, tt.wantErr)
			}
		})
	}
}

func TestSpamFilter_WordBoundaries(t *testing.T) {
	// Test that word boundaries are respected
	customList := []string{"ass", "casino"}
	sf := NewSpamFilter(customList, 5)

	tests := []struct {
		name    string
		content string
		wantErr bool
	}{
		{"exact match", "ass", true},
		{"with space before", " this ass", true},
		{"with space after", "ass ", true},
		{"in middle", "this ass here", true},
		{"with punctuation", "ass.", true},
		{"substring should not match", "this is classic", false},    // "ass" in "classic"
		{"substring should not match 2", "cassandra is cool", false}, // "ass" in "cassandra"
		{"casino exact", "casino", true},
		{"casino plural", "casinos", false}, // different word
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := sf.Check(tt.content)
			if (err != nil) != tt.wantErr {
				t.Errorf("Check(%q) error = %v, wantErr %v", tt.content, err, tt.wantErr)
			}
		})
	}
}

func TestNormalize(t *testing.T) {
	sf := NewSpamFilter(nil, 5)

	tests := []struct {
		input    string
		expected string
	}{
		{"ViAgRA", "viagra"},
		{"V1@GR@", "viagra"},   // 1→i, @→a
		{"C451N0", "casino"},   // 4→a, 5→s, 0→o, 1→i
		{"Test!", "testi"},     // !→i
		{"Mixed CASE", "mixed case"},
		{"0wned", "owned"},     // 0→o
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := sf.normalize(tt.input)
			if got != tt.expected {
				t.Errorf("normalize(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}
