// Command gscouter scores a team build against a library rotation.
//
// Usage:
//
//	gscouter -team teams/hutao-xingqiu-zhongli-yelan -build teams/hutao-xingqiu-zhongli-yelan/build.txt
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
	"github.com/cenoda/gscouter/internal/steady"
)

func main() {
	var (
		teamDir    = flag.String("team", "", "team library directory")
		buildPath  = flag.String("build", "", "build file (gcsim character setup lines)")
		iterations = flag.Int("iter", 1000, "number of iterations")
		warmup     = flag.Float64("warmup", 150, "seconds of the looped fight excluded from the score")
		duration   = flag.Float64("duration", 450, "length of the looped fight in seconds")
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

	build := string(raw)
	for m, p := range team.Params {
		build = config.AddParams(build, m, p)
	}
	once := config.Assemble(config.Conditions{Iterations: iterations}, build, team.Rotation)
	chars, err := config.Characters(once)
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}
	if !team.Matches(chars) {
		return fmt.Errorf("build characters %v do not match team %q %v", chars, team.ID, team.Members)
	}
	assemble := func(looped string) string {
		return config.Assemble(config.Conditions{Iterations: iterations, Duration: duration}, build, looped)
	}
	if dump {
		looped, err := config.Loop(team.Rotation)
		if err != nil {
			return err
		}
		fmt.Print(assemble(looped))
		return nil
	}

	start := time.Now()
	out, err := steady.Score(team.Rotation, team.Members, func(looped string) (*score.Result, error) {
		return score.Run(context.Background(), assemble(looped), score.Options{
			Iterations: iterations,
			Warmup:     warmup,
			Seed:       seed,
		})
	})
	if err != nil {
		return err
	}
	res := out.Result
	oneRes, err := score.Run(context.Background(), once, score.Options{Iterations: iterations, Seed: seed})
	if err != nil {
		return err
	}

	fmt.Printf("Team:      %s (rotation v%d, %s)\n", team.Name, team.RotationVersion, team.Status)
	fmt.Printf("Score:     %.0f DPS ± %.0f (1 s.e.; s.d. %.0f)\n", res.DPS, res.StdErr, res.StdDev)
	fmt.Printf("Window:    %.0fs–%.0fs of the looped rotation, %d iterations, %s\n", warmup, warmup+res.Window, res.Iterations, time.Since(start).Round(time.Millisecond))
	fmt.Printf("One pass:  %.0f DPS over %.1fs from full energy (community gcsim measure)\n", oneRes.DPS, oneRes.Window)
	if out.Filled {
		fmt.Printf("Fillers:   %s\n", out)
	}
	fmt.Println()
	fmt.Printf("%-12s %10s %7s %14s\n", "Character", "DPS", "Share", "Energy wait")
	for _, c := range res.Characters {
		fmt.Printf("%-12s %10.0f %6.1f%% %11.2fs/m\n", c.Name, c.DPS, c.Share*100, c.EnergyWait)
	}
	return nil
}
