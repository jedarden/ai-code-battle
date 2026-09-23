package validator

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aicodebattle/acb/engine"
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

func TestVerifyTurnRequestAuthRejections(t *testing.T) {
	var requestCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		body, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		headers := map[string]string{
			"X-ACB-Match-Id":  r.Header.Get("X-ACB-Match-Id"),
			"X-ACB-Turn":      r.Header.Get("X-ACB-Turn"),
			"X-ACB-Timestamp": r.Header.Get("X-ACB-Timestamp"),
			"X-ACB-Bot-Id":    r.Header.Get("X-ACB-Bot-Id"),
			"X-ACB-Signature": r.Header.Get("X-ACB-Signature"),
		}
		auth, err := engine.ParseAuthHeaders(headers)
		if err == nil {
			err = engine.VerifyRequest(smokeSecret, auth, body)
		}
		if err != nil {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	if err := verifyTurnRequestAuthRejections(context.Background(), server.Client(), server.Listener.Addr().String()); err != nil {
		t.Fatalf("verifyTurnRequestAuthRejections() error = %v", err)
	}
	if got, want := requestCount.Load(), int32(11); got != want {
		t.Fatalf("rejection probe count = %d, want %d", got, want)
	}
}

func TestVerifyTurnRequestAuthRejections_RequiresHTTP401(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	err := verifyTurnRequestAuthRejections(context.Background(), server.Client(), server.Listener.Addr().String())
	if err == nil {
		t.Fatal("verifyTurnRequestAuthRejections() accepted HTTP 200")
	}
	if !strings.Contains(err.Error(), "expected HTTP 401") {
		t.Fatalf("verifyTurnRequestAuthRejections() error = %q, want HTTP 401 requirement", err)
	}
}

func TestSendTurnRequest_VerifiesResponseSignature(t *testing.T) {
	tests := []struct {
		name        string
		signature   string
		useValidSig bool
		wantErr     bool
	}{
		{name: "valid", useValidSig: true},
		{name: "missing", wantErr: true},
		{name: "invalid", signature: "00", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			responseBody := []byte(`{"moves":[]}`)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				signature := tc.signature
				if tc.useValidSig {
					signature = engine.SignResponse(smokeSecret, smokeMatchID, 3, responseBody)
				}
				if signature != "" {
					w.Header().Set("X-ACB-Signature", signature)
				}
				_, _ = w.Write(responseBody)
			}))
			defer server.Close()

			err := sendTurnRequest(context.Background(), server.Client(), server.Listener.Addr().String(), 3)
			if (err != nil) != tc.wantErr {
				t.Fatalf("sendTurnRequest() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func TestSendTurnRequest_RejectsOversizedSignedPrefix(t *testing.T) {
	prefix := append([]byte(`{"moves":[]}`), bytes.Repeat([]byte(" "), maxSmokeResponseBytes-len(`{"moves":[]}`))...)
	responseBody := append(append([]byte(nil), prefix...), ' ')
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-ACB-Signature", engine.SignResponse(smokeSecret, smokeMatchID, 3, prefix))
		_, _ = w.Write(responseBody)
	}))
	defer server.Close()

	err := sendTurnRequest(context.Background(), server.Client(), server.Listener.Addr().String(), 3)
	if err == nil {
		t.Fatal("sendTurnRequest() accepted a response body over the read cap")
	}
	if !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("sendTurnRequest() error = %q, want response-size error", err)
	}
}

func TestValidateMoveResponse_StrictSchema(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		wantErr bool
	}{
		{name: "empty moves", body: `{"moves":[]}`},
		{name: "nested move", body: `{"moves":[{"position":{"row":10,"col":15},"direction":"N"}]}`},
		{name: "stay", body: `{"moves":[{"position":{"row":0,"col":0},"direction":"stay"}]}`},
		{name: "additive fields", body: `{"moves":[{"position":{"row":1,"col":2,"extra":true},"direction":"E","meta":{"score":1}}],"debug":{},"future":{"enabled":true}}`},
		{name: "engine debug shape", body: `{"moves":[],"debug":{"reasoning":"hold","targets":[{"position":{"row":1,"col":2},"label":"target","priority":0.5}],"values":{"score":1},"heatmap":{"name":"heat","data":[[1]]}}}`},
		{name: "invalid JSON", body: `{`, wantErr: true},
		{name: "top-level null", body: `null`, wantErr: true},
		{name: "top-level array", body: `[]`, wantErr: true},
		{name: "missing moves", body: `{"debug":{}}`, wantErr: true},
		{name: "debug string", body: `{"moves":[],"debug":"bad"}`, wantErr: true},
		{name: "debug array", body: `{"moves":[],"debug":[]}`, wantErr: true},
		{name: "debug reasoning wrong type", body: `{"moves":[],"debug":{"reasoning":1}}`, wantErr: true},
		{name: "debug targets wrong type", body: `{"moves":[],"debug":{"targets":{}}}`, wantErr: true},
		{name: "debug values wrong type", body: `{"moves":[],"debug":{"values":[]}}`, wantErr: true},
		{name: "debug heatmap wrong type", body: `{"moves":[],"debug":{"heatmap":"bad"}}`, wantErr: true},
		{name: "moves null", body: `{"moves":null}`, wantErr: true},
		{name: "moves object", body: `{"moves":{}}`, wantErr: true},
		{name: "moves string", body: `{"moves":"N"}`, wantErr: true},
		{name: "null move", body: `{"moves":[null]}`, wantErr: true},
		{name: "non-object move", body: `{"moves":["N"]}`, wantErr: true},
		{name: "missing position", body: `{"moves":[{"direction":"N"}]}`, wantErr: true},
		{name: "flat position", body: `{"moves":[{"row":1,"col":2,"direction":"N"}]}`, wantErr: true},
		{name: "null position", body: `{"moves":[{"position":null,"direction":"N"}]}`, wantErr: true},
		{name: "missing row", body: `{"moves":[{"position":{"col":2},"direction":"N"}]}`, wantErr: true},
		{name: "null row", body: `{"moves":[{"position":{"row":null,"col":2},"direction":"N"}]}`, wantErr: true},
		{name: "string row", body: `{"moves":[{"position":{"row":"1","col":2},"direction":"N"}]}`, wantErr: true},
		{name: "fractional row", body: `{"moves":[{"position":{"row":1.5,"col":2},"direction":"N"}]}`, wantErr: true},
		{name: "negative position", body: `{"moves":[{"position":{"row":-1,"col":2},"direction":"N"}]}`, wantErr: true},
		{name: "missing col", body: `{"moves":[{"position":{"row":1},"direction":"N"}]}`, wantErr: true},
		{name: "missing direction", body: `{"moves":[{"position":{"row":1,"col":2}}]}`, wantErr: true},
		{name: "null direction", body: `{"moves":[{"position":{"row":1,"col":2},"direction":null}]}`, wantErr: true},
		{name: "numeric direction", body: `{"moves":[{"position":{"row":1,"col":2},"direction":1}]}`, wantErr: true},
		{name: "empty direction", body: `{"moves":[{"position":{"row":1,"col":2},"direction":""}]}`, wantErr: true},
		{name: "unknown direction", body: `{"moves":[{"position":{"row":1,"col":2},"direction":"spin"}]}`, wantErr: true},
		{name: "invalid move rejects envelope", body: `{"moves":[{"position":{"row":1,"col":2},"direction":"N"},{"direction":"S"}]}`, wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateMoveResponse([]byte(tc.body))
			if (err != nil) != tc.wantErr {
				t.Fatalf("validateMoveResponse() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func TestValidateMoveResponse_NormalizedDebugLimit(t *testing.T) {
	encode := func(debug *engine.DebugInfo) []byte {
		body, err := json.Marshal(map[string]interface{}{
			"moves": []json.RawMessage{},
			"debug": debug,
		})
		if err != nil {
			t.Fatalf("marshal debug response: %v", err)
		}
		return body
	}

	exact := &engine.DebugInfo{Reasoning: strings.Repeat("x", maxDebugJSONBytes-16)}
	normalized, err := json.Marshal(exact)
	if err != nil {
		t.Fatalf("marshal normalized debug: %v", err)
	}
	if len(normalized) != maxDebugJSONBytes {
		t.Fatalf("normalized debug test size = %d, want %d", len(normalized), maxDebugJSONBytes)
	}
	if err := validateMoveResponse(encode(exact)); err != nil {
		t.Fatalf("exactly 10 KiB normalized debug rejected: %v", err)
	}

	oversized := &engine.DebugInfo{Reasoning: strings.Repeat("x", maxDebugJSONBytes+1)}
	if err := validateMoveResponse(encode(oversized)); err == nil {
		t.Fatal("normalized debug over 10 KiB was accepted")
	}

	unknownPadding := []byte(`{"moves":[],"debug":{"future":"` + strings.Repeat("x", maxDebugJSONBytes) + `"}}`)
	if err := validateMoveResponse(unknownPadding); err != nil {
		t.Fatalf("unknown debug fields should normalize away, got: %v", err)
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
// validation stages using the protocol signature format.
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
	"strconv"
	"time"
)

type MoveResponse struct {
	Moves []interface{} ` + "`json:\"moves\"`" + `
}

type RequestIdentity struct {
	MatchID *string ` + "`json:\"match_id\"`" + `
	Turn    *int    ` + "`json:\"turn\"`" + `
}

func verifySignature(secret, matchID, turn, timestamp, botID, signature string, body []byte) error {
	if matchID == "" {
		return fmt.Errorf("missing X-ACB-Match-Id")
	}
	turnNumber, err := strconv.Atoi(turn)
	if err != nil {
		return fmt.Errorf("invalid X-ACB-Turn")
	}
	if turnNumber < 0 {
		return fmt.Errorf("negative X-ACB-Turn")
	}
	if timestamp == "" {
		return fmt.Errorf("missing X-ACB-Timestamp")
	}
	if botID == "" {
		return fmt.Errorf("missing X-ACB-Bot-Id")
	}
	if signature == "" {
		return fmt.Errorf("missing X-ACB-Signature")
	}
	timestampUnix, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid X-ACB-Timestamp")
	}
	age := time.Since(time.Unix(timestampUnix, 0))
	if age < -30*time.Second || age > 30*time.Second {
		return fmt.Errorf("stale X-ACB-Timestamp")
	}
	h := sha256.Sum256(body)
	signingString := fmt.Sprintf("%s.%s.%s.%s", matchID, turn, timestamp, hex.EncodeToString(h[:]))
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(signingString))
	expected := hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(signature), []byte(expected)) {
		return fmt.Errorf("invalid signature")
	}
	var identity RequestIdentity
	if err := json.Unmarshal(body, &identity); err != nil {
		return fmt.Errorf("invalid request body")
	}
	if identity.MatchID == nil || identity.Turn == nil {
		return fmt.Errorf("missing request identity")
	}
	if *identity.MatchID != matchID || *identity.Turn != turnNumber {
		return fmt.Errorf("request identity mismatch")
	}
	return nil
}

func signResponse(secret, matchID string, turn int, body []byte) string {
	h := sha256.Sum256(body)
	signingString := fmt.Sprintf("%s.%d.%s", matchID, turn, hex.EncodeToString(h[:]))
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(signingString))
	return hex.EncodeToString(mac.Sum(nil))
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
		timestamp := r.Header.Get("X-ACB-Timestamp")
		botID := r.Header.Get("X-ACB-Bot-Id")
		if err := verifySignature(secret, matchID, turn, timestamp, botID, sig, body); err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		turnNumber, _ := strconv.Atoi(turn)
		resp := MoveResponse{Moves: []interface{}{}}
		out, _ := json.Marshal(resp)
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-ACB-Signature", signResponse(secret, matchID, turnNumber, out))
		w.WriteHeader(http.StatusOK)
		w.Write(out)
	})

	log.Printf("bot starting on :%s", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal(err)
	}
}
`
}
