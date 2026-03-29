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

func TestExtractCandidates_unlabeledBlock_proseSkipped(t *testing.T) {
	// An unlabelled block that looks like prose should be skipped
	text := "```\nThis is just prose, not code.\n```"
	_, err := ExtractCandidates(text, "")
	if err == nil {
		t.Error("expected error for prose-only unlabeled block")
	}
}

func TestExtractCandidates_unlabeledBlock_codeKept(t *testing.T) {
	// An unlabelled block that looks like code should be kept
	text := "```\nfunc main() { return; }\n```"
	candidates, err := ExtractCandidates(text, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(candidates) != 1 {
		t.Errorf("expected 1 candidate, got %d", len(candidates))
	}
}

func TestExtractCandidates_javaScriptAlias(t *testing.T) {
	text := "```javascript\nconst x = 1;\n```"
	candidates, err := ExtractCandidates(text, "typescript")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(candidates) != 1 || candidates[0].Language != "typescript" {
		t.Errorf("expected javascript to map to typescript, got %+v", candidates)
	}
}

func TestExtractCandidates_jsAlias(t *testing.T) {
	text := "```js\nconst x = 1;\n```"
	candidates, err := ExtractCandidates(text, "typescript")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(candidates) != 1 || candidates[0].Language != "typescript" {
		t.Errorf("expected js to map to typescript, got %+v", candidates)
	}
}

func TestExtractCandidates_rustAlias(t *testing.T) {
	text := "```rs\nfn main() {}\n```"
	candidates, err := ExtractCandidates(text, "rust")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(candidates) != 1 || candidates[0].Language != "rust" {
		t.Errorf("expected rs to map to rust, got %+v", candidates)
	}
}

func TestExtractCandidates_whitespaceInLanguageTag(t *testing.T) {
	// Language tag with trailing whitespace
	text := "```go  \npackage main\n```"
	candidates, err := ExtractCandidates(text, "go")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(candidates) != 1 {
		t.Errorf("expected 1 candidate, got %d", len(candidates))
	}
}

func TestExtractCandidates_noLanguageTag(t *testing.T) {
	// Block with no language tag but code-like content
	text := "```\nif (x > 0) { return x; }\n```"
	candidates, err := ExtractCandidates(text, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(candidates) != 1 {
		t.Errorf("expected 1 candidate, got %d", len(candidates))
	}
}

func TestExtractBestCandidate_allSameLength(t *testing.T) {
	// When all candidates are same length, first one wins
	text := "```go\nabc\n```\n```go\nxyz\n```"
	best, err := ExtractBestCandidate(text, "go")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if best.Code != "abc" && best.Code != "xyz" {
		t.Errorf("unexpected code: %q", best.Code)
	}
}

func TestExtractCandidates_codeWithBackticks(t *testing.T) {
	// Code that contains backticks (nested)
	text := "```go\nconst msg = `hello`\n```"
	candidates, err := ExtractCandidates(text, "go")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(candidates) != 1 {
		t.Errorf("expected 1 candidate, got %d", len(candidates))
	}
}

func TestValidLanguages(t *testing.T) {
	validLangs := []string{"go", "python", "rust", "typescript", "java", "php"}
	for _, lang := range validLangs {
		if !ValidLanguages[lang] {
			t.Errorf("expected %q to be a valid language", lang)
		}
	}
}

func TestExtractCandidates_multipleBlocksSameLang(t *testing.T) {
	// Multiple blocks of same language
	text := "```go\npackage main\n```\nSome text\n```go\nfunc main() {}\n```"
	candidates, err := ExtractCandidates(text, "go")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(candidates) != 2 {
		t.Errorf("expected 2 candidates, got %d", len(candidates))
	}
}

func TestExtractCandidates_trailingNewlines(t *testing.T) {
	// Code with trailing newlines
	text := "```go\npackage main\n\n\n```"
	candidates, err := ExtractCandidates(text, "go")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(candidates) != 1 {
		t.Errorf("expected 1 candidate, got %d", len(candidates))
	}
}
