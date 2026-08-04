# anvil

An AXI-compliant, agent-first CLI for interacting with Git forges — GitHub, GitLab, Forgejo — through TOON output on stdout.

anvil normalizes the differences between forge APIs behind a single interface. Issue, label, and PR operations work the same way whether you are talking to GitHub, GitLab, or Forgejo.

## Quick Start

```bash
go install github.com/tnikic/anvil/cmd/anvil@latest

anvil auth set github.com <token>
anvil issue list
```

anvil auto-detects the forge and repo from your git remote. Pass `--forge` and `--repo` to override.

### Agent Skill

anvil ships an agent skill that teaches AI harnesses how to use it. Install it once:

```bash
anvil skills install
```

The skill is extracted from the binary to `~/.agents/skills/anvil/SKILL.md` — always version-locked to your binary. Run `anvil skills status` to check, `anvil skills update` to refresh after an upgrade, and `anvil skills uninstall` to remove.

## Commands

| Group | Subcommands |
|---|---|
| `issue` | `list`, `view`, `create`, `update`, `close`, `reopen`, `blocked-by`, `blocking`, `children`, `parent`, `comment`, `relation` |
| `pr` | `list`, `view`, `create`, `merge` |
| `label` | `list`, `create`, `update`, `delete` |
| `auth` | `status`, `set`, `unset` |
## How to Build and Test

```bash
go build ./cmd/anvil
go test ./...
```

## Folder Structure

```
.
├── cmd/anvil/                  # Binary entry point
├── internal/
│   ├── auth/                   # Credential storage and token resolution
│   ├── commands/               # Cobra commands and middleware
│   │   └── blocking/           #   Client-side blocking filter for issue list
│   ├── forge/                  # Domain types, provider interface, detection
│   │   ├── forgetest/          # In-memory fake forge for testing
│   │   ├── forgejo/            # Forgejo/Gitea REST API adapter
│   │   ├── github/             # GitHub REST API adapter
│   │   └── gitlab/             # GitLab REST API adapter
│   ├── format/                 # TOON output formatter
│   ├── skills/                 # Embedded agent skill files
│   │   └── anvil/              #   SKILL.md
│   └── stack/                  # PR stack ordering and renumbering
└── docs/
    ├── adr/                    # Architecture Decision Records
    ├── CONTEXT.md              # Domain glossary
    └── TESTING.md              # Test portfolio
```

## Documentation Index

- **Domain glossary:** [`docs/CONTEXT.md`](docs/CONTEXT.md) — terminology used throughout the codebase
- **Test portfolio:** [`docs/TESTING.md`](docs/TESTING.md) — test layers and how to run them
- **ADRs:** [`docs/adr/`](docs/adr/) — architecture decisions (auth storage, provider interface, PR stack model)
