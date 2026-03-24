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

func main() {
	// Command-line flags
	seed := flag.Int64("seed", time.Now().UnixNano(), "Random seed")
	rows := flag.Int("rows", 60, "Grid rows")
	cols := flag.Int("cols", 60, "Grid columns")
	maxTurns := flag.Int("max-turns", 500, "Maximum turns")
	output := flag.String("output", "replay.json", "Output replay file")
	verbose := flag.Bool("verbose", false, "Verbose output")
	help := flag.Bool("help", false, "Show help")

	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage: acb-local [options]\n\n")
		fmt.Fprintf(flag.CommandLine.Output(), "Run a match between two local bots (using stdin/stdout).\n\n")
		fmt.Fprintf(flag.CommandLine.Output(), "The game state is sent to each bot via stdout, and moves are read from stdin.\n")
		fmt.Fprintf(flag.CommandLine.Output(), "Bots should be implemented as separate processes that communicate via pipes.\n\n")
		fmt.Fprintf(flag.CommandLine.Output(), "Options:\n")
		flag.PrintDefaults()
	}

	flag.Parse()

	if *help {
		flag.Usage()
		os.Exit(0)
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

	// For Phase 1, we use idle bots as placeholders
	// In a real scenario, these would be external processes communicating via pipes
	// For testing the engine, we'll use two random bots
	bot0 := engine.NewRandomBot(rng.Int63())
	bot1 := engine.NewRandomBot(rng.Int63())

	mr.AddBot(bot0, "RandomBot1")
	mr.AddBot(bot1, "RandomBot2")

	if *verbose {
		log.Printf("Starting match with seed %d", *seed)
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
	fmt.Printf("  Winner: Player %d\n", result.Winner)
	fmt.Printf("  Reason: %s\n", result.Reason)
	fmt.Printf("  Turns: %d\n", result.Turns)
	fmt.Printf("  Scores: %v\n", result.Scores)
	if *output != "" {
		fmt.Printf("  Replay: %s\n", *output)
	}
}
