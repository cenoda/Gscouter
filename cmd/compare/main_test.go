package main

import "testing"

func TestClassify(t *testing.T) {
	for _, tc := range []struct {
		name         string
		a, b         map[string]int
		runsA, runsB int
		want         verdict
	}{
		{"same once", map[string]int{"x": 1}, map[string]int{"x": 1}, 1, 1, same},
		{"same repeated", map[string]int{"x": 20}, map[string]int{"x": 20}, 20, 20, same},
		{"differ once", map[string]int{"x": 1}, map[string]int{"y": 1}, 1, 1, diff},
		{"repeats on one side", map[string]int{"x": 20}, map[string]int{"y": 20}, 20, 20, regression},
		{"repeats twice", map[string]int{"x": 18, "y": 2}, map[string]int{"x": 20}, 20, 20, regression},
		// The Klee / Citlali / Xilonen / Xianyun case (design §5.4).
		{"seen once", map[string]int{"x": 19, "y": 1}, map[string]int{"x": 20}, 20, 20, nondeterministic},
		{"one side run once", map[string]int{"y": 1}, map[string]int{"x": 20}, 1, 20, diff},
	} {
		if got := classify(tc.a, tc.b, tc.runsA, tc.runsB); got != tc.want {
			t.Errorf("%s: %q, want %q", tc.name, got, tc.want)
		}
	}
}
