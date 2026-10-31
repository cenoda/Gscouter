package config

import (
	"strings"
)

// DropBursts rewrites a rotation without char's bursts: each burst is
// taken out of the action statements of char, and a statement left with
// no action is dropped. Conditions that read the burst (.char.burst.ready)
// are left as they are. char may be any of gcsim's names for the member:
// statements are matched by gcsim key, as Fill matches them, so that
// "fischl" and "amy" each take the bursts out of both "fischl burst;" and
// "amy burst;". A statement kept is written under the rotation's name.
func DropBursts(rotation, char string) string {
	var b strings.Builder
	start := true // at the start of a statement
	for i := 0; i < len(rotation); {
		if start {
			if st, ok := scanStatement(rotation[i:]); ok && member(st.name, []string{char}) == char {
				var keep []string
				for _, item := range splitActions(st.items) {
					item = strings.TrimSpace(item)
					if name := parseAction(item).name; name != "" && name != "burst" {
						keep = append(keep, item)
					}
				}
				b.WriteString(st.lead)
				if len(keep) > 0 {
					b.WriteString(st.name + " " + strings.Join(keep, ", ") + ";")
				}
				i += st.end
				continue
			}
		}
		c := rotation[i]
		b.WriteByte(c)
		i++
		switch c {
		case ';', '{', '}':
			start = true
		case ' ', '\t', '\n', '\r':
		default:
			start = false
		}
	}
	return b.String()
}
