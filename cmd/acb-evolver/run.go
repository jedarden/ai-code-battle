// Package main provides the autonomous evolution loop command.
//
// The 'run' subcommand executes the full evolution pipeline autonomously:
//  1. Select island (round-robin)
//  2. Select parents via tournament selection
//  3. Build prompt with meta context
//  4. Generate candidate via LLM ensemble
//  5. Insert candidate into programs database
//  6. Run 3-stage validation (syntax → schema → sandbox)
//  7. If validation fails, retry with error feedback (up to N times)
//  8. Run arena tournament (10 matches vs live opponents)
//  9. Apply promotion gate (Nash + MAP-Elites)
//  10. If promoted, deploy to K8s and register in bots table
//  11. Enforce retirement policy
//  12. Export live.json for dashboard
//  13. Repeat
package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	_ "github.com/lib/pq"

	evolverdb "github.com/aicodebattle/acb/cmd/acb-evolver/internal/db"
	"github.com/aicodebattle/acb/cmd/acb-evolver/internal/arena"
	"github.com/aicodebattle/acb/cmd/acb-evolver/internal/live"
	"github.com/aicodebattle/acb/cmd/acb-evolver/internal/llm"
	"github.com/aicodebattle/acb/cmd/acb-evolver/internal/mapelites"
	"github.com/aicodebattle/acb/cmd/acb-evolver/internal/meta"
	"github.com/aicodebattle/acb/cmd/acb-evolver/internal/promoter"
	"github.com/aicodebattle/acb/cmd/acb-evolver/internal/prompt"
	"github.com/aicodebattle/acb/cmd/acb-evolver/internal/selector"
	"github.com/aicodebattle/acb/cmd/acb-evolver/internal/validator"
)

// RunConfig holds configuration for the autonomous evolution loop.
type RunConfig struct {
	// Evolution parameters
	NumParents   int     // number of parents for tournament selection
	TournamentK  int     // tournament size
	MaxRetries   int     // max LLM retries on validation failure
	TopBotLimit  int     // number of top bots for meta description

	// Gate thresholds
	NashThreshold     float64 // Nash value threshold for promotion
	WinRateLowerBound float64 // Wilson CI lower bound threshold

	// Retirement
	RatingThreshold float64 // minimum display rating to keep
	PopCap          int     // max evolved bots in fleet

	// Timing
	CycleInterval  time.Duration // delay between cycles (0 = continuous)
	IslandCooldown time.Duration // min time between same-island evolutions

	// Infrastructure
	LLMURL         string
	RepoDir        string
	Registry       string
	KubectlServer  string
	EncryptionKey  string
	UseNsjail      bool
	LiveExportPath string
	UploadR2       bool

	// Languages to evolve (in priority order)
	Languages []string
}

// DefaultRunConfig returns production-ready defaults.
func DefaultRunConfig() RunConfig {
	return RunConfig{
		NumParents:        2,
		TournamentK:       3,
		MaxRetries:        2,
		TopBotLimit:       10,
		NashThreshold:     0.50,
		WinRateLowerBound: 0.40,
		RatingThreshold:   1000.0,
		PopCap:            50,
		CycleInterval:     5 * time.Minute,
		IslandCooldown:    2 * time.Minute,
		LLMURL:            envOrDefault("ACB_LLM_URL", "http://zai-proxy-apexalgo.tail1b1987.ts.net:8080"),
		RepoDir:           envOrDefault("ACB_REPO_DIR", "."),
		Registry:          envOrDefault("ACB_REGISTRY", "forgejo.ardenone.com/ai-code-battle"),
		KubectlServer:     envOrDefault("ACB_KUBECTL_SERVER", "http://kubectl-ardenone-cluster:8001"),
		EncryptionKey:     os.Getenv("ACB_ENCRYPTION_KEY"),
		UseNsjail:         true,
		LiveExportPath:    envOrDefault("ACB_EVOLUTION_OUT", "evolution/live.json"),
		UploadR2:          false,
		Languages:         []string{"go", "python", "rust", "typescript", "java", "php"},
	}
}

// RunStats tracks evolution loop statistics.
type RunStats struct {
	Cycles           int
	Generated        int
	Validated        int
	ValidationFailed int
	Evaluated        int
	Promoted         int
	Retired          int
	Errors           int
	StartTime        time.Time
}

// RunEvolutionLoop executes the autonomous evolution pipeline.
//
// Usage: acb-evolver run [-continuous] [-island alpha] [-lang go] [-v]
func RunEvolutionLoop(ctx context.Context, dbURL string, args []string) {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	continuous := fs.Bool("continuous", false, "run continuously until interrupted")
	singleIsland := fs.String("island", "", "evolve only this island (empty = round-robin)")
	singleLang := fs.String("lang", "", "use only this language (empty = rotate)")
	seed := fs.Int64("seed", 0, "random seed (0 = time)")
	verbose := fs.Bool("v", false, "verbose output")
	dryRun := fs.Bool("dry-run", false, "simulate without deploying")
	maxCycles := fs.Int("max-cycles", 0, "stop after N cycles (0 = unlimited)")

	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}

	// Initialize RNG
	rng := rand.New(rand.NewSource(*seed))
	if *seed == 0 {
		rng = rand.New(rand.NewSource(time.Now().UnixNano()))
	}

	// Open database
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer db.Close()

	store := evolverdb.NewStore(db)

	// Load config from env with overrides
	cfg := DefaultRunConfig()
	if *singleLang != "" {
		cfg.Languages = []string{*singleLang}
	}

	// Track last evolution time per island for cooldown
	lastEvolved := make(map[string]time.Time)

	// Stats
	stats := RunStats{StartTime: time.Now()}

	// Setup signal handling for graceful shutdown
	ctx, cancel := context.WithCancel(ctx)
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		log.Println("Received shutdown signal, finishing current cycle...")
		cancel()
	}()

	langIdx := 0
	islandIdx := 0

	log.Printf("Evolution loop starting (continuous=%v, dry-run=%v)", *continuous, *dryRun)
	if *verbose {
		log.Printf("Config: nash=%.2f, win-lower=%.2f, max-retries=%d, languages=%v",
			cfg.NashThreshold, cfg.WinRateLowerBound, cfg.MaxRetries, cfg.Languages)
	}

	for {
		select {
		case <-ctx.Done():
			printStats(&stats)
			return
		default:
		}

		// Select island (round-robin with cooldown)
		var island string
		if *singleIsland != "" {
			island = *singleIsland
		} else {
			island = selectNextIsland(lastEvolved, cfg.IslandCooldown, islandIdx)
			islandIdx = (islandIdx + 1) % len(evolverdb.AllIslands)
		}

		// Select language (rotate)
		lang := cfg.Languages[langIdx%len(cfg.Languages)]
		langIdx++

		if *verbose {
			log.Printf("=== Cycle %d: island=%s lang=%s ===", stats.Cycles+1, island, lang)
		}

		// Run one evolution cycle
		promoted, err := runCycle(ctx, db, store, island, lang, cfg, rng, *verbose, *dryRun)
		if err != nil {
			log.Printf("Cycle failed: %v", err)
			stats.Errors++
		}
		if promoted {
			stats.Promoted++
		}

		stats.Cycles++
		stats.Generated++

		// Check cycle limit
		if *maxCycles > 0 && stats.Cycles >= *maxCycles {
			log.Printf("Reached max cycles (%d), stopping", *maxCycles)
			printStats(&stats)
			return
		}

		// Export live.json after each cycle
		exportLive(ctx, db, cfg, *verbose)

		// Continuous mode: wait for next cycle
		if *continuous {
			lastEvolved[island] = time.Now()
			if cfg.CycleInterval > 0 {
				if *verbose {
					log.Printf("Sleeping %v until next cycle...", cfg.CycleInterval)
				}
				select {
				case <-ctx.Done():
					printStats(&stats)
					return
				case <-time.After(cfg.CycleInterval):
				}
			}
		} else {
			// Single-shot mode
			printStats(&stats)
			return
		}
	}
}

// selectNextIsland picks the next island to evolve, respecting cooldown.
func selectNextIsland(lastEvolved map[string]time.Time, cooldown time.Duration, startIdx int) string {
	now := time.Now()

	// Try each island starting from startIdx
	for i := 0; i < len(evolverdb.AllIslands); i++ {
		idx := (startIdx + i) % len(evolverdb.AllIslands)
		island := evolverdb.AllIslands[idx]

		last, ok := lastEvolved[island]
		if !ok || now.Sub(last) >= cooldown {
			return island
		}
	}

	// All islands on cooldown - pick the one with longest time since last evolve
	var oldestIsland string
	var oldestTime time.Time
	for _, island := range evolverdb.AllIslands {
		last, ok := lastEvolved[island]
		if !ok {
			return island
		}
		if oldestTime.IsZero() || last.Before(oldestTime) {
			oldestTime = last
			oldestIsland = island
		}
	}
	return oldestIsland
}

// runCycle executes one complete evolution cycle for the given island.
func runCycle(ctx context.Context, db *sql.DB, store *evolverdb.Store,
	island, lang string, cfg RunConfig, rng *rand.Rand, verbose, dryRun bool) (bool, error) {

	// 1. Load programs from the island
	programs, err := store.ListByIsland(ctx, island)
	if err != nil {
		return false, fmt.Errorf("load programs: %w", err)
	}
	if len(programs) == 0 {
		return false, fmt.Errorf("no programs on island %s - seed the database first", island)
	}

	// 2. Select parents via tournament selection
	parents := selector.SelectParents(programs, cfg.NumParents, cfg.TournamentK, rng)
	if verbose {
		for i, p := range parents {
			log.Printf("  Parent %d: id=%d fitness=%.3f", i+1, p.ID, p.Fitness)
		}
	}

	// 3. Build meta description
	metaBuilder := meta.NewBuilder(store)
	metaDesc, err := metaBuilder.Build(ctx, cfg.TopBotLimit)
	if err != nil {
		log.Printf("warn: meta build failed: %v", err)
		metaDesc = &meta.Description{TotalBots: len(programs), IslandStats: make(map[string]meta.IslandStats)}
	}

	// 4. Determine generation number
	maxGen := 0
	for _, p := range programs {
		if p.Generation > maxGen {
			maxGen = p.Generation
		}
	}
	generation := maxGen + 1

	// 5. Generate candidate with retry loop
	var programID int64
	var code string
	var program *evolverdb.Program
	var report *validator.Report

	for retry := 0; retry <= cfg.MaxRetries; retry++ {
		if retry > 0 && verbose {
			log.Printf("  Retry %d/%d with error feedback...", retry, cfg.MaxRetries)
		}

		// Assemble prompt (with error feedback if retry)
		req := prompt.BuildRequest(parents, nil, metaDesc, island, lang, generation)
		if retry > 0 && report != nil {
			// Add error feedback to prompt
			req.TaskOverride = buildRetryPrompt(report, lang)
		}
		fullPrompt := prompt.Assemble(req)

		// Run LLM ensemble
		client := llm.NewClient(cfg.LLMURL, "")
		ensembleCfg := llm.DefaultEnsembleConfig()
		ensembleCfg.NumCandidates = 3
		ensembleCfg.RefineTop = true

		result, err := client.Ensemble(ctx, fullPrompt, lang, ensembleCfg)
		if err != nil {
			log.Printf("LLM ensemble failed: %v", err)
			continue
		}
		if result.Best == nil {
			log.Printf("No valid candidate from LLM")
			continue
		}

		code = result.Best.Code

		// Estimate behavior vector from code
		behaviorVec := estimateBehaviorVector(code, lang)

		// Insert into database first (so we have a program ID for tracking)
		parentIDs := make([]int64, len(parents))
		for i, p := range parents {
			parentIDs[i] = p.ID
		}

		programID, err = store.Create(ctx, &evolverdb.Program{
			Code:           code,
			Language:       lang,
			Island:         island,
			Generation:     generation,
			ParentIDs:      parentIDs,
			BehaviorVector: behaviorVec,
			Fitness:        0.0,
			Promoted:       false,
		})
		if err != nil {
			return false, fmt.Errorf("insert program: %w", err)
		}

		if verbose {
			log.Printf("  Created program %d (gen %d)", programID, generation)
		}

		// Run validation
		valCfg := validator.DefaultConfig()
		valCfg.UseNsjail = cfg.UseNsjail

		report, err = validator.Validate(ctx, code, lang, result.Best.Code, valCfg)
		if err != nil {
			log.Printf("Validation infrastructure error: %v", err)
			store.Delete(ctx, programID)
			programID = 0
			continue
		}

		// Log validation result
		valLog := &evolverdb.ValidationLog{
			Island:    island,
			Language:  lang,
			Stage:     string(report.LastStage()),
			Passed:    report.Passed,
			LLMOutput: report.LLMOutput,
		}
		if !report.Passed {
			for _, sr := range report.Stages {
				if !sr.Passed {
					valLog.ErrorText = sr.Error
					break
				}
			}
		}
		store.RecordValidation(ctx, valLog)

		if !report.Passed {
			if verbose {
				log.Printf("  Validation FAILED at stage %s: %s", report.LastStage(), valLog.ErrorText)
			}
			store.Delete(ctx, programID)
			programID = 0
			continue // retry
		}

		// Validation passed - break out of retry loop
		if verbose {
			log.Printf("  Validation PASSED (all 3 stages)")
		}

		// Fetch the program for later use
		program, _ = store.Get(ctx, programID)
		break
	}

	// Check if we have a valid program
	if programID == 0 || code == "" {
		return false, fmt.Errorf("all retries exhausted without valid candidate")
	}

	// 6. Run arena evaluation
	arenaCfg := arena.DefaultConfig()
	arenaCfg.EncryptionKey = cfg.EncryptionKey
	a := arena.New(db, arenaCfg)

	if verbose {
		log.Printf("  Running %d-match arena tournament...", arena.DefaultNumMatches)
	}

	arenaResult, err := a.Run(ctx, code, lang)
	if err != nil {
		store.Delete(ctx, programID)
		return false, fmt.Errorf("arena: %w", err)
	}

	// Compute fitness (overall win rate)
	wr := arena.ComputeFromResult(arenaResult)
	fitness := wr.Rate

	// Get behavior vector
	var behaviorVec []float64
	if program != nil && len(program.BehaviorVector) >= 2 {
		behaviorVec = program.BehaviorVector
	} else {
		behaviorVec = []float64{0.5, 0.5}
	}

	// Update fitness in database
	store.UpdateFitness(ctx, programID, fitness, behaviorVec)

	if verbose {
		log.Printf("  Arena result: %d W / %d L / %d D / %d err  win rate=%.3f",
			arenaResult.Wins, arenaResult.Losses, arenaResult.Draws, arenaResult.Errors, fitness)
	}

	// 7. Load MAP-Elites grid and apply promotion gate
	grid := mapelites.New(10)
	promotedPrograms, _ := store.ListPromoted(ctx)
	for _, pp := range promotedPrograms {
		if len(pp.BehaviorVector) >= 2 {
			grid.TryPlace(pp.ProgramID, pp.Fitness, pp.BehaviorVector[0], pp.BehaviorVector[1])
		}
	}

	gateCfg := arena.GateConfig{
		NashThreshold:     cfg.NashThreshold,
		WinRateLowerBound: cfg.WinRateLowerBound,
	}
	gate := arena.NewGate(gateCfg, grid)
	gateResult := gate.Evaluate(arenaResult, programID, fitness, behaviorVec)

	if verbose {
		log.Printf("  Gate: %s", gateResult.Reason)
	}

	if !gateResult.Promoted {
		if verbose {
			log.Printf("  Decision: REJECTED")
		}
		return false, nil
	}

	if verbose {
		log.Printf("  Decision: PROMOTED")
	}

	if dryRun {
		log.Printf("  [dry-run] Would promote program %d", programID)
		return true, nil
	}

	// 8. Deploy the promoted bot
	if program == nil {
		program, _ = store.Get(ctx, programID)
	}
	if program == nil {
		return false, fmt.Errorf("program %d not found after gate pass", programID)
	}

	promCfg := promoter.DefaultConfig()
	promCfg.Registry = cfg.Registry
	promCfg.RepoDir = cfg.RepoDir
	promCfg.KubectlServer = cfg.KubectlServer
	promCfg.EncryptionKey = cfg.EncryptionKey
	promCfg.RatingThreshold = cfg.RatingThreshold
	promCfg.PopCap = cfg.PopCap

	p := promoter.New(store, db, promCfg)
	promResult, err := p.Promote(ctx, program)
	if err != nil {
		return false, fmt.Errorf("promote: %w", err)
	}

	log.Printf("  Promoted: bot_name=%s bot_id=%s endpoint=%s",
		promResult.BotName, promResult.BotID, promResult.Endpoint)

	// 9. Enforce retirement policy
	retired, err := p.EnforcePolicy(ctx)
	if err != nil {
		log.Printf("warn: retirement policy error: %v", err)
	}
	if len(retired) > 0 {
		log.Printf("  Retired %d bot(s)", len(retired))
	}

	return true, nil
}

// estimateBehaviorVector analyzes code to estimate aggression/economy behavior.
func estimateBehaviorVector(code, lang string) []float64 {
	// Default to balanced behavior
	aggression := 0.5
	economy := 0.5

	codeLower := strings.ToLower(code)

	// Aggression indicators
	aggressivePatterns := []string{
		"attack", "rush", "hunt", "target", "enemy", "combat", "aggress",
		"move_toward", "path_to_enemy", "closest_enemy", "attack_radius",
	}
	aggressiveCount := 0
	for _, p := range aggressivePatterns {
		aggressiveCount += strings.Count(codeLower, p)
	}

	// Economy indicators
	economyPatterns := []string{
		"energy", "collect", "gather", "resource", "pickup", "spawn",
		"score", "efficiency", "path_to_energy", "nearest_energy",
	}
	economyCount := 0
	for _, p := range economyPatterns {
		economyCount += strings.Count(codeLower, p)
	}

	// Defensive indicators
	defensivePatterns := []string{
		"defend", "guard", "protect", "perimeter", "patrol", "safe",
		"retreat", "flee", "avoid", "home", "core_defense",
	}
	defensiveCount := 0
	for _, p := range defensivePatterns {
		defensiveCount += strings.Count(codeLower, p)
	}

	// Normalize and adjust behavior vector
	total := aggressiveCount + economyCount + defensiveCount
	if total > 0 {
		aggression = float64(aggressiveCount) / float64(total)
		// Economy is relative to energy/gather focus vs combat
		economy = float64(economyCount) / float64(total+1)
		// Adjust aggression based on defensive patterns
		if defensiveCount > aggressiveCount {
			aggression = aggression * 0.5 // reduce aggression for defensive bots
		}
	}

	// Clamp to [0.1, 0.9] to avoid edge cases
	aggression = clamp(aggression, 0.1, 0.9)
	economy = clamp(economy, 0.1, 0.9)

	return []float64{aggression, economy}
}

func clamp(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

// buildRetryPrompt creates a task prompt that includes error feedback.
func buildRetryPrompt(report *validator.Report, lang string) string {
	var failedStage string
	var errorMsg string
	for _, sr := range report.Stages {
		if !sr.Passed {
			failedStage = string(sr.Stage)
			errorMsg = sr.Error
			break
		}
	}

	return fmt.Sprintf(`The previous candidate failed validation at the %s stage with this error:

%s

Please fix this issue and generate an improved bot in %s. The bot must:
1. Have valid syntax that compiles without errors
2. Expose GET /health and POST /turn HTTP endpoints
3. Return JSON in the format {"moves": [{"bot_id": "x", "move": "up|down|left|right|attack"}]}

Focus on fixing the specific error above while maintaining all required functionality.`, failedStage, errorMsg, lang)
}

// exportLive exports the evolution state to live.json.
func exportLive(ctx context.Context, db *sql.DB, cfg RunConfig, verbose bool) {
	data, err := live.Export(ctx, db)
	if err != nil {
		log.Printf("warn: live export failed: %v", err)
		return
	}

	if err := live.WriteFile(data, cfg.LiveExportPath); err != nil {
		log.Printf("warn: write live.json: %v", err)
		return
	}

	if cfg.UploadR2 {
		r2Cfg := live.R2ConfigFromEnv()
		if r2Cfg.HasCredentials() {
			r2Client, err := live.NewR2Client(r2Cfg)
			if err == nil {
				r2Client.UploadLiveJSON(ctx, data)
			}
		}
	}

	if verbose {
		log.Printf("  Exported live.json (%d programs)", data.TotalPrograms)
	}
}

// printStats displays evolution loop statistics.
func printStats(stats *RunStats) {
	elapsed := time.Since(stats.StartTime)
	log.Printf("=== Evolution Loop Stats ===")
	log.Printf("  Cycles: %d (%.1f/min)", stats.Cycles, float64(stats.Cycles)/elapsed.Minutes())
	log.Printf("  Generated: %d", stats.Generated)
	log.Printf("  Validated: %d", stats.Validated)
	log.Printf("  Evaluated: %d", stats.Evaluated)
	log.Printf("  Promoted: %d", stats.Promoted)
	log.Printf("  Retired: %d", stats.Retired)
	log.Printf("  Errors: %d", stats.Errors)
	log.Printf("  Uptime: %v", elapsed.Round(time.Second))
}
