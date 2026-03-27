package validator

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"
)

const (
	// smokeMatchID and smokeSecret are fixed values used only during smoke tests.
	smokeMatchID = "smoke-test-match"
	smokeSecret  = "smoke-test-secret-for-validation"
	smokeBotID   = "b_smoketest"

	// healthPollInterval is how often we ping /health while waiting for startup.
	healthPollInterval = 200 * time.Millisecond
	// healthTimeout is how long we wait for the bot to become healthy.
	healthStartupTimeout = 20 * time.Second
)

// RunSmokeTest builds and starts the bot in an isolated sandbox, then sends
// cfg.SmokeRequests test /turn requests and verifies all responses are valid.
//
// When nsjail is found in PATH (and cfg.UseNsjail is true), it wraps the bot
// process for CPU/memory resource limits.  Otherwise it runs the bot directly
// with a context deadline.
func RunSmokeTest(ctx context.Context, code, language string, cfg Config) error {
	ctx, cancel := context.WithTimeout(ctx, cfg.SandboxTimeout)
	defer cancel()

	dir, err := os.MkdirTemp("", "acb-sandbox-*")
	if err != nil {
		return fmt.Errorf("mkdirtemp: %w", err)
	}
	defer os.RemoveAll(dir)

	// Build / prepare the bot.
	execPath, execArgs, err := buildBot(ctx, code, language, dir)
	if err != nil {
		return fmt.Errorf("build: %w", err)
	}

	// Allocate a free port so the bot can be reached from this process.
	port, err := freePort()
	if err != nil {
		return fmt.Errorf("allocate port: %w", err)
	}
	addr := fmt.Sprintf("127.0.0.1:%d", port)

	// Compose the bot's environment.
	env := append(os.Environ(),
		fmt.Sprintf("BOT_PORT=%d", port),
		"BOT_SECRET="+smokeSecret,
	)

	// Construct the run command, optionally wrapped in nsjail.
	cmd := makeBotCmd(ctx, execPath, execArgs, dir, env, cfg)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start bot: %w", err)
	}
	defer func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
	}()

	// Wait for the /health endpoint to respond before sending turn requests.
	if err := waitForHealth(ctx, addr, healthStartupTimeout); err != nil {
		diag := truncate(stderr.String(), 256)
		return fmt.Errorf("health startup timeout: %w (stderr: %s)", err, diag)
	}

	// Fire cfg.SmokeRequests test requests.
	client := &http.Client{Timeout: 5 * time.Second}
	for i := 1; i <= cfg.SmokeRequests; i++ {
		if err := sendTurnRequest(ctx, client, addr, i); err != nil {
			return fmt.Errorf("smoke request %d/%d failed: %w", i, cfg.SmokeRequests, err)
		}
	}
	return nil
}

// makeBotCmd returns an *exec.Cmd that runs the bot, optionally wrapped in
// nsjail when it is available and cfg.UseNsjail is true.
func makeBotCmd(ctx context.Context, execPath string, execArgs []string, dir string, env []string, cfg Config) *exec.Cmd {
	if cfg.UseNsjail {
		if nsjailBin, err := exec.LookPath(cfg.NsjailPath); err == nil {
			return buildNsjailCmd(ctx, nsjailBin, execPath, execArgs, dir, env)
		}
	}
	// Plain exec fallback.
	cmd := exec.CommandContext(ctx, execPath, execArgs...)
	cmd.Env = env
	cmd.Dir = dir
	return cmd
}

// buildNsjailCmd wraps execPath in nsjail for CPU/memory resource limiting.
// Network isolation is not applied so the bot can bind its HTTP port and
// receive requests from the test loop running in the same network namespace.
func buildNsjailCmd(ctx context.Context, nsjailBin, execPath string, execArgs []string, dir string, env []string) *exec.Cmd {
	args := []string{
		"--mode", "o",        // single-shot: run one command then exit
		"--time_limit", "30", // 30-second wall-clock limit
		"--rlimit_as", "512", // 512 MiB virtual address space
		"--rlimit_cpu", "15", // 15 CPU seconds
		"--rlimit_nofile", "64",
		"--chdir", dir,
		"--bindmount", dir, // bot workspace, read-write
	}

	// Read-only bind-mounts for language runtimes and system libraries.
	for _, p := range []string{"/bin", "/usr", "/lib", "/lib64", "/etc/alternatives", "/proc", "/dev"} {
		if _, err := os.Stat(p); err == nil {
			args = append(args, "--bindmount_ro", p)
		}
	}
	args = append(args, "--tmpfsmount", "/tmp")
	args = append(args, "--")
	args = append(args, execPath)
	args = append(args, execArgs...)

	cmd := exec.CommandContext(ctx, nsjailBin, args...)
	cmd.Env = env
	cmd.Dir = dir
	return cmd
}

// buildBot writes the bot source to dir, compiles it where necessary, and
// returns the executable path plus any arguments needed to run it.
func buildBot(ctx context.Context, code, language, dir string) (string, []string, error) {
	switch language {
	case "go":
		return buildGo(ctx, code, dir)
	case "python":
		src := filepath.Join(dir, "bot.py")
		if err := os.WriteFile(src, []byte(code), 0o600); err != nil {
			return "", nil, err
		}
		return "python3", []string{src}, nil

	case "rust":
		return buildRust(ctx, code, dir)

	case "typescript":
		return buildTypeScript(ctx, code, dir)

	case "java":
		return buildJava(ctx, code, dir)

	case "php":
		src := filepath.Join(dir, "bot.php")
		if err := os.WriteFile(src, []byte(code), 0o600); err != nil {
			return "", nil, err
		}
		return "php", []string{src}, nil

	default:
		return "", nil, fmt.Errorf("unsupported language: %s", language)
	}
}

func buildGo(ctx context.Context, code, dir string) (string, []string, error) {
	if err := os.WriteFile(filepath.Join(dir, "bot.go"), []byte(code), 0o600); err != nil {
		return "", nil, err
	}
	// Minimal go.mod so `go build` works outside a workspace.
	gomod := "module bot\n\ngo 1.21\n"
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(gomod), 0o600); err != nil {
		return "", nil, err
	}
	binPath := filepath.Join(dir, "bot")
	cmd := exec.CommandContext(ctx, "go", "build", "-o", binPath, ".")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", nil, fmt.Errorf("go build: %s", truncate(string(out), 512))
	}
	return binPath, nil, nil
}

func buildRust(ctx context.Context, code, dir string) (string, []string, error) {
	src := filepath.Join(dir, "main.rs")
	if err := os.WriteFile(src, []byte(code), 0o600); err != nil {
		return "", nil, err
	}
	binPath := filepath.Join(dir, "bot")
	cmd := exec.CommandContext(ctx, "rustc", "--edition", "2021", src, "-o", binPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", nil, fmt.Errorf("rustc: %s", truncate(string(out), 512))
	}
	return binPath, nil, nil
}

func buildTypeScript(ctx context.Context, code, dir string) (string, []string, error) {
	if err := os.WriteFile(filepath.Join(dir, "bot.ts"), []byte(code), 0o600); err != nil {
		return "", nil, err
	}
	tsconfig := `{"compilerOptions":{"target":"ES2020","module":"commonjs","outDir":"./"},"files":["bot.ts"]}`
	if err := os.WriteFile(filepath.Join(dir, "tsconfig.json"), []byte(tsconfig), 0o600); err != nil {
		return "", nil, err
	}
	cmd := exec.CommandContext(ctx, "tsc", "--project", filepath.Join(dir, "tsconfig.json"))
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", nil, fmt.Errorf("tsc: %s", truncate(string(out), 512))
	}
	return "node", []string{filepath.Join(dir, "bot.js")}, nil
}

func buildJava(ctx context.Context, code, dir string) (string, []string, error) {
	className := extractJavaPublicClass(code)
	if className == "" {
		className = "Bot"
	}
	src := filepath.Join(dir, className+".java")
	if err := os.WriteFile(src, []byte(code), 0o600); err != nil {
		return "", nil, err
	}
	cmd := exec.CommandContext(ctx, "javac", src)
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", nil, fmt.Errorf("javac: %s", truncate(string(out), 512))
	}
	return "java", []string{"-cp", dir, className}, nil
}

// freePort returns an unused TCP port on localhost.
func freePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()
	return port, nil
}

// waitForHealth polls GET /health until it returns 200 or timeout elapses.
func waitForHealth(ctx context.Context, addr string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	client := &http.Client{Timeout: 500 * time.Millisecond}
	for time.Now().Before(deadline) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+addr+"/health", nil)
		if err != nil {
			return err
		}
		resp, err := client.Do(req)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(healthPollInterval):
		}
	}
	return fmt.Errorf("bot did not become healthy within %s", timeout)
}

// sendTurnRequest sends one POST /turn request to the bot and validates the
// JSON response.
func sendTurnRequest(ctx context.Context, client *http.Client, addr string, turn int) error {
	state := makeTestState(turn)
	body, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("marshal state: %w", err)
	}

	sig := signSmokeRequest(smokeSecret, smokeMatchID, turn, body)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"http://"+addr+"/turn", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-ACB-Match-Id", smokeMatchID)
	req.Header.Set("X-ACB-Turn", strconv.Itoa(turn))
	req.Header.Set("X-ACB-Timestamp", strconv.FormatInt(time.Now().Unix(), 10))
	req.Header.Set("X-ACB-Bot-Id", smokeBotID)
	req.Header.Set("X-ACB-Signature", sig)

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("http: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bot returned HTTP %d", resp.StatusCode)
	}

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}
	return validateMoveResponse(respBody)
}

// signSmokeRequest computes the HMAC-SHA256 signature used by reference bot
// implementations.  The signing string matches the format in bots/*/main.go:
//
//	"{match_id}.{turn}.{sha256hex(body)}"
//
// Note: this format does NOT include a timestamp, matching the reference bots
// (bots/gatherer, bots/rusher, etc.) that LLM candidates are shown as templates.
func signSmokeRequest(secret, matchID string, turn int, body []byte) string {
	bodyHash := sha256.Sum256(body)
	msg := fmt.Sprintf("%s.%d.%s", matchID, turn, hex.EncodeToString(bodyHash[:]))
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(msg))
	return hex.EncodeToString(mac.Sum(nil))
}

// ── Test state types ──────────────────────────────────────────────────────

type smokeState struct {
	MatchID string       `json:"match_id"`
	Turn    int          `json:"turn"`
	Config  smokeConfig  `json:"config"`
	You     smokePlayer  `json:"you"`
	Bots    []smokeBot   `json:"bots"`
	Energy  []smokePos   `json:"energy"`
	Cores   []smokeCore  `json:"cores"`
	Walls   []smokePos   `json:"walls"`
	Dead    []smokeBot   `json:"dead"`
}

type smokeConfig struct {
	Rows           int `json:"rows"`
	Cols           int `json:"cols"`
	MaxTurns       int `json:"max_turns"`
	VisionRadius2  int `json:"vision_radius2"`
	AttackRadius2  int `json:"attack_radius2"`
	SpawnCost      int `json:"spawn_cost"`
	EnergyInterval int `json:"energy_interval"`
}

type smokePlayer struct {
	ID     int `json:"id"`
	Energy int `json:"energy"`
	Score  int `json:"score"`
}

type smokeBot struct {
	Position smokePos `json:"position"`
	Owner    int      `json:"owner"`
}

type smokePos struct {
	Row int `json:"row"`
	Col int `json:"col"`
}

type smokeCore struct {
	Position smokePos `json:"position"`
	Owner    int      `json:"owner"`
	Active   bool     `json:"active"`
}

// makeTestState returns a minimal, syntactically valid game state for smoke testing.
func makeTestState(turn int) smokeState {
	return smokeState{
		MatchID: smokeMatchID,
		Turn:    turn,
		Config: smokeConfig{
			Rows: 60, Cols: 60,
			MaxTurns: 500, VisionRadius2: 49,
			AttackRadius2: 1, SpawnCost: 3,
			EnergyInterval: 10,
		},
		You: smokePlayer{ID: 1, Energy: 10, Score: 0},
		Bots: []smokeBot{
			{Position: smokePos{Row: 10, Col: 10}, Owner: 1},
			{Position: smokePos{Row: 20, Col: 15}, Owner: 2},
		},
		Energy: []smokePos{
			{Row: 12, Col: 12},
			{Row: 5, Col: 30},
		},
		Cores: []smokeCore{
			{Position: smokePos{Row: 15, Col: 15}, Owner: 1, Active: true},
		},
		Walls: []smokePos{},
		Dead:  []smokeBot{},
	}
}

// moveResponse is the expected JSON structure of a /turn response.
type moveResponse struct {
	Moves []json.RawMessage `json:"moves"`
}

// validateMoveResponse checks the bot returned valid JSON with a "moves" array.
// An empty moves array is accepted (the bot may legally choose to idle).
func validateMoveResponse(body []byte) error {
	var resp moveResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return fmt.Errorf("invalid JSON in /turn response: %w (body: %.200s)", err, body)
	}
	return nil
}
