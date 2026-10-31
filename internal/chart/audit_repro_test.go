// Repro tests for AUDIT.md. They fail while the defects described there
// are present. They are not a fix.
package chart

import (
	"math"
	"strconv"
	"testing"
)

// TestRepro_WaitSumCannotWrapIntoRange shows that adding wait tokens
// uses unchecked int addition. Two MaxInt waits wrap to a small
// positive value, which Validate then accepts.
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
