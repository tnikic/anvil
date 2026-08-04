# Test portfolio — anvil

## Static analysis

**Formatter:** `go tool gofumpt -l .` (check mode) / `go tool gofumpt -w .` (apply) — runs in CI on every push. Opinionated, gofmt-compatible formatter with stricter rules (modern octal literals, consistent spacing, no empty lines at block boundaries).

**Linter:** `go tool golangci-lint run ./...` — runs in CI with the default preset (errcheck, gosimple, govet, ineffassign, staticcheck, unused). Version pinned in `go.mod` via `tool` directive (v2, module path `github.com/golangci/golangci-lint/v2`).

**Typecheck:** `go vet ./...` — also runs as part of `go test` and inside `golangci-lint`. Listed separately for fast standalone checks during development.

**Vulnerability scan:** `go tool govulncheck ./...` — scans dependencies against the Go vulnerability database. Runs in CI. Version pinned in `go.mod` via `tool` directive.

## Test suite

**Unit tests:** `go test ./internal/...` — covers `auth` (token storage, resolution, forge type inference), `forge` (Detect orchestration, ParseRemote for HTTPS/SSH/edge cases), `format` (TOON formatting helpers), and `stack` (PR stack ordering). No external dependencies; everything outside `commands/` is unit-tested in isolation.

**Command-level tests:** `go test ./internal/commands/...` — every subcommand exercised through cobra's `Execute()`, using `FakeForge` (an in-memory `forge.Forge` implementation) to intercept adapter calls. The `skills` subcommand is tested with real filesystem operations via `t.TempDir`. Covers: flag parsing, argument validation, structured error output, TOON formatting, auth error messaging, unknown flag detection, `embed.FS` extraction. This is the heaviest layer — it exercises the full path from `cobra.Command` → `wrapForge` → handler → `forge.Forge` interface → `printFormatted`.

**Contract tests:** `go test ./internal/forge/... -run Contract` — verifies FakeForge and the GitHub adapter produce identical shapes for labels, issues, and errors. Uses `httptest.Server` for the GitHub side so no real network is hit. Covers: label normalization (scoped/unscoped), issue field consistency (list + get), structured error contracts (not found, auth failure), and label CRUD identity.

**Smoke tests:** `go test ./cmd/anvil/...` — compiles the binary (`go build`), runs it, and asserts exit codes and output. Covers `main()`'s three exit paths: exit 0 (`--help`, `--forge`/`--repo` home view, subcommand `--help`), exit 1 (no flags outside a git repo — Detect failure), exit 2 (unknown flag — usage error). No forge API is hit.

**Integration tests:** `go test ./cmd/anvil/ -run '^TestIntegration'` — in-process tests that exercise the full stack (cobra → resolveForge → ForgeFn → adapter → HTTP → `httptest.Server`) without compiling the binary. Wires real GitHub and GitLab adapters to a local test server, verifies both adapters dispatch correctly and produce correct output. Covers: issue list/view, label list, 404 not found → structured error, auth failure (401) → structured error, forge dispatch (github.com → GitHub adapter, gitlab.com → GitLab adapter). The tipping point was reached when two adapters (GitHub, GitLab) were registered in `adapterConstructors`.

Run everything: `go test -race ./...` — the `-race` flag enables the Go race detector; also used in CI.

## Future

| Test type / tool | When | Why |
|---|---|---|
| End-to-end test | When CI runs against a real forge instance | Verify real API responses match expectations; too heavy for local iteration |
