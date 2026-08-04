---
name: anvil
description: Work with Git forges — create, view, and manage issues, pull requests, and labels on GitHub, GitLab, and Forgejo. Use when the user asks to work with issues, PRs, labels, or needs to authenticate with a forge.
compatibility: Requires the anvil binary (go install github.com/tnikic/anvil/cmd/anvil@latest) and network access to the target forge.
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
| `issue` | `list`, `view`, `create`, `update`, `close`, `reopen` |
| `pr` | `list`, `view`, `create`, `merge` |
| `label` | `list`, `create`, `update`, `delete` |
| `auth` | `status`, `set`, `unset` |

Run `anvil <command> --help` for flags. Key behaviours:

- **PR stacks.** Dependent PR chains tracked via `[stackname:N/M]` title prefix. Stack name auto-derived from the branch. `pr merge` renumbers remaining open PRs in the stack.
- **Scoped labels.** `--scope kind --name bug` normalizes to each forge's format (`kind:bug` on GitHub, `kind::bug` on GitLab, `kind/bug` on Forgejo).
- **Idempotent mutations.** Closing an already-closed issue exits 0 with a no-op message. Non-zero only when the intent cannot be satisfied.
- **Errors on stdout.** Structured `error:` / `help:` lines the agent can read and act on.

## Output format

Lists use TOON tabular output with a count aggregate:

```
issues[2]{number,title,state}:
  42,Fix auth bug,open
  87,Add pagination,open
count: 2 of 47 total
```

Detail views include truncated bodies (500 chars) with total `body_size`. Pass `--full` for the complete body.

Errors:

```
error: --title is required
help: anvil issue create --title "..." [--body "..."]
```
