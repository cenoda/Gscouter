package main

import (
	"slices"
	"testing"
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
