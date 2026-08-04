# Title-prefix-driven PR stacks with zero external metadata

PR stacks (linear dependent chains) are tracked entirely through title prefixes — `[stackname:N/M]` embedded in each PR's title — with no GitHub labels, no local files, and no external metadata store. Stack membership and ordering are reconstructed on `pr list` by parsing titles and walking `base.ref`. Titles are updated eagerly via the API when stack state changes (PR added or merged); merged and closed PRs are never edited, retaining their prefix as permanent history.

This is the lightest possible approach: zero setup, zero cleanup, and zero drift between stored state and reality. The cost is that PR titles carry visible prefix noise, and a broken stack (middle PR closed without merging) requires diagnosis and manual repair.

**Status:** accepted

**Considered Options:**
- **GitHub labels for stack identity** — a `stack:name` label on each PR. Cleaner titles, but requires label creation/cleanup, adds API calls, and labels can be removed accidentally.
- **Local metadata file** — a `.anvil/stacks.json` tracking state. Source of truth drift is inevitable, and it forces the tool to maintain state across clones.
- **Title prefix only** — chosen. The PR title is the single source of truth; no drift possible, no cleanup needed.
