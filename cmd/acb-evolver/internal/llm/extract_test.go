package llm

import (
	"strings"
	"testing"
)

func TestExtractCandidates_singleBlock(t *testing.T) {
	text := "Here is the code:\n```go\npackage main\nfunc main() {}\n```\nDone."
	candidates, err := ExtractCandidates(text, "go")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(candidates) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(candidates))
	}
	if candidates[0].Language != "go" {
		t.Errorf("expected language=go, got %q", candidates[0].Language)
	}
	if !strings.Contains(candidates[0].Code, "func main()") {
		t.Errorf("expected code to contain func main(), got %q", candidates[0].Code)
	}
}

func TestExtractCandidates_noBlocks(t *testing.T) {
	_, err := ExtractCandidates("just plain text, no code blocks here", "go")
	if err == nil {
		t.Fatal("expected error for text with no code blocks")
	}
}

func TestExtractCandidates_wrongLanguage(t *testing.T) {
	text := "```python\nprint('hello')\n```"
	_, err := ExtractCandidates(text, "go")
	if err == nil {
		t.Fatal("expected error when no blocks match target language")
	}
}

func TestExtractCandidates_languageAlias_golang(t *testing.T) {
	text := "```golang\npackage main\nfunc main() {}\n```"
	candidates, err := ExtractCandidates(text, "go")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(candidates) != 1 || candidates[0].Language != "go" {
		t.Errorf("expected 1 go candidate, got %+v", candidates)
	}
}

func TestExtractCandidates_languageAlias_ts(t *testing.T) {
	text := "```ts\nconst x = 1;\n```"
	candidates, err := ExtractCandidates(text, "typescript")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(candidates) != 1 || candidates[0].Language != "typescript" {
		t.Errorf("expected 1 typescript candidate, got %+v", candidates)
	}
}

func TestExtractCandidates_languageAlias_py(t *testing.T) {
	text := "```py\nx = 1\n```"
	candidates, err := ExtractCandidates(text, "python")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(candidates) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(candidates))
	}
}

func TestExtractCandidates_multipleBlocks_noFilter(t *testing.T) {
	text := "```go\npackage main\n```\n```python\nprint('hi')\n```"
	candidates, err := ExtractCandidates(text, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(candidates) != 2 {
		t.Fatalf("expected 2 candidates, got %d", len(candidates))
	}
}

func TestExtractCandidates_multipleBlocks_filterByLang(t *testing.T) {
	text := "```go\npackage main\n```\n```python\nprint('hi')\n```\n```go\npackage other\n```"
	candidates, err := ExtractCandidates(text, "go")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(candidates) != 2 {
		t.Fatalf("expected 2 go candidates, got %d", len(candidates))
	}
	for _, c := range candidates {
		if c.Language != "go" {
			t.Errorf("expected language=go, got %q", c.Language)
		}
	}
}

func TestExtractBestCandidate_picksLongest(t *testing.T) {
	short := "package main\nfunc a() {}"
	long := "package main\n\nfunc a() {}\nfunc b() {}\nfunc c() {}\n// extra content here"
	text := "```go\n" + short + "\n```\n```go\n" + long + "\n```"

	best, err := ExtractBestCandidate(text, "go")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if best.Code != long {
		t.Errorf("expected longest code block, got %q", best.Code)
	}
}

func TestExtractBestCandidate_singleBlock(t *testing.T) {
	text := "```rust\nfn main() {}\n```"
	best, err := ExtractBestCandidate(text, "rust")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if best.Language != "rust" {
		t.Errorf("expected language=rust, got %q", best.Language)
	}
}

func TestExtractCandidates_caseInsensitiveTag(t *testing.T) {
	text := "```Go\npackage main\nfunc main() {}\n```"
	candidates, err := ExtractCandidates(text, "go")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(candidates) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(candidates))
	}
}

func TestExtractCandidates_emptyCodeBlock_skipped(t *testing.T) {
	text := "```go\n\n```\n```go\npackage main\nfunc main(){}\n```"
	candidates, err := ExtractCandidates(text, "go")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// The empty block should be skipped.
	if len(candidates) != 1 {
		t.Fatalf("expected 1 non-empty candidate, got %d", len(candidates))
	}
}

func TestLooksLikeCode(t *testing.T) {
	if !looksLikeCode("func main() {}") {
		t.Error("expected func main() {} to look like code")
	}
	if looksLikeCode("this is plain prose without any code chars") {
		t.Error("expected plain prose not to look like code")
	}
}
