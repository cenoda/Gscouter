package web

import (
	"context"
	"errors"
	"os"
	"slices"
	"testing"
	"time"

	"github.com/cenoda/gscouter/internal/config"
	"github.com/cenoda/gscouter/internal/timeline"
)

// A timeline stops when its request is done and frees the simulation
// slot: the default settings have only one.
func TestTimelineStopsOnCancel(t *testing.T) {
	srv := newTestSite(t)
	build, err := os.ReadFile("../score/testdata/build.txt")
	if err != nil {
		t.Fatal(err)
	}
	// A fight that never ends, in a window too long to bound it.
	cfg := config.Assemble(config.Conditions{Iterations: 1}, string(build), "active escoffier;\nwhile 1 { wait(60); }\n")
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	start := time.Now()
	if _, err := srv.timeline(ctx, cfg, 0, 1e6); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("err = %v, want %v", err, context.DeadlineExceeded)
	}
	if took := time.Since(start); took > 300*time.Millisecond {
		t.Errorf("returned after %v", took)
	}
	if n := len(srv.sem); n != 0 {
		t.Errorf("%d simulation slots still held", n)
	}
}

func TestActionChips(t *testing.T) {
	r := func(kind string, n int) timeline.Run { return timeline.Run{Kind: kind, Count: n} }
	rf := func(kind, form string, n int) timeline.Run { return timeline.Run{Kind: kind, Form: form, Count: n} }
	runs := []timeline.Run{r("skill", 1)}
	for range 3 {
		runs = append(runs, r("attack", 2), r("charge", 1), r("dash", 1))
	}
	runs = append(runs, r("burst", 1))
	ko := localizer{Lang: langKorean}
	got := actionChips(ko, runs)
	want := []string{"E", "(평타×2 · 강공격 · 대시) ×3", "Q"}
	if !slices.Equal(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
	if got := actionChips(ko, []timeline.Run{r("skill", 1), r("burst", 1), r("attack", 4)}); !slices.Equal(got, []string{"E", "Q", "평타×4"}) {
		t.Errorf("got %q", got)
	}
	if got := actionChips(english, runs); !slices.Equal(got, []string{"E", "(NA×2 · CA · Dash) ×3", "Q"}) {
		t.Errorf("got %q", got)
	}
	// Mavuika's stint on Mavuika / Citlali / Xilonen / Nicole.
	mavuika := []timeline.Run{
		rf("skill", "recast", 1), r("charge", 1), r("dash", 1), r("attack", 1), r("charge", 1), r("burst", 1),
		rf("charge", "hold", 1), r("dash", 1), rf("charge", "hold", 1), r("dash", 1),
		r("attack", 1), r("skill", 1), r("attack", 1),
	}
	want = []string{"E (재시전)", "강공격", "대시", "평타", "강공격", "Q", "(홀드 강공격 · 대시) ×2", "평타", "E", "평타"}
	if got := actionChips(ko, mavuika); !slices.Equal(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
	// A form without a name of its own is named by its kind.
	if got := actionChips(english, []timeline.Run{rf("burst", "hold", 1), rf("skill", "hold", 2)}); !slices.Equal(got, []string{"Q", "Hold E×2"}) {
		t.Errorf("got %q", got)
	}
}
