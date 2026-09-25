package conformance

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// CaseResult records the outcome of one golden case against one target.
type CaseResult struct {
	Case   string
	Pass   bool
	Detail string
}

// SuiteResult is the full outcome for one target.
type SuiteResult struct {
	Passed int
	Failed []CaseResult
}

// RunSuite sends every golden case to the bot at baseURL and validates the
// responses. It returns per-case results; a target conforms only when no
// case failed.
func RunSuite(ctx context.Context, client *http.Client, baseURL, secret string) SuiteResult {
	cases := BuildCases(secret, time.Now())
	result := SuiteResult{}
	for _, c := range cases {
		res := RunCase(ctx, client, baseURL, secret, c)
		if res.Pass {
			result.Passed++
		} else {
			result.Failed = append(result.Failed, res)
		}
	}
	return result
}

// RunCase executes one golden case and validates the bot's response.
func RunCase(ctx context.Context, client *http.Client, baseURL, secret string, c Case) CaseResult {
	res := CaseResult{Case: c.Name}

	method := c.Method
	if method == "" {
		method = http.MethodPost
	}
	path := c.Path
	if path == "" {
		path = "/turn"
	}

	var bodyReader io.Reader
	if c.Body != nil {
		bodyReader = bytes.NewReader(c.Body)
	}
	req, err := http.NewRequestWithContext(ctx, method, baseURL+path, bodyReader)
	if err != nil {
		res.Detail = fmt.Sprintf("build request: %v", err)
		return res
	}
	for k, v := range c.Headers {
		req.Header.Set(k, v)
	}
	if c.ContentType != "" {
		req.Header.Set("Content-Type", c.ContentType)
	}

	resp, err := client.Do(req)
	if err != nil {
		res.Detail = fmt.Sprintf("transport: %v", err)
		return res
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		res.Detail = fmt.Sprintf("read response: %v", err)
		return res
	}

	if path == "/health" {
		return checkHealth(res, resp)
	}
	if c.Signed {
		return checkTurnResponse(res, resp, raw, secret, c)
	}
	if c.AcceptOr401 {
		// The doc lets a bot reject a non-canonical but well-formed value
		// outright; a 401 is a conformant outcome without further checks.
		if resp.StatusCode == http.StatusUnauthorized {
			res.Pass = true
			return res
		}
		return checkTurnResponse(res, resp, raw, secret, c)
	}
	return checkRejection(res, resp, c)
}

// checkHealth pins the health contract: a direct 200 with no redirect.
func checkHealth(res CaseResult, resp *http.Response) CaseResult {
	if resp.StatusCode != http.StatusOK {
		res.Detail = fmt.Sprintf("health status = %d, want 200 (redirects are not followed; return 200 directly)", resp.StatusCode)
		return res
	}
	if loc := resp.Header.Get("Location"); loc != "" {
		res.Detail = fmt.Sprintf("health responded with redirect to %q; return 200 directly", loc)
		return res
	}
	res.Pass = true
	return res
}

// checkRejection validates a must-not-execute case: status must match the
// pinned code (or be any 4xx where the doc does not pin one), and the bot
// must not hand back a 200-style turn response.
func checkRejection(res CaseResult, resp *http.Response, c Case) CaseResult {
	status := resp.StatusCode
	if c.Want4xx {
		if status < 400 || status > 499 {
			res.Detail = fmt.Sprintf("status = %d, want any 4xx (request must be rejected, never executed)", status)
			return res
		}
	} else if status != c.WantStatus {
		res.Detail = fmt.Sprintf("status = %d, want %d per docs/bot-protocol.md", status, c.WantStatus)
		return res
	}
	res.Pass = true
	return res
}

// checkTurnResponse validates a positive case: 200, JSON content type, a
// valid X-ACB-Signature over the exact response bytes, and a schema-correct
// moves array.
func checkTurnResponse(res CaseResult, resp *http.Response, raw []byte, secret string, c Case) CaseResult {
	if resp.StatusCode != http.StatusOK {
		if c.AcceptOr401 {
			res.Detail = fmt.Sprintf("status = %d, want 401 (rejected) or a fully signed 200 (accepted)", resp.StatusCode)
		} else {
			res.Detail = fmt.Sprintf("status = %d, want 200 for a canonical signed request", resp.StatusCode)
		}
		return res
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		res.Detail = fmt.Sprintf("response Content-Type = %q, want application/json", ct)
		return res
	}
	sig := resp.Header.Get("X-ACB-Signature")
	if sig == "" {
		res.Detail = "missing X-ACB-Signature response header"
		return res
	}
	if sig != strings.ToLower(sig) {
		res.Detail = "response signature must be lowercase hex"
		return res
	}
	expected := signPayload(secret, CanonicalResponsePayload(ConformanceMatchID, ConformanceTurn, raw))
	if sig != expected {
		res.Detail = "response signature does not verify over the exact response bytes ({match_id}.{turn}.{sha256_hex(raw_body)})"
		return res
	}

	var response struct {
		Moves *[]json.RawMessage `json:"moves"`
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	if err := decoder.Decode(&response); err != nil {
		res.Detail = fmt.Sprintf("response is not one JSON object: %v", err)
		return res
	}
	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); err != io.EOF {
		res.Detail = "response must be exactly one JSON value"
		return res
	}
	if response.Moves == nil {
		res.Detail = `response missing required "moves" array`
		return res
	}
	for i, moveRaw := range *response.Moves {
		if err := validateMove(moveRaw); err != nil {
			res.Detail = fmt.Sprintf("moves[%d] invalid: %v", i, err)
			return res
		}
	}
	res.Pass = true
	return res
}

func validateMove(moveRaw json.RawMessage) error {
	var move struct {
		Position *struct {
			Row *int `json:"row"`
			Col *int `json:"col"`
		} `json:"position"`
		Direction *string `json:"direction"`
	}
	if err := json.Unmarshal(moveRaw, &move); err != nil {
		return fmt.Errorf("not an object: %v", err)
	}
	if move.Position == nil || move.Position.Row == nil || move.Position.Col == nil {
		return fmt.Errorf(`requires integer "position":{"row","col"}`)
	}
	if *move.Position.Row < 0 || *move.Position.Col < 0 {
		return fmt.Errorf("position row/col must be non-negative")
	}
	if move.Direction == nil {
		return fmt.Errorf(`requires string "direction"`)
	}
	switch *move.Direction {
	case "N", "E", "S", "W", "stay":
		return nil
	default:
		return fmt.Errorf("direction %q must be N, E, S, W, or stay", *move.Direction)
	}
}
