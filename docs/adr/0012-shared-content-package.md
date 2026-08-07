# Agent-facing content lives in a dedicated `internal/content` package

Description, command list, and contextual tips — text that appears in both the dashboard and the skill — live in an `internal/content` package as Go structs. Both `runHome` (dashboard rendering) and `cmd/skillgen` (skill generator) import this package as the single source of truth.

Cobra `Short`/`Long` help text is independent and can be more verbose for human `--help`. The `content` package is the agent-facing vocabulary; Cobra is the human-facing one. A CI check ensures the generated skill never drifts from what the current binary's content package would produce.

**Considered alternative:** Extract command names and help text from the Cobra command tree at runtime. Rejected because it couples the skill format to Cobra internals and makes it harder to tune agent-facing descriptions independently of `--help` output.
