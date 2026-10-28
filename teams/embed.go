// Package teams embeds the team library so the server is a single binary.
package teams

import "embed"

//go:embed */team.yaml */rotation.gcsl */build.txt
var FS embed.FS
