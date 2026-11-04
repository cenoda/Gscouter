// Command pagebench times a result page's simulations for library teams'
// library builds at the site's settings (50 iterations, ranking runs on
// 10, warmup 150 s, duration 450 s, seed 1, one share per CPU), on gcsim
// and on a gscouter-engine module, in the ways compute can run them:
//
//   - gcsim: the search and the one pass on gcsim, as before the engine;
//   - twice: the search on the engine, again on gcsim when it went past
//     the rotation as written, gcsim's pick and the one pass on the engine
//     (keepGcsimPick as of 2026-11-04);
//   - once: the engine's search stopped at its second run, which is when
//     keepGcsimPick would run the search on gcsim; then as twice.
//
// It prints, per team and mode, the median of -n runs and the time spent
// on each engine, and checks that twice and once pick the same rotation.
package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/cenoda/gscouter/internal/config"
	"github.com/cenoda/gscouter/internal/engine"
	"github.com/cenoda/gscouter/internal/library"
	"github.com/cenoda/gscouter/internal/score"
	"github.com/cenoda/gscouter/internal/steady"
	"github.com/cenoda/gscouter/teams"
)

const iters, explore = 50, 10

var errStop = errors.New("stop: the search goes past the rotation as written")

type timing struct{ total, gcsim, eng time.Duration }

type page struct {
	builds []string
	runs   []steady.Rotation
	eng    score.Engine
	t      timing
}

func (p *page) run(i int, looped string, rank, onEngine bool) (*score.Result, error) {
	n := iters
	if rank {
		n = explore
	}
	cfg := config.Assemble(config.Conditions{Iterations: n, Duration: 450}, p.builds[i], looped)
	o := score.Options{Iterations: n, Warmup: 150, Seed: 1}
	if onEngine {
		o.Engine = p.eng
	}
	start := time.Now()
	res, err := score.Run(context.Background(), cfg, o)
	d := time.Since(start)
	if onEngine {
		p.t.eng += d
	} else {
		p.t.gcsim += d
	}
	return res, err
}

func (p *page) once(best int, rotation string, onEngine bool) {
	cfg := config.Assemble(config.Conditions{Iterations: iters}, p.builds[best], rotation)
	o := score.Options{Iterations: iters, Seed: 1}
	if onEngine {
		o.Engine = p.eng
	}
	start := time.Now()
	score.Run(context.Background(), cfg, o)
	if onEngine {
		p.t.eng += time.Since(start)
	} else {
		p.t.gcsim += time.Since(start)
	}
}

// search runs the page's search in mode, returns the pick.
func (p *page) search(mode string) (int, string) {
	start := time.Now()
	defer func() { p.t.total = time.Since(start) }()
	gcsimSearch := func() (int, *steady.Outcome) {
		best, out, err := steady.Best(p.runs, func(i int, l string, r bool) (*score.Result, error) { return p.run(i, l, r, false) })
		if err != nil {
			panic(err)
		}
		return best, out
	}
	switch mode {
	case "gcsim":
		best, out := gcsimSearch()
		p.once(best, p.runs[best].Rotation, false)
		return best, out.Looped
	}
	n := 0
	best, out, err := steady.Best(p.runs, func(i int, l string, r bool) (*score.Result, error) {
		n++
		if mode == "once" && n > 1 {
			return nil, steady.Fatal(errStop)
		}
		return p.run(i, l, r, true)
	})
	if err != nil && !errors.Is(err, errStop) {
		panic(err)
	}
	if n > 1 {
		best, out = gcsimSearch()
		p.run(best, out.Looped, false, true)
	}
	p.once(best, p.runs[best].Rotation, true)
	return best, out.Looped
}

func main() {
	wasm := flag.String("wasm", "", "engine module")
	n := flag.Int("n", 3, "runs per team and mode")
	flag.Parse()
	mod, err := os.ReadFile(*wasm)
	if err != nil {
		panic(err)
	}
	sum := sha256.Sum256(mod)
	man := fmt.Sprintf(`{"engine":{"commit":"local","wasm_sha256":%q},"teams":{}}`, hex.EncodeToString(sum[:]))
	e, _, err := engine.New(mod, []byte(man), nil, teams.FS, nil)
	if err != nil {
		panic(err)
	}
	if c, ok := e.Runner().(interface{ Compile() error }); ok {
		start := time.Now()
		if err := c.Compile(); err != nil {
			panic(err)
		}
		fmt.Printf("compile %v\n", time.Since(start).Round(time.Millisecond))
	}
	lib, err := library.LoadLibrary(teams.FS)
	if err != nil {
		panic(err)
	}
	byID := map[string]*library.Team{}
	for _, t := range lib.Teams {
		byID[t.ID] = t
	}
	fmt.Printf("%-44s %-6s %8s %8s %8s %s\n", "team", "mode", "total", "gcsim", "engine", "pick")
	for _, id := range flag.Args() {
		t := byID[id]
		raw, err := fs.ReadFile(teams.FS, id+"/build.txt")
		if err != nil {
			panic(err)
		}
		rots := t.RotationsFor(library.BuildConstellations(string(raw)))
		p := &page{eng: e.Runner()}
		for _, r := range rots {
			b := string(raw)
			for m, prm := range r.Params {
				b = config.AddParams(b, m, prm)
			}
			p.builds = append(p.builds, b)
			p.runs = append(p.runs, steady.Rotation{Rotation: r.Rotation, Members: r.Members})
		}
		picks := map[string]string{}
		for _, mode := range []string{"gcsim", "twice", "once"} {
			var ts []timing
			var pick string
			for k := 0; k < *n; k++ {
				p.t = timing{}
				b, looped := p.search(mode)
				h := sha256.Sum256([]byte(looped))
				pick = fmt.Sprintf("%d:%x", b, h[:4])
				ts = append(ts, p.t)
			}
			sort.Slice(ts, func(i, j int) bool { return ts[i].total < ts[j].total })
			m := ts[len(ts)/2]
			picks[mode] = pick
			fmt.Printf("%-44s %-6s %7.2fs %7.2fs %7.2fs %s\n", id, mode, m.total.Seconds(), m.gcsim.Seconds(), m.eng.Seconds(), pick)
		}
		if picks["twice"] != picks["once"] || picks["gcsim"] != picks["once"] {
			fmt.Printf("%-44s PICKS DIFFER: %s\n", id, strings.Join([]string{picks["gcsim"], picks["twice"], picks["once"]}, " "))
		}
	}
}
