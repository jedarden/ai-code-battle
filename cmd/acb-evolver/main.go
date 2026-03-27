// acb-evolver manages the evolution pipeline programs database.
//
// Subcommands:
//
//	init-schema       Create or migrate the programs table in PostgreSQL
//	seed              Insert the 6 built-in strategy bots as generation-0 programs
//	stats             Print program counts per island
//	validate          Run the 3-stage validation pipeline on a bot source file
//	validation-stats  Show per-island validation pass-rate metrics
package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	_ "github.com/lib/pq"

	evolverdb "github.com/aicodebattle/acb/cmd/acb-evolver/internal/db"
	"github.com/aicodebattle/acb/cmd/acb-evolver/internal/validator"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: acb-evolver <init-schema|seed|stats|validate|validation-stats>")
		os.Exit(1)
	}

	dbURL := os.Getenv("ACB_DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://localhost:5432/acb?sslmode=disable"
	}

	ctx := context.Background()

	switch os.Args[1] {
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
		fmt.Fprintln(os.Stderr, "usage: acb-evolver <init-schema|seed|stats|validate|validation-stats>")
		os.Exit(1)
	}
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

func mustOpenDB(url string) *sql.DB {
	db, err := sql.Open("postgres", url)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	return db
}
