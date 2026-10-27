package validate

import (
	"testing"

	"github.com/cenoda/gscouter/internal/library"
	"github.com/cenoda/gscouter/internal/score"
)

func TestUnpromotable(t *testing.T) {
	report := func(source, once, common, looped float64) *Report {
		return &Report{
			Team:   &library.Team{Reference: &library.Reference{DPS: source}},
			Once:   &score.Result{DPS: once},
			Common: &score.Result{DPS: common},
			Res:    &score.Result{DPS: looped},
		}
	}
	failed := report(100, 100, 100, 100)
	failed.Failures = []string{"stalls"}
	noRef := report(100, 100, 100, 100)
	noRef.Team.Reference = nil
	for name, tc := range map[string]struct {
		r    *Report
		want bool
	}{
		"meets all":        {report(100, 104, 90, 80), true},
		"kept over 100%":   {report(100, 96, 90, 95), true},
		"too far below":    {report(100, 94, 90, 90), false},
		"too far above":    {report(100, 106, 90, 90), false},
		"keeps too little": {report(100, 100, 100, 79), false},
		"fails the check":  {failed, false},
		"no reference":     {noRef, false},
	} {
		if got := tc.r.Unpromotable() == ""; got != tc.want {
			t.Errorf("%s: promotable %v, want %v (%s)", name, got, tc.want, tc.r.Unpromotable())
		}
	}
}
