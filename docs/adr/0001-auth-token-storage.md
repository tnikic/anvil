# Single-file auth token storage with no environment variables

`anvil` stores all auth tokens in a single JSON file at `$XDG_CACHE_HOME/anvil/credentials.json`, keyed by host. Environment variables are deliberately excluded as a token source — the file is the single source of truth.

This avoids the silent-precedence bugs that come with env-var + file fallback chains, and gives agents a single, predictable place to manage credentials. The forge type (GitHub, GitLab, Forgejo) is inferred from the host at runtime, so the file needs only the host and token.

**Status:** accepted

**Considered Options:**
- **Environment variables only** — familiar pattern (`GITHUB_TOKEN`), but forces agents to manage tokens in the shell environment and creates ambiguity when multiple forges are configured.
- **File + env var fallback** — checks env var first, then file. Rejected: creates silent precedence bugs where an agent sets the file but an old env var overrides it.
- **Single file, keyed by host** — chosen. One place to read and write, one command to manage.
