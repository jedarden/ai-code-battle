// Package validator implements a three-stage validation pipeline for
// LLM-generated bot candidates:
//
//  1. Syntax — parse the generated code for the target language
//  2. Schema — verify the bot exposes the required HTTP endpoints
//  3. Sandbox — run the bot in a nsjail container, send 5 test /turn
//     requests, and verify valid JSON responses
//
// The pipeline is fail-fast: if any stage fails, subsequent stages are
// skipped.  The raw LLM output is preserved in Report so the evolution
// loop can embed it in retry prompts.
package validator

import (
	"context"
	"time"
)

// Stage identifies a validation pipeline stage.
type Stage string

const (
	StageSyntax  Stage = "syntax"
	StageSchema  Stage = "schema"
	StageSandbox Stage = "sandbox"
)

// StageResult holds the outcome of one pipeline stage.
type StageResult struct {
	Stage    Stage
	Passed   bool
	Error    string        // non-empty on failure
	Duration time.Duration // wall-clock time for the stage
}

// Report is the complete outcome of a validation run.  It is returned
// even when a stage fails so callers can log the partial results.
type Report struct {
	Language  string
	Stages    []StageResult
	Passed    bool   // true only when all three stages pass
	LLMOutput string // raw LLM response; preserved for retry / learning
}

// LastStage returns the name of the last stage that was executed (whether
// it passed or not).  Returns "" when no stages ran.
func (r *Report) LastStage() Stage {
	if len(r.Stages) == 0 {
		return ""
	}
	return r.Stages[len(r.Stages)-1].Stage
}

// Config controls pipeline behaviour.
type Config struct {
	// SyntaxTimeout caps the external process used for syntax checking.
	SyntaxTimeout time.Duration
	// SandboxTimeout caps the entire sandbox smoke test (build + run + requests).
	SandboxTimeout time.Duration
	// SmokeRequests is the number of /turn requests sent during the smoke test.
	SmokeRequests int
	// UseNsjail enables nsjail-based process isolation during the smoke test.
	// Falls back to plain exec when nsjail is not found in PATH.
	UseNsjail bool
	// NsjailPath overrides the nsjail binary name / path (default "nsjail").
	NsjailPath string
}

// DefaultConfig returns a Config with production-ready defaults.
func DefaultConfig() Config {
	return Config{
		SyntaxTimeout:  15 * time.Second,
		SandboxTimeout: 60 * time.Second,
		SmokeRequests:  5,
		UseNsjail:      true,
		NsjailPath:     "nsjail",
	}
}

// Validate runs the full three-stage pipeline for the given bot code.
// llmOutput is the raw text from which code was extracted; it is stored
// in the report for retry or learning.
//
// The returned error is only non-nil for unexpected infrastructure failures
// (e.g. temp-dir creation).  Validation failures are encoded in Report.Passed
// and the individual StageResult.Error fields.
func Validate(ctx context.Context, code, language, llmOutput string, cfg Config) (*Report, error) {
	r := &Report{
		Language:  language,
		LLMOutput: llmOutput,
	}

	type step struct {
		name Stage
		fn   func(context.Context) error
	}

	steps := []step{
		{
			StageSyntax,
			func(ctx context.Context) error {
				return CheckSyntax(ctx, code, language, cfg.SyntaxTimeout)
			},
		},
		{
			StageSchema,
			func(_ context.Context) error {
				return CheckSchema(code, language)
			},
		},
		{
			StageSandbox,
			func(ctx context.Context) error {
				return RunSmokeTest(ctx, code, language, cfg)
			},
		},
	}

	allPassed := true
	for _, s := range steps {
		t0 := time.Now()
		err := s.fn(ctx)
		sr := StageResult{
			Stage:    s.name,
			Passed:   err == nil,
			Duration: time.Since(t0),
		}
		if err != nil {
			sr.Error = err.Error()
			allPassed = false
		}
		r.Stages = append(r.Stages, sr)
		if err != nil {
			break // fail-fast: skip remaining stages
		}
	}
	r.Passed = allPassed
	return r, nil
}
