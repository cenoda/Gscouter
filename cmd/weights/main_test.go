package main

import (
	"fmt"
	"slices"
	"testing"

	"github.com/cenoda/gscouter/internal/batch"
)

func TestSpread(t *testing.T) {
	dirs := []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j"}
	for n, want := range map[int][]string{
		0:  dirs,
		10: dirs,
		20: dirs,
		1:  {"a"},
		2:  {"a", "f"},
		3:  {"a", "d", "g"},
		5:  {"a", "c", "e", "g", "i"},
	} {
		if got := spread(dirs, n); !slices.Equal(got, want) {
			t.Errorf("spread %d = %v, want %v", n, got, want)
		}
	}
}

// TestShardsCoverLimit checks what the weights workflow runs: n shards
// of the spread teams hold every one of them exactly once.
func TestShardsCoverLimit(t *testing.T) {
	dirs, err := batch.Dirs("../../teams", nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct{ limit, shards int }{{100, 4}, {100, 20}, {0, 20}} {
		want := spread(dirs, c.limit)
		seen := map[string]int{}
		for k := 1; k <= c.shards; k++ {
			part, err := batch.Shard(want, fmt.Sprintf("%d/%d", k, c.shards))
			if err != nil {
				t.Fatal(err)
			}
			if len(part) == 0 {
				t.Errorf("limit %d: shard %d/%d is empty", c.limit, k, c.shards)
			}
			for _, d := range part {
				seen[d]++
			}
		}
		if len(seen) != len(want) {
			t.Errorf("limit %d, %d shards: %d teams, want %d", c.limit, c.shards, len(seen), len(want))
		}
		for d, n := range seen {
			if n != 1 {
				t.Errorf("limit %d, %d shards: %s in %d shards", c.limit, c.shards, d, n)
			}
		}
	}
}
