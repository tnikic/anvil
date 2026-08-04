# Label auto-create and idempotent label creation

When `anvil issue create --label` or `issue update --label` references a label that doesn't exist on the forge, anvil auto-creates it rather than erroring. This is the default — no opt-in flag needed. Auto-created labels get a fixed placeholder color `#333333` (dark gray) and empty description, making them conspicuously "unfinished" on the forge UI. The TOON confirmation includes an `auto_created_labels` field so agents know to follow up with `anvil label update`. Separately, `anvil label create` is idempotent: it acts as a create-or-update with partial merge — provided flags overwrite existing values, unprovided flags leave them alone. This means `label create --scope kind --name bug` is a safe no-op if the label already exists, and `label create --scope kind --name bug --color ff0000` updates only the color.

**Status**: accepted
**Considered Options**: auto-create-by-default vs opt-in `--auto-create-labels` flag; deterministic color hash vs fixed placeholder. The fixed conspicuous color was chosen over a hash because a hash could produce a plausible-looking color and the label would never get refined.
