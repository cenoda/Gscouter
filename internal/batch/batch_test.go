package batch

import (
	"slices"
	"testing"
)

func TestShard(t *testing.T) {
	dirs := []string{"a", "b", "c", "d", "e"}
	for spec, want := range map[string][]string{
		"":    dirs,
		"1/2": {"a", "c", "e"},
		"2/2": {"b", "d"},
		"3/3": {"c"},
		"1/1": dirs,
	} {
		got, err := Shard(dirs, spec)
		if err != nil || !slices.Equal(got, want) {
			t.Errorf("Shard %q = %v, %v; want %v", spec, got, err, want)
		}
	}
	for _, spec := range []string{"0/2", "3/2", "x", "1/0"} {
		if _, err := Shard(dirs, spec); err == nil {
			t.Errorf("Shard %q: no error", spec)
		}
	}
}

func TestParallel(t *testing.T) {
	out := Parallel(20, 4, func(i int) int { return i * i })
	for i, ch := range out {
		if got := <-ch; got != i*i {
			t.Errorf("result %d = %d, want %d", i, got, i*i)
		}
	}
}
