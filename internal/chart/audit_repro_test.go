// Regression tests for the defect described in AUDIT.md (item 5).
package chart

import (
	"math"
	"strconv"
	"testing"
)

// TestRepro_WaitSumCannotWrapIntoRange checks that adding wait tokens
// does not wrap around int: two MaxInt waits must not wrap to a small
// positive value that Validate would accept.
func TestRepro_WaitSumCannotWrapIntoRange(t *testing.T) {
	huge := strconv.FormatInt(math.MaxInt, 10)
	c, err := Parse("~" + huge + " ~" + huge + " ~100 0N")
	if err != nil {
		return
	}
	if c.Notes[0].Wait >= 0 && c.Notes[0].Wait <= MaxWait {
		t.Fatalf("waits of MaxInt+MaxInt+100 were accepted as %d frames", c.Notes[0].Wait)
	}
}
