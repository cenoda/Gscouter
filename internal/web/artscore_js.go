//go:build js && wasm

package web

// scoreArtifacts is the server's (artscore.go): the browser build
// renders the scores the Job brings and never prepares a team.
func scoreArtifacts(*teamResult) {}
