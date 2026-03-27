// Package arena implements the 10-match mini-tournament evaluation system
// for evolved bot candidates.
//
// The arena starts the candidate as a local subprocess (the same way the
// sandbox does during validation), selects a diverse set of live opponents
// from the PostgreSQL database, and runs one match per opponent using the
// game engine directly. No job queue or ACB API calls are needed for
// evaluation matches.
package arena

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"database/sql"
	"encoding/hex"
	"fmt"
	"log"
	"math/rand"
	"net"
	"net/http"
	"os"
	"os/exec"
	"sort"
	"time"

	"github.com/aicodebattle/acb/engine"
	_ "github.com/lib/pq"
)

const (
	// DefaultNumMatches is the tournament size (10 per spec).
	DefaultNumMatches = 10

	// evalSecret is used for HMAC signing when the candidate runs locally.
	// The candidate subprocess is started with BOT_SECRET=evalSecret so that
	// the engine's request signatures match what the bot verifies.
	evalSecret = "acb-eval-secret-for-tournament-evaluation-only"

	// evalBotID is a placeholder bot ID for arena authentication headers.
	evalBotID = "b_evalcandidate"

	healthPollInterval   = 200 * time.Millisecond
	healthStartupTimeout = 30 * time.Second
)

// BotRecord holds a live bot's connection details queried from the database.
type BotRecord struct {
	BotID       string
	Name        string
	EndpointURL string
	Secret      string  // plaintext (decrypted when encryption key is provided)
	RatingMu    float64
}

// MatchOutcome records the result of one evaluation match.
type MatchOutcome struct {
	OpponentBotID string
	OpponentName  string
	CandidateSlot int   // player slot (0 or 1) assigned to the candidate
	Winner        int   // 0=player0, 1=player1, -1=draw
	Scores        []int
	Turns         int
	Err           error
}

// CandidateWon returns true when the candidate won this match.
func (o *MatchOutcome) CandidateWon() bool {
	return o.Err == nil && o.Winner == o.CandidateSlot
}

// CandidateLost returns true when the candidate lost (not a draw or error).
func (o *MatchOutcome) CandidateLost() bool {
	return o.Err == nil && o.Winner != -1 && o.Winner != o.CandidateSlot
}

// Result aggregates mini-tournament outcomes for a candidate.
type Result struct {
	CandidateEndpoint string
	Outcomes          []MatchOutcome

	// Aggregate tallies (errors excluded from win/loss/draw counts).
	Wins   int
	Losses int
	Draws  int
	Errors int

	// OpponentWinRates maps opponent BotID → candidate win rate vs that bot.
	OpponentWinRates map[string]float64

	// WinRateVec is an ordered slice of per-opponent win rates (one entry per
	// distinct opponent played, in match order, errors omitted).  Used by PSRO.
	WinRateVec []float64
}

// Config controls arena behaviour.
type Config struct {
	// NumMatches is the tournament size (default: DefaultNumMatches = 10).
	NumMatches int
	// BotTimeout is the per-turn HTTP timeout for both bots.
	BotTimeout time.Duration
	// EncryptionKey is the AES-256-GCM key (hex) used to decrypt opponent
	// secrets from the database.  Empty means secrets are stored plaintext.
	EncryptionKey string
}

// DefaultConfig returns production-ready arena defaults.
func DefaultConfig() Config {
	return Config{
		NumMatches: DefaultNumMatches,
		BotTimeout: 3 * time.Second,
	}
}

// Arena orchestrates mini-tournament evaluation of bot candidates.
type Arena struct {
	db  *sql.DB
	cfg Config
	rng *rand.Rand
	log *log.Logger
}

// New creates an Arena backed by the given database connection.
func New(db *sql.DB, cfg Config) *Arena {
	return &Arena{
		db:  db,
		cfg: cfg,
		rng: rand.New(rand.NewSource(time.Now().UnixNano())),
		log: log.Default(),
	}
}

// Run executes a mini-tournament for the candidate bot.
//
// code is the candidate's source code; language is one of
// go|python|rust|typescript|java|php.
//
// The candidate is built and started as a local subprocess, then played
// against cfg.NumMatches opponents sampled from the live bot fleet.
func (a *Arena) Run(ctx context.Context, code, language string) (*Result, error) {
	proc, err := startCandidate(ctx, code, language)
	if err != nil {
		return nil, fmt.Errorf("start candidate subprocess: %w", err)
	}
	defer proc.stop()

	candidateURL := fmt.Sprintf("http://127.0.0.1:%d", proc.port)

	opponents, err := a.selectOpponents(ctx, a.cfg.NumMatches)
	if err != nil {
		return nil, fmt.Errorf("select opponents: %w", err)
	}
	if len(opponents) == 0 {
		return nil, fmt.Errorf("no active opponents available in live bot fleet")
	}

	result := &Result{
		CandidateEndpoint: candidateURL,
		OpponentWinRates:  make(map[string]float64),
	}

	for i, opp := range opponents {
		a.log.Printf("arena: match %d/%d vs %s (%s)", i+1, len(opponents), opp.Name, opp.BotID)
		outcome := a.runMatch(ctx, candidateURL, opp)
		result.Outcomes = append(result.Outcomes, outcome)

		switch {
		case outcome.Err != nil:
			result.Errors++
			a.log.Printf("arena: match %d error: %v", i+1, outcome.Err)
		case outcome.CandidateWon():
			result.Wins++
		case outcome.CandidateLost():
			result.Losses++
		default:
			result.Draws++
		}
	}

	// Compute per-opponent win rates.
	oppWins := make(map[string]int)
	oppTotal := make(map[string]int)
	for _, o := range result.Outcomes {
		if o.Err != nil {
			continue
		}
		oppTotal[o.OpponentBotID]++
		if o.CandidateWon() {
			oppWins[o.OpponentBotID]++
		}
	}
	for id, total := range oppTotal {
		if total > 0 {
			result.OpponentWinRates[id] = float64(oppWins[id]) / float64(total)
		}
	}

	// Build ordered win-rate vector for PSRO (one entry per distinct opponent).
	seen := make(map[string]bool)
	for _, o := range result.Outcomes {
		if o.Err != nil || seen[o.OpponentBotID] {
			continue
		}
		seen[o.OpponentBotID] = true
		result.WinRateVec = append(result.WinRateVec, result.OpponentWinRates[o.OpponentBotID])
	}

	return result, nil
}

// selectOpponents queries active bots from the database and picks n opponents
// spread across the rating distribution for behavioral diversity.
func (a *Arena) selectOpponents(ctx context.Context, n int) ([]BotRecord, error) {
	rows, err := a.db.QueryContext(ctx, `
		SELECT bot_id, name, endpoint_url, shared_secret, rating_mu
		FROM bots
		WHERE status = 'active' AND endpoint_url <> ''
		ORDER BY rating_mu DESC`)
	if err != nil {
		return nil, fmt.Errorf("query bots: %w", err)
	}
	defer rows.Close()

	var all []BotRecord
	for rows.Next() {
		var b BotRecord
		if err := rows.Scan(&b.BotID, &b.Name, &b.EndpointURL, &b.Secret, &b.RatingMu); err != nil {
			return nil, fmt.Errorf("scan bot: %w", err)
		}
		if a.cfg.EncryptionKey != "" {
			if plain, err := decryptAESGCM(b.Secret, a.cfg.EncryptionKey); err == nil {
				b.Secret = plain
			}
			// Leave as-is on error (may be stored plaintext in dev).
		}
		all = append(all, b)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return selectDiverse(all, n, a.rng), nil
}

// selectDiverse picks n bots spread evenly across the rating-sorted slice.
// When fewer than n bots exist, opponents are reused (shuffled for variety).
func selectDiverse(all []BotRecord, n int, rng *rand.Rand) []BotRecord {
	if len(all) == 0 {
		return nil
	}
	sort.Slice(all, func(i, j int) bool { return all[i].RatingMu > all[j].RatingMu })

	selected := make([]BotRecord, 0, n)
	if len(all) >= n {
		for i := 0; i < n; i++ {
			idx := int(float64(i) / float64(n) * float64(len(all)))
			selected = append(selected, all[idx])
		}
	} else {
		for len(selected) < n {
			perm := rng.Perm(len(all))
			for _, idx := range perm {
				selected = append(selected, all[idx])
				if len(selected) >= n {
					break
				}
			}
		}
	}
	rng.Shuffle(len(selected), func(i, j int) { selected[i], selected[j] = selected[j], selected[i] })
	return selected
}

// runMatch runs one match between the local candidate and a live opponent.
func (a *Arena) runMatch(ctx context.Context, candidateURL string, opp BotRecord) MatchOutcome {
	outcome := MatchOutcome{
		OpponentBotID: opp.BotID,
		OpponentName:  opp.Name,
	}

	// Randomise player slot for positional fairness.
	candidateSlot := a.rng.Intn(2)
	outcome.CandidateSlot = candidateSlot

	matchID := fmt.Sprintf("eval-%d", time.Now().UnixNano())
	mr := engine.NewMatchRunner(
		engine.DefaultConfig(),
		engine.WithTimeout(a.cfg.BotTimeout),
		engine.WithRNG(rand.New(rand.NewSource(a.rng.Int63()))),
	)

	candidateBot := engine.NewHTTPBot(candidateURL,
		engine.AuthConfig{BotID: evalBotID, Secret: evalSecret, MatchID: matchID},
		engine.WithHTTPTimeout(a.cfg.BotTimeout))

	oppBot := engine.NewHTTPBot(opp.EndpointURL,
		engine.AuthConfig{BotID: opp.BotID, Secret: opp.Secret, MatchID: matchID},
		engine.WithHTTPTimeout(a.cfg.BotTimeout))

	if candidateSlot == 0 {
		mr.AddBot(candidateBot, "candidate")
		mr.AddBot(oppBot, opp.Name)
	} else {
		mr.AddBot(oppBot, opp.Name)
		mr.AddBot(candidateBot, "candidate")
	}

	res, _, err := mr.Run()
	if err != nil {
		outcome.Err = fmt.Errorf("match runner: %w", err)
		return outcome
	}
	outcome.Winner = res.Winner
	outcome.Scores = res.Scores
	outcome.Turns = res.Turns
	return outcome
}

// ── candidate subprocess management ──────────────────────────────────────────

type botProcess struct {
	port   int
	cmd    *exec.Cmd
	tmpDir string
}

func (p *botProcess) stop() {
	if p.cmd != nil && p.cmd.Process != nil {
		_ = p.cmd.Process.Kill()
		_ = p.cmd.Wait()
	}
	if p.tmpDir != "" {
		os.RemoveAll(p.tmpDir)
	}
}

func startCandidate(ctx context.Context, code, language string) (*botProcess, error) {
	tmpDir, err := os.MkdirTemp("", "acb-arena-*")
	if err != nil {
		return nil, fmt.Errorf("mkdirtemp: %w", err)
	}

	execPath, execArgs, err := buildCandidate(ctx, code, language, tmpDir)
	if err != nil {
		os.RemoveAll(tmpDir)
		return nil, fmt.Errorf("build: %w", err)
	}

	port, err := allocateFreePort()
	if err != nil {
		os.RemoveAll(tmpDir)
		return nil, fmt.Errorf("allocate port: %w", err)
	}

	env := append(os.Environ(),
		fmt.Sprintf("BOT_PORT=%d", port),
		"BOT_SECRET="+evalSecret,
	)

	var args []string
	args = append(args, execArgs...)
	cmd := exec.CommandContext(ctx, execPath, args...)
	cmd.Env = env
	cmd.Dir = tmpDir

	if err := cmd.Start(); err != nil {
		os.RemoveAll(tmpDir)
		return nil, fmt.Errorf("start process: %w", err)
	}

	proc := &botProcess{port: port, cmd: cmd, tmpDir: tmpDir}
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	if err := waitForHealth(ctx, addr); err != nil {
		proc.stop()
		return nil, fmt.Errorf("candidate health: %w", err)
	}
	return proc, nil
}

func buildCandidate(ctx context.Context, code, language, dir string) (string, []string, error) {
	switch language {
	case "go":
		if err := os.WriteFile(dir+"/bot.go", []byte(code), 0o600); err != nil {
			return "", nil, err
		}
		if err := os.WriteFile(dir+"/go.mod", []byte("module bot\n\ngo 1.21\n"), 0o600); err != nil {
			return "", nil, err
		}
		bin := dir + "/bot"
		cmd := exec.CommandContext(ctx, "go", "build", "-o", bin, ".")
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			return "", nil, fmt.Errorf("go build: %s", truncate(string(out), 512))
		}
		return bin, nil, nil

	case "python":
		src := dir + "/bot.py"
		if err := os.WriteFile(src, []byte(code), 0o600); err != nil {
			return "", nil, err
		}
		return "python3", []string{src}, nil

	case "rust":
		src := dir + "/main.rs"
		if err := os.WriteFile(src, []byte(code), 0o600); err != nil {
			return "", nil, err
		}
		bin := dir + "/bot"
		cmd := exec.CommandContext(ctx, "rustc", "--edition", "2021", src, "-o", bin)
		if out, err := cmd.CombinedOutput(); err != nil {
			return "", nil, fmt.Errorf("rustc: %s", truncate(string(out), 512))
		}
		return bin, nil, nil

	case "typescript":
		if err := os.WriteFile(dir+"/bot.ts", []byte(code), 0o600); err != nil {
			return "", nil, err
		}
		tsconfig := `{"compilerOptions":{"target":"ES2020","module":"commonjs","outDir":"./"},"files":["bot.ts"]}`
		if err := os.WriteFile(dir+"/tsconfig.json", []byte(tsconfig), 0o600); err != nil {
			return "", nil, err
		}
		cmd := exec.CommandContext(ctx, "tsc", "--project", dir+"/tsconfig.json")
		if out, err := cmd.CombinedOutput(); err != nil {
			return "", nil, fmt.Errorf("tsc: %s", truncate(string(out), 512))
		}
		return "node", []string{dir + "/bot.js"}, nil

	case "java":
		src := dir + "/Bot.java"
		if err := os.WriteFile(src, []byte(code), 0o600); err != nil {
			return "", nil, err
		}
		cmd := exec.CommandContext(ctx, "javac", src)
		if out, err := cmd.CombinedOutput(); err != nil {
			return "", nil, fmt.Errorf("javac: %s", truncate(string(out), 512))
		}
		return "java", []string{"-cp", dir, "Bot"}, nil

	case "php":
		src := dir + "/bot.php"
		if err := os.WriteFile(src, []byte(code), 0o600); err != nil {
			return "", nil, err
		}
		return "php", []string{src}, nil

	default:
		return "", nil, fmt.Errorf("unsupported language: %s", language)
	}
}

// allocateFreePort finds an unused TCP port on localhost.
func allocateFreePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()
	return port, nil
}

// waitForHealth polls GET /health until 200 OK or healthStartupTimeout elapses.
func waitForHealth(ctx context.Context, addr string) error {
	deadline := time.Now().Add(healthStartupTimeout)
	client := &http.Client{Timeout: 500 * time.Millisecond}
	for time.Now().Before(deadline) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+addr+"/health", nil)
		if err != nil {
			return err
		}
		if resp, err := client.Do(req); err == nil {
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
	return fmt.Errorf("candidate did not become healthy within %s", healthStartupTimeout)
}

// decryptAESGCM decrypts an AES-256-GCM ciphertext (hex-encoded) with the
// given hex-encoded 32-byte key.
func decryptAESGCM(ciphertextHex, keyHex string) (string, error) {
	key, err := hex.DecodeString(keyHex)
	if err != nil {
		return "", fmt.Errorf("decode key: %w", err)
	}
	if len(key) != 32 {
		return "", fmt.Errorf("key must be 32 bytes (64 hex chars)")
	}
	ciphertext, err := hex.DecodeString(ciphertextHex)
	if err != nil {
		return "", fmt.Errorf("decode ciphertext: %w", err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	ns := aead.NonceSize()
	if len(ciphertext) < ns {
		return "", fmt.Errorf("ciphertext too short")
	}
	plain, err := aead.Open(nil, ciphertext[:ns], ciphertext[ns:], nil)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}
