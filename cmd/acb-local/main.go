// Command acb-local runs a match between two local bots.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
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
	rows := flag.Int("rows", 60, "Grid rows")
	cols := flag.Int("cols", 60, "Grid columns")
	maxTurns := flag.Int("max-turns", 500, "Maximum turns")
	output := flag.String("output", "replay.json", "Output replay file")
	verbose := flag.Bool("verbose", false, "Verbose output")
	bot0Name := flag.String("bot0", "gatherer", "Bot 0 strategy (idle, random, gatherer, rusher, guardian, swarm, hunter)")
	bot1Name := flag.String("bot1", "rusher", "Bot 1 strategy (idle, random, gatherer, rusher, guardian, swarm, hunter)")
	listBots := flag.Bool("list-bots", false, "List available bot strategies")
	help := flag.Bool("help", false, "Show help")

	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage: acb-local [options]\n\n")
		fmt.Fprintf(flag.CommandLine.Output(), "Run a match between two local bots.\n\n")
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

	// Validate bot names
	bot0Factory, ok := availableBots[*bot0Name]
	if !ok {
		log.Fatalf("Unknown bot strategy: %s (use -list-bots to see available strategies)", *bot0Name)
	}

	bot1Factory, ok := availableBots[*bot1Name]
	if !ok {
		log.Fatalf("Unknown bot strategy: %s (use -list-bots to see available strategies)", *bot1Name)
	}

	// Create game config
	config := engine.DefaultConfig()
	config.Rows = *rows
	config.Cols = *cols
	config.MaxTurns = *maxTurns

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

	// Create bots with different seeds
	bot0 := bot0Factory(rng.Int63())
	bot1 := bot1Factory(rng.Int63())

	mr.AddBot(bot0, *bot0Name)
	mr.AddBot(bot1, *bot1Name)

	if *verbose {
		log.Printf("Starting match with seed %d", *seed)
		log.Printf("Bot 0: %s, Bot 1: %s", *bot0Name, *bot1Name)
		log.Printf("Config: %dx%d, max %d turns", config.Rows, config.Cols, config.MaxTurns)
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
	fmt.Printf("  Players: %s vs %s\n", *bot0Name, *bot1Name)
	fmt.Printf("  Winner: Player %d\n", result.Winner)
	fmt.Printf("  Reason: %s\n", result.Reason)
	fmt.Printf("  Turns: %d\n", result.Turns)
	fmt.Printf("  Scores: %v\n", result.Scores)
	if *output != "" {
		fmt.Printf("  Replay: %s\n", *output)
	}
}
