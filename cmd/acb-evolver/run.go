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
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	_ "github.com/lib/pq"

	"github.com/aicodebattle/acb/cmd/acb-evolver/internal/arena"
	"github.com/aicodebattle/acb/cmd/acb-evolver/internal/crosspoll"
	evolverdb "github.com/aicodebattle/acb/cmd/acb-evolver/internal/db"
	"github.com/aicodebattle/acb/cmd/acb-evolver/internal/live"
	"github.com/aicodebattle/acb/cmd/acb-evolver/internal/llm"
	"github.com/aicodebattle/acb/cmd/acb-evolver/internal/mapelites"
	"github.com/aicodebattle/acb/cmd/acb-evolver/internal/meta"
	"github.com/aicodebattle/acb/cmd/acb-evolver/internal/promoter"
	"github.com/aicodebattle/acb/cmd/acb-evolver/internal/prompt"
	"github.com/aicodebattle/acb/cmd/acb-evolver/internal/selector"
	"github.com/aicodebattle/acb/cmd/acb-evolver/internal/validator"
	"github.com/aicodebattle/acb/metrics"
)

// RunConfig holds configuration for the autonomous evolution loop.
type RunConfig struct {
	// Evolution parameters
	NumParents  int // number of parents for tournament selection
	TournamentK int // tournament size
	MaxRetries  int // max LLM retries on validation failure
	TopBotLimit int // number of top bots for meta description

	// Gate thresholds
	NashThreshold     float64 // Nash value threshold for promotion
	WinRateLowerBound float64 // Wilson CI lower bound threshold

	// Retirement
	RatingThreshold float64 // minimum display rating to keep
	PopCap          int     // max evolved bots in fleet

	// Timing
	CycleInterval           time.Duration // delay between cycles (0 = continuous)
	IslandCooldown          time.Duration // min time between same-island evolutions
	RetirementCheckInterval time.Duration // interval between periodic retirement checks

	// Infrastructure
	LLMURL         string
	RepoDir        string
	Registry       string
	KubectlServer  string
	EncryptionKey  string
	UseNsjail      bool
	LiveExportPath string
	UploadR2       bool

	// Declarative config for K8s manifests (§10.8)
	DeclarativeConfigRepo   string // git repo URL for K8s manifests
	DeclarativeConfigBranch string // git branch for K8s manifests

	// Languages to evolve (in priority order)
	Languages []string

	// PagesBaseURL is the Cloudflare Pages base URL for reading static indexes
	// such as community_hints.json. Empty disables community hint loading.
	PagesBaseURL string

	// Map evolution ticker (§14.6)
	MapEvolutionEnabled  bool           // whether to trigger weekly map evolution
	MapEvolutionSchedule WeeklySchedule // when to run map evolution
}

// WeeklySchedule configures when the weekly evolution run fires.
type WeeklySchedule struct {
	Weekday time.Weekday // 0=Sunday, 1=Monday, ..., 6=Saturday
	Hour    int          // 0-23 (UTC)
	Minute  int          // 0-59

	// PagesBaseURL is the Cloudflare Pages base URL for reading static indexes
	// such as community_hints.json. Empty disables community hint loading.
	PagesBaseURL string
}

// DefaultRunConfig returns production-ready defaults.
func DefaultRunConfig() RunConfig {
	return RunConfig{
		NumParents:              2,
		TournamentK:             3,
		MaxRetries:              2,
		TopBotLimit:             10,
		NashThreshold:           0.50,
		WinRateLowerBound:       0.40,
		RatingThreshold:         800.0,
		PopCap:                  50,
		CycleInterval:           5 * time.Minute,
		RetirementCheckInterval: 24 * time.Hour,
		IslandCooldown:          2 * time.Minute,
		LLMURL:                  envOrDefault("ACB_LLM_URL", "http://zai-proxy-apexalgo.tail1b1987.ts.net:8080"),
		RepoDir:                 envOrDefault("ACB_REPO_DIR", "."),
		Registry:                envOrDefault("ACB_REGISTRY", "forgejo.ardenone.com/ai-code-battle"),
		KubectlServer:           envOrDefault("ACB_KUBECTL_SERVER", "http://kubectl-ardenone-cluster:8001"),
		EncryptionKey:           os.Getenv("ACB_ENCRYPTION_KEY"),
		UseNsjail:               true,
		LiveExportPath:          envOrDefault("ACB_EVOLUTION_OUT", "evolution/live.json"),
		UploadR2:                envOrDefault("ACB_R2_UPLOAD_ENABLED", "false") == "true",
		DeclarativeConfigRepo:   envOrDefault("ACB_DECLARATIVE_CONFIG_REPO", "https://forgejo.ardenone.com/infra/ardenone-cluster.git"),
		DeclarativeConfigBranch: envOrDefault("ACB_DECLARATIVE_CONFIG_BRANCH", "main"),
		Languages:               []string{"go", "python", "rust", "typescript", "java", "php"},
		MapEvolutionEnabled:     envOrDefault("ACB_MAP_EVOLUTION_ENABLED", "false") == "true",
		MapEvolutionSchedule: WeeklySchedule{
			Weekday: time.Sunday, // Default: Sunday 03:00 UTC
			Hour:    3,
			Minute:  0,
		},
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
	CrossPollinated  int
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
	enableMapEvolution := fs.Bool("enable-map-evolution", false, "enable weekly map evolution ticker")

	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}

	// Initialize RNG
	rng := rand.New(rand.NewSource(*seed))
	if *seed == 0 {
		rng = rand.New(rand.NewSource(time.Now().UnixNano()))
	}

	// Start Prometheus metrics server
	metricsSrv := metrics.StartServer()
	defer metricsSrv.Close()

	// Open database
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer db.Close()

	// Initialize database schema and seed initial population if needed
	ctx = context.Background()
	if err := evolverdb.EnsureSchema(ctx, db); err != nil {
		log.Fatalf("ensure schema: %v", err)
	}

	// Seed initial population if programs table is empty
	store := evolverdb.NewStore(db)
	if inserted, err := evolverdb.SeedPopulation(ctx, store); err != nil {
		log.Fatalf("seed population: %v", err)
	} else if inserted > 0 {
		log.Printf("Seeded %d initial programs", inserted)
	} else {
		log.Println("Programs table already seeded")
	}

	// Load config from env with overrides
	cfg := DefaultRunConfig()
	if *singleLang != "" {
		cfg.Languages = []string{*singleLang}
	}
	if *enableMapEvolution {
		cfg.MapEvolutionEnabled = true
	}

	// Parse weekly schedule from env (format: "WEEKDAY:HH:MM" e.g., "0:03:00" for Sunday 03:00)
	if v := os.Getenv("ACB_MAP_EVOLUTION_SCHEDULE"); v != "" {
		var weekday, hour, minute int
		if _, err := fmt.Sscanf(v, "%d:%d:%d", &weekday, &hour, &minute); err == nil {
			if weekday >= 0 && weekday <= 6 && hour >= 0 && hour <= 23 && minute >= 0 && minute <= 59 {
				cfg.MapEvolutionSchedule.Weekday = time.Weekday(weekday)
				cfg.MapEvolutionSchedule.Hour = hour
				cfg.MapEvolutionSchedule.Minute = minute
			}
		}
	}

	// Track last evolution time per island for cooldown
	lastEvolved := make(map[string]time.Time)

	// Track per-island generation counters for cross-pollination boundary detection.
	// Load persisted state from DB so we don't re-trigger on restart.
	prevGens, err := store.LoadCrossPollState(ctx)
	if err != nil {
		log.Printf("warn: could not load cross-pollination state (starting fresh): %v", err)
		prevGens = make(map[string]int)
	}
	if *verbose {
		log.Printf("Cross-pollination state: %v", prevGens)
	}

	// Stats
	stats := RunStats{StartTime: time.Now()}

	// Shared cycle state for live observatory
	cycleState := live.NewCycleState()

	// Setup signal handling for graceful shutdown
	ctx, cancel := context.WithCancel(ctx)
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		log.Println("Received shutdown signal, finishing current cycle...")
		cancel()
	}()

	// Start periodic retirement ticker (§10.8)
	if cfg.RetirementCheckInterval > 0 {
		go startRetirementTicker(ctx, db, store, cfg, &stats, *verbose)
	}

	// Start weekly map evolution ticker (§14.6)
	if cfg.MapEvolutionEnabled {
		go startMapEvolutionTicker(ctx, db, cfg, *verbose)
	}

	langIdx := 0
	islandIdx := 0

	log.Printf("Evolution loop starting (continuous=%v, dry-run=%v)", *continuous, *dryRun)
	if *verbose {
		log.Printf("Config: nash=%.2f, win-lower=%.2f, max-retries=%d, languages=%v, retirement-check=%v",
			cfg.NashThreshold, cfg.WinRateLowerBound, cfg.MaxRetries, cfg.Languages, cfg.RetirementCheckInterval)
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
		cycleState.SetGeneration(stats.Cycles + 1)
		promoted, err := runCycle(ctx, db, store, island, lang, cfg, rng, *verbose, *dryRun, cycleState)
		if err != nil {
			log.Printf("Cycle failed: %v", err)
			stats.Errors++
			cycleState.SetPhase("idle")
			exportLive(ctx, db, cfg, *verbose, cycleState)
		}
		if promoted {
			stats.Promoted++
		}

		stats.Cycles++
		stats.Generated++
		metrics.EvolverGenerations.Inc()

		// Check cycle limit
		if *maxCycles > 0 && stats.Cycles >= *maxCycles {
			log.Printf("Reached max cycles (%d), stopping", *maxCycles)
			printStats(&stats)
			return
		}

		// Export live.json after each cycle
		cycleState.SetPhase("idle")
		exportLive(ctx, db, cfg, *verbose, cycleState)

		// Check for cross-pollination (§10.2: every 50 generations per island)
		cpChecker := crosspoll.NewChecker(store, llm.NewClient(cfg.LLMURL, ""), rng)
		cpResults, err := cpChecker.CheckAndPollinate(ctx, prevGens, *verbose)
		if err != nil {
			log.Printf("Cross-pollination check error: %v", err)
		}
		stats.CrossPollinated += len(cpResults)

		// Persist updated cross-pollination state so we don't re-trigger on restart.
		for isl, gen := range prevGens {
			if err := store.SaveCrossPollState(ctx, isl, gen); err != nil {
				log.Printf("warn: could not save crosspoll state for %s: %v", isl, err)
			}
		}

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
	island, lang string, cfg RunConfig, rng *rand.Rand, verbose, dryRun bool, cycleState *live.CycleState) (bool, error) {

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

	// Set up cycle state for live observatory
	cycleState.SetGeneration(generation)
	cycleState.SetPhase("generating")
	exportLiveQuiet(ctx, db, cfg, cycleState)

	// 5. Generate candidate with retry loop
	var programID int64
	var code string
	var program *evolverdb.Program
	var report *validator.Report

	// Build parent info for cycle state (with ratings)
	parentInfos := make([]string, len(parents))
	for i, p := range parents {
		parentInfos[i] = fmt.Sprintf("%s-%d", island, p.ID)
	}
	cycleState.SetCandidate(fmt.Sprintf("%s-%d", lang, generation), island, lang, parentInfos)

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
		cycleState.SetPhase("validating")
		exportLiveQuiet(ctx, db, cfg, cycleState)
		if err != nil {
			cycleState.SetValidationError("infrastructure", err.Error())
			log.Printf("Validation infrastructure error: %v", err)
			store.Delete(ctx, programID)
			programID = 0
			continue
		}

		// Track validation results in cycle state
		for _, stage := range report.Stages {
			timeMs := int(stage.Duration.Milliseconds())
			switch stage.Stage {
			case "syntax":
				cycleState.SetValidationSyntax(stage.Passed, timeMs)
			case "schema":
				cycleState.SetValidationSchema(stage.Passed, timeMs)
			case "smoke":
				cycleState.SetValidationSmoke(stage.Passed, timeMs)
			}
			if !stage.Passed && stage.Error != "" {
				cycleState.SetValidationError(string(stage.Stage), stage.Error)
			}
		}
		exportLiveQuiet(ctx, db, cfg, cycleState)
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
	cycleState.SetPhase("evaluating")
	exportLiveQuiet(ctx, db, cfg, cycleState)
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

	// Compute fitness (weighted combination of win rate and kill rate)
	wr := arena.ComputeFromResult(arenaResult)
	winRate := wr.Rate
	killRate := arenaResult.KillRate

	// Fitness = 70% win rate + 30% kill rate
	// This encourages combat aggression while still rewarding winning
	fitness := 0.7*winRate + 0.3*killRate

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
		log.Printf("  Arena result: %d W / %d L / %d D / %d err  win_rate=%.3f  kill_rate=%.3f (%d kills/%d matches)  fitness=%.3f",
			arenaResult.Wins, arenaResult.Losses, arenaResult.Draws, arenaResult.Errors,
			winRate, killRate, arenaResult.TotalKills, arenaResult.TotalMatches, fitness)
	}

	// 7. Load MAP-Elites grid and apply promotion gate
	grid := mapelites.New(10)
	promotedPrograms, _ := store.ListPromoted(ctx)
	for _, pp := range promotedPrograms {
		if len(pp.BehaviorVector) >= 2 {
			expl, form := 0.5, 0.5
			if len(pp.BehaviorVector) >= 4 {
				expl, form = pp.BehaviorVector[2], pp.BehaviorVector[3]
			}
			grid.TryPlace(pp.ProgramID, pp.Fitness, pp.BehaviorVector[0], pp.BehaviorVector[1], expl, form)
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

// estimateBehaviorVector analyzes code to estimate aggression/economy/exploration/formation behavior.
func estimateBehaviorVector(code, lang string) []float64 {
	// Default to balanced behavior
	aggression := 0.5
	economy := 0.5
	exploration := 0.5
	formation := 0.5

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

	// Exploration indicators
	explorationPatterns := []string{
		"explore", "scout", "scan", "discover", "map", "bfs", "visibility",
		"vision", "uncover", "spread",
	}
	explorationCount := 0
	for _, p := range explorationPatterns {
		explorationCount += strings.Count(codeLower, p)
	}

	// Formation indicators
	formationPatterns := []string{
		"formation", "group", "cluster", "cohesion", "together", "swarm",
		"center_of_mass", "rally", "merge", "assemble",
	}
	formationCount := 0
	for _, p := range formationPatterns {
		formationCount += strings.Count(codeLower, p)
	}

	// Normalize and adjust behavior vector
	total := aggressiveCount + economyCount + defensiveCount
	if total > 0 {
		aggression = float64(aggressiveCount) / float64(total)
		economy = float64(economyCount) / float64(total+1)
		if defensiveCount > aggressiveCount {
			aggression = aggression * 0.5
		}
	}

	// Exploration and formation have independent scaling
	if explorationCount > 0 {
		exploration = clamp(float64(explorationCount)/10.0, 0.1, 0.9)
	}
	if formationCount > 0 {
		formation = clamp(float64(formationCount)/10.0, 0.1, 0.9)
	}

	// Clamp to [0.1, 0.9] to avoid edge cases
	aggression = clamp(aggression, 0.1, 0.9)
	economy = clamp(economy, 0.1, 0.9)

	return []float64{aggression, economy, exploration, formation}
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
func exportLive(ctx context.Context, db *sql.DB, cfg RunConfig, verbose bool, cycleState *live.CycleState) {
	data, err := live.Export(ctx, db, cycleState)
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

// exportLiveQuiet is like exportLive but without verbose logging (for mid-cycle exports).
func exportLiveQuiet(ctx context.Context, db *sql.DB, cfg RunConfig, cycleState *live.CycleState) {
	data, err := live.Export(ctx, db, cycleState)
	if err != nil {
		return
	}
	_ = live.WriteFile(data, cfg.LiveExportPath)
	if cfg.UploadR2 {
		r2Cfg := live.R2ConfigFromEnv()
		if r2Cfg.HasCredentials() {
			r2Client, err := live.NewR2Client(r2Cfg)
			if err == nil {
				_ = r2Client.UploadLiveJSON(ctx, data)
			}
		}
	}
}

// startRetirementTicker runs periodic retirement checks (§10.8).
// This enforces the 7-day low-rating rule and 50-bot population cap.
func startRetirementTicker(ctx context.Context, db *sql.DB, store *evolverdb.Store, cfg RunConfig, stats *RunStats, verbose bool) {
	log.Printf("starting retirement ticker (every %s)", cfg.RetirementCheckInterval)
	ticker := time.NewTicker(cfg.RetirementCheckInterval)
	defer ticker.Stop()

	promCfg := promoter.DefaultConfig()
	promCfg.Registry = cfg.Registry
	promCfg.RepoDir = cfg.RepoDir
	promCfg.KubectlServer = cfg.KubectlServer
	promCfg.EncryptionKey = cfg.EncryptionKey
	promCfg.RatingThreshold = cfg.RatingThreshold
	promCfg.PopCap = cfg.PopCap
	promCfg.DeclarativeConfigRepo = cfg.DeclarativeConfigRepo
	promCfg.DeclarativeConfigBranch = cfg.DeclarativeConfigBranch

	p := promoter.New(store, db, promCfg)

	for {
		select {
		case <-ctx.Done():
			log.Printf("stopping retirement ticker")
			return
		case <-ticker.C:
			retired, err := p.EnforcePolicy(ctx)
			if err != nil {
				log.Printf("retirement ticker error: %v", err)
				continue
			}
			if len(retired) > 0 {
				stats.Retired += len(retired)
				for _, r := range retired {
					if verbose {
						log.Printf("  Retired %s (rating %.0f): %s", r.BotID, r.DisplayRating, r.Reason)
					}
				}
				log.Printf("retirement ticker: retired %d bot(s)", len(retired))
			}
		}
	}
}

// startMapEvolutionTicker runs weekly map evolution (§14.6).
// This triggers the acb-map-evolver to evolve maps based on engagement scores.
func startMapEvolutionTicker(ctx context.Context, db *sql.DB, cfg RunConfig, verbose bool) {
	schedule := cfg.MapEvolutionSchedule
	log.Printf("starting map evolution ticker (schedule: %s %02d:%02d UTC)",
		schedule.Weekday, schedule.Hour, schedule.Minute)

	// Calculate first scheduled run time
	nextRun := nextMapEvolutionTime(schedule)
	log.Printf("map evolution: first run scheduled for %s (in %v)",
		nextRun.Format(time.RFC3339), time.Until(nextRun).Round(time.Second))

	for {
		// Sleep until the scheduled time
		waitDuration := time.Until(nextRun)
		if waitDuration > 0 {
			select {
			case <-ctx.Done():
				log.Printf("stopping map evolution ticker")
				return
			case <-time.After(waitDuration):
			}
		}

		// Run map evolution
		log.Printf("map evolution: starting weekly map evolution run")
		if err := runMapEvolution(ctx, db, verbose); err != nil {
			log.Printf("map evolution: error: %v", err)
		} else {
			log.Printf("map evolution: weekly run complete")
		}

		// Calculate next scheduled run (7 days later)
		nextRun = nextRun.Add(7 * 24 * time.Hour)
		log.Printf("map evolution: next run scheduled for %s",
			nextRun.Format(time.RFC3339))

		// Check for cancellation before sleeping again
		select {
		case <-ctx.Done():
			log.Printf("stopping map evolution ticker")
			return
		default:
		}
	}
}

// nextMapEvolutionTime calculates the next occurrence of the map evolution schedule.
func nextMapEvolutionTime(schedule WeeklySchedule) time.Time {
	now := time.Now().UTC()

	// Start with today at the scheduled time
	scheduled := time.Date(now.Year(), now.Month(), now.Day(),
		schedule.Hour, schedule.Minute, 0, 0, time.UTC)

	// Check if we're on the correct weekday
	daysUntil := int(schedule.Weekday) - int(now.Weekday())
	if daysUntil < 0 {
		daysUntil += 7
	}

	// Add the days until the scheduled weekday
	scheduled = scheduled.AddDate(0, 0, daysUntil)

	// If the scheduled time has already passed today, move to next week
	if scheduled.Before(now) || scheduled.Equal(now) {
		scheduled = scheduled.Add(7 * 24 * time.Hour)
	}

	return scheduled
}

// runMapEvolution executes the map evolution by running the acb-map-evolver binary
// with the --once flag to trigger a single evolution run for all player counts.
func runMapEvolution(ctx context.Context, db *sql.DB, verbose bool) error {
	// Path to acb-map-evolver binary (built into same container)
	const mapEvolverBin = "/app/acb-map-evolver"

	// Verify binary exists
	if _, err := os.Stat(mapEvolverBin); err != nil {
		return fmt.Errorf("acb-map-evolver binary not found at %s: %w", mapEvolverBin, err)
	}

	// Prepare environment with database URL
	cmdEnv := append(os.Environ(),
		fmt.Sprintf("ACB_DATABASE_URL=%s", os.Getenv("ACB_DATABASE_URL")),
	)

	cmd := exec.CommandContext(ctx, mapEvolverBin, "--once")
	cmd.Env = cmdEnv
	if verbose {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		log.Printf("map evolution: executing %s --once", mapEvolverBin)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("acb-map-evolver failed: %w, output: %s", err, string(output))
	}

	if verbose && len(output) > 0 {
		log.Printf("map evolution: %s", string(output))
	}

	return nil
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
	log.Printf("  Cross-pollinated: %d", stats.CrossPollinated)
	log.Printf("  Errors: %d", stats.Errors)
	log.Printf("  Uptime: %v", elapsed.Round(time.Second))
}
