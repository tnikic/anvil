# Issue blocking (blocked_by) normalization

Issue blocking is normalized as an `IssueDependency` (or `IssueRelation`) type with three canonical relationship directions: `blocks`, `is_blocked_by`, and `relates_to`. All three forges now support issue blocking natively: GitHub via the REST dependencies API (GA August 2025), GitLab via the Issue Links API, and Forgejo via Gitea-compatible dependencies/blocks endpoints. The type is separate from the `Issue` struct — relationships are edges between issues, not properties of one issue. The CLI exposes two read commands (`anvil issue blocked-by` and `anvil issue blocking`) and a unified mutation subcommand (`anvil issue relation add/remove`). A `Parent` field (nullable `int`) is partially normalized on `Issue` for sub-issues; children is a derived query (`WHERE parent = N`), not a stored field.

**Status**: accepted
**Considered Options**: separate `IssueDependency` type vs storing blockers as a field on `Issue`. The separate-type approach treats relationships as edges, matching the forge APIs' own model and allowing future expansion to `parent_of` and `relates_to` without bloating the issue struct.
