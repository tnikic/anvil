package forge_test

import (
	"context"
	"testing"

	"github.com/tnikic/anvil/internal/forge"
	"github.com/tnikic/anvil/internal/forge/forgetest"
)

// ---- Tests ----

func TestFakeForgeSatisfiesInterface(t *testing.T) {
	// The compile-time assertion in forgetest already verifies this.
	// This test exercises the interface at runtime.
	ctx := context.Background()

	fake := forgetest.NewFakeForge()
	fake.IssueSvc.ListFn = func(ctx context.Context, opts forge.IssueListOptions) ([]forge.Issue, *forge.ListMeta, error) {
		return []forge.Issue{
			{Number: 1, Title: "Hello", State: "open"},
		}, &forge.ListMeta{Total: 1, Count: 1}, nil
	}

	issues, meta, err := fake.Issues().List(ctx, forge.IssueListOptions{State: "open"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(issues))
	}
	if issues[0].Number != 1 {
		t.Errorf("expected issue number 1, got %d", issues[0].Number)
	}
	if meta.Total != 1 || meta.Count != 1 {
		t.Errorf("expected meta {Total: 1, Count: 1}, got %+v", meta)
	}
}

func TestFakeIssueServiceAllMethods(t *testing.T) {
	ctx := context.Background()
	svc := &forgetest.FakeIssueService{}
	svc.GetFn = func(ctx context.Context, opts forge.IssueGetOptions) (*forge.Issue, error) {
		return &forge.Issue{Number: opts.Number, Title: "Test", State: "open"}, nil
	}
	svc.CreateFn = func(ctx context.Context, opts forge.IssueCreateOptions) (*forge.Issue, error) {
		return &forge.Issue{Number: 42, Title: *opts.Title, State: "open"}, nil
	}
	svc.UpdateFn = func(ctx context.Context, opts forge.IssueUpdateOptions) (*forge.Issue, error) {
		return &forge.Issue{Number: opts.Number, Title: *opts.Title, State: "open"}, nil
	}
	svc.CloseFn = func(ctx context.Context, opts forge.IssueCloseOptions) (*forge.Issue, error) {
		return &forge.Issue{Number: opts.Number, State: "closed"}, nil
	}
	svc.ReopenFn = func(ctx context.Context, opts forge.IssueReopenOptions) (*forge.Issue, error) {
		return &forge.Issue{Number: opts.Number, State: "open"}, nil
	}

	issue, err := svc.Get(ctx, forge.IssueGetOptions{Number: 5})
	if err != nil || issue.Number != 5 {
		t.Errorf("Get: expected number 5, got %+v, err=%v", issue, err)
	}

	title := "New Issue"
	created, err := svc.Create(ctx, forge.IssueCreateOptions{Title: &title})
	if err != nil || created.Number != 42 || created.Title != "New Issue" {
		t.Errorf("Create: got %+v, err=%v", created, err)
	}

	newTitle := "Updated"
	updated, err := svc.Update(ctx, forge.IssueUpdateOptions{Number: 1, Title: &newTitle})
	if err != nil || updated.Number != 1 || updated.Title != "Updated" {
		t.Errorf("Update: got %+v, err=%v", updated, err)
	}

	closed, err := svc.Close(ctx, forge.IssueCloseOptions{Number: 1})
	if err != nil || closed.State != "closed" {
		t.Errorf("Close: got %+v, err=%v", closed, err)
	}

	reopened, err := svc.Reopen(ctx, forge.IssueReopenOptions{Number: 1})
	if err != nil || reopened.State != "open" {
		t.Errorf("Reopen: got %+v, err=%v", reopened, err)
	}
}

func TestPointerHelpers(t *testing.T) {
	s := forge.String("hello")
	if s == nil || *s != "hello" {
		t.Errorf("String: got %v", s)
	}

	i := forge.Int(42)
	if i == nil || *i != 42 {
		t.Errorf("Int: got %v", i)
	}

	b := forge.Bool(true)
	if b == nil || *b != true {
		t.Errorf("Bool: got %v", b)
	}

	b2 := forge.Bool(false)
	if b2 == nil || *b2 != false {
		t.Errorf("Bool(false): got %v", b2)
	}
}

func TestDomainTypesConstructable(t *testing.T) {
	// Verify domain types can be constructed with all normalized fields.
	issue := forge.Issue{
		Number: 1,
		Title:  "Test Issue",
		State:  "open",
		Body:   "Description",
		Labels: []forge.Label{
			{Name: "bug", Scope: "kind", Color: "ff0000", Exclusive: true},
		},
		Author: "user",
		URL:    "https://github.com/owner/repo/issues/1",
		Extras: map[string]any{"reactions": 3},
	}
	if issue.Number != 1 {
		t.Error("issue.Number mismatch")
	}
	if len(issue.Labels) != 1 {
		t.Error("issue.Labels length mismatch")
	}

	label := forge.Label{
		Name:        "bug",
		Scope:       "kind",
		Color:       "ff0000",
		Description: "Something isn't working",
		Exclusive:   true,
		Extras:      map[string]any{"is_default": false},
	}
	if label.Name != "bug" || label.Scope != "kind" {
		t.Error("label fields mismatch")
	}
	if !label.Exclusive {
		t.Error("label.Exclusive should be true")
	}

	pr := forge.PR{
		Number:       10,
		Title:        "[auth:1/2] Add OAuth",
		State:        "open",
		BaseRef:      "main",
		HeadRef:      "feat/auth",
		Stack:        "auth",
		DependsOn:    nil,
		DependedOnBy: []int{11},
		Author:       "dev",
		URL:          "https://github.com/owner/repo/pull/10",
	}
	if pr.Number != 10 || pr.Stack != "auth" {
		t.Error("pr fields mismatch")
	}
	if len(pr.DependedOnBy) != 1 || pr.DependedOnBy[0] != 11 {
		t.Error("pr.DependedOnBy mismatch")
	}
}

func TestLabelCreateOptionsPointerConvention(t *testing.T) {
	// Verify that optional fields use pointers, so "unset" is distinguishable from zero value.
	opts := forge.LabelCreateOptions{
		Name:        "bug",
		Scope:       forge.String("kind"),
		Color:       forge.String("ff0000"),
		Description: forge.String("A bug"),
		Exclusive:   forge.Bool(true),
	}
	if opts.Name != "bug" {
		t.Error("Name mismatch")
	}
	if opts.Scope == nil || *opts.Scope != "kind" {
		t.Error("Scope mismatch")
	}
	if opts.Exclusive == nil || *opts.Exclusive != true {
		t.Error("Exclusive mismatch")
	}

	// Unset fields should be nil, not zero values.
	minimal := forge.LabelCreateOptions{Name: "feature"}
	if minimal.Scope != nil {
		t.Error("Scope should be nil when unset")
	}
	if minimal.Exclusive != nil {
		t.Error("Exclusive should be nil when unset")
	}
	if minimal.Description != nil {
		t.Error("Description should be nil when unset")
	}
}

func TestStateConstants(t *testing.T) {
	if forge.StateOpen != "open" {
		t.Error("StateOpen")
	}
	if forge.StateClosed != "closed" {
		t.Error("StateClosed")
	}
	if forge.StateMerged != "merged" {
		t.Error("StateMerged")
	}
	if forge.StateAll != "all" {
		t.Error("StateAll")
	}
}

func TestLabelScopeAndExclusive(t *testing.T) {
	// Labels use Scope and Exclusive normalized across forges.
	// Scope may be empty for unscoped labels.
	scoped := forge.Label{
		Name:      "bug",
		Scope:     "priority",
		Color:     "d73a4a",
		Exclusive: true,
	}
	if scoped.Scope != "priority" {
		t.Error("scoped label: scope mismatch")
	}

	unscoped := forge.Label{
		Name:      "good-first-issue",
		Scope:     "",
		Color:     "7057ff",
		Exclusive: false,
	}
	if unscoped.Scope != "" {
		t.Error("unscoped label: scope should be empty")
	}
	if unscoped.Exclusive {
		t.Error("unscoped label: exclusive should be false")
	}
}

func TestListMetaFields(t *testing.T) {
	meta := forge.ListMeta{Total: 50, Count: 10}
	if meta.Total != 50 {
		t.Error("Total mismatch")
	}
	if meta.Count != 10 {
		t.Error("Count mismatch")
	}
}

func TestBaseError(t *testing.T) {
	// Verify BaseError satisfies StructuredError.
	var se forge.StructuredError = forge.NewBaseError("something failed", "try again")

	if se.Message() != "something failed" {
		t.Errorf("Message() = %q, want %q", se.Message(), "something failed")
	}
	if se.Help() != "try again" {
		t.Errorf("Help() = %q, want %q", se.Help(), "try again")
	}
	if se.Error() != "something failed" {
		t.Errorf("Error() = %q, want %q", se.Error(), "something failed")
	}

	// Empty help is valid.
	err := forge.NewBaseError("msg only", "")
	if err.Help() != "" {
		t.Errorf("Help() = %q, want empty", err.Help())
	}
}
