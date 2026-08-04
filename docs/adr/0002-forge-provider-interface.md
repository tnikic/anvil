# Small service interfaces with normalize+extras pattern

The forge abstraction is three small interfaces — `IssueService`, `LabelService`, `PRService` — accessed through a `Forge` facade, rather than one large interface. Domain types use a **normalize + Extras** pattern: fields present in ≥2 forges are normalized into typed struct fields; forge-specific fields go into an `Extras map[string]any`.

This keeps each service cohesive and testable, avoids forcing every adapter to stub operations it doesn't support, and makes it obvious which fields are portable across forges vs. which are platform-specific. TOON conversion happens at the CLI output boundary — the provider layer never touches TOON, only Go structs.

**Status:** accepted

**Considered Options:**
- **One large `Forge` interface** — simpler to discover, but forces all adapters to implement every method and creates a single massive file. Rejected.
- **Pure passthrough (no normalization)** — each adapter returns its own types, CLI handles the differences. Rejected: pushes forge-specific logic into every command.
- **Full normalization (no Extras)** — only the intersection of all forges. Rejected: loses valuable forge-specific data that agents may need via `--fields`.
