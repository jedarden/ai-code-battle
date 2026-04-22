// acb-evolver manages the evolution pipeline programs database.
//
// Subcommands:
//
//	init-schema       Create or migrate the programs table in PostgreSQL
//	seed              Insert the 6 built-in strategy bots as generation-0 programs
//	stats             Print program counts per island
//	validate          Run the 3-stage validation pipeline on a bot source file
//	validation-stats  Show per-island validation pass-rate metrics
//	evaluate          Run the 10-match arena tournament and apply the promotion gate
//	retire            Enforce retirement policy (rating threshold + population cap)
//	live-export       Export evolution state to live.json for the dashboard
package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"strings"
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
	"github.com/aicodebattle/acb/cmd/acb-evolver/internal/replay"
	"github.com/aicodebattle/acb/cmd/acb-evolver/internal/selector"
	"github.com/aicodebattle/acb/cmd/acb-evolver/internal/validator"
	"github.com/aicodebattle/acb/engine"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: acb-evolver <init-schema|seed|stats|validate|validation-stats|evolve|run|evaluate|retire|live-export>")
		os.Exit(1)
	}

	dbURL := os.Getenv("ACB_DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://localhost:5432/acb?sslmode=disable"
	}

	ctx := context.Background()

	switch os.Args[1] {
	case "run":
		RunEvolutionLoop(ctx, dbURL, os.Args[2:])

	case "live-export":
		db := mustOpenDB(dbURL)
		defer db.Close()
		runLiveExport(ctx, db, os.Args[2:])

	case "evolve":
		runEvolve(ctx, dbURL, os.Args[2:])

	case "evaluate":
		db := mustOpenDB(dbURL)
		defer db.Close()
		runEvaluate(ctx, db, os.Args[2:])

	case "retire":
		db := mustOpenDB(dbURL)
		defer db.Close()
		runRetire(ctx, db, os.Args[2:])

	case "init-schema":
		db := mustOpenDB(dbURL)
		defer db.Close()
		if err := evolverdb.EnsureSchema(ctx, db); err != nil {
			log.Fatalf("init-schema: %v", err)
		}
		log.Println("schema ready")

	case "seed":
		db := mustOpenDB(dbURL)
		defer db.Close()
		store := evolverdb.NewStore(db)
		if err := evolverdb.EnsureSchema(ctx, db); err != nil {
			log.Fatalf("ensure schema: %v", err)
		}
		n, err := evolverdb.SeedPopulation(ctx, store)
		if err != nil {
			log.Fatalf("seed: %v", err)
		}
		if n == 0 {
			log.Println("programs table already seeded; nothing to do")
		} else {
			log.Printf("seeded %d programs", n)
		}

	case "stats":
		db := mustOpenDB(dbURL)
		defer db.Close()
		store := evolverdb.NewStore(db)
		counts, err := store.CountByIsland(ctx)
		if err != nil {
			log.Fatalf("stats: %v", err)
		}
		total := 0
		for _, island := range evolverdb.AllIslands {
			n := counts[island]
			total += n
			fmt.Printf("  %-8s %d\n", island, n)
		}
		fmt.Printf("  %-8s %d\n", "total", total)

	case "validate":
		runValidate(ctx, dbURL, os.Args[2:])

	case "validation-stats":
		db := mustOpenDB(dbURL)
		defer db.Close()
		store := evolverdb.NewStore(db)
		runValidationStats(ctx, store)

	default:
		fmt.Fprintf(os.Stderr, "unknown subcommand %q\n", os.Args[1])
		fmt.Fprintln(os.Stderr, "usage: acb-evolver <init-schema|seed|stats|validate|validation-stats|evolve|evaluate|retire|live-export>")
		os.Exit(1)
	}
}

// runEvolve generates a new candidate bot using the LLM ensemble.
//
//	evolve -island alpha -lang go [-replay file.json] [-llm-url URL] [-num-parents 2] [-seed N] [-out file.go]
func runEvolve(ctx context.Context, dbURL string, args []string) {
	fs := flag.NewFlagSet("evolve", flag.ExitOnError)
	island := fs.String("island", "", "island name (alpha|beta|gamma|delta) [required]")
	lang := fs.String("lang", "", "target language (go|python|rust|typescript|java|php) [required]")
	replayFile := fs.String("replay", "", "optional replay JSON file for analysis (can be specified multiple times as comma-separated)")
	llmURL := fs.String("llm-url", envOrDefault("ACB_LLM_URL", "http://zai-proxy-apexalgo.tail1b1987.ts.net:8080"), "LLM proxy URL")
	numParents := fs.Int("num-parents", 2, "number of parents to select via tournament selection")
	tournamentK := fs.Int("tournament-k", 3, "tournament size for parent selection")
	seed := fs.Int64("seed", 0, "random seed (0 = use time)")
	topBotLimit := fs.Int("top-bots", 10, "number of top bots to include in meta description")
	outFile := fs.String("out", "", "output file for generated code (default: stdout)")
	verbose := fs.Bool("v", false, "verbose output")

	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}
	if *island == "" {
		fmt.Fprintln(os.Stderr, "evolve: -island is required")
		fs.Usage()
		os.Exit(1)
	}
	if *lang == "" {
		fmt.Fprintln(os.Stderr, "evolve: -lang is required")
		fs.Usage()
		os.Exit(1)
	}

	// Validate island
	validIsland := false
	for _, i := range evolverdb.AllIslands {
		if i == *island {
			validIsland = true
			break
		}
	}
	if !validIsland {
		fmt.Fprintf(os.Stderr, "evolve: invalid island %q (must be one of: alpha, beta, gamma, delta)\n", *island)
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

	// 1. Load programs from the island
	if *verbose {
		log.Printf("Loading programs from island %s...", *island)
	}
	programs, err := store.ListByIsland(ctx, *island)
	if err != nil {
		log.Fatalf("list programs: %v", err)
	}
	if len(programs) == 0 {
		log.Fatalf("no programs found on island %s - seed the database first", *island)
	}
	if *verbose {
		log.Printf("Found %d programs on island %s", len(programs), *island)
	}

	// 2. Select parents via tournament selection
	if *verbose {
		log.Printf("Selecting %d parents via %d-way tournament selection...", *numParents, *tournamentK)
	}
	parents := selector.SelectParents(programs, *numParents, *tournamentK, rng)
	if *verbose {
		for i, p := range parents {
			log.Printf("  Parent %d: id=%d fitness=%.3f lang=%s", i+1, p.ID, p.Fitness, p.Language)
		}
	}

	// 3. Load and analyze replays (if provided)
	var analyses []*replay.Analysis
	if *replayFile != "" {
		analyzer := replay.NewAnalyzer()
		replayFiles := strings.Split(*replayFile, ",")
		for _, rf := range replayFiles {
			rf = strings.TrimSpace(rf)
			if rf == "" {
				continue
			}
			if *verbose {
				log.Printf("Loading replay: %s", rf)
			}
			rep, err := engine.LoadReplayFile(rf)
			if err != nil {
				log.Printf("warn: failed to load replay %s: %v", rf, err)
				continue
			}
			analysis := analyzer.Analyze(rep)
			if analysis != nil {
				analyses = append(analyses, analysis)
			}
		}
		if *verbose {
			log.Printf("Analyzed %d replays", len(analyses))
		}
	}

	// 4. Build meta description
	if *verbose {
		log.Printf("Building meta description...")
	}
	metaBuilder := meta.NewBuilder(store)
	metaDesc, err := metaBuilder.Build(ctx, *topBotLimit)
	if err != nil {
		log.Printf("warn: failed to build meta description: %v", err)
		// Create a minimal meta description
		metaDesc = &meta.Description{
			TotalBots:   len(programs),
			IslandStats: make(map[string]meta.IslandStats),
		}
	}

	// 5. Determine generation number (max generation on island + 1)
	maxGen := 0
	for _, p := range programs {
		if p.Generation > maxGen {
			maxGen = p.Generation
		}
	}
	generation := maxGen + 1

	// 6. Assemble the prompt
	if *verbose {
		log.Printf("Assembling prompt for generation %d...", generation)
	}
	req := prompt.BuildRequest(parents, analyses, metaDesc, *island, *lang, generation)
	fullPrompt := prompt.Assemble(req)

	if *verbose {
		log.Printf("Prompt length: %d bytes", len(fullPrompt))
	}

	// 7. Create LLM client and run ensemble
	if *verbose {
		log.Printf("Connecting to LLM at %s...", *llmURL)
	}
	client := llm.NewClient(*llmURL, "")

	cfg := llm.DefaultEnsembleConfig()
	cfg.NumCandidates = 3
	cfg.RefineTop = true

	if *verbose {
		log.Printf("Running ensemble generation (%d candidates, refinement enabled)...", cfg.NumCandidates)
	}

	result, err := client.Ensemble(ctx, fullPrompt, *lang, cfg)
	if err != nil {
		log.Fatalf("LLM ensemble failed: %v", err)
	}

	if result.Best == nil {
		log.Fatal("No valid candidate generated")
	}

	// 8. Output the result
	if *verbose {
		log.Printf("Generation complete!")
		log.Printf("  Candidates generated: %d", len(result.AllCandidates))
		log.Printf("  Refinement applied: %v", result.RefinementApplied)
		log.Printf("  Best candidate length: %d bytes", len(result.Best.Code))
		if len(result.Errors) > 0 {
			log.Printf("  Errors: %d", len(result.Errors))
		}
	}

	if *outFile != "" {
		if err := os.WriteFile(*outFile, []byte(result.Best.Code), 0644); err != nil {
			log.Fatalf("write output file: %v", err)
		}
		if *verbose {
			log.Printf("Wrote candidate to %s", *outFile)
		}
	} else {
		fmt.Print(result.Best.Code)
	}
}

// runEvaluate runs the 10-match mini-tournament and applies the promotion gate.
//
//	evaluate -lang go -island alpha [-program-id 0] [-promote] [-nash 0.5] [-win-lower 0.4] [-nolog] <file>
func runEvaluate(ctx context.Context, db *sql.DB, args []string) {
	fs := flag.NewFlagSet("evaluate", flag.ExitOnError)
	lang := fs.String("lang", "", "bot language (go|python|rust|typescript|java|php) [required]")
	programID := fs.Int64("program-id", 0, "programs.id to update fitness after evaluation (0 = skip)")
	doPromote := fs.Bool("promote", false, "promote the candidate if the gate passes")
	nashThreshold := fs.Float64("nash", 0.50, "Nash value threshold for promotion")
	winLower := fs.Float64("win-lower", 0.40, "Wilson CI lower-bound threshold (0 to disable)")
	nolog := fs.Bool("nolog", false, "skip writing validation result to DB")

	// Promoter flags (used only when -promote is set)
	repoDir := fs.String("repo-dir", envOrDefault("ACB_REPO_DIR", "."), "git repo root for K8s manifests")
	registry := fs.String("registry", envOrDefault("ACB_REGISTRY", "forgejo.ardenone.com/ai-code-battle"), "container registry")
	kubectlServer := fs.String("kubectl-server", envOrDefault("ACB_KUBECTL_SERVER", "http://kubectl-ardenone-cluster:8001"), "kubectl API server URL")
	encKey := fs.String("enc-key", os.Getenv("ACB_ENCRYPTION_KEY"), "AES-256-GCM encryption key (hex) for bots table")

	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}
	if *lang == "" {
		fmt.Fprintln(os.Stderr, "evaluate: -lang is required")
		fs.Usage()
		os.Exit(1)
	}
	if fs.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "evaluate: file argument is required")
		fs.Usage()
		os.Exit(1)
	}

	code, err := os.ReadFile(fs.Arg(0))
	if err != nil {
		log.Fatalf("read file: %v", err)
	}

	store := evolverdb.NewStore(db)

	// Pre-populate MAP-Elites grid from existing promoted programs so the gate
	// can detect niche collisions against the current population.
	const gridSize = 10
	grid := mapelites.New(gridSize)
	if promoted, err := store.ListPromoted(ctx); err == nil {
		for _, pp := range promoted {
			if len(pp.BehaviorVector) >= 2 {
				expl, form := 0.5, 0.5
					if len(pp.BehaviorVector) >= 4 {
						expl, form = pp.BehaviorVector[2], pp.BehaviorVector[3]
					}
					grid.TryPlace(pp.ProgramID, pp.Fitness, pp.BehaviorVector[0], pp.BehaviorVector[1], expl, form)
			}
		}
	}

	// Run the arena tournament.
	arenaCfg := arena.DefaultConfig()
	a := arena.New(db, arenaCfg)

	fmt.Printf("evaluate: running %d-match tournament for %s bot…\n", arena.DefaultNumMatches, *lang)
	result, err := a.Run(ctx, string(code), *lang)
	if err != nil {
		log.Fatalf("arena: %v", err)
	}

	// Print match summary.
	total := result.Wins + result.Losses + result.Draws
	fmt.Printf("\nTournament result: %d W / %d L / %d D / %d err  (total=%d)\n",
		result.Wins, result.Losses, result.Draws, result.Errors, total)
	wr := arena.ComputeFromResult(result)
	fmt.Printf("Win rate: %.3f  (95%% CI %.3f–%.3f)\n", wr.Rate, wr.Lower, wr.Upper)

	nash := arena.ComputeNash(result.WinRateVec)
	fmt.Printf("Nash value (PSRO): %.3f  (opponent mix: %v)\n", nash.NashValue, nash.WinRatePerOpponent)

	// Compute fitness as overall win rate.
	fitness := wr.Rate

	// Look up the program if -program-id was given.
	var program *evolverdb.Program
	if *programID > 0 {
		program, err = store.Get(ctx, *programID)
		if err != nil {
			log.Fatalf("get program %d: %v", *programID, err)
		}
		if program == nil {
			log.Fatalf("program %d not found", *programID)
		}
		// Update fitness in DB.
		if !*nolog {
			if err := store.UpdateFitness(ctx, *programID, fitness, program.BehaviorVector); err != nil {
				log.Printf("warn: update fitness: %v", err)
			} else {
				fmt.Printf("Updated program %d fitness to %.3f\n", *programID, fitness)
			}
		}
	}

	// Apply the promotion gate.
	gateCfg := arena.GateConfig{
		NashThreshold:     *nashThreshold,
		WinRateLowerBound: *winLower,
	}
	gate := arena.NewGate(gateCfg, grid)

	var behaviorVec []float64
	if program != nil {
		behaviorVec = program.BehaviorVector
	}
	gateResult := gate.Evaluate(result, *programID, fitness, behaviorVec)

	fmt.Printf("\nGate: %s\n", gateResult.Reason)
	fmt.Printf("MAP-Elites: placed=%v improved=%v cell=[%d,%d]\n",
		gateResult.MapElitesPlaced, gateResult.MapElitesImproved,
		gateResult.Placement.X, gateResult.Placement.Y)

	if !gateResult.Promoted {
		fmt.Println("Decision: REJECTED")
		return
	}

	fmt.Println("Decision: PROMOTED")

	if !*doPromote {
		fmt.Println("(pass -promote to execute deployment)")
		return
	}
	if program == nil {
		log.Fatalf("promote: -program-id is required when -promote is set")
	}

	promCfg := promoter.DefaultConfig()
	promCfg.Registry = *registry
	promCfg.RepoDir = *repoDir
	promCfg.KubectlServer = *kubectlServer
	promCfg.EncryptionKey = *encKey

	p := promoter.New(store, db, promCfg)
	res, err := p.Promote(ctx, program)
	if err != nil {
		log.Fatalf("promote: %v", err)
	}
	fmt.Printf("Promoted: bot_name=%s bot_id=%s endpoint=%s\n", res.BotName, res.BotID, res.Endpoint)
}

// runRetire enforces the retirement policy (rating threshold + population cap).
//
//	retire [-threshold 1000] [-cap 50] [-dry-run] [-kubectl-server URL]
func runRetire(ctx context.Context, db *sql.DB, args []string) {
	fs := flag.NewFlagSet("retire", flag.ExitOnError)
	threshold := fs.Float64("threshold", 1000.0, "minimum display rating (mu-2*phi) to keep a bot")
	cap := fs.Int("cap", 50, "maximum number of simultaneously promoted evolved bots")
	dryRun := fs.Bool("dry-run", false, "print what would be retired without making changes")
	repoDir := fs.String("repo-dir", envOrDefault("ACB_REPO_DIR", "."), "git repo root")
	registry := fs.String("registry", envOrDefault("ACB_REGISTRY", "forgejo.ardenone.com/ai-code-battle"), "container registry")
	kubectlServer := fs.String("kubectl-server", envOrDefault("ACB_KUBECTL_SERVER", "http://kubectl-ardenone-cluster:8001"), "kubectl API server URL")
	encKey := fs.String("enc-key", os.Getenv("ACB_ENCRYPTION_KEY"), "AES-256-GCM encryption key (hex)")

	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}

	store := evolverdb.NewStore(db)

	promCfg := promoter.DefaultConfig()
	promCfg.RatingThreshold = *threshold
	promCfg.PopCap = *cap
	promCfg.RepoDir = *repoDir
	promCfg.Registry = *registry
	promCfg.KubectlServer = *kubectlServer
	promCfg.EncryptionKey = *encKey

	if *dryRun {
		// Simulate by temporarily setting an impossible cap to list candidates.
		fmt.Println("retire: dry-run mode — no changes will be made")
	}

	p := promoter.New(store, db, promCfg)

	if *dryRun {
		// Read-only preview using the same DB query logic without executing retirements.
		rows, err := db.QueryContext(ctx, `
			SELECT p.id, p.bot_id, COALESCE(p.bot_name, ''),
			       b.rating_mu - 2*b.rating_phi AS display_rating
			FROM programs p
			JOIN bots b ON p.bot_id = b.bot_id
			WHERE p.promoted = TRUE AND p.bot_id IS NOT NULL
			  AND b.status = 'active' AND b.owner = 'acb-evolver'
			ORDER BY display_rating ASC`)
		if err != nil {
			log.Fatalf("query: %v", err)
		}
		defer rows.Close()
		type row struct {
			programID     int64
			botID, botName string
			displayRating float64
		}
		var bots []row
		for rows.Next() {
			var r row
			if err := rows.Scan(&r.programID, &r.botID, &r.botName, &r.displayRating); err != nil {
				log.Fatalf("scan: %v", err)
			}
			bots = append(bots, r)
		}
		_ = rows.Err()

		remaining := len(bots)
		fmt.Printf("Active evolved bots: %d  (threshold=%.0f cap=%d)\n", remaining, *threshold, *cap)
		for _, b := range bots {
			var why string
			if b.displayRating < *threshold {
				why = fmt.Sprintf("rating %.0f < threshold", b.displayRating)
			} else if remaining > *cap {
				why = "over cap"
			}
			mark := "  keep"
			if why != "" {
				mark = "  RETIRE"
				remaining--
			}
			fmt.Printf("%s  bot_id=%-12s bot_name=%-20s rating=%.0f  %s\n",
				mark, b.botID, b.botName, b.displayRating, why)
		}
		return
	}

	retired, err := p.EnforcePolicy(ctx)
	if err != nil {
		log.Fatalf("enforce policy: %v", err)
	}

	if len(retired) == 0 {
		fmt.Println("retire: nothing to retire")
		return
	}
	fmt.Printf("retire: retired %d bot(s):\n", len(retired))
	for _, r := range retired {
		fmt.Printf("  bot_id=%-12s bot_name=%-20s rating=%.0f  reason=%s\n",
			r.BotID, r.BotName, r.DisplayRating, r.Reason)
	}
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// runValidate parses flags, runs the three-stage validation pipeline on a bot
// source file, and optionally logs the result to the database.
//
//	validate -lang go [-island alpha] [-nsjail] <file>
func runValidate(ctx context.Context, dbURL string, args []string) {
	fs := flag.NewFlagSet("validate", flag.ExitOnError)
	lang := fs.String("lang", "", "bot language (go|python|rust|typescript|java|php) [required]")
	island := fs.String("island", "alpha", "island name for DB logging (alpha|beta|gamma|delta)")
	useNsjail := fs.Bool("nsjail", false, "wrap sandbox in nsjail (requires nsjail in PATH)")
	nolog := fs.Bool("nolog", false, "skip writing result to the database")
	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}

	if *lang == "" {
		fmt.Fprintln(os.Stderr, "validate: -lang is required")
		fs.Usage()
		os.Exit(1)
	}
	if fs.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "validate: file argument is required")
		fs.Usage()
		os.Exit(1)
	}

	filePath := fs.Arg(0)
	code, err := os.ReadFile(filePath)
	if err != nil {
		log.Fatalf("read file: %v", err)
	}

	cfg := validator.DefaultConfig()
	cfg.UseNsjail = *useNsjail

	report, err := validator.Validate(ctx, string(code), *lang, "", cfg)
	if err != nil {
		log.Fatalf("validate: %v", err)
	}

	printReport(report, filePath)

	// Persist the result unless -nolog was set.
	if !*nolog {
		db, err := sql.Open("postgres", dbURL)
		if err != nil {
			log.Printf("warn: could not open DB for logging (%v) — skipping", err)
		} else {
			defer db.Close()
			store := evolverdb.NewStore(db)
			entry := &evolverdb.ValidationLog{
				Island:    *island,
				Language:  *lang,
				Stage:     string(report.LastStage()),
				Passed:    report.Passed,
				LLMOutput: report.LLMOutput,
			}
			if !report.Passed {
				for _, sr := range report.Stages {
					if !sr.Passed {
						entry.ErrorText = sr.Error
						break
					}
				}
			}
			if logErr := store.RecordValidation(ctx, entry); logErr != nil {
				log.Printf("warn: DB log failed: %v", logErr)
			}
		}
	}

	if !report.Passed {
		os.Exit(1)
	}
}

// printReport prints a human-readable summary of the validation report.
func printReport(r *validator.Report, src string) {
	fmt.Printf("Validation report for %s (%s)\n", src, r.Language)
	fmt.Println(strings.Repeat("─", 50))
	for _, sr := range r.Stages {
		status := "PASS"
		if !sr.Passed {
			status = "FAIL"
		}
		fmt.Printf("  %-8s %s  (%s)\n", sr.Stage, status, sr.Duration.Round(1000000))
		if !sr.Passed && sr.Error != "" {
			fmt.Printf("           %s\n", sr.Error)
		}
	}
	fmt.Println(strings.Repeat("─", 50))
	if r.Passed {
		fmt.Println("  Result:  PASSED — all 3 stages OK")
	} else {
		fmt.Printf("  Result:  FAILED at stage %q\n", r.LastStage())
	}
}

// runValidationStats queries and prints per-island validation statistics.
func runValidationStats(ctx context.Context, store *evolverdb.Store) {
	stats, err := store.IslandValidationStats(ctx)
	if err != nil {
		log.Fatalf("validation-stats: %v", err)
	}
	if len(stats) == 0 {
		fmt.Println("no validation records found")
		return
	}

	fmt.Printf("%-8s  %6s  %6s  %7s  %7s  %7s  %7s\n",
		"island", "total", "passed", "rate", "!syntax", "!schema", "!sandbox")
	fmt.Println(strings.Repeat("─", 66))
	for _, v := range stats {
		fmt.Printf("%-8s  %6d  %6d  %6.1f%%  %7d  %7d  %7d\n",
			v.Island, v.Total, v.Passed, v.PassRate*100,
			v.ByStage["syntax"], v.ByStage["schema"], v.ByStage["sandbox"])
	}
}

// runLiveExport exports the current evolution state to live.json.
//
//	live-export [-out evolution/live.json] [-r2] [-r2-only]
func runLiveExport(ctx context.Context, db *sql.DB, args []string) {
	fs := flag.NewFlagSet("live-export", flag.ExitOnError)
	out := fs.String("out", envOrDefault("ACB_EVOLUTION_OUT", "evolution/live.json"), "output file path")
	uploadR2 := fs.Bool("r2", false, "upload to R2 in addition to writing local file")
	r2Only := fs.Bool("r2-only", false, "upload to R2 only, skip local file")
	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}

	data, err := live.Export(ctx, db)
	if err != nil {
		log.Fatalf("live-export: %v", err)
	}

	// Write local file unless r2-only is set
	if !*r2Only {
		if err := live.WriteFile(data, *out); err != nil {
			log.Fatalf("live-export write: %v", err)
		}
		log.Printf("live-export: wrote %d programs (%d promoted) to %s",
			data.TotalPrograms, data.PromotedCount, *out)
	}

	// Upload to R2 if requested
	if *uploadR2 || *r2Only {
		r2Cfg := live.R2ConfigFromEnv()
		if !r2Cfg.HasCredentials() {
			log.Fatalf("live-export: R2 credentials not configured (set ACB_R2_ACCESS_KEY, ACB_R2_SECRET_KEY, ACB_R2_ENDPOINT, ACB_R2_BUCKET)")
		}
		r2Client, err := live.NewR2Client(r2Cfg)
		if err != nil {
			log.Fatalf("live-export: create R2 client: %v", err)
		}
		if err := r2Client.UploadLiveJSON(ctx, data); err != nil {
			log.Fatalf("live-export: upload to R2: %v", err)
		}
		log.Printf("live-export: uploaded to R2 at evolution/live.json (%d programs)", data.TotalPrograms)
	}
}

func mustOpenDB(url string) *sql.DB {
	db, err := sql.Open("postgres", url)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	return db
}
