package validator

import (
	"context"
	"os/exec"
	"testing"
	"time"
)

// ── Syntax tests ──────────────────────────────────────────────────────────

func TestCheckSyntax_Go_Valid(t *testing.T) {
	code := `package main

import "net/http"

func main() { http.ListenAndServe(":8080", nil) }
`
	if err := CheckSyntax(context.Background(), code, "go", 5*time.Second); err != nil {
		t.Fatalf("expected valid Go to pass, got: %v", err)
	}
}

func TestCheckSyntax_Go_Invalid(t *testing.T) {
	code := `package main

func main() {
	x := // missing value
}
`
	if err := CheckSyntax(context.Background(), code, "go", 5*time.Second); err == nil {
		t.Fatal("expected invalid Go to fail, but got nil")
	}
}

func TestCheckSyntax_Go_UnmatchedBrace(t *testing.T) {
	code := `package main

func main() {
	if true {
`
	if err := CheckSyntax(context.Background(), code, "go", 5*time.Second); err == nil {
		t.Fatal("expected unmatched brace to fail, but got nil")
	}
}

func TestCheckSyntax_UnsupportedLanguage(t *testing.T) {
	if err := CheckSyntax(context.Background(), "code", "cobol", 5*time.Second); err == nil {
		t.Fatal("expected unsupported language to fail")
	}
}

func TestCheckSyntax_Python_Valid(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 not in PATH")
	}
	code := `
import json, os
from http.server import HTTPServer, BaseHTTPRequestHandler

class Handler(BaseHTTPRequestHandler):
    def do_GET(self):
        if self.path == '/health':
            self.send_response(200)
            self.end_headers()
            self.wfile.write(b'OK')

if __name__ == '__main__':
    HTTPServer(('', int(os.getenv('BOT_PORT', 8080))), Handler).serve_forever()
`
	if err := CheckSyntax(context.Background(), code, "python", 10*time.Second); err != nil {
		t.Fatalf("expected valid Python to pass, got: %v", err)
	}
}

func TestCheckSyntax_Python_Invalid(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 not in PATH")
	}
	code := `def foo(
    x = 1
    y = 2  # missing comma / closing paren
`
	if err := CheckSyntax(context.Background(), code, "python", 10*time.Second); err == nil {
		t.Fatal("expected invalid Python to fail")
	}
}

// ── Schema tests ──────────────────────────────────────────────────────────

func TestCheckSchema_Go_Complete(t *testing.T) {
	code := `package main

import "net/http"

func main() {
	http.HandleFunc("/health", handleHealth)
	http.HandleFunc("/turn", handleTurn)
	http.ListenAndServe(":8080", nil)
}

func handleHealth(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) }
func handleTurn(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(` + "`" + `{"moves":[]}` + "`" + `))
}
`
	if err := CheckSchema(code, "go"); err != nil {
		t.Fatalf("expected complete bot to pass schema: %v", err)
	}
}

func TestCheckSchema_Go_MissingHealth(t *testing.T) {
	code := `package main

import "net/http"

func main() {
	http.HandleFunc("/turn", handleTurn)
	http.ListenAndServe(":8080", nil)
}

func handleTurn(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte(` + "`" + `{"moves":[]}` + "`" + `))
}
`
	if err := CheckSchema(code, "go"); err == nil {
		t.Fatal("expected missing /health to fail schema check")
	}
}

func TestCheckSchema_Go_MissingTurn(t *testing.T) {
	code := `package main

import "net/http"

func main() {
	http.HandleFunc("/health", handleHealth)
	http.ListenAndServe(":8080", nil)
}

func handleHealth(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) }
`
	if err := CheckSchema(code, "go"); err == nil {
		t.Fatal("expected missing /turn to fail schema check")
	}
}

func TestCheckSchema_Go_MissingMoves(t *testing.T) {
	code := `package main

import "net/http"

func main() {
	http.HandleFunc("/health", handleHealth)
	http.HandleFunc("/turn", handleTurn)
	http.ListenAndServe(":8080", nil)
}

func handleHealth(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) }
func handleTurn(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte(` + "`" + `{"result":"ok"}` + "`" + `))
}
`
	if err := CheckSchema(code, "go"); err == nil {
		t.Fatal("expected missing moves field to fail schema check")
	}
}

func TestCheckSchema_UnsupportedLanguage(t *testing.T) {
	if err := CheckSchema("code", "brainfuck"); err == nil {
		t.Fatal("expected unsupported language to fail")
	}
}

// ── Pipeline tests ────────────────────────────────────────────────────────

func TestValidate_FailFastOnSyntax(t *testing.T) {
	code := `package main
func main() {  // missing closing brace`
	cfg := DefaultConfig()
	cfg.SandboxTimeout = 5 * time.Second

	report, err := Validate(context.Background(), code, "go", "raw llm output", cfg)
	if err != nil {
		t.Fatalf("Validate returned unexpected error: %v", err)
	}
	if report.Passed {
		t.Fatal("expected pipeline to fail")
	}
	if report.LastStage() != StageSyntax {
		t.Fatalf("expected to stop at syntax stage, got %q", report.LastStage())
	}
	if len(report.Stages) != 1 {
		t.Fatalf("expected 1 stage result (fail-fast), got %d", len(report.Stages))
	}
	if report.LLMOutput != "raw llm output" {
		t.Fatalf("LLMOutput not preserved: %q", report.LLMOutput)
	}
}

func TestValidate_FailFastOnSchema(t *testing.T) {
	// Syntactically valid Go, but missing /turn endpoint.
	code := `package main

import "net/http"

func main() {
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })
	http.ListenAndServe(":8080", nil)
}
`
	cfg := DefaultConfig()
	cfg.SandboxTimeout = 5 * time.Second

	report, err := Validate(context.Background(), code, "go", "", cfg)
	if err != nil {
		t.Fatalf("Validate returned unexpected error: %v", err)
	}
	if report.Passed {
		t.Fatal("expected pipeline to fail")
	}
	if report.LastStage() != StageSchema {
		t.Fatalf("expected to stop at schema stage, got %q", report.LastStage())
	}
	if len(report.Stages) != 2 {
		t.Fatalf("expected 2 stage results, got %d", len(report.Stages))
	}
}

// TestValidate_FullPipeline_Go runs the complete pipeline including the
// sandbox smoke test using a minimal but complete Go bot.
// It is skipped when the `go` binary is not in PATH.
func TestValidate_FullPipeline_Go(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go not in PATH")
	}

	code := minimalGoBot()
	cfg := DefaultConfig()
	cfg.UseNsjail = false // nsjail may not be available in CI
	cfg.SmokeRequests = 5
	cfg.SandboxTimeout = 45 * time.Second
	cfg.SyntaxTimeout = 10 * time.Second

	report, err := Validate(context.Background(), code, "go", "test llm output", cfg)
	if err != nil {
		t.Fatalf("Validate returned unexpected error: %v", err)
	}
	if !report.Passed {
		for _, sr := range report.Stages {
			if !sr.Passed {
				t.Errorf("stage %s failed: %s", sr.Stage, sr.Error)
			}
		}
		t.Fatalf("expected pipeline to pass")
	}
	if len(report.Stages) != 3 {
		t.Fatalf("expected 3 stage results, got %d", len(report.Stages))
	}
}

// minimalGoBot returns a minimal, complete Go bot that passes all three
// validation stages.  It uses the reference signature format from bots/gatherer.
func minimalGoBot() string {
	return `package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
)

type MoveResponse struct {
	Moves []interface{} ` + "`json:\"moves\"`" + `
}

func verifySignature(secret, matchID, turnStr string, body []byte, signature string) error {
	h := sha256.Sum256(body)
	import_str := fmt.Sprintf("%s.%s.%s", matchID, turnStr, hex.EncodeToString(h[:]))
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(import_str))
	expected := hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(signature), []byte(expected)) {
		return fmt.Errorf("invalid signature")
	}
	return nil
}

func main() {
	secret := os.Getenv("BOT_SECRET")
	if secret == "" {
		log.Fatal("BOT_SECRET required")
	}
	port := os.Getenv("BOT_PORT")
	if port == "" {
		port = "8080"
	}

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	http.HandleFunc("/turn", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "read error", http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		sig := r.Header.Get("X-ACB-Signature")
		matchID := r.Header.Get("X-ACB-Match-Id")
		turn := r.Header.Get("X-ACB-Turn")
		if err := verifySignature(secret, matchID, turn, body, sig); err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		resp := MoveResponse{Moves: []interface{}{}}
		out, _ := json.Marshal(resp)
		w.Header().Set("Content-Type", "application/json")
		w.Write(out)
	})

	log.Printf("bot starting on :%s", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal(err)
	}
}
`
}
