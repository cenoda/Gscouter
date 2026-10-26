package web

import (
	"slices"
	"testing"

	"github.com/cenoda/gscouter/internal/timeline"
)

func TestActionChips(t *testing.T) {
	r := func(kind string, n int) timeline.Run { return timeline.Run{Kind: kind, Count: n} }
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
}
