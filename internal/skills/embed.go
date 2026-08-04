// Package skills provides embedded skill files for distribution via `anvil skills install`.
package skills

import "embed"

// SkillsFS embeds the anvil skill directory.
//
//go:embed anvil
var SkillsFS embed.FS
