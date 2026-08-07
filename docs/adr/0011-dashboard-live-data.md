# Dashboard shows live forge state, not static help

The `anvil` no-arguments view shows live data from the current forge — the 3 most recent open issues and 3 most recent open PRs — rather than a usage manual. This follows AXI §8 ("Content first"): when an agent sees actual state it can act immediately; when it sees help text, it has to make a second call.

Field schema is deliberately minimal (issue: number, title, state, author; PR: number, title, author) to keep token cost low. PR review state omitted to stay forge-agnostic. Count aggregates and definitive empty states prevent the agent from re-running to verify nothing was missed.

**Considered alternative:** Include PR review state. Rejected because GitLab's approval model differs from GitHub's — no single normalized field captures it without forge-specific translation, which would bloat the dashboard and break the "minimal default schema" rule.
