// Package teams embeds the team library so the server is a single binary.
package teams

import "embed"

// The second line embeds the teams' alternatives (library.AltDir). go:embed
// fails on a pattern that matches nothing, so it goes if none is left.
//
//go:embed */team.yaml */rotation.gcsl */build.txt
//go:embed */alt/*/team.yaml */alt/*/rotation.gcsl */alt/*/build.txt
var FS embed.FS
