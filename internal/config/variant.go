package config

import (
	"strings"
)

// DropBursts rewrites a rotation without char's bursts: each burst is
// taken out of the action statements of char, and a statement left with
// no action is dropped. Conditions that read the burst (.char.burst.ready)
// are left as they are. char is named as the rotation writes it.
func DropBursts(rotation, char string) string {
	var b strings.Builder
	start := true // at the start of a statement
	for i := 0; i < len(rotation); {
		if start {
			if m := actionRe.FindStringSubmatchIndex(rotation[i:]); m != nil && rotation[i+m[4]:i+m[5]] == char {
				var keep []string
				for _, item := range splitActions(rotation[i+m[6] : i+m[7]]) {
					item = strings.TrimSpace(item)
					if name := parseAction(item).name; name != "" && name != "burst" {
						keep = append(keep, item)
					}
				}
				b.WriteString(rotation[i+m[2] : i+m[3]])
				if len(keep) > 0 {
					b.WriteString(char + " " + strings.Join(keep, ", ") + ";")
				}
				i += m[1]
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
