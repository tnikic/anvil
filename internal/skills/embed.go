// Package skills provides embedded skill files for distribution via `anvil skills install`.
package skills

import "embed"

//go:generate go run github.com/tnikic/anvil/cmd/skillgen -o anvil/SKILL.md

// SkillsFS embeds the anvil skill directory.
//
//go:embed anvil
var SkillsFS embed.FS
