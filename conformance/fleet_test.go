package conformance

import (
	"context"
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
	spec := os.Getenv("ACB_CONFORMANCE_FLEET")
	if spec == "" || testing.Short() {
		t.Skip("fleet sweep is opt-in: set ACB_CONFORMANCE_FLEET=all (or a comma-separated target list)")
	}

	root, err := RepoRoot()
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}

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
			t.Fatalf("unknown target %q (see Targets())", name)
		}
		want[name] = true
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
