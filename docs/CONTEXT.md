# anvil

An AXI-compliant, agent-first CLI for interacting with Git forges — GitHub, GitLab, Forgejo — through TOON output on stdout.

## Language

**Forge**:
A Git hosting platform (GitHub, GitLab, Forgejo, Gitea). Identified at runtime from the git remote or a `--forge` flag.
_Avoid_: Platform, provider, host (host means the domain, not the platform)

**Host**:
The domain of a forge instance — `github.com`, `gitlab.com`, `gitlab.mycorp.com`. Auth tokens are keyed by host.
_Avoid_: Server, URL, endpoint

**Provider / Adapter**:
A forge-specific implementation of the `Forge` interface. "Provider" is the interface abstraction; "adapter" is the concrete implementation (GitHub adapter, GitLab adapter).
_Avoid_: Driver, plugin, backend

**Issue**:
A tracked work item on a forge. Has number, title, state (open/closed), body, labels, author, timestamps.
_Avoid_: Ticket (used for planning-phase issues on the repo itself)

**Label**:
A tag applied to issues and PRs, optionally scoped. Scoped labels use two parts (scope + name) that translate to each forge's native format (`kind:bug` on GitHub, `kind::bug` on GitLab, `kind/bug` on Forgejo).
_Avoid_: Tag, category

**Scope**:
A label namespace prefix — e.g., `kind` in `kind:bug`. Scopes group mutually-exclusive or related labels. Unscoped labels have an empty scope.
_Avoid_: Namespace, prefix, group

**PR (Pull Request)**:
A request to merge code from a head branch into a base branch. Called "merge request" on GitLab; normalized as PR throughout.
_Avoid_: Merge request, MR, patch

**Stack**:
A linear chain of dependent PRs sharing a name (e.g., `auth-fix`). Identified by title prefix `[stackname:N/M]`; no external metadata. PRs within a stack depend on each other via `base.ref` ordering.
_Avoid_: Chain, series, queue, patchset

**TOON**:
Token-Oriented Object Notation — the output format `anvil` writes to stdout. A key-value and tabular text format designed for minimal token consumption by AI agents.
_Avoid_: JSON, YAML, structured output (these are the alternatives, not synonyms)

**AXI**:
Agent eXperience Interface — the design standard `anvil` follows. Governs output format, error handling, flag design, and other ergonomics for agent-facing CLIs.
_Avoid_: CLI guidelines, agent standards

**Comment**:
A user-written message on an issue. GitHub and Forgejo call them "comments"; GitLab calls them "notes". The normalized term is "Comment". System-generated entries (GitLab's `System: true` notes, GitHub timeline events) are not user comments.
_Avoid_: Note (use for GitLab-specific references only)

**Issue Relationship**:
A directed edge between two issues. Three canonical types: `blocks` (source blocks target), `parent_of` (source is parent of target), `relates_to` (general bi-directional link). Type is always from the source perspective.
_Avoid_: Link (use for GitLab-specific references only), dependency (ambiguous direction)

**Blocked-by**:
A relationship where an issue cannot be completed until its blockers are resolved. "Issue A is blocked by Issue B" means B must close first. The inverse is **blocking** — B blocks A. Stored as a directed edge with type `blocks`.
_Avoid_: Dependency (ambiguous — which direction?), prerequisite (only one side of the relationship)

**Sub-issue**:
GitHub's hierarchical parent/child concept — a child issue that belongs to a parent. Partially normalized: `Parent` (nullable int) on `Issue`; children is a derived query. Forgejo simulates via convention. Distinct from blocked-by (sub-issues imply ownership, blocked-by implies sequencing).
_Avoid_: Child issue (use only when the forge has a native parent/child model)

**@me**:
A CLI-level variable that resolves to the currently authenticated user's login. Resolved in the command layer via `CurrentUser()` before assignee lists reach the forge adapter. Works on every `--assignee` flag and any future login-bearing field.
_Avoid_: Current user (that's the resolved value, not the placeholder)

**Credentials**:
Auth tokens stored per-host in `$XDG_CACHE_HOME/anvil/credentials.json`. A single JSON file, no environment variables.
_Avoid_: Token, secret, API key (these refer to the value; "credentials" is the storage concept)
