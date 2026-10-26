// Package teams embeds the team library so the server is a single binary.
package teams

import "embed"

//go:embed */team.yaml */rotation.gcsl
var FS embed.FS
