# Issue relationship CLI surface

Issue relationships (blocked_by, blocking, parent, children) use a hybrid CLI: separate read commands for discovery (`anvil issue blocked-by`, `anvil issue blocking`, `anvil issue children`, `anvil issue parent`) and a unified mutation subcommand (`anvil issue relation add/remove`). Read commands are terse and discoverable — they're the high-traffic path ("what's blocking this?"). Mutation is unified under one subcommand rather than proliferating flags on `issue update`. Relationships are deliberately not managed through `issue update` because they are edges between issues, not properties of one issue — a clean separation from field mutations (title, body, labels). In `issue view`, relationships appear as counts with hints (e.g., `blocked_by: 2 — use 'anvil issue blocked-by 42'`), consistent with the comments pattern.

**Status**: accepted
**Considered Options**: separate read commands vs unified `issue relation` for everything; `issue update` flags vs separate mutation command. Separate read commands win on discovery; separate mutation preserves the distinction between "change this issue" and "change how issues relate."
