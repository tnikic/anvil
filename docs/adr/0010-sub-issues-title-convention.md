# Sub-issues via title prefix convention for forges without native parent/child API

Forgejo has no native parent/child sub-issues API (only blocking dependencies). To normalize sub-issues across all three forges, the Forgejo adapter uses a title prefix convention `[parent:N]` — the same pattern already used for PR stacks (`[stackname:N/M]`).

On read, the adapter strips the prefix from the title and stores the parent number in `Issue.Parent`. On create/update, if `Issue.Parent` is set, the adapter injects `[parent:N]` at the front of the title before sending it to the Forgejo API.

**Why title over body**: Visibility in issue lists (both CLI and web), and the codebase already has a proven title-based convention for PR stacks. The body convention (`<!-- parent: N -->`) was rejected because it's invisible in list views. Repurposing blocking relationships was rejected because blocking implies sequencing, not ownership.
