package commands_test

import (
	"strings"
	"testing"

	"github.com/tnikic/anvil/internal/forge"
	"github.com/tnikic/anvil/internal/forge/forgetest"
)

// ---- Setup with label service ----

func setupForgeTestWithLabels(t *testing.T) (*forgetest.FakeLabelService, *forgetest.FakeForge) {
	t.Helper()
	fk := forgetest.Setup(t)
	return fk.LabelSvc, fk
}

// ---- Tests ----

func TestLabelListDefaultOutput(t *testing.T) {
	labelSvc, fk := setupForgeTestWithLabels(t)
	_ = fk
	labelSvc.Labels = []forge.Label{
		{Name: "bug", Scope: "kind", Color: "d73a4a", Description: "Something broken"},
		{Name: "enhancement", Scope: "kind", Color: "a2eeef", Description: "New feature"},
		{Name: "good-first-issue", Scope: "", Color: "7057ff", Description: "Good for newcomers"},
	}

	buf, err := runCmd("label", "list", "--forge", "github.com", "--repo", "test/repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "bug") {
		t.Errorf("should contain 'bug', got: %s", out)
	}
	if !strings.Contains(out, "kind") {
		t.Errorf("should contain scope 'kind', got: %s", out)
	}
	if !strings.Contains(out, "d73a4a") {
		t.Errorf("should contain color, got: %s", out)
	}
	if !strings.Contains(out, "Something broken") {
		t.Errorf("should contain description, got: %s", out)
	}
	if !strings.Contains(out, "good-first-issue") {
		t.Errorf("should contain unscoped label, got: %s", out)
	}
	if !strings.Contains(out, "3 labels") {
		t.Errorf("should show count: '3 labels', got: %s", out)
	}
}

func TestLabelListFields(t *testing.T) {
	labelSvc, fk := setupForgeTestWithLabels(t)
	_ = fk
	labelSvc.Labels = []forge.Label{
		{Name: "bug", Scope: "kind", Color: "d73a4a", Description: "Bug", Exclusive: true},
	}

	buf, err := runCmd("label", "list",
		"--forge", "github.com", "--repo", "test/repo",
		"--fields", "name,scope,color,description,exclusive",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "1 labels") {
		t.Errorf("should show count, got: %s", out)
	}
}

func TestLabelListEmpty(t *testing.T) {
	_, fk := setupForgeTestWithLabels(t)
	_ = fk
	// No labels set → empty slice returned

	buf, err := runCmd("label", "list", "--forge", "github.com", "--repo", "test/repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "0 labels") {
		t.Errorf("should show '0 labels' for empty list, got: %s", out)
	}
}

func TestLabelCreateScoped(t *testing.T) {
	labelSvc, fk := setupForgeTestWithLabels(t)
	_ = fk

	buf, err := runCmd("label", "create",
		"--forge", "github.com", "--repo", "test/repo",
		"--scope", "kind",
		"--name", "bug",
		"--color", "#d73a4a",
		"--description", "Something broken",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "bug") {
		t.Errorf("should contain label name, got: %s", out)
	}
	if !strings.Contains(out, "created") {
		t.Errorf("should contain 'created', got: %s", out)
	}

	// Verify the adapter was called correctly
	opts := labelSvc.LastCreateOpts
	if opts.Name != "bug" {
		t.Errorf("Name should be 'bug', got: %q", opts.Name)
	}
	if opts.Scope == nil || *opts.Scope != "kind" {
		t.Errorf("Scope should be 'kind', got: %v", opts.Scope)
	}
	if opts.Color == nil || *opts.Color != "d73a4a" {
		t.Errorf("Color should be 'd73a4a' (without #), got: %v", opts.Color)
	}
	if opts.Description == nil || *opts.Description != "Something broken" {
		t.Errorf("Description should be 'Something broken', got: %v", opts.Description)
	}
}

func TestLabelCreateUnscoped(t *testing.T) {
	labelSvc, fk := setupForgeTestWithLabels(t)
	_ = fk

	buf, err := runCmd("label", "create",
		"--forge", "github.com", "--repo", "test/repo",
		"--name", "unscoped-label",
		"--color", "#0052cc",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "unscoped-label") {
		t.Errorf("should contain unscoped label name, got: %s", out)
	}

	opts := labelSvc.LastCreateOpts
	if opts.Scope != nil {
		t.Errorf("Scope should be nil for unscoped label, got: %v", opts.Scope)
	}
	if opts.Color == nil || *opts.Color != "0052cc" {
		t.Errorf("Color should be '0052cc', got: %v", opts.Color)
	}
}

func TestLabelCreateMissingName(t *testing.T) {
	_, _ = setupForgeTestWithLabels(t)

	_, err := runCmd("label", "create",
		"--forge", "github.com", "--repo", "test/repo",
		"--color", "#ff0000",
	)
	if err == nil {
		t.Fatal("expected error for missing --name")
	}

	msg := err.Error()
	if !strings.Contains(msg, "missing required flag") || !strings.Contains(msg, "name") {
		t.Errorf("should show missing name error, got: %s", msg)
	}
}

func TestLabelUpdate(t *testing.T) {
	labelSvc, fk := setupForgeTestWithLabels(t)
	_ = fk
	labelSvc.Labels = []forge.Label{
		{Name: "bug", Scope: "kind", Color: "d73a4a", Description: "Something broken"},
	}

	buf, err := runCmd("label", "update", "bug",
		"--forge", "github.com", "--repo", "test/repo",
		"--scope", "kind",
		"--color", "#ff0000",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "bug") {
		t.Errorf("should contain label name, got: %s", out)
	}
	if !strings.Contains(out, "updated") {
		t.Errorf("should contain confirmation, got: %s", out)
	}

	opts := labelSvc.LastUpdateOpts
	if opts.Name != "bug" {
		t.Errorf("Name should be 'bug', got: %q", opts.Name)
	}
	if opts.Scope != "kind" {
		t.Errorf("Scope should be 'kind', got: %q", opts.Scope)
	}
	if opts.Color == nil || *opts.Color != "ff0000" {
		t.Errorf("Color should be 'ff0000', got: %v", opts.Color)
	}
}

func TestLabelUpdateUnscoped(t *testing.T) {
	labelSvc, fk := setupForgeTestWithLabels(t)
	_ = fk
	labelSvc.Labels = []forge.Label{
		{Name: "good-first-issue", Scope: "", Color: "7057ff", Description: "Good for newcomers"},
	}

	buf, err := runCmd("label", "update", "good-first-issue",
		"--forge", "github.com", "--repo", "test/repo",
		"--description", "Updated description",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "good-first-issue") {
		t.Errorf("should contain label name, got: %s", out)
	}

	opts := labelSvc.LastUpdateOpts
	if opts.Scope != "" {
		t.Errorf("Scope should be empty for unscoped label, got: %q", opts.Scope)
	}
	if opts.Description == nil || *opts.Description != "Updated description" {
		t.Errorf("Description should be updated, got: %v", opts.Description)
	}
}

func TestLabelUpdateRename(t *testing.T) {
	labelSvc, fk := setupForgeTestWithLabels(t)
	_ = fk
	labelSvc.Labels = []forge.Label{
		{Name: "bug", Scope: "kind", Color: "d73a4a"},
	}

	buf, err := runCmd("label", "update", "bug",
		"--forge", "github.com", "--repo", "test/repo",
		"--scope", "kind",
		"--new-name", "defect",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "defect") {
		t.Errorf("should contain new name, got: %s", out)
	}

	opts := labelSvc.LastUpdateOpts
	if opts.NewName == nil || *opts.NewName != "defect" {
		t.Errorf("NewName should be 'defect', got: %v", opts.NewName)
	}
}

func TestLabelUpdateEmptyArgs(t *testing.T) {
	_, _ = setupForgeTestWithLabels(t)

	_, err := runCmd("label", "update", "--forge", "github.com", "--repo", "test/repo")
	if err == nil {
		t.Fatal("expected error for missing name arg")
	}
}

func TestLabelSubcommandHelp(t *testing.T) {
	_, _ = setupForgeTestWithLabels(t)

	subs := []string{"list", "create", "update"}
	for _, sub := range subs {
		t.Run(sub, func(t *testing.T) {
			buf, err := runCmd("label", sub, "--help")
			if err != nil {
				t.Fatalf("unexpected error for label %s --help: %v", sub, err)
			}
			out := buf.String()
			if out == "" {
				t.Errorf("help output for label %s should not be empty", sub)
			}
		})
	}
}

func TestLabelListExclusiveField(t *testing.T) {
	labelSvc, fk := setupForgeTestWithLabels(t)
	_ = fk
	labelSvc.Labels = []forge.Label{
		{Name: "bug", Scope: "kind", Color: "d73a4a", Description: "Bug", Exclusive: true},
		{Name: "feature", Scope: "kind", Color: "a2eeef", Description: "Feature", Exclusive: false},
	}

	buf, err := runCmd("label", "list",
		"--forge", "github.com", "--repo", "test/repo",
		"--fields", "exclusive",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	// The exclusive field should be present in extended output
	if !strings.Contains(out, "true") {
		t.Errorf("should show exclusive=true for scoped exclusive label, got: %s", out)
	}
	if !strings.Contains(out, "2 labels") {
		t.Errorf("should show count, got: %s", out)
	}
}

func TestLabelListEmptyScopeCell(t *testing.T) {
	labelSvc, fk := setupForgeTestWithLabels(t)
	_ = fk
	labelSvc.Labels = []forge.Label{
		{Name: "unscoped", Scope: "", Color: "cccccc", Description: "No scope"},
		{Name: "bug", Scope: "kind", Color: "d73a4a", Description: "Has scope"},
	}

	buf, err := runCmd("label", "list", "--forge", "github.com", "--repo", "test/repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	// Both labels should appear; unscoped label should have empty scope
	if !strings.Contains(out, "unscoped") {
		t.Errorf("should contain unscoped label, got: %s", out)
	}
	if !strings.Contains(out, "bug") {
		t.Errorf("should contain scoped label, got: %s", out)
	}
	if !strings.Contains(out, "2 labels") {
		t.Errorf("should show count, got: %s", out)
	}
}

func TestLabelListLimit(t *testing.T) {
	labelSvc, fk := setupForgeTestWithLabels(t)
	_ = fk
	labelSvc.Labels = []forge.Label{
		{Name: "bug", Scope: "kind", Color: "d73a4a"},
	}

	_, err := runCmd("label", "list",
		"--forge", "github.com", "--repo", "test/repo",
		"--limit", "50",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if labelSvc.LastListOpts.Limit != 50 {
		t.Errorf("Limit should be 50, got: %d", labelSvc.LastListOpts.Limit)
	}
}

// ---- Idempotent label create tests ----

func TestLabelCreateIdempotentExistingNoChanges(t *testing.T) {
	labelSvc, fk := setupForgeTestWithLabels(t)
	_ = fk
	labelSvc.Labels = []forge.Label{
		{Name: "bug", Scope: "kind", Color: "d73a4a", Description: "Something broken"},
	}

	// Call create with the same name/scope but no new flags — should be a no-op.
	_, err := runCmd("label", "create",
		"--forge", "github.com", "--repo", "test/repo",
		"--scope", "kind",
		"--name", "bug",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// No-op: creating an existing label with no changes returns nil silently.
}

func TestLabelCreateIdempotentExistingPartialMerge(t *testing.T) {
	labelSvc, fk := setupForgeTestWithLabels(t)
	_ = fk
	labelSvc.Labels = []forge.Label{
		{Name: "bug", Scope: "kind", Color: "d73a4a", Description: "Something broken"},
	}

	// Call create with the same name/scope, providing only a new color.
	buf, err := runCmd("label", "create",
		"--forge", "github.com", "--repo", "test/repo",
		"--scope", "kind",
		"--name", "bug",
		"--color", "#ff0000",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "updated") {
		t.Errorf("should show 'updated' for partial merge on existing label, got: %s", out)
	}

	// Verify that Update was called (not Create).
	opts := labelSvc.LastUpdateOpts
	if opts.Name != "bug" {
		t.Errorf("Name should be 'bug', got: %q", opts.Name)
	}
	if opts.Scope != "kind" {
		t.Errorf("Scope should be 'kind', got: %q", opts.Scope)
	}
	if opts.Color == nil || *opts.Color != "ff0000" {
		t.Errorf("Color should be 'ff0000', got: %v", opts.Color)
	}
	// Description should not be set (partial merge — only provided fields).
	if opts.Description != nil {
		t.Errorf("Description should be nil (not provided), got: %v", opts.Description)
	}
}

func TestLabelCreateIdempotentNewLabelStillCreates(t *testing.T) {
	labelSvc, fk := setupForgeTestWithLabels(t)
	_ = fk

	// Label doesn't exist — should create normally.
	buf, err := runCmd("label", "create",
		"--forge", "github.com", "--repo", "test/repo",
		"--scope", "kind",
		"--name", "feature",
		"--color", "#a2eeef",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "created") {
		t.Errorf("should show 'created' for new label, got: %s", out)
	}

	// Verify Create was called.
	opts := labelSvc.LastCreateOpts
	if opts.Name != "feature" {
		t.Errorf("Name should be 'feature', got: %q", opts.Name)
	}
}

func TestLabelCreateIdempotentUnscoped(t *testing.T) {
	labelSvc, fk := setupForgeTestWithLabels(t)
	_ = fk
	labelSvc.Labels = []forge.Label{
		{Name: "good-first-issue", Scope: "", Color: "7057ff", Description: "Good for newcomers"},
	}

	// Create with same unscoped name, providing description only — partial merge.
	buf, err := runCmd("label", "create",
		"--forge", "github.com", "--repo", "test/repo",
		"--name", "good-first-issue",
		"--description", "Updated desc",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "updated") {
		t.Errorf("should show 'updated' for existing unscoped label, got: %s", out)
	}

	opts := labelSvc.LastUpdateOpts
	if opts.Scope != "" {
		t.Errorf("Scope should be empty for unscoped label, got: %q", opts.Scope)
	}
	if opts.Description == nil || *opts.Description != "Updated desc" {
		t.Errorf("Description should be 'Updated desc', got: %v", opts.Description)
	}
}
