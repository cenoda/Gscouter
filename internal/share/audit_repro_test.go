// Regression tests for the defect described in AUDIT.md (item 6).
package share

import "strings"
import "testing"

// TestRepro_SplitConfigDoesNotSwallowNextStatement checks that a setup
// line without a semicolon is not concatenated with the next statement,
// so the active line, which the rotation needs, stays in the rotation.
func TestRepro_SplitConfigDoesNotSwallowNextStatement(t *testing.T) {
	cfg := "nahida char lvl=90/90 cons=6 talent=1,1,1\nactive nahida;\nnahida skill;\n"
	sp := SplitConfig(cfg)
	if strings.Contains(sp.Build, "active") || !strings.Contains(sp.Rotation, "active nahida") {
		t.Fatalf("missing semicolon swallowed the next statement\nbuild:\n%srotation:\n%s", sp.Build, sp.Rotation)
	}
}
