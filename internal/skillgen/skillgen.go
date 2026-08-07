// Package skillgen renders SKILL.md from the content package.
// Used by cmd/skillgen (dev-time) and by the skills commands (runtime check/update).
package skillgen

import (
	"bytes"
	"embed"
	"fmt"
	"strings"
	"text/template"

	"github.com/tnikic/anvil/internal/content"
)

// skillTemplate is the SKILL.md template with YAML frontmatter.
var skillTemplate = template.Must(template.New("skill").Funcs(template.FuncMap{
	"backtick": func() string { return "`" },
}).Parse(`---
name: anvil
description: {{.Description}}
compatibility: Requires the anvil binary (` + "`" + `go install github.com/tnikic/anvil/cmd/anvil@latest` + "`" + `) and network access to the target forge.
---

# anvil

CLI for GitHub, GitLab, and Forgejo. Every command writes TOON &mdash; a compact key-value and tabular format agents parse with fewer tokens than JSON.

## Setup

` + "```" + `bash
which anvil || go install github.com/tnikic/anvil/cmd/anvil@latest
anvil auth set <host> <token>
` + "```" + `

anvil detects the forge and repo from ` + "`" + `git remote origin` + "`" + `. Override with ` + "`" + `--forge` + "`" + ` and ` + "`" + `--repo` + "`" + `.

## Commands

| Group | Subcommands |
|---|---|
{{range .Commands}}| ` + "`" + `{{.Name}}` + "`" + ` | {{.Short}} |
{{end}}
Run ` + "`" + `anvil <command> --help` + "`" + ` for flags. Key behaviours:

- **PR stacks.** Dependent PR chains tracked via ` + "`" + `[stackname:N/M]` + "`" + ` title prefix. Stack name auto-derived from the branch. ` + "`" + `pr merge` + "`" + ` renumbers remaining open PRs in the stack.
- **Scoped labels.** ` + "`" + `--scope kind --name bug` + "`" + ` normalizes to each forge's format (` + "`" + `kind:bug` + "`" + ` on GitHub, ` + "`" + `kind::bug` + "`" + ` on GitLab, ` + "`" + `kind/bug` + "`" + ` on Forgejo).
- **Issue relationships.** ` + "`" + `issue blocked-by` + "`" + `, ` + "`" + `blocking` + "`" + `, ` + "`" + `children` + "`" + `, and ` + "`" + `parent` + "`" + ` query hierarchical relationships. Add with ` + "`" + `issue relation add --blocks` + "`" + ` or ` + "`" + `--parent-of` + "`" + `. Use ` + "`" + `--parent <N>` + "`" + ` on ` + "`" + `issue create` + "`" + ` to set the parent in a single command.
- **Idempotent mutations.** Closing an already-closed issue exits 0 with a no-op message. Non-zero only when the intent cannot be satisfied.
- **Errors on stdout.** Structured ` + "`" + `error:` + "`" + ` / ` + "`" + `help:` + "`" + ` lines the agent can read and act on.
- **Setup hooks.** ` + "`" + `anvil setup hooks` + "`" + ` installs SessionStart hooks for Claude Code, Codex, and OpenCode, injecting live forge context at session start.

## Output format

Lists use TOON tabular output with a count aggregate:

` + "```" + `
issues[2]{number,title,state}:
  42,Fix auth bug,open
  87,Add pagination,open
count: 2 of 47 total
` + "```" + `

Dashboard (` + "`" + `anvil` + "`" + ` with no arguments) shows live forge state:

` + "```" + `
bin: ~/bin/anvil
description: AXI-compliant Git forge CLI for AI agents

forge: github.com
repo: owner/name

issues[3 of 47]{number, title, state, author}:
  1  Fix login timeout                      open   alice
count: 3 of 47 total

prs[1 of 5]{number, title, author}:
  100  Refactor auth module                 dave
count: 1 of 5 total

help[2]:
  Run ` + "`" + `anvil issue list` + "`" + ` for all 47 open issues
  Run ` + "`" + `anvil pr list` + "`" + ` for all 5 open PRs
` + "```" + `

Detail views include truncated bodies (500 chars) with total ` + "`" + `body_size` + "`" + `. Pass ` + "`" + `--full` + "`" + ` for the complete body. ` + "`" + `comment list` + "`" + ` truncates to 80 chars; ` + "`" + `--full` + "`" + ` returns complete comment bodies.

Errors:

` + "```" + `
error: --title is required
help: anvil issue create --title "..." [--body "..."]
` + "```" + `

## Tips

{{range .GlobalTips}}- {{.}}
{{end}}`))

type skillData struct {
	Description string
	Commands    []content.CommandDef
	GlobalTips  []string
}

// Render generates the SKILL.md content from the content package.
func Render() (string, error) {
	data := skillData{
		Description: skillDescription(),
		Commands:    content.Commands,
		GlobalTips:  content.GlobalTips,
	}

	var buf bytes.Buffer
	if err := skillTemplate.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("rendering SKILL.md: %w", err)
	}
	return buf.String(), nil
}

// skillDescription returns the agent-facing description used in the skill frontmatter.
// This is richer than content.Description — it includes the use-case guidance.
func skillDescription() string {
	return "Work with Git forges — create, view, and manage issues, pull requests, and labels on GitHub, GitLab, and Forgejo. Use when the user asks to work with issues, PRs, labels, or needs to authenticate with a forge."
}

// Check reports whether embedded matches what Render would produce.
// Returns true if they match, false with a diff description if they differ.
func Check(embedded string) (ok bool, diff string, err error) {
	rendered, err := Render()
	if err != nil {
		return false, "", err
	}

	if rendered == embedded {
		return true, "", nil
	}

	return false, diffLines(embedded, rendered), nil
}

// ReadEmbedded reads the embedded SKILL.md from the provided FS.
func ReadEmbedded(efs embed.FS) (string, error) {
	data, err := efs.ReadFile("anvil/SKILL.md")
	if err != nil {
		return "", fmt.Errorf("reading embedded SKILL.md: %w", err)
	}
	return string(data), nil
}

// diffLines produces a human-readable diff between two strings.
func diffLines(a, b string) string {
	aLines := strings.Split(a, "\n")
	bLines := strings.Split(b, "\n")

	var out strings.Builder
	maxLen := len(aLines)
	if len(bLines) > maxLen {
		maxLen = len(bLines)
	}

	for i := 0; i < maxLen; i++ {
		var lineA, lineB string
		if i < len(aLines) {
			lineA = aLines[i]
		}
		if i < len(bLines) {
			lineB = bLines[i]
		}
		if lineA != lineB {
			_, _ = fmt.Fprintf(&out, "line %d:\n  - %s\n  + %s\n", i+1, lineA, lineB)
		}
	}
	return out.String()
}
