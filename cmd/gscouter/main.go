// Command gscouter scores a team build against a library rotation.
//
// Usage:
//
//	gscouter -team teams/skirk-yelan-furina-escoffier -build teams/skirk-yelan-furina-escoffier/build.txt
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/cenoda/gscouter/internal/config"
	"github.com/cenoda/gscouter/internal/library"
	"github.com/cenoda/gscouter/internal/score"
)

func main() {
	var (
		teamDir    = flag.String("team", "", "team library directory")
		buildPath  = flag.String("build", "", "build file (gcsim character setup lines)")
		iterations = flag.Int("iter", 1000, "number of iterations")
		warmup     = flag.Float64("warmup", 300, "seconds excluded from the score (T1)")
		duration   = flag.Float64("duration", 900, "fight duration in seconds (T2)")
		seed       = flag.Int64("seed", 1, "seed of the first iteration")
		dump       = flag.Bool("dump", false, "print the assembled gcsim config and exit")
	)
	flag.Parse()
	if *teamDir == "" || *buildPath == "" {
		flag.Usage()
		os.Exit(2)
	}
	if err := run(*teamDir, *buildPath, *iterations, *warmup, *duration, *seed, *dump); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(teamDir, buildPath string, iterations int, warmup, duration float64, seed int64, dump bool) error {
	team, err := library.Load(teamDir)
	if err != nil {
		return err
	}
	raw, err := os.ReadFile(buildPath)
	if err != nil {
		return err
	}
	if team.Status == library.StatusBlocked {
		return fmt.Errorf("team %q is blocked: %s", team.ID, team.Notes)
	}

	cfg := config.Assemble(config.Conditions{Duration: duration, Iterations: iterations}, string(raw), team.Rotation)
	chars, err := config.Characters(cfg)
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}
	if !team.Matches(chars) {
		return fmt.Errorf("build characters %v do not match team %q %v", chars, team.ID, team.Members)
	}
	if dump {
		fmt.Print(cfg)
		return nil
	}

	start := time.Now()
	res, err := score.Run(context.Background(), cfg, score.Options{
		Iterations: iterations,
		Warmup:     warmup,
		Seed:       seed,
	})
	if err != nil {
		return err
	}

	fmt.Printf("Team:      %s (rotation v%d, %s)\n", team.Name, team.RotationVersion, team.Status)
	fmt.Printf("Score:     %.0f DPS ± %.0f (1 s.e.; s.d. %.0f)\n", res.DPS, res.StdErr, res.StdDev)
	fmt.Printf("Window:    %.0fs–%.0fs, %d iterations, %s\n", warmup, warmup+res.Window, res.Iterations, time.Since(start).Round(time.Millisecond))
	fmt.Printf("Drift:     %+.2f%% (second half vs. first half of the window)\n", res.Drift*100)
	fmt.Println()
	fmt.Printf("%-12s %10s %7s %14s\n", "Character", "DPS", "Share", "Energy wait")
	for _, c := range res.Characters {
		fmt.Printf("%-12s %10.0f %6.1f%% %11.2fs/m\n", c.Name, c.DPS, c.Share*100, c.EnergyWait)
	}
	return nil
}
