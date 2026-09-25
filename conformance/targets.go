package conformance

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Build timeout hints; generous because cold cargo builds of the axum bots
// can take minutes on a loaded shared box.
const (
	cargoBuildTimeout  = 8 * time.Minute
	mavenBuildTimeout  = 8 * time.Minute
	dotnetBuildTimeout = 5 * time.Minute
	npmBuildTimeout    = 3 * time.Minute
	goBuildTimeout     = 2 * time.Minute
)

// Target describes one conformance subject: a bot under bots/ or a starter
// template under starters/. Build and Run are argv templates executed with
// Dir as the working directory; the harness injects BOT_PORT/BOT_SECRET (or
// the starter's PORT/SHARED_SECRET spelling) into the environment.
type Target struct {
	// Name is the repo-relative directory, e.g. "bots/farmer".
	Name string
	// Lang selects the toolchain probe and default env spelling.
	Lang string // go, python, node, typescript, rust, csharp, java, php
	// Build is run once before boot; empty means no build step.
	Build []string
	// BuildTimeout bounds the build step.
	BuildTimeoutHint time.Duration
	// Run starts the bot server. Empty with a non-empty Binary means the
	// binary is resolved after the build (see BootTarget).
	Run []string
	// Binary is the executable name a cargo build produces; BootTarget
	// locates it after building because the effective CARGO_TARGET_DIR
	// varies (fleet cargo wrappers redirect build output to a shared
	// per-repo directory).
	Binary string
	// EnvVarNames is the port/secret environment spelling the target reads.
	EnvVarNames EnvSpelling
	// RunFrom is the directory Run executes in when it differs from the
	// target directory (used by prebuilt-node targets).
	RunFrom string
}

// EnvSpelling names the env vars a target actually reads. The strategy bots
// standardised on BOT_PORT/BOT_SECRET; several starter templates still read
// PORT/SHARED_SECRET.
type EnvSpelling struct {
	Port   string
	Secret string
}

var (
	envBot     = EnvSpelling{Port: "BOT_PORT", Secret: "BOT_SECRET"}
	envStarter = EnvSpelling{Port: "PORT", Secret: "SHARED_SECRET"}
)

// Targets returns the full registry: every bot under bots/ and every starter
// template, in stable order.
func Targets() []Target {
	return []Target{
		// Go strategy bots (separate modules, one directory each).
		{Name: "bots/farmer", Lang: "go", Build: goBuild("farmer"), BuildTimeoutHint: goBuildTimeout, Run: runBuilt("farmer"), EnvVarNames: envBot},
		{Name: "bots/gatherer", Lang: "go", Build: goBuild("gatherer"), BuildTimeoutHint: goBuildTimeout, Run: runBuilt("gatherer"), EnvVarNames: envBot},
		{Name: "bots/opportunist", Lang: "go", Build: goBuild("opportunist"), BuildTimeoutHint: goBuildTimeout, Run: runBuilt("opportunist"), EnvVarNames: envBot},
		{Name: "bots/siege", Lang: "go", Build: goBuild("siege"), BuildTimeoutHint: goBuildTimeout, Run: runBuilt("siege"), EnvVarNames: envBot},
		// Python strategy bots (stdlib only, no install step).
		{Name: "bots/economist", Lang: "python", Run: []string{"python3", "bot.py"}, EnvVarNames: envBot},
		{Name: "bots/nomad", Lang: "python", Run: []string{"python3", "main.py"}, EnvVarNames: envBot},
		{Name: "bots/random", Lang: "python", Run: []string{"python3", "main.py"}, EnvVarNames: envBot},
		{Name: "bots/scout", Lang: "python", Run: []string{"python3", "main.py"}, EnvVarNames: envBot},
		// Node strategy bots (plain JS, zero dependencies).
		{Name: "bots/kamikaze", Lang: "node", Run: []string{"node", "index.js"}, EnvVarNames: envBot},
		{Name: "bots/pacifist", Lang: "node", Run: []string{"node", "index.js"}, EnvVarNames: envBot},
		// TypeScript strategy bots compile from src via npm ci (dist/ is
		// gitignored, so a fresh checkout has no build to run).
		{Name: "bots/coordinator", Lang: "typescript", Build: npmCiBuild(), BuildTimeoutHint: npmBuildTimeout, Run: []string{"node", "dist/index.js"}, EnvVarNames: envBot},
		{Name: "bots/swarm", Lang: "typescript", Build: npmCiBuild(), BuildTimeoutHint: npmBuildTimeout, Run: []string{"node", "dist/index.js"}, EnvVarNames: envBot},
		// Rust strategy bots (cargo, offline-friendly locked builds).
		{Name: "bots/assassin", Lang: "rust", Build: cargoBuildLocked(), BuildTimeoutHint: cargoBuildTimeout, Binary: "assassin-bot", EnvVarNames: envBot},
		{Name: "bots/phalanx", Lang: "rust", Build: cargoBuildLocked(), BuildTimeoutHint: cargoBuildTimeout, Binary: "phalanx-bot", EnvVarNames: envBot},
		{Name: "bots/rusher", Lang: "rust", Build: cargoBuildLocked(), BuildTimeoutHint: cargoBuildTimeout, Binary: "rusher-bot", EnvVarNames: envBot},
		{Name: "bots/zone-driver", Lang: "rust", Build: cargoBuildLocked(), BuildTimeoutHint: cargoBuildTimeout, Binary: "zone-driver-bot", EnvVarNames: envBot},
		// C# strategy bots (dotnet publish into the target's own bin/).
		{Name: "bots/defender", Lang: "csharp", Build: dotnetPublish(), BuildTimeoutHint: dotnetBuildTimeout, Run: []string{"dotnet", filepath.Join("bin", "defender.dll")}, EnvVarNames: envBot},
		// Java strategy bots (maven shade jar, target/<artifact>-<version>.jar).
		{Name: "bots/hunter", Lang: "java", Build: mavenPackage(), BuildTimeoutHint: mavenBuildTimeout, Run: []string{"java", "-jar", filepath.Join("target", "hunter-bot-1.0.0.jar")}, EnvVarNames: envBot},
		{Name: "bots/leader-targeter", Lang: "java", Build: mavenPackage(), BuildTimeoutHint: mavenBuildTimeout, Run: []string{"java", "-jar", filepath.Join("target", "leader-targeter-bot-1.0.0.jar")}, EnvVarNames: envBot},
		{Name: "bots/raider", Lang: "java", Build: mavenPackage(), BuildTimeoutHint: mavenBuildTimeout, Run: []string{"java", "-jar", filepath.Join("target", "raider-bot-1.0.0.jar")}, EnvVarNames: envBot},
		// PHP strategy bots (stream_socket_server script, no build step).
		{Name: "bots/guardian", Lang: "php", Run: []string{"php", "index.php"}, EnvVarNames: envBot},
		// Starter templates.
		{Name: "starters/go", Lang: "go", Build: goBuild("acb-starter-bot"), BuildTimeoutHint: goBuildTimeout, Run: runBuilt("acb-starter-bot"), EnvVarNames: envStarter},
		{Name: "starters/python", Lang: "python", Run: []string{"python3", "main.py"}, EnvVarNames: envStarter},
		{Name: "starters/javascript", Lang: "node", Run: []string{"node", "index.js"}, EnvVarNames: envBot},
		{Name: "starters/typescript", Lang: "typescript", Build: npmCiBuild(), BuildTimeoutHint: npmBuildTimeout, Run: []string{"node", "dist/index.js"}, EnvVarNames: envBot},
		{Name: "starters/rust", Lang: "rust", Build: cargoBuildUnlocked(), BuildTimeoutHint: cargoBuildTimeout, Binary: "acb-starter-bot", EnvVarNames: envStarter},
		{Name: "starters/csharp", Lang: "csharp", Build: dotnetPublish(), BuildTimeoutHint: dotnetBuildTimeout, Run: []string{"dotnet", filepath.Join("bin", "acb-starter-csharp.dll")}, EnvVarNames: envBot},
		{Name: "starters/java", Lang: "java", Build: mavenPackage(), BuildTimeoutHint: mavenBuildTimeout, Run: []string{"java", "-jar", filepath.Join("target", "starter-bot-1.0.0.jar")}, EnvVarNames: envBot},
		{Name: "starters/php", Lang: "php", Run: []string{"php", "index.php"}, EnvVarNames: envBot},
	}
}

// TargetByName resolves one registry entry.
func TargetByName(name string) (Target, error) {
	for _, t := range Targets() {
		if t.Name == name {
			return t, nil
		}
	}
	return Target{}, fmt.Errorf("unknown target %q (see TargetByName list output)", name)
}

func goBuild(binary string) []string {
	// Bots are separate modules; build each into the harness-owned bin dir.
	return []string{"go", "build", "-o", filepath.Join(harnessBinDir(), binary), "."}
}

func runBuilt(binary string) []string {
	return []string{filepath.Join(harnessBinDir(), binary)}
}

func cargoBuildLocked() []string {
	return []string{"cargo", "build", "--release", "--locked", "--quiet"}
}

func cargoBuildUnlocked() []string {
	// starters/rust ships no Cargo.lock, so --locked cannot apply.
	return []string{"cargo", "build", "--release", "--quiet"}
}

func mavenPackage() []string {
	return []string{"mvn", "-q", "-DskipTests", "package"}
}

func dotnetPublish() []string {
	return []string{"dotnet", "publish", "-c", "Release", "-o", "bin"}
}

// npmCiBuild installs the starter's locked dev dependencies (tsc is not
// committed) and compiles, so the harness is self-sufficient on a fresh
// checkout.
func npmCiBuild() []string {
	return []string{"bash", "-c", "npm ci --no-audit --no-fund --loglevel=error && npm run build"}
}

// Toolchain probes. A target whose toolchain is absent is reported as
// skipped, never silently dropped.
func toolchainMissing(lang string) string {
	switch lang {
	case "go":
		return probe("go", "go", "version")
	case "python":
		return probe("python3", "python3", "--version")
	case "node", "typescript":
		return probe("node", "node", "--version")
	case "rust":
		return probe("cargo", "cargo", "--version")
	case "csharp":
		return probe("dotnet", "dotnet", "--version")
	case "java":
		if out := probe("java", "java", "-version"); out != "" {
			return out
		}
		return probe("mvn", "mvn", "--version")
	case "php":
		return probe("php", "php", "--version")
	default:
		return "unknown toolchain " + lang
	}
}

func probe(label string, name string, args ...string) string {
	if _, err := exec.LookPath(name); err != nil {
		return label + " not installed"
	}
	return ""
}

// harnessBinDir returns the scratch directory built binaries are placed in.
func harnessBinDir() string {
	dir := filepath.Join(os.TempDir(), "acb-bot-conformance", "bin")
	_ = os.MkdirAll(dir, 0o755)
	return dir
}

// DescribeSkips explains, per target, why it cannot run in this environment.
// A target with an empty return value is runnable.
func (t Target) DescribeSkips() string {
	if reason := toolchainMissing(t.Lang); reason != "" {
		return reason
	}
	if len(t.Run) == 0 && t.Binary == "" {
		return "no run command configured"
	}
	if len(t.Build) > 0 {
		return ""
	}
	// No build step: any file argument the run command needs must already
	// exist inside the target directory (e.g. dist/index.js). Paths resolve
	// against the repository root — the go test working directory is the
	// package directory, not the repo root.
	root, err := RepoRoot()
	if err != nil {
		return fmt.Sprintf("resolve repository root: %v", err)
	}
	for _, arg := range t.Run[1:] {
		if strings.HasSuffix(arg, ".js") || strings.HasSuffix(arg, ".py") {
			if _, err := os.Stat(filepath.Join(root, t.Dir(), arg)); err != nil {
				return fmt.Sprintf("entry point %s is not built (run the target's build step first)", arg)
			}
		}
	}
	return ""
}

// Dir is the working directory for build and boot of this target, resolved
// against the repository root when the harness runs from elsewhere.
func (t Target) Dir() string {
	if t.RunFrom != "" {
		return t.RunFrom
	}
	return t.Name
}
