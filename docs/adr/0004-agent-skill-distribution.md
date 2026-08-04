# Embedded agent skill distributed via binary, not npx skills add

anvil ships its Agent Skill (`SKILL.md`) embedded in the Go binary via `embed.FS`. Users install the skill by running `anvil skills install`, which extracts it to `~/.agents/skills/anvil/`. Binary releases use goreleaser to cross-compile for macOS and Linux (amd64 + arm64) and publish to GitHub Releases, with an `install.sh` script shipped in the repo for one-line installation.

This keeps the skill version-locked to the binary (no drift), avoids a dependency on the `npx skills` CLI or any third-party skill registry, and follows the pattern established by ecspresso. Rather than adopting an external library, the embed-and-extract logic is implemented directly — the behaviour is self-contained and owning it outright eliminates the ongoing maintenance risk of a third-party dependency.

**Status:** accepted

**Considered Options:**
- **Skillsmith library** — proven, but adds a dependency for what amounts to `embed.FS` + file copy logic. Rejected: the logic is not complex enough to justify a third-party dependency with its own release cadence.
- **`npx skills add tnikic/anvil`** — the ecosystem standard, but requires Node.js/npm on the user's machine and ties skill discovery to a registry. Rejected: Go users may not have Node; the binary should be self-sufficient.
- **Separate skill repo** — cleanest separation, but two repos to version, tag, and release in lockstep. Rejected: drift risk.
- **Embedded + goreleaser** — chosen. Binary and skill are a single artifact, versioned together, and distributed through a single release pipeline.
