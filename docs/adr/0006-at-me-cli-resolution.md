# @me resolution in the command layer

`@me` is a CLI-level variable that resolves to the currently authenticated user's login. Resolution happens in the command layer — a helper scans `--assignee` values for `@me`, calls `forge.CurrentUser(ctx)` to get the actual username, and replaces it before passing clean data to the forge adapter. The forge adapter never sees `@me`. This keeps `@me` as a CLI concept (forges don't have it) and avoids duplicating resolution logic across three adapters. `@me` works on every `--assignee` flag (`issue create`, `issue update`, `issue list`) and any future login-bearing flag. If the user isn't authenticated, it produces the standard auth error pointing to `anvil auth set`.

**Status**: accepted
**Considered Options**: resolve in command layer vs resolve in forge adapter. The adapter approach would mean three separate implementations and would conflate a CLI concept with the forge abstraction. The `Forge` interface exposes a `CurrentUser(ctx) (string, error)` method (or a lightweight `UserService`) that each adapter implements natively via `GET /user`.
