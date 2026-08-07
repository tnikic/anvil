# Skill file is generated at dev time and committed, not built at release time

SKILL.md is produced by `cmd/skillgen/main.go` (invoked via `go generate ./...`), which imports the `internal/content` package and renders the skill. The generated file is committed to the repo and embedded via `embed.FS`.

`anvil skills status --check` regenerates in-memory at runtime and compares against the embedded file, failing CI if they differ. `anvil skills update` uses the same in-binary generator to refresh the installed skill regardless of what was committed.

**Considered alternative:** Generate SKILL.md at build time (goreleaser hook). Rejected because it complicates the release pipeline and makes local builds potentially produce different skill content than CI builds. A committed-and-checked file is simpler and the CI guard catches staleness before it ships.
