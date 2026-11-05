package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"strings"

	"github.com/cenoda/gscouter/internal/engine"
	"github.com/cenoda/gscouter/internal/library"
	"github.com/cenoda/gscouter/internal/score"
	"github.com/cenoda/gscouter/internal/steady"
)

// The verdict's run, as gscouter-engine's compare runs a library build
// (reference/compare: gscouter -dump, then the engine from seed 1 with a
// warmup of 150 s): the carry check runs both modules the same way.
const (
	carryWarmup   = 150.0
	carrySeed     = 1
	dumpIter      = 1000 // cmd/gscouter's default -iter, written in the config's options
	dumpDuration  = 450  // cmd/gscouter's default -duration
	carryHowExact = "identical"
)

// release is a pinned or new release: its module, manifest and the team
// library at its Gscouter commit.
type release struct {
	module   []byte
	manifest engine.Manifest
	library  fs.FS
	runner   score.Engine
}

func (r *release) prefix() string { return short(r.manifest.Engine.WasmSHA256) }

// carry decides, for each group B approval held for another module than
// next's, whether it carries forward to next: the team is still verified,
// its library files are the same at both releases' Gscouter commits, and
// its library build, run on both modules at the iterations of its
// verdict in prev's manifest, seed and warmup, gives the same team DPS
// and the same DPS for every character, to the last digit. carried holds
// the approvals rewritten for next; dropped, why each other one does not
// carry. An approval for a module that is not prev's, or a missing prev,
// is an error: pinengine refuses rather than carry what it cannot check.
func carry(ctx context.Context, list []engine.Approval, prev, next *release, today string) (carried map[string]engine.Approval, dropped map[string]string, err error) {
	carried, dropped = map[string]engine.Approval{}, map[string]string{}
	var todo []engine.Approval
	for _, a := range list {
		if a.Release != "" && !strings.HasPrefix(next.manifest.Engine.WasmSHA256, a.Release) {
			todo = append(todo, a)
		}
	}
	if len(todo) == 0 {
		return carried, dropped, nil
	}
	if prev == nil || prev.module == nil {
		return nil, nil, fmt.Errorf("%d group B approvals are for another module, and the pinned module to carry them from is missing: pin with -carry=false to drop them instead", len(todo))
	}
	if hashOf(prev.module) != prev.manifest.Engine.WasmSHA256 {
		return nil, nil, errors.New("the pinned module is not its manifest's (SHA-256 differs): cannot carry group B approvals from it")
	}
	for _, a := range todo {
		if a.Release != prev.prefix() {
			return nil, nil, fmt.Errorf("%s is approved for module %s, but the pinned module is %s: cannot carry it", a.Team, a.Release, prev.prefix())
		}
		why, err := carryOne(ctx, a, prev, next)
		if err != nil {
			return nil, nil, err
		}
		if why != "" {
			dropped[a.Team] = why
			continue
		}
		c := a
		c.Release = next.prefix()
		c.Carried = append(append([]engine.Carry(nil), a.Carried...), engine.Carry{From: a.Release, How: carryHowExact, Date: today})
		carried[a.Team] = c
	}
	return carried, dropped, nil
}

// carryOne says why a does not carry from prev to next, or "".
func carryOne(ctx context.Context, a engine.Approval, prev, next *release) (string, error) {
	held := a
	held.Release = next.prefix()
	if why := engine.Dropped(next.manifest, held); why != "" {
		return why, nil
	}
	hPrev, err1 := engine.TeamHash(prev.library, a.Team)
	hNext, err2 := engine.TeamHash(next.library, a.Team)
	if err1 != nil || err2 != nil || hPrev != hNext {
		return fmt.Sprintf("library files differ between Gscouter %s and %s", short(prev.manifest.Gscouter), short(next.manifest.Gscouter)), nil
	}
	iter := 0
	for _, s := range prev.manifest.Teams[a.Team].Scores {
		if s.Build == "library" {
			iter = s.Iterations
		}
	}
	if iter <= 0 {
		return fmt.Sprintf("no library verdict for it in module %s's manifest", prev.prefix()), nil
	}
	team, err := library.LoadFS(prev.library, a.Team)
	if err != nil {
		return "", err
	}
	cfg, err := steady.LibraryConfig(team, team.Build, dumpIter, dumpDuration)
	if err != nil {
		return "", fmt.Errorf("%s: %w", a.Team, err)
	}
	opts := score.Options{Iterations: iter, Warmup: carryWarmup, Seed: carrySeed}
	var res [2]*score.Result
	for i, r := range []*release{prev, next} {
		its, err := r.runner.RunIterations(ctx, cfg, opts)
		if err != nil {
			return fmt.Sprintf("module %s failed its library build: %v", r.prefix(), err), nil
		}
		res[i] = score.Aggregate(its, carryWarmup)
	}
	if d := difference(res[0], res[1]); d != "" {
		return fmt.Sprintf("module %s scores it differently at %d iterations: %s", next.prefix(), iter, d), nil
	}
	return "", nil
}

// difference is the first number that differs between a and b, or "":
// team DPS, then each character's DPS, compared as float64.
func difference(a, b *score.Result) string {
	if a.DPS != b.DPS {
		return fmt.Sprintf("team DPS %v, now %v", a.DPS, b.DPS)
	}
	if len(a.Characters) != len(b.Characters) {
		return fmt.Sprintf("%d characters, now %d", len(a.Characters), len(b.Characters))
	}
	for i, c := range a.Characters {
		d := b.Characters[i]
		if c.Name != d.Name || c.DPS != d.DPS {
			return fmt.Sprintf("%s DPS %v, now %s %v", c.Name, c.DPS, d.Name, d.DPS)
		}
	}
	return ""
}

func hashOf(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
