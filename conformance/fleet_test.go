package conformance

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestTargetsRegistryCoversWorkspace pins that every directory under bots/
// and starters/ is registered as a conformance target, so a new bot cannot
// land without entering the suite.
func TestTargetsRegistryCoversWorkspace(t *testing.T) {
	root, err := RepoRoot()
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}
	registered := map[string]bool{}
	for _, target := range Targets() {
		if registered[target.Name] {
			t.Errorf("duplicate target %q", target.Name)
		}
		registered[target.Name] = true
		if len(target.Run) == 0 && target.Binary == "" {
			t.Errorf("target %q has neither Run nor Binary", target.Name)
		}
	}
	for _, group := range []string{"bots", "starters"} {
		entries, err := os.ReadDir(filepath.Join(root, group))
		if err != nil {
			t.Fatalf("read %s/: %v", group, err)
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			name := group + "/" + entry.Name()
			if !registered[name] {
				t.Errorf("%s exists but is not a registered conformance target", name)
			}
		}
	}
}

// fleetSelection resolves an ACB_CONFORMANCE_FLEET spec — "all" or a
// comma-separated target list — against the registry.
func fleetSelection(spec string) (map[string]bool, error) {
	all := map[string]bool{}
	for _, target := range Targets() {
		all[target.Name] = true
	}
	want := map[string]bool{}
	for _, name := range strings.Split(spec, ",") {
		name = strings.TrimSpace(name)
		if name == "all" {
			for n := range all {
				want[n] = true
			}
			continue
		}
		if !all[name] {
			return nil, fmt.Errorf("unknown target %q (see Targets())", name)
		}
		want[name] = true
	}
	return want, nil
}

// skipUnavailableTarget keeps local fleet runs useful when a developer has
// only some language toolchains installed. CI sets ACB_CONFORMANCE_REQUIRE_ALL
// so a missing or misconfigured runtime fails the gate instead of silently
// reducing its language coverage.
func skipUnavailableTarget(t *testing.T, target Target) bool {
	t.Helper()
	if reason := target.DescribeSkips(); reason != "" {
		if os.Getenv("ACB_CONFORMANCE_REQUIRE_ALL") == "1" {
			t.Fatalf("required conformance target %s is unavailable: %s", target.Name, reason)
		}
		t.Skipf("not runnable here: %s", reason)
		return true
	}
	return false
}

func requestedFleet(t *testing.T) string {
	t.Helper()
	spec := os.Getenv("ACB_CONFORMANCE_FLEET")
	if spec == "" || testing.Short() {
		if os.Getenv("ACB_CONFORMANCE_REQUIRE_ALL") == "1" {
			t.Fatal("strict conformance mode requires ACB_CONFORMANCE_FLEET and cannot run with -short")
		}
		t.Skip("fleet sweep is opt-in: set ACB_CONFORMANCE_FLEET=all (or a comma-separated target list)")
	}
	return spec
}

// TestFleetConformance boots every registered target against the full golden
// case table. It is opt-in — the sweep builds and boots real bots across six
// toolchains and does not belong in the default gate:
//
//	ACB_CONFORMANCE_FLEET=all go test ./conformance/ -run TestFleetConformance -v -timeout 120m
//
// ACB_CONFORMANCE_FLEET also accepts a comma-separated target list, e.g.
// ACB_CONFORMANCE_FLEET=bots/farmer,starters/go. Targets whose toolchain is
// missing from the environment are reported as skipped, never silently
// dropped.
func TestFleetConformance(t *testing.T) {
	spec := requestedFleet(t)

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
			if skipUnavailableTarget(t, target) {
				return
			}
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
			defer cancel()
			start := time.Now()
			_, result, err := RunAgainstTarget(ctx, root, target, DefaultConformanceSecret)
			if err != nil {
				t.Errorf("boot failed: %v", err)
				return
			}
			t.Logf("%d/%d cases passed in %s",
				result.Passed, result.Passed+len(result.Failed), time.Since(start).Round(time.Second))
			for _, f := range result.Failed {
				t.Errorf("%s: %s", f.Case, f.Detail)
			}
		})
	}
}
