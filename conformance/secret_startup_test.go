package conformance

import (
	"context"
	"testing"
	"time"
)

// TestDocumentedSecretSpellingsBoot proves that the two environment variable
// spellings documented for bots are wired to the corresponding targets. The
// missing-secret tests prove fail-fast behavior; this positive check proves a
// correctly provisioned BOT_SECRET or SHARED_SECRET actually reaches startup.
func TestDocumentedSecretSpellingsBoot(t *testing.T) {
	root, err := RepoRoot()
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}

	tests := []struct {
		name       string
		targetName string
		secretName string
	}{
		{name: "strategy BOT_SECRET", targetName: "bots/random", secretName: "BOT_SECRET"},
		{name: "Go starter SHARED_SECRET", targetName: "starters/go", secretName: "SHARED_SECRET"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target, err := TargetByName(tt.targetName)
			if err != nil {
				t.Fatal(err)
			}
			if target.EnvVarNames.Secret != tt.secretName {
				t.Fatalf("target %s uses %s, want %s", target.Name, target.EnvVarNames.Secret, tt.secretName)
			}
			if skip := target.DescribeSkips(); skip != "" {
				t.Skipf("not runnable here: %s", skip)
			}

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			instance, err := BootTarget(ctx, root, target, DefaultConformanceSecret)
			if err != nil {
				t.Fatalf("boot with %s: %v", tt.secretName, err)
			}
			instance.Stop()
		})
	}
}
