// Repro tests for AUDIT.md. They fail while the defects described there
// are present. They are not a fix.
package share

import "strings"
import "testing"

// TestRepro_SplitConfigDoesNotSwallowNextStatement shows that a setup
// line without a semicolon is concatenated with the next statement.
// The active line, which the rotation needs, disappears into the build.
func TestRepro_SplitConfigDoesNotSwallowNextStatement(t *testing.T) {
	cfg := "nahida char lvl=90/90 cons=6 talent=1,1,1\nactive nahida;\nnahida skill;\n"
	sp := SplitConfig(cfg)
	if strings.Contains(sp.Build, "active") || !strings.Contains(sp.Rotation, "active nahida") {
		t.Fatalf("missing semicolon swallowed the next statement\nbuild:\n%srotation:\n%s", sp.Build, sp.Rotation)
	}
}
