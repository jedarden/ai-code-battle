// Command acb-local runs a match between local bots.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"strings"
	"time"

	"github.com/aicodebattle/acb/engine"
)

// availableBots maps bot names to constructor functions.
var availableBots = map[string]func(int64) engine.BotInterface{
	"idle":     func(seed int64) engine.BotInterface { return engine.NewIdleBot() },
	"random":   func(seed int64) engine.BotInterface { return engine.NewRandomBot(seed) },
	"gatherer": func(seed int64) engine.BotInterface { return engine.NewGathererBot(seed) },
	"rusher":   func(seed int64) engine.BotInterface { return engine.NewRusherBot(seed) },
	"guardian": func(seed int64) engine.BotInterface { return engine.NewGuardianBot(seed) },
	"swarm":    func(seed int64) engine.BotInterface { return engine.NewSwarmBot(seed) },
	"hunter":   func(seed int64) engine.BotInterface { return engine.NewHunterBot(seed) },
}

func main() {
	// Command-line flags
	seed := flag.Int64("seed", time.Now().UnixNano(), "Random seed")
	rows := flag.Int("rows", 0, "Grid rows (0 = auto-scale for player count)")
	cols := flag.Int("cols", 0, "Grid columns (0 = auto-scale for player count)")
	maxTurns := flag.Int("max-turns", 0, "Maximum turns (0 = auto-scale)")
	coresPerPlayer := flag.Int("cores", 1, "Cores (bases) per player")
	output := flag.String("output", "replay.json", "Output replay file")
	verbose := flag.Bool("verbose", false, "Verbose output")
	botsFlag := flag.String("bots", "gatherer,rusher", "Comma-separated bot strategies (2-8 players)")
	listBots := flag.Bool("list-bots", false, "List available bot strategies")
	help := flag.Bool("help", false, "Show help")

	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage: acb-local [options]\n\n")
		fmt.Fprintf(flag.CommandLine.Output(), "Run a match between local bots (2-8 players).\n\n")
		fmt.Fprintf(flag.CommandLine.Output(), "Examples:\n")
		fmt.Fprintf(flag.CommandLine.Output(), "  acb-local -bots swarm,hunter                    # 2-player\n")
		fmt.Fprintf(flag.CommandLine.Output(), "  acb-local -bots swarm,hunter,gatherer,rusher     # 4-player\n")
		fmt.Fprintf(flag.CommandLine.Output(), "  acb-local -bots swarm,hunter,gatherer,rusher,guardian,random -cores 2  # 6-player, 2 bases each\n\n")
		fmt.Fprintf(flag.CommandLine.Output(), "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(flag.CommandLine.Output(), "\nAvailable bot strategies:\n")
		for name := range availableBots {
			fmt.Fprintf(flag.CommandLine.Output(), "  %s\n", name)
		}
	}

	flag.Parse()

	if *help {
		flag.Usage()
		os.Exit(0)
	}

	if *listBots {
		fmt.Println("Available bot strategies:")
		for name := range availableBots {
			fmt.Printf("  %s\n", name)
		}
		os.Exit(0)
	}

	// Parse bot list
	botNames := strings.Split(*botsFlag, ",")
	for i := range botNames {
		botNames[i] = strings.TrimSpace(botNames[i])
	}

	if len(botNames) < 2 {
		log.Fatal("Need at least 2 bots. Use -bots gatherer,rusher")
	}
	if len(botNames) > 8 {
		log.Fatal("Maximum 8 players supported")
	}

	// Validate bot names
	factories := make([]func(int64) engine.BotInterface, len(botNames))
	for i, name := range botNames {
		f, ok := availableBots[name]
		if !ok {
			log.Fatalf("Unknown bot strategy: %s (use -list-bots to see available)", name)
		}
		factories[i] = f
	}

	// Create config scaled for player count
	numPlayers := len(botNames)
	config := engine.ConfigForPlayers(numPlayers, *coresPerPlayer)

	// Override with explicit flags if provided
	if *rows > 0 {
		config.Rows = *rows
	}
	if *cols > 0 {
		config.Cols = *cols
	}
	if *maxTurns > 0 {
		config.MaxTurns = *maxTurns
	}

	// Create random source
	rng := rand.New(rand.NewSource(*seed))

	// Create match runner
	opts := []engine.MatchOption{
		engine.WithRNG(rng),
		engine.WithVerbose(*verbose),
	}
	if *verbose {
		opts = append(opts, engine.WithLogger(log.New(os.Stderr, "[acb] ", log.LstdFlags)))
	}

	mr := engine.NewMatchRunner(config, opts...)

	// Add bots
	for i, factory := range factories {
		bot := factory(rng.Int63())
		mr.AddBot(bot, botNames[i])
		_ = i
	}

	if *verbose {
		log.Printf("Starting match: %s", strings.Join(botNames, " vs "))
		log.Printf("Seed: %d, Grid: %dx%d, MaxTurns: %d, Cores/player: %d",
			*seed, config.Rows, config.Cols, config.MaxTurns, config.CoresPerPlayer)
	}

	// Run the match
	result, replay, err := mr.Run()
	if err != nil {
		log.Fatalf("Match failed: %v", err)
	}

	// Write replay to file
	if *output != "" {
		replayData, err := json.MarshalIndent(replay, "", "  ")
		if err != nil {
			log.Fatalf("Failed to marshal replay: %v", err)
		}
		if err := os.WriteFile(*output, replayData, 0644); err != nil {
			log.Fatalf("Failed to write replay: %v", err)
		}
		if *verbose {
			log.Printf("Replay written to %s", *output)
		}
	}

	// Print result
	fmt.Printf("Match complete!\n")
	fmt.Printf("  Players: %s\n", strings.Join(botNames, " vs "))
	fmt.Printf("  Grid: %dx%d (%d tiles), Cores: %d/player\n", config.Rows, config.Cols, config.Rows*config.Cols, config.CoresPerPlayer)
	fmt.Printf("  Winner: Player %d (%s)\n", result.Winner, botNames[result.Winner])
	fmt.Printf("  Reason: %s\n", result.Reason)
	fmt.Printf("  Turns: %d\n", result.Turns)
	fmt.Printf("  Scores: %v\n", result.Scores)
	if *output != "" {
		fmt.Printf("  Replay: %s\n", *output)
	}
}
