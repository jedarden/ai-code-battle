package conformance

import (
	"context"
	"os"
	"testing"
	"time"
)

// TestMissingSecretFailFast holds every zero-build target to the
// provisioning contract of docs/bot-protocol.md: "A bot that boots without
// its secret must fail fast and refuse to serve /turn". The interpreted bots
// and starters (Python, Node, PHP) boot instantly and need no compilation,
// so the whole set runs in the default gate; compiled targets run the same
// check under the opt-in fleet sweep (TestFleetMissingSecretFailFast).
func TestMissingSecretFailFast(t *testing.T) {
	root, err := RepoRoot()
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}
	for _, target := range Targets() {
		if len(target.Build) != 0 {
			continue // compiled targets: opt-in fleet sweep
		}
		target := target
		t.Run(target.Name, func(t *testing.T) {
			if skip := target.DescribeSkips(); skip != "" {
				t.Skipf("not runnable here: %s", skip)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			start := time.Now()
			if err := CheckMissingSecretFailFast(ctx, root, target); err != nil {
				t.Errorf("%s", err)
				return
			}
			t.Logf("exited without a secret in %s", time.Since(start).Round(time.Millisecond))
		})
	}
}

// TestFleetMissingSecretFailFast runs the missing-secret check across the
// whole registry, compiled targets included. It shares the main sweep's
// opt-in gate because it builds and boots real bots across every toolchain:
//
//	ACB_CONFORMANCE_FLEET=all go test ./conformance/ -run TestFleetMissingSecretFailFast -v -timeout 60m
func TestFleetMissingSecretFailFast(t *testing.T) {
	spec := os.Getenv("ACB_CONFORMANCE_FLEET")
	if spec == "" || testing.Short() {
		t.Skip("fleet sweep is opt-in: set ACB_CONFORMANCE_FLEET=all (or a comma-separated target list)")
	}
	want, err := fleetSelection(spec)
	if err != nil {
		t.Fatal(err)
	}
	root, err := RepoRoot()
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}
	for _, target := range Targets() {
		if !want[target.Name] {
			continue
		}
		target := target
		t.Run(target.Name, func(t *testing.T) {
			if skip := target.DescribeSkips(); skip != "" {
				t.Skipf("not runnable here: %s", skip)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
			defer cancel()
			start := time.Now()
			if err := CheckMissingSecretFailFast(ctx, root, target); err != nil {
				t.Errorf("%s", err)
				return
			}
			t.Logf("exited without a secret in %s", time.Since(start).Round(time.Millisecond))
		})
	}
}
