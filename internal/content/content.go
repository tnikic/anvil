// Package content is the single source of truth for agent-facing text
// shared between the dashboard and the skill generator.
package content

import "fmt"

// Description is the one-line description of anvil.
const Description = "AXI-compliant Git forge CLI for AI agents"

// CommandDef describes a top-level command group for agent-facing output.
type CommandDef struct {
	Name  string   // command name (e.g., "issue", "pr")
	Short string   // one-line summary
	Tips  []string // agent-facing help hints
}

// Commands lists the top-level command groups in display order.
var Commands = []CommandDef{
	{Name: "issue", Short: "Manage issues"},
	{Name: "pr", Short: "Manage pull requests"},
	{Name: "label", Short: "Manage labels"},
	{Name: "auth", Short: "Manage authentication"},
	{Name: "skills", Short: "Manage the anvil agent skill"},
}

// GlobalTips are static help hints shown in fallback output and in the skill.
var GlobalTips = []string{
	"Run `anvil --forge <host> --repo <owner/name>` to target a repository",
	"Run `anvil auth set <host> <token>` to authenticate",
}

// DashboardTips returns contextual help hints based on live dashboard data.
// When the dashboard can query live data, these replace GlobalTips.
func DashboardTips(issueTotal, prTotal int) []string {
	var tips []string
	if issueTotal > 3 {
		tips = append(tips, fmt.Sprintf("Run `anvil issue list` for all %d open issues", issueTotal))
	} else {
		tips = append(tips, "Run `anvil issue list` to see open issues")
	}
	if prTotal > 3 {
		tips = append(tips, fmt.Sprintf("Run `anvil pr list` for all %d open PRs", prTotal))
	} else {
		tips = append(tips, "Run `anvil pr list` to see open PRs")
	}
	return tips
}
