---
name: anvil
description: Work with Git forges — create, view, and manage issues, pull requests, and labels on GitHub, GitLab, and Forgejo. Use when the user asks to work with issues, PRs, labels, or needs to authenticate with a forge.
compatibility: Requires the anvil binary (`go install github.com/tnikic/anvil/cmd/anvil@latest`) and network access to the target forge.
---

# anvil

CLI for GitHub, GitLab, and Forgejo. Every command writes TOON &mdash; a compact key-value and tabular format agents parse with fewer tokens than JSON.

## Setup

```bash
which anvil || go install github.com/tnikic/anvil/cmd/anvil@latest
anvil auth set <host> <token>
```

anvil detects the forge and repo from `git remote origin`. Override with `--forge` and `--repo`.

## Commands

| Group | Subcommands |
|---|---|
| `issue` | Manage issues |
| `pr` | Manage pull requests |
| `label` | Manage labels |
| `auth` | Manage authentication |
| `skills` | Manage the anvil agent skill |

Run `anvil <command> --help` for flags. Key behaviours:

- **PR stacks.** Dependent PR chains tracked via `[stackname:N/M]` title prefix. Stack name auto-derived from the branch. `pr merge` renumbers remaining open PRs in the stack.
- **Scoped labels.** `--scope kind --name bug` normalizes to each forge's format (`kind:bug` on GitHub, `kind::bug` on GitLab, `kind/bug` on Forgejo).
- **Issue relationships.** `issue blocked-by`, `blocking`, `children`, and `parent` query hierarchical relationships. Add with `issue relation add --blocks` or `--parent-of`.
- **Idempotent mutations.** Closing an already-closed issue exits 0 with a no-op message. Non-zero only when the intent cannot be satisfied.
- **Errors on stdout.** Structured `error:` / `help:` lines the agent can read and act on.
- **Setup hooks.** `anvil setup hooks` installs SessionStart hooks for Claude Code, Codex, and OpenCode, injecting live forge context at session start.

## Output format

Lists use TOON tabular output with a count aggregate:

```
issues[2]{number,title,state}:
  42,Fix auth bug,open
  87,Add pagination,open
count: 2 of 47 total
```

Dashboard (`anvil` with no arguments) shows live forge state:

```
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
  Run `anvil issue list` for all 47 open issues
  Run `anvil pr list` for all 5 open PRs
```

Detail views include truncated bodies (500 chars) with total `body_size`. Pass `--full` for the complete body.

Errors:

```
error: --title is required
help: anvil issue create --title "..." [--body "..."]
```

## Tips

- Run `anvil --forge <host> --repo <owner/name>` to target a repository
- Run `anvil auth set <host> <token>` to authenticate
