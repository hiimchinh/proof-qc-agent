// Package results ships the saved eval runs inside the binary, so the demo's
// leaderboard shows the same numbers as the README without calling a model.
package results

import (
	"embed"
	"io/fs"
)

// The glob matches v2 (order rules in code) and v2-prompt-only (rules left to
// the prompt); either may still be filling up while an eval runs.
//
//go:embed v2*/*.json
var runs embed.FS

// Embedded returns the saved runs, one directory per variant.
func Embedded() fs.FS { return runs }
