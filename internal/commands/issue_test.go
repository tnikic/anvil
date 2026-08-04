package commands_test

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/tnikic/anvil/internal/commands"
	"github.com/tnikic/anvil/internal/forge"
	"github.com/tnikic/anvil/internal/forge/forgetest"
)

// ---- Test helpers ----

func setupForgeTest(t *testing.T) *forgetest.FakeForge {
	t.Helper()
	return forgetest.Setup(t)
}

func runCmd(args ...string) (*bytes.Buffer, error) {
	cmd := commands.NewRoot()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return buf, err
}

// ---- Tests ----

func TestIssueListDefaultOutput(t *testing.T) {
	fk := setupForgeTest(t)
	fk.IssueSvc.Issues = []forge.Issue{
		{Number: 1, Title: "Fix login", State: "open", Author: "alice"},
		{Number: 2, Title: "Add rate limit", State: "closed", Author: "bob"},
	}

	buf, err := runCmd("issue", "list", "--forge", "github.com", "--repo", "test/repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Fix login") {
		t.Errorf("should contain issue title, got: %s", out)
	}
	if !strings.Contains(out, "Add rate limit") {
		t.Errorf("should contain second issue, got: %s", out)
	}
	if !strings.Contains(out, "2 of 2 total") {
		t.Errorf("should show count aggregate, got: %s", out)
	}
}

func TestIssueListFilterOptions(t *testing.T) {
	fk := setupForgeTest(t)
	fk.IssueSvc.Issues = []forge.Issue{
		{Number: 1, Title: "Bug", State: "closed"},
	}

	buf, err := runCmd("issue", "list",
		"--forge", "github.com", "--repo", "test/repo",
		"--state", "closed",
		"--label", "kind:bug",
		"--assignee", "alice",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Bug") {
		t.Errorf("should show filtered issue, got: %s", out)
	}

	opts := fk.IssueSvc.LastListOpts
	if opts.State != "closed" {
		t.Errorf("State should be 'closed', got: %q", opts.State)
	}
	if len(opts.Labels) != 1 || opts.Labels[0] != "kind:bug" {
		t.Errorf("Labels should contain 'kind:bug', got: %v", opts.Labels)
	}
}

func TestIssueListFields(t *testing.T) {
	fk := setupForgeTest(t)
	fk.IssueSvc.Issues = []forge.Issue{
		{Number: 1, Title: "Test", State: "open", Author: "alice"},
	}

	buf, err := runCmd("issue", "list",
		"--forge", "github.com", "--repo", "test/repo",
		"--fields", "number,title,state,author,labels",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "1 of 1 total") {
		t.Errorf("should show count, got: %s", out)
	}
}

func TestIssueListLimit(t *testing.T) {
	fk := setupForgeTest(t)
	fk.IssueSvc.Issues = []forge.Issue{
		{Number: 1, Title: "A", State: "open"},
		{Number: 2, Title: "B", State: "open"},
	}

	buf, err := runCmd("issue", "list",
		"--forge", "github.com", "--repo", "test/repo",
		"--limit", "50",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if fk.IssueSvc.LastListOpts.Limit != 50 {
		t.Errorf("Limit should be 50, got: %d", fk.IssueSvc.LastListOpts.Limit)
	}
	_ = buf
}

func TestIssueView(t *testing.T) {
	fk := setupForgeTest(t)
	fk.IssueSvc.Issues = []forge.Issue{
		{
			Number:    42,
			Title:     "Test Issue",
			State:     "open",
			Body:      "This is a test body with some content.",
			Author:    "alice",
			CreatedAt: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
			UpdatedAt: time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC),
			URL:       "https://github.com/test/repo/issues/42",
		},
	}

	buf, err := runCmd("issue", "view", "42", "--forge", "github.com", "--repo", "test/repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Test Issue") {
		t.Errorf("should contain issue title, got: %s", out)
	}
	if !strings.Contains(out, "open") {
		t.Errorf("should contain state, got: %s", out)
	}
	if !strings.Contains(out, "body_size") {
		t.Errorf("should contain body_size, got: %s", out)
	}
}

func TestIssueViewFull(t *testing.T) {
	fk := setupForgeTest(t)
	fk.IssueSvc.Issues = []forge.Issue{
		{
			Number: 42,
			Title:  "Long Body Issue",
			State:  "open",
			Body:   strings.Repeat("x", 600),
			URL:    "https://github.com/test/repo/issues/42",
		},
	}

	buf, err := runCmd("issue", "view", "42", "--full", "--forge", "github.com", "--repo", "test/repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "600") {
		t.Errorf("should show body_size 600, got: %s", out)
	}
}

func TestIssueViewTruncation(t *testing.T) {
	fk := setupForgeTest(t)
	body := strings.Repeat("x", 600)
	fk.IssueSvc.Issues = []forge.Issue{
		{
			Number: 42,
			Title:  "Truncated",
			State:  "open",
			Body:   body,
			URL:    "https://github.com/test/repo/issues/42",
		},
	}

	buf, err := runCmd("issue", "view", "42", "--forge", "github.com", "--repo", "test/repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if strings.Contains(out, body) {
		t.Errorf("body should be truncated without --full, got full body")
	}
	if !strings.Contains(out, "600") {
		t.Errorf("should show body_size 600, got: %s", out)
	}
	if !strings.Contains(out, "--full") {
		t.Errorf("should show --full hint, got: %s", out)
	}
}

func TestIssueCreate(t *testing.T) {
	_ = setupForgeTest(t)

	buf, err := runCmd("issue", "create",
		"--forge", "github.com", "--repo", "test/repo",
		"--title", "Fix bug",
		"--body", "Details...",
		"--label", "kind:bug",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Fix bug") {
		t.Errorf("should contain title, got: %s", out)
	}
	if !strings.Contains(out, "created") {
		t.Errorf("should contain 'created', got: %s", out)
	}
}

func TestIssueCreateMissingTitle(t *testing.T) {
	_ = setupForgeTest(t)

	_, err := runCmd("issue", "create",
		"--forge", "github.com", "--repo", "test/repo",
	)
	if err == nil {
		t.Fatal("expected error for missing --title")
	}

	msg := err.Error()
	if !strings.Contains(msg, "missing required flag") || !strings.Contains(msg, "title") {
		t.Errorf("should show missing title error, got: %s", msg)
	}
}

func TestIssueUpdate(t *testing.T) {
	fk := setupForgeTest(t)
	fk.IssueSvc.Issues = []forge.Issue{
		{Number: 42, Title: "Old Title", State: "open", URL: "https://github.com/test/repo/issues/42"},
	}

	buf, err := runCmd("issue", "update", "42",
		"--forge", "github.com", "--repo", "test/repo",
		"--title", "New title",
		"--state", "closed",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "New title") {
		t.Errorf("should contain updated title, got: %s", out)
	}

	opts := fk.IssueSvc.LastUpdateOpts
	if opts.Number != 42 {
		t.Errorf("should update issue 42, got: %d", opts.Number)
	}
	if opts.Title == nil || *opts.Title != "New title" {
		t.Errorf("title should be 'New title', got: %v", opts.Title)
	}
	if opts.State == nil || *opts.State != "closed" {
		t.Errorf("state should be 'closed', got: %v", opts.State)
	}
}

func TestIssueUpdateAddLabel(t *testing.T) {
	fk := setupForgeTest(t)
	fk.IssueSvc.Issues = []forge.Issue{
		{
			Number: 42, Title: "Test", State: "open",
			URL:    "https://github.com/test/repo/issues/42",
			Labels: []forge.Label{{Name: "bug", Scope: "kind"}},
		},
	}

	buf, err := runCmd("issue", "update", "42",
		"--forge", "github.com", "--repo", "test/repo",
		"--add-label", "kind:feature",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "updated") {
		t.Errorf("should contain 'updated', got: %s", out)
	}
	if !strings.Contains(out, "kind:bug") {
		t.Errorf("should show existing label kind:bug, got: %s", out)
	}
	if !strings.Contains(out, "kind:feature") {
		t.Errorf("should show added label kind:feature, got: %s", out)
	}

	// Verify the issue's labels were updated in the fake.
	issue := fk.IssueSvc.Issues[0]
	if len(issue.Labels) != 2 {
		t.Fatalf("expected 2 labels, got %d: %+v", len(issue.Labels), issue.Labels)
	}

	// Verify AddLabelCalls were recorded.
	if len(fk.IssueSvc.AddLabelCalls) != 1 {
		t.Fatalf("expected 1 AddLabelCalls entry, got %d", len(fk.IssueSvc.AddLabelCalls))
	}
	if fk.IssueSvc.AddLabelCalls[0][0] != "kind:feature" {
		t.Errorf("AddLabelCalls[0][0] = %q, want kind:feature", fk.IssueSvc.AddLabelCalls[0][0])
	}
}

func TestIssueUpdateRemoveLabel(t *testing.T) {
	fk := setupForgeTest(t)
	fk.IssueSvc.Issues = []forge.Issue{
		{
			Number: 42, Title: "Test", State: "open",
			URL: "https://github.com/test/repo/issues/42",
			Labels: []forge.Label{
				{Name: "bug", Scope: "kind"},
				{Name: "feature", Scope: "kind"},
			},
		},
	}

	buf, err := runCmd("issue", "update", "42",
		"--forge", "github.com", "--repo", "test/repo",
		"--remove-label", "kind:bug",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "updated") {
		t.Errorf("should contain 'updated', got: %s", out)
	}
	if strings.Contains(out, "kind:bug") {
		t.Errorf("should NOT contain removed label kind:bug, got: %s", out)
	}
	if !strings.Contains(out, "kind:feature") {
		t.Errorf("should still show kind:feature, got: %s", out)
	}

	// Verify RemoveLabelCalls were recorded.
	if len(fk.IssueSvc.RemoveLabelCalls) != 1 {
		t.Fatalf("expected 1 RemoveLabelCalls entry, got %d", len(fk.IssueSvc.RemoveLabelCalls))
	}
	if fk.IssueSvc.RemoveLabelCalls[0][0] != "kind:bug" {
		t.Errorf("RemoveLabelCalls[0][0] = %q, want kind:bug", fk.IssueSvc.RemoveLabelCalls[0][0])
	}
}

func TestIssueUpdateAddLabelRepeatable(t *testing.T) {
	fk := setupForgeTest(t)
	fk.IssueSvc.Issues = []forge.Issue{
		{Number: 42, Title: "Test", State: "open", URL: "https://github.com/test/repo/issues/42"},
	}

	_, err := runCmd("issue", "update", "42",
		"--forge", "github.com", "--repo", "test/repo",
		"--add-label", "kind:bug",
		"--add-label", "priority:high",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Both labels should be in the AddLabelCalls.
	if len(fk.IssueSvc.AddLabelCalls) != 1 {
		t.Fatalf("expected 1 AddLabelCalls entry, got %d", len(fk.IssueSvc.AddLabelCalls))
	}
	calls := fk.IssueSvc.AddLabelCalls[0]
	if len(calls) != 2 {
		t.Fatalf("expected 2 labels in call, got %d: %v", len(calls), calls)
	}
	if calls[0] != "kind:bug" || calls[1] != "priority:high" {
		t.Errorf("labels = %v, want [kind:bug priority:high]", calls)
	}

	// Issue should have both labels.
	issue := fk.IssueSvc.Issues[0]
	if len(issue.Labels) != 2 {
		t.Errorf("expected 2 labels on issue, got %d: %+v", len(issue.Labels), issue.Labels)
	}
}

func TestIssueUpdateRemoveLabelRepeatable(t *testing.T) {
	fk := setupForgeTest(t)
	fk.IssueSvc.Issues = []forge.Issue{
		{
			Number: 42, Title: "Test", State: "open",
			URL: "https://github.com/test/repo/issues/42",
			Labels: []forge.Label{
				{Name: "bug", Scope: "kind"},
				{Name: "high", Scope: "priority"},
			},
		},
	}

	_, err := runCmd("issue", "update", "42",
		"--forge", "github.com", "--repo", "test/repo",
		"--remove-label", "kind:bug",
		"--remove-label", "priority:high",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	calls := fk.IssueSvc.RemoveLabelCalls[0]
	if len(calls) != 2 {
		t.Fatalf("expected 2 labels in call, got %d: %v", len(calls), calls)
	}

	// Issue should have no labels.
	issue := fk.IssueSvc.Issues[0]
	if len(issue.Labels) != 0 {
		t.Errorf("expected 0 labels on issue, got %d: %+v", len(issue.Labels), issue.Labels)
	}
}

func TestIssueUpdateLabelMutualExclusion(t *testing.T) {
	_ = setupForgeTest(t)

	// --label + --add-label
	_, err := runCmd("issue", "update", "42",
		"--forge", "github.com", "--repo", "test/repo",
		"--label", "kind:bug",
		"--add-label", "priority:high",
	)
	if err == nil {
		t.Fatal("expected error when combining --label and --add-label")
	}
	if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Errorf("error should mention mutual exclusivity, got: %s", err.Error())
	}

	// --label + --remove-label
	_, err = runCmd("issue", "update", "42",
		"--forge", "github.com", "--repo", "test/repo",
		"--label", "kind:bug",
		"--remove-label", "priority:high",
	)
	if err == nil {
		t.Fatal("expected error when combining --label and --remove-label")
	}
	if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Errorf("error should mention mutual exclusivity, got: %s", err.Error())
	}
}

func TestIssueUpdateRemoveLabelAbsentNoOp(t *testing.T) {
	fk := setupForgeTest(t)
	fk.IssueSvc.Issues = []forge.Issue{
		{
			Number: 42, Title: "Test", State: "open",
			URL:    "https://github.com/test/repo/issues/42",
			Labels: []forge.Label{{Name: "bug", Scope: "kind"}},
		},
	}

	buf, err := runCmd("issue", "update", "42",
		"--forge", "github.com", "--repo", "test/repo",
		"--remove-label", "kind:feature",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "updated") {
		t.Errorf("should succeed (idempotent no-op), got: %s", out)
	}

	// Issue should still have its original label.
	issue := fk.IssueSvc.Issues[0]
	if len(issue.Labels) != 1 {
		t.Errorf("expected 1 label, got %d: %+v", len(issue.Labels), issue.Labels)
	}
}

func TestIssueUpdateConfirmationShowsLabels(t *testing.T) {
	fk := setupForgeTest(t)
	fk.IssueSvc.Issues = []forge.Issue{
		{
			Number: 42, Title: "Test", State: "open",
			URL:    "https://github.com/test/repo/issues/42",
			Labels: []forge.Label{{Name: "bug", Scope: "kind"}},
		},
	}

	buf, err := runCmd("issue", "update", "42",
		"--forge", "github.com", "--repo", "test/repo",
		"--add-label", "priority:high",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "updated") {
		t.Errorf("should contain 'updated', got: %s", out)
	}
	if !strings.Contains(out, "kind:bug") {
		t.Errorf("should show existing label kind:bug in confirmation, got: %s", out)
	}
	if !strings.Contains(out, "priority:high") {
		t.Errorf("should show added label priority:high in confirmation, got: %s", out)
	}
}

func TestIssueUpdateScopedLabels(t *testing.T) {
	fk := setupForgeTest(t)
	fk.IssueSvc.Issues = []forge.Issue{
		{Number: 42, Title: "Test", State: "open", URL: "https://github.com/test/repo/issues/42"},
	}

	_, err := runCmd("issue", "update", "42",
		"--forge", "github.com", "--repo", "test/repo",
		"--add-label", "kind:bug",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	issue := fk.IssueSvc.Issues[0]
	if len(issue.Labels) != 1 {
		t.Fatalf("expected 1 label, got %d", len(issue.Labels))
	}
	if issue.Labels[0].Scope != "kind" || issue.Labels[0].Name != "bug" {
		t.Errorf("label = {Scope:%q Name:%q}, want {Scope:kind Name:bug}",
			issue.Labels[0].Scope, issue.Labels[0].Name)
	}
}

func TestIssueUpdateAddAndRemoveTogether(t *testing.T) {
	fk := setupForgeTest(t)
	fk.IssueSvc.Issues = []forge.Issue{
		{
			Number: 42, Title: "Test", State: "open",
			URL:    "https://github.com/test/repo/issues/42",
			Labels: []forge.Label{{Name: "bug", Scope: "kind"}},
		},
	}

	_, err := runCmd("issue", "update", "42",
		"--forge", "github.com", "--repo", "test/repo",
		"--add-label", "priority:high",
		"--remove-label", "kind:bug",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	issue := fk.IssueSvc.Issues[0]
	if len(issue.Labels) != 1 {
		t.Fatalf("expected 1 label (removed kind:bug, added priority:high), got %d: %+v",
			len(issue.Labels), issue.Labels)
	}
	if issue.Labels[0].Scope != "priority" || issue.Labels[0].Name != "high" {
		t.Errorf("label = {Scope:%q Name:%q}, want {Scope:priority Name:high}",
			issue.Labels[0].Scope, issue.Labels[0].Name)
	}
}

func TestIssueClose(t *testing.T) {
	fk := setupForgeTest(t)
	fk.IssueSvc.Issues = []forge.Issue{
		{Number: 42, Title: "To Close", State: "open"},
	}

	buf, err := runCmd("issue", "close", "42", "--forge", "github.com", "--repo", "test/repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	// Should print TOON confirmation for a fresh close
	if !strings.Contains(out, "closed") {
		t.Errorf("should show closed confirmation, got: %s", out)
	}
}

func TestIssueCloseAlreadyClosed(t *testing.T) {
	fk := setupForgeTest(t)
	fk.IssueSvc.Issues = []forge.Issue{
		{Number: 42, Title: "Already Closed", State: "closed"},
	}

	_, err := runCmd("issue", "close", "42", "--forge", "github.com", "--repo", "test/repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// No-op: closing an already-closed issue returns nil silently.
}

func TestIssueReopen(t *testing.T) {
	fk := setupForgeTest(t)
	fk.IssueSvc.Issues = []forge.Issue{
		{Number: 42, Title: "To Reopen", State: "closed"},
	}

	buf, err := runCmd("issue", "reopen", "42", "--forge", "github.com", "--repo", "test/repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "closed") {
		t.Errorf("should show reopened confirmation, got: %s", out)
	}
}

func TestIssueReopenAlreadyOpen(t *testing.T) {
	fk := setupForgeTest(t)
	fk.IssueSvc.Issues = []forge.Issue{
		{Number: 42, Title: "Already Open", State: "open"},
	}

	_, err := runCmd("issue", "reopen", "42", "--forge", "github.com", "--repo", "test/repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// No-op: reopening an already-open issue returns nil silently.
}

func TestIssueViewInvalidNumber(t *testing.T) {
	_ = setupForgeTest(t)

	_, err := runCmd("issue", "view", "notanumber", "--forge", "github.com", "--repo", "test/repo")
	if err == nil {
		t.Fatal("expected error for invalid number")
	}
}

func TestIssueViewNotFound(t *testing.T) {
	fk := setupForgeTest(t)
	// Simulate a 404 by making Get return an error
	fk.IssueSvc.GetErr = forge.NewBaseError("not found (404)", "Run \"anvil issue list\"")

	_, err := runCmd("issue", "view", "999", "--forge", "github.com", "--repo", "test/repo")
	if err == nil {
		t.Fatal("expected error for not found issue")
	}

	msg := err.Error()
	if !strings.Contains(msg, "not found") {
		t.Errorf("should mention not found, got: %s", msg)
	}
}

func TestIssueCreateWithAtMeAssignee(t *testing.T) {
	fk := setupForgeTest(t)
	// Default CurrentUser returns "test-user" when CurrentUserFn is nil.

	buf, err := runCmd("issue", "create",
		"--forge", "github.com", "--repo", "test/repo",
		"--title", "Fix bug",
		"--assignee", "@me",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Fix bug") {
		t.Errorf("should contain title, got: %s", out)
	}

	// Verify @me was resolved to the CurrentUser value before reaching the service.
	opts := fk.IssueSvc.LastCreateOpts
	if len(opts.Assignees) != 1 || opts.Assignees[0] != "test-user" {
		t.Errorf("expected assignees=[test-user] (resolved from @me), got: %v", opts.Assignees)
	}
}

func TestIssueCreateWithAtMeAndOtherAssignees(t *testing.T) {
	fk := setupForgeTest(t)

	buf, err := runCmd("issue", "create",
		"--forge", "github.com", "--repo", "test/repo",
		"--title", "Fix bug",
		"--assignee", "@me",
		"--assignee", "alice",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Fix bug") {
		t.Errorf("should contain title, got: %s", out)
	}

	opts := fk.IssueSvc.LastCreateOpts
	if len(opts.Assignees) != 2 {
		t.Fatalf("expected 2 assignees, got: %v", opts.Assignees)
	}
	if opts.Assignees[0] != "test-user" {
		t.Errorf("expected first assignee=test-user (resolved from @me), got: %s", opts.Assignees[0])
	}
	if opts.Assignees[1] != "alice" {
		t.Errorf("expected second assignee=alice (passed through), got: %s", opts.Assignees[1])
	}
}

func TestIssueCreateWithoutAtMePassesThrough(t *testing.T) {
	fk := setupForgeTest(t)

	_, err := runCmd("issue", "create",
		"--forge", "github.com", "--repo", "test/repo",
		"--title", "Fix bug",
		"--assignee", "alice",
		"--assignee", "bob",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	opts := fk.IssueSvc.LastCreateOpts
	if len(opts.Assignees) != 2 {
		t.Fatalf("expected 2 assignees, got: %v", opts.Assignees)
	}
	if opts.Assignees[0] != "alice" || opts.Assignees[1] != "bob" {
		t.Errorf("assignees should pass through unchanged, got: %v", opts.Assignees)
	}
}

func TestIssueCreateWithAtMeAuthError(t *testing.T) {
	fk := setupForgeTest(t)
	fk.CurrentUserFn = func(ctx context.Context) (string, error) {
		return "", forge.NewBaseError("authentication failed", "Run \"anvil auth set\"")
	}

	_, err := runCmd("issue", "create",
		"--forge", "github.com", "--repo", "test/repo",
		"--title", "Fix bug",
		"--assignee", "@me",
	)
	if err == nil {
		t.Fatal("expected error when CurrentUser fails")
	}

	out := err.Error()
	if !strings.Contains(out, "authentication failed") {
		t.Errorf("error should contain original CurrentUser error, got: %s", out)
	}
}

func TestIssueUpdateWithAtMeAssignee(t *testing.T) {
	fk := setupForgeTest(t)
	fk.IssueSvc.Issues = []forge.Issue{
		{Number: 42, Title: "Old Title", State: "open", URL: "https://github.com/test/repo/issues/42"},
	}

	buf, err := runCmd("issue", "update", "42",
		"--forge", "github.com", "--repo", "test/repo",
		"--assignee", "@me",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Old Title") {
		t.Errorf("should contain title, got: %s", out)
	}

	opts := fk.IssueSvc.LastUpdateOpts
	if len(opts.Assignees) != 1 || opts.Assignees[0] != "test-user" {
		t.Errorf("expected assignees=[test-user] (resolved from @me), got: %v", opts.Assignees)
	}
}

func TestIssueUpdateWithAtMeAuthError(t *testing.T) {
	fk := setupForgeTest(t)
	fk.IssueSvc.Issues = []forge.Issue{
		{Number: 42, Title: "Old Title", State: "open", URL: "https://github.com/test/repo/issues/42"},
	}
	fk.CurrentUserFn = func(ctx context.Context) (string, error) {
		return "", forge.NewBaseError("authentication failed", "Run \"anvil auth set\"")
	}

	_, err := runCmd("issue", "update", "42",
		"--forge", "github.com", "--repo", "test/repo",
		"--assignee", "@me",
	)
	if err == nil {
		t.Fatal("expected error when CurrentUser fails")
	}

	out := err.Error()
	if !strings.Contains(out, "authentication failed") {
		t.Errorf("error should contain original CurrentUser error, got: %s", out)
	}
}

func TestIssueSubcommandHelp(t *testing.T) {
	_ = setupForgeTest(t)

	subs := []string{"list", "view", "create", "update", "close", "reopen", "blocked-by", "blocking", "children", "parent", "comment", "relation"}
	for _, sub := range subs {
		t.Run(sub, func(t *testing.T) {
			buf, err := runCmd("issue", sub, "--help")
			if err != nil {
				t.Fatalf("unexpected error for issue %s --help: %v", sub, err)
			}
			out := buf.String()
			if out == "" {
				t.Errorf("help output for issue %s should not be empty", sub)
			}
		})
	}
}

// ---- Auto-create label tests ----

func TestIssueCreateAutoCreatesLabel(t *testing.T) {
	fk := setupForgeTest(t)
	// LabelSvc starts with no labels — "kind:bug" doesn't exist.

	buf, err := runCmd("issue", "create",
		"--forge", "github.com", "--repo", "test/repo",
		"--title", "Fix bug",
		"--label", "kind:bug",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	// The confirmation should include auto_created_labels.
	if !strings.Contains(out, "auto_created_labels") {
		t.Errorf("should contain auto_created_labels, got: %s", out)
	}
	if !strings.Contains(out, "333333") {
		t.Errorf("should contain placeholder color 333333, got: %s", out)
	}
	if !strings.Contains(out, "kind:bug") {
		t.Errorf("should contain the auto-created label name, got: %s", out)
	}

	// Verify LabelService.Create was called with the right args.
	opts := fk.LabelSvc.LastCreateOpts
	if opts.Name != "bug" {
		t.Errorf("Name should be 'bug', got: %q", opts.Name)
	}
	if opts.Scope == nil || *opts.Scope != "kind" {
		t.Errorf("Scope should be 'kind', got: %v", opts.Scope)
	}
	if opts.Color == nil || *opts.Color != "333333" {
		t.Errorf("Color should be '333333', got: %v", opts.Color)
	}
}

func TestIssueCreateAutoCreateDoesNotDuplicate(t *testing.T) {
	fk := setupForgeTest(t)
	// Pre-populate the label service with an existing label.
	fk.LabelSvc.Labels = []forge.Label{
		{Name: "bug", Scope: "kind", Color: "d73a4a", Description: "Something broken"},
	}

	buf, err := runCmd("issue", "create",
		"--forge", "github.com", "--repo", "test/repo",
		"--title", "Fix bug",
		"--label", "kind:bug",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	// Should NOT auto-create an existing label.
	if strings.Contains(out, "auto_created_labels") {
		t.Errorf("should NOT contain auto_created_labels for existing labels, got: %s", out)
	}

	// Verify LabelService.Create was NOT called (no new labels).
	if fk.LabelSvc.LastCreateOpts.Name != "" {
		t.Errorf("LabelService.Create should not have been called, got opts: %+v", fk.LabelSvc.LastCreateOpts)
	}
}

func TestIssueCreateAutoCreatesUnscopedLabel(t *testing.T) {
	fk := setupForgeTest(t)

	buf, err := runCmd("issue", "create",
		"--forge", "github.com", "--repo", "test/repo",
		"--title", "Fix bug",
		"--label", "priority-high",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "auto_created_labels") {
		t.Errorf("should contain auto_created_labels for unscoped label, got: %s", out)
	}

	opts := fk.LabelSvc.LastCreateOpts
	if opts.Name != "priority-high" {
		t.Errorf("Name should be 'priority-high', got: %q", opts.Name)
	}
	if opts.Scope != nil {
		t.Errorf("Scope should be nil for unscoped label, got: %v", opts.Scope)
	}
}

func TestIssueUpdateLabelAutoCreates(t *testing.T) {
	fk := setupForgeTest(t)
	fk.IssueSvc.Issues = []forge.Issue{
		{Number: 42, Title: "Test", State: "open", URL: "https://github.com/test/repo/issues/42"},
	}

	buf, err := runCmd("issue", "update", "42",
		"--forge", "github.com", "--repo", "test/repo",
		"--label", "kind:feature",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "auto_created_labels") {
		t.Errorf("should contain auto_created_labels in update confirmation, got: %s", out)
	}

	opts := fk.LabelSvc.LastCreateOpts
	if opts.Name != "feature" {
		t.Errorf("Name should be 'feature', got: %q", opts.Name)
	}
}

func TestIssueUpdateAddLabelAutoCreates(t *testing.T) {
	fk := setupForgeTest(t)
	fk.IssueSvc.Issues = []forge.Issue{
		{Number: 42, Title: "Test", State: "open", URL: "https://github.com/test/repo/issues/42"},
	}

	buf, err := runCmd("issue", "update", "42",
		"--forge", "github.com", "--repo", "test/repo",
		"--add-label", "priority:high",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "auto_created_labels") {
		t.Errorf("should contain auto_created_labels for --add-label, got: %s", out)
	}

	opts := fk.LabelSvc.LastCreateOpts
	if opts.Name != "high" {
		t.Errorf("Name should be 'high', got: %q", opts.Name)
	}
	if opts.Scope == nil || *opts.Scope != "priority" {
		t.Errorf("Scope should be 'priority', got: %v", opts.Scope)
	}
}

func TestIssueUpdateRemoveLabelNoAutoCreate(t *testing.T) {
	fk := setupForgeTest(t)
	fk.IssueSvc.Issues = []forge.Issue{
		{
			Number: 42, Title: "Test", State: "open",
			URL:    "https://github.com/test/repo/issues/42",
			Labels: []forge.Label{{Name: "bug", Scope: "kind"}},
		},
	}

	buf, err := runCmd("issue", "update", "42",
		"--forge", "github.com", "--repo", "test/repo",
		"--remove-label", "kind:bug",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	// Remove-label should NOT trigger auto-creation.
	if strings.Contains(out, "auto_created_labels") {
		t.Errorf("should NOT auto-create labels when using --remove-label, got: %s", out)
	}
}

// ---- Relationship subcommand tests ----

func TestIssueBlockedBy(t *testing.T) {
	fk := setupForgeTest(t)
	fk.RelationSvc.BlockedByItems = []forge.IssueDependency{
		{Number: 10, Title: "Blocking issue", State: "open", Direction: forge.DirBlockedBy},
		{Number: 11, Title: "Closed blocker", State: "closed", Direction: forge.DirBlockedBy},
	}

	buf, err := runCmd("issue", "blocked-by", "42", "--forge", "github.com", "--repo", "test/repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Blocking issue") {
		t.Errorf("should contain first blocking issue, got: %s", out)
	}
	// Closed issues should be filtered out.
	if strings.Contains(out, "Closed blocker") {
		t.Errorf("should NOT contain closed blocker, got: %s", out)
	}
	if !strings.Contains(out, "1 issue(s)") {
		t.Errorf("should show count of 1 (open only), got: %s", out)
	}

	if fk.RelationSvc.LastBlockedByNumber != 42 {
		t.Errorf("BlockedBy should be called with 42, got: %d", fk.RelationSvc.LastBlockedByNumber)
	}
}

func TestIssueBlocking(t *testing.T) {
	fk := setupForgeTest(t)
	fk.RelationSvc.BlockingItems = []forge.IssueDependency{
		{Number: 12, Title: "Blocked issue", State: "open", Direction: forge.DirBlocks},
	}

	buf, err := runCmd("issue", "blocking", "42", "--forge", "github.com", "--repo", "test/repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Blocked issue") {
		t.Errorf("should contain blocked issue, got: %s", out)
	}
	if !strings.Contains(out, "1 issue(s)") {
		t.Errorf("should show count of 1, got: %s", out)
	}

	if fk.RelationSvc.LastBlockingNumber != 42 {
		t.Errorf("Blocking should be called with 42, got: %d", fk.RelationSvc.LastBlockingNumber)
	}
}

func TestIssueChildren(t *testing.T) {
	fk := setupForgeTest(t)
	fk.RelationSvc.ChildrenItems = []forge.IssueDependency{
		{Number: 13, Title: "Sub-task", State: "open", Direction: forge.DirChild},
	}

	buf, err := runCmd("issue", "children", "42", "--forge", "github.com", "--repo", "test/repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Sub-task") {
		t.Errorf("should contain sub-task, got: %s", out)
	}

	if fk.RelationSvc.LastChildrenNumber != 42 {
		t.Errorf("Children should be called with 42, got: %d", fk.RelationSvc.LastChildrenNumber)
	}
}

func TestIssueParent(t *testing.T) {
	fk := setupForgeTest(t)
	fk.RelationSvc.ParentItem = &forge.IssueDependency{
		Number: 5, Title: "Parent issue", State: "open", Direction: forge.DirParent,
	}

	buf, err := runCmd("issue", "parent", "42", "--forge", "github.com", "--repo", "test/repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Parent issue") {
		t.Errorf("should contain parent issue title, got: %s", out)
	}

	if fk.RelationSvc.LastParentNumber != 42 {
		t.Errorf("Parent should be called with 42, got: %d", fk.RelationSvc.LastParentNumber)
	}
}

func TestIssueParentNone(t *testing.T) {
	fk := setupForgeTest(t)
	fk.RelationSvc.ParentItem = nil

	buf, err := runCmd("issue", "parent", "42", "--forge", "github.com", "--repo", "test/repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "none") {
		t.Errorf("should output 'none' for missing parent, got: %s", out)
	}
}

func TestIssueViewWithRelationshipHints(t *testing.T) {
	fk := setupForgeTest(t)
	fk.IssueSvc.Issues = []forge.Issue{
		{
			Number: 42, Title: "Test Issue", State: "open",
			Body: "Body",
			URL:  "https://github.com/test/repo/issues/42",
		},
	}
	fk.RelationSvc.BlockedByItems = []forge.IssueDependency{
		{Number: 1, Title: "Blocker", State: "open", Direction: forge.DirBlockedBy},
		{Number: 2, Title: "Blocker 2", State: "open", Direction: forge.DirBlockedBy},
	}
	fk.RelationSvc.BlockingItems = []forge.IssueDependency{
		{Number: 3, Title: "Blocked", State: "open", Direction: forge.DirBlocks},
	}
	fk.RelationSvc.ChildrenItems = []forge.IssueDependency{
		{Number: 4, Title: "Child", State: "open", Direction: forge.DirChild},
	}
	fk.RelationSvc.ParentItem = &forge.IssueDependency{
		Number: 0, Title: "Epic", State: "open", Direction: forge.DirParent,
	}

	buf, err := runCmd("issue", "view", "42", "--forge", "github.com", "--repo", "test/repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	// Check for relationship hints.
	if !strings.Contains(out, "blocked_by") {
		t.Errorf("should show blocked_by hint, got: %s", out)
	}
	if !strings.Contains(out, "use 'anvil issue blocked-by 42'") {
		t.Errorf("should show blocked-by command hint, got: %s", out)
	}
	if !strings.Contains(out, "blocking") {
		t.Errorf("should show blocking hint, got: %s", out)
	}
	if !strings.Contains(out, "use 'anvil issue blocking 42'") {
		t.Errorf("should show blocking command hint, got: %s", out)
	}
	if !strings.Contains(out, "children") {
		t.Errorf("should show children hint, got: %s", out)
	}
	if !strings.Contains(out, "use 'anvil issue children 42'") {
		t.Errorf("should show children command hint, got: %s", out)
	}
	if !strings.Contains(out, "parent") {
		t.Errorf("should show parent hint, got: %s", out)
	}
	if !strings.Contains(out, "use 'anvil issue parent 42'") {
		t.Errorf("should show parent command hint, got: %s", out)
	}
}

func TestIssueViewNoRelationshipHints(t *testing.T) {
	fk := setupForgeTest(t)
	fk.IssueSvc.Issues = []forge.Issue{
		{
			Number: 42, Title: "Test Issue", State: "open",
			Body: "Body",
			URL:  "https://github.com/test/repo/issues/42",
		},
	}
	// No relation data — hints should be absent.

	buf, err := runCmd("issue", "view", "42", "--forge", "github.com", "--repo", "test/repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if strings.Contains(out, "blocked_by") {
		t.Errorf("should NOT show blocked_by hint when none exist, got: %s", out)
	}
	if strings.Contains(out, "blocking") {
		t.Errorf("should NOT show blocking hint when none exist, got: %s", out)
	}
	if strings.Contains(out, "children") {
		t.Errorf("should NOT show children hint when none exist, got: %s", out)
	}
	if strings.Contains(out, "parent") {
		t.Errorf("should NOT show parent hint when none exist, got: %s", out)
	}
}

func TestIssueBlockedByInvalidNumber(t *testing.T) {
	_ = setupForgeTest(t)

	_, err := runCmd("issue", "blocked-by", "notanumber", "--forge", "github.com", "--repo", "test/repo")
	if err == nil {
		t.Fatal("expected error for invalid number")
	}
}

func TestIssueBlockingInvalidNumber(t *testing.T) {
	_ = setupForgeTest(t)

	_, err := runCmd("issue", "blocking", "notanumber", "--forge", "github.com", "--repo", "test/repo")
	if err == nil {
		t.Fatal("expected error for invalid number")
	}
}

func TestIssueChildrenInvalidNumber(t *testing.T) {
	_ = setupForgeTest(t)

	_, err := runCmd("issue", "children", "notanumber", "--forge", "github.com", "--repo", "test/repo")
	if err == nil {
		t.Fatal("expected error for invalid number")
	}
}

func TestIssueParentInvalidNumber(t *testing.T) {
	_ = setupForgeTest(t)

	_, err := runCmd("issue", "parent", "notanumber", "--forge", "github.com", "--repo", "test/repo")
	if err == nil {
		t.Fatal("expected error for invalid number")
	}
}

func TestIssueBlockedByEmptyList(t *testing.T) {
	_ = setupForgeTest(t)

	buf, err := runCmd("issue", "blocked-by", "42", "--forge", "github.com", "--repo", "test/repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "0 issue(s)") {
		t.Errorf("should show 0 issues, got: %s", out)
	}
}

// ---- Issue comment tests ----

func TestIssueCommentList(t *testing.T) {
	fk := setupForgeTest(t)
	fk.CommentSvc.Comments = []forge.Comment{
		{ID: 1, Body: "First comment", Author: "alice", System: false},
		{ID: 2, Body: "Second comment", Author: "bob", System: false},
		{ID: 3, Body: "System note", Author: "", System: true},
	}

	buf, err := runCmd("issue", "comment", "list", "42", "--forge", "github.com", "--repo", "test/repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "First comment") {
		t.Errorf("should contain first comment, got: %s", out)
	}
	if !strings.Contains(out, "Second comment") {
		t.Errorf("should contain second comment, got: %s", out)
	}
	// System notes should be filtered by default.
	if strings.Contains(out, "System note") {
		t.Errorf("should NOT contain system note by default, got: %s", out)
	}
	if !strings.Contains(out, "2 of 3 comments") {
		t.Errorf("should show count '2 of 3 comments', got: %s", out)
	}

	// Verify IssueNumber was correct (from the last call, which fetches total with IncludeSystem:true).
	if fk.CommentSvc.LastListOpts.IssueNumber != 42 {
		t.Errorf("IssueNumber should be 42, got: %d", fk.CommentSvc.LastListOpts.IssueNumber)
	}
}

func TestIssueCommentListIncludeSystem(t *testing.T) {
	fk := setupForgeTest(t)
	fk.CommentSvc.Comments = []forge.Comment{
		{ID: 1, Body: "User comment", Author: "alice", System: false},
		{ID: 2, Body: "System note", Author: "", System: true},
	}

	buf, err := runCmd("issue", "comment", "list", "42", "--include-system", "--forge", "github.com", "--repo", "test/repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "System note") {
		t.Errorf("should contain system note with --include-system, got: %s", out)
	}
	if fk.CommentSvc.LastListOpts.IncludeSystem != true {
		t.Errorf("IncludeSystem should be true")
	}
}

func TestIssueCommentView(t *testing.T) {
	fk := setupForgeTest(t)
	fk.CommentSvc.Comments = []forge.Comment{
		{
			ID:        1,
			Body:      "A thoughtful comment with some detail.",
			Author:    "alice",
			System:    false,
			CreatedAt: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
			UpdatedAt: time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC),
			URL:       "https://github.com/test/repo/issues/42#comment-1",
		},
	}

	buf, err := runCmd("issue", "comment", "view", "42", "1", "--forge", "github.com", "--repo", "test/repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "A thoughtful comment") {
		t.Errorf("should contain comment body, got: %s", out)
	}
	if !strings.Contains(out, "alice") {
		t.Errorf("should contain author, got: %s", out)
	}
	if !strings.Contains(out, "body_size") {
		t.Errorf("should contain body_size, got: %s", out)
	}
	if !strings.Contains(out, "1") {
		t.Errorf("should contain comment ID, got: %s", out)
	}
}

func TestIssueCommentViewNotFound(t *testing.T) {
	fk := setupForgeTest(t)
	fk.CommentSvc.GetErr = forge.NewBaseError("comment not found", "Run \"anvil issue comment list <number>\"")

	_, err := runCmd("issue", "comment", "view", "42", "999", "--forge", "github.com", "--repo", "test/repo")
	if err == nil {
		t.Fatal("expected error for not found comment")
	}

	msg := err.Error()
	if !strings.Contains(msg, "comment not found") {
		t.Errorf("should mention not found, got: %s", msg)
	}
}

func TestIssueCommentCreate(t *testing.T) {
	_ = setupForgeTest(t)

	buf, err := runCmd("issue", "comment", "create", "42",
		"--forge", "github.com", "--repo", "test/repo",
		"--body", "Nice issue!",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "created") {
		t.Errorf("should contain 'created', got: %s", out)
	}
	if !strings.Contains(out, "42") {
		t.Errorf("should contain issue number, got: %s", out)
	}
}

func TestIssueCommentCreateMissingBody(t *testing.T) {
	_ = setupForgeTest(t)

	_, err := runCmd("issue", "comment", "create", "42",
		"--forge", "github.com", "--repo", "test/repo",
	)
	if err == nil {
		t.Fatal("expected error for missing --body")
	}

	out := err.Error()
	if !strings.Contains(out, "missing required flag") || !strings.Contains(out, "body") {
		t.Errorf("should show missing body error, got: %s", out)
	}
}

func TestIssueCommentUpdate(t *testing.T) {
	fk := setupForgeTest(t)
	fk.CommentSvc.Comments = []forge.Comment{
		{
			ID:   1,
			Body: "Old body",
			URL:  "https://github.com/test/repo/issues/42#comment-1",
		},
	}

	buf, err := runCmd("issue", "comment", "update", "42", "1",
		"--forge", "github.com", "--repo", "test/repo",
		"--body", "Updated body!",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "updated") {
		t.Errorf("should contain 'updated', got: %s", out)
	}

	// Verify the comment body was updated.
	if fk.CommentSvc.Comments[0].Body != "Updated body!" {
		t.Errorf("comment body should be updated, got: %q", fk.CommentSvc.Comments[0].Body)
	}
}

func TestIssueCommentUpdateMissingBody(t *testing.T) {
	_ = setupForgeTest(t)

	_, err := runCmd("issue", "comment", "update", "42", "1",
		"--forge", "github.com", "--repo", "test/repo",
	)
	if err == nil {
		t.Fatal("expected error for missing --body")
	}

	out := err.Error()
	if !strings.Contains(out, "missing required flag") || !strings.Contains(out, "body") {
		t.Errorf("should show missing body error, got: %s", out)
	}
}

func TestIssueCommentDelete(t *testing.T) {
	fk := setupForgeTest(t)
	fk.CommentSvc.Comments = []forge.Comment{
		{ID: 1, Body: "To delete"},
		{ID: 2, Body: "To keep"},
	}

	buf, err := runCmd("issue", "comment", "delete", "42", "1",
		"--forge", "github.com", "--repo", "test/repo",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "deleted") {
		t.Errorf("should contain 'deleted', got: %s", out)
	}

	// Verify the comment was removed.
	if len(fk.CommentSvc.Comments) != 1 {
		t.Fatalf("expected 1 comment remaining, got %d", len(fk.CommentSvc.Comments))
	}
	if fk.CommentSvc.Comments[0].Body != "To keep" {
		t.Errorf("wrong comment remaining: %q", fk.CommentSvc.Comments[0].Body)
	}
}

func TestIssueCommentDeleteNotFound(t *testing.T) {
	_ = setupForgeTest(t)
	// Deleting a non-existent comment is a no-op.

	buf, err := runCmd("issue", "comment", "delete", "42", "999",
		"--forge", "github.com", "--repo", "test/repo",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "deleted") {
		t.Errorf("deleting non-existent comment should still succeed (no-op), got: %s", out)
	}
}

func TestIssueViewWithCommentHint(t *testing.T) {
	fk := setupForgeTest(t)
	fk.IssueSvc.Issues = []forge.Issue{
		{
			Number: 42, Title: "Test Issue", State: "open",
			Body: "Body",
			URL:  "https://github.com/test/repo/issues/42",
		},
	}
	fk.CommentSvc.Comments = []forge.Comment{
		{ID: 1, Body: "Comment 1", Author: "alice"},
		{ID: 2, Body: "Comment 2", Author: "bob"},
	}

	buf, err := runCmd("issue", "view", "42", "--forge", "github.com", "--repo", "test/repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "use 'anvil issue comment list 42'") {
		t.Errorf("should show comment list hint, got: %s", out)
	}
	if !strings.Contains(out, "2") {
		t.Errorf("should show comment count, got: %s", out)
	}
}

func TestIssueViewNoCommentHint(t *testing.T) {
	fk := setupForgeTest(t)
	fk.IssueSvc.Issues = []forge.Issue{
		{
			Number: 42, Title: "Test Issue", State: "open",
			Body: "Body",
			URL:  "https://github.com/test/repo/issues/42",
		},
	}
	// No comments — hint should be absent.

	buf, err := runCmd("issue", "view", "42", "--forge", "github.com", "--repo", "test/repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if strings.Contains(out, "use 'anvil issue comment list") {
		t.Errorf("should NOT show comment hint when no comments exist, got: %s", out)
	}
}

func TestIssueCommentInvalidIssueNumber(t *testing.T) {
	_ = setupForgeTest(t)

	_, err := runCmd("issue", "comment", "list", "notanumber", "--forge", "github.com", "--repo", "test/repo")
	if err == nil {
		t.Fatal("expected error for invalid issue number")
	}
}

func TestIssueCommentHelp(t *testing.T) {
	_ = setupForgeTest(t)

	subs := []string{"list", "view", "create", "update", "delete"}
	for _, sub := range subs {
		t.Run(sub, func(t *testing.T) {
			buf, err := runCmd("issue", "comment", sub, "--help")
			if err != nil {
				t.Fatalf("unexpected error for issue comment %s --help: %v", sub, err)
			}
			out := buf.String()
			if out == "" {
				t.Errorf("help output for issue comment %s should not be empty", sub)
			}
		})
	}
}

// ---- Relation mutation tests ----

func TestIssueRelationAddBlocks(t *testing.T) {
	fk := setupForgeTest(t)

	buf, err := runCmd("issue", "relation", "add", "42",
		"--forge", "github.com", "--repo", "test/repo",
		"--blocks", "100",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "added") {
		t.Errorf("should contain 'added', got: %s", out)
	}
	if !strings.Contains(out, "42") && !strings.Contains(out, "source") {
		t.Errorf("should contain source issue number, got: %s", out)
	}
	if !strings.Contains(out, "100") && !strings.Contains(out, "target") {
		t.Errorf("should contain target issue number, got: %s", out)
	}
	if !strings.Contains(out, "blocks") {
		t.Errorf("should contain 'blocks' type, got: %s", out)
	}

	// Verify AddBlocks was called with correct args.
	if fk.RelationSvc.LastAddBlocksNumber != 42 {
		t.Errorf("AddBlocks number should be 42, got: %d", fk.RelationSvc.LastAddBlocksNumber)
	}
	if fk.RelationSvc.LastAddBlocksTarget != 100 {
		t.Errorf("AddBlocks target should be 100, got: %d", fk.RelationSvc.LastAddBlocksTarget)
	}
}

func TestIssueRelationAddParentOf(t *testing.T) {
	fk := setupForgeTest(t)

	buf, err := runCmd("issue", "relation", "add", "42",
		"--forge", "github.com", "--repo", "test/repo",
		"--parent-of", "100",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "added") {
		t.Errorf("should contain 'added', got: %s", out)
	}
	if !strings.Contains(out, "parent_of") {
		t.Errorf("should contain 'parent_of' type, got: %s", out)
	}

	// Verify AddParentOf was called with correct args.
	if fk.RelationSvc.LastAddParentOfNumber != 42 {
		t.Errorf("AddParentOf number should be 42, got: %d", fk.RelationSvc.LastAddParentOfNumber)
	}
	if fk.RelationSvc.LastAddParentOfChild != 100 {
		t.Errorf("AddParentOf child should be 100, got: %d", fk.RelationSvc.LastAddParentOfChild)
	}
}

func TestIssueRelationRemoveBlocks(t *testing.T) {
	fk := setupForgeTest(t)

	buf, err := runCmd("issue", "relation", "remove", "42",
		"--forge", "github.com", "--repo", "test/repo",
		"--blocks", "100",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "removed") {
		t.Errorf("should contain 'removed', got: %s", out)
	}
	if !strings.Contains(out, "blocks") {
		t.Errorf("should contain 'blocks' type, got: %s", out)
	}

	// Verify RemoveBlocks was called with correct args.
	if fk.RelationSvc.LastRemoveBlocksNumber != 42 {
		t.Errorf("RemoveBlocks number should be 42, got: %d", fk.RelationSvc.LastRemoveBlocksNumber)
	}
	if fk.RelationSvc.LastRemoveBlocksTarget != 100 {
		t.Errorf("RemoveBlocks target should be 100, got: %d", fk.RelationSvc.LastRemoveBlocksTarget)
	}
}

func TestIssueRelationRemoveParentOf(t *testing.T) {
	fk := setupForgeTest(t)

	buf, err := runCmd("issue", "relation", "remove", "42",
		"--forge", "github.com", "--repo", "test/repo",
		"--parent-of", "100",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "removed") {
		t.Errorf("should contain 'removed', got: %s", out)
	}
	if !strings.Contains(out, "parent_of") {
		t.Errorf("should contain 'parent_of' type, got: %s", out)
	}

	// Verify RemoveParentOf was called with correct args.
	if fk.RelationSvc.LastRemoveParentOfNumber != 42 {
		t.Errorf("RemoveParentOf number should be 42, got: %d", fk.RelationSvc.LastRemoveParentOfNumber)
	}
	if fk.RelationSvc.LastRemoveParentOfChild != 100 {
		t.Errorf("RemoveParentOf child should be 100, got: %d", fk.RelationSvc.LastRemoveParentOfChild)
	}
}

func TestIssueRelationAddIdempotentBlocks(t *testing.T) {
	fk := setupForgeTest(t)
	// Pre-populate state so AddBlocks finds it already exists.
	fk.RelationSvc.AddBlocksFn = func(ctx context.Context, number int, target int) error {
		return nil // idempotent no-op
	}

	// First call should succeed.
	buf, err := runCmd("issue", "relation", "add", "42",
		"--forge", "github.com", "--repo", "test/repo",
		"--blocks", "100",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "added") {
		t.Errorf("should still show 'added' confirmation (idempotent), got: %s", out)
	}
}

func TestIssueRelationRemoveIdempotentBlocks(t *testing.T) {
	fk := setupForgeTest(t)
	// RemoveBlocksFn returns nil (no-op for non-existent relationship).
	fk.RelationSvc.RemoveBlocksFn = func(ctx context.Context, number int, target int) error {
		return nil // idempotent no-op
	}

	buf, err := runCmd("issue", "relation", "remove", "42",
		"--forge", "github.com", "--repo", "test/repo",
		"--blocks", "999",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "removed") {
		t.Errorf("should show 'removed' confirmation (idempotent), got: %s", out)
	}
}

func TestIssueRelationAddIdempotentParentOf(t *testing.T) {
	fk := setupForgeTest(t)
	fk.RelationSvc.AddParentOfFn = func(ctx context.Context, number int, child int) error {
		return nil // idempotent no-op
	}

	buf, err := runCmd("issue", "relation", "add", "42",
		"--forge", "github.com", "--repo", "test/repo",
		"--parent-of", "100",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "added") {
		t.Errorf("should still show 'added' confirmation (idempotent), got: %s", out)
	}
}

func TestIssueRelationRemoveIdempotentParentOf(t *testing.T) {
	fk := setupForgeTest(t)
	fk.RelationSvc.RemoveParentOfFn = func(ctx context.Context, number int, child int) error {
		return nil // idempotent no-op
	}

	buf, err := runCmd("issue", "relation", "remove", "42",
		"--forge", "github.com", "--repo", "test/repo",
		"--parent-of", "999",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "removed") {
		t.Errorf("should show 'removed' confirmation (idempotent), got: %s", out)
	}
}

func TestIssueRelationAddNoFlag(t *testing.T) {
	_ = setupForgeTest(t)

	_, err := runCmd("issue", "relation", "add", "42",
		"--forge", "github.com", "--repo", "test/repo",
	)
	if err == nil {
		t.Fatal("expected error when neither --blocks nor --parent-of provided")
	}

	out := err.Error()
	if !strings.Contains(out, "exactly one") {
		t.Errorf("should mention 'exactly one', got: %s", out)
	}
}

func TestIssueRelationAddBothFlags(t *testing.T) {
	_ = setupForgeTest(t)

	_, err := runCmd("issue", "relation", "add", "42",
		"--forge", "github.com", "--repo", "test/repo",
		"--blocks", "100",
		"--parent-of", "200",
	)
	if err == nil {
		t.Fatal("expected error when both --blocks and --parent-of provided")
	}

	out := err.Error()
	if !strings.Contains(out, "mutually exclusive") {
		t.Errorf("should mention 'mutually exclusive', got: %s", out)
	}
}

func TestIssueRelationRemoveNoFlag(t *testing.T) {
	_ = setupForgeTest(t)

	_, err := runCmd("issue", "relation", "remove", "42",
		"--forge", "github.com", "--repo", "test/repo",
	)
	if err == nil {
		t.Fatal("expected error when neither --blocks nor --parent-of provided")
	}

	out := err.Error()
	if !strings.Contains(out, "exactly one") {
		t.Errorf("should mention 'exactly one', got: %s", out)
	}
}

func TestIssueRelationRemoveBothFlags(t *testing.T) {
	_ = setupForgeTest(t)

	_, err := runCmd("issue", "relation", "remove", "42",
		"--forge", "github.com", "--repo", "test/repo",
		"--blocks", "100",
		"--parent-of", "200",
	)
	if err == nil {
		t.Fatal("expected error when both --blocks and --parent-of provided")
	}

	out := err.Error()
	if !strings.Contains(out, "mutually exclusive") {
		t.Errorf("should mention 'mutually exclusive', got: %s", out)
	}
}

func TestIssueRelationAddInvalidNumber(t *testing.T) {
	_ = setupForgeTest(t)

	_, err := runCmd("issue", "relation", "add", "notanumber",
		"--forge", "github.com", "--repo", "test/repo",
		"--blocks", "100",
	)
	if err == nil {
		t.Fatal("expected error for invalid issue number")
	}
}

func TestIssueRelationRemoveInvalidNumber(t *testing.T) {
	_ = setupForgeTest(t)

	_, err := runCmd("issue", "relation", "remove", "notanumber",
		"--forge", "github.com", "--repo", "test/repo",
		"--blocks", "100",
	)
	if err == nil {
		t.Fatal("expected error for invalid issue number")
	}
}

func TestIssueRelationHelp(t *testing.T) {
	_ = setupForgeTest(t)

	subs := []string{"add", "remove"}
	for _, sub := range subs {
		t.Run(sub, func(t *testing.T) {
			buf, err := runCmd("issue", "relation", sub, "--help")
			if err != nil {
				t.Fatalf("unexpected error for issue relation %s --help: %v", sub, err)
			}
			out := buf.String()
			if out == "" {
				t.Errorf("help output for issue relation %s should not be empty", sub)
			}
		})
	}
}

// ---- Blocked/unblocked filtering tests ----

func TestIssueListUnblocked(t *testing.T) {
	fk := setupForgeTest(t)
	fk.IssueSvc.Issues = []forge.Issue{
		{Number: 1, Title: "No blockers", State: "open"},
		{Number: 2, Title: "Has open blocker", State: "open"},
		{Number: 3, Title: "Only closed blocker", State: "open"},
	}

	// Return different BlockedBy results per issue number.
	fk.RelationSvc.BlockedByFn = func(ctx context.Context, number int) ([]forge.IssueDependency, error) {
		switch number {
		case 1:
			return nil, nil
		case 2:
			return []forge.IssueDependency{
				{Number: 10, Title: "Blocker", State: forge.StateOpen, Direction: forge.DirBlockedBy},
			}, nil
		case 3:
			return []forge.IssueDependency{
				{Number: 11, Title: "Closed", State: forge.StateClosed, Direction: forge.DirBlockedBy},
			}, nil
		default:
			return nil, nil
		}
	}

	buf, err := runCmd("issue", "list", "--unblocked",
		"--forge", "github.com", "--repo", "test/repo",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "No blockers") {
		t.Errorf("should contain unblocked issue, got: %s", out)
	}
	if !strings.Contains(out, "Only closed blocker") {
		t.Errorf("should contain issue with only closed blockers (unblocked), got: %s", out)
	}
	if strings.Contains(out, "Has open blocker") {
		t.Errorf("should NOT contain issue with open blocker, got: %s", out)
	}
}

func TestIssueListBlocked(t *testing.T) {
	fk := setupForgeTest(t)
	fk.IssueSvc.Issues = []forge.Issue{
		{Number: 1, Title: "No blockers", State: "open"},
		{Number: 2, Title: "Has open blocker", State: "open"},
		{Number: 3, Title: "Has two open blockers", State: "open"},
	}

	fk.RelationSvc.BlockedByFn = func(ctx context.Context, number int) ([]forge.IssueDependency, error) {
		switch number {
		case 1:
			return nil, nil
		case 2:
			return []forge.IssueDependency{
				{Number: 10, Title: "Blocker", State: forge.StateOpen, Direction: forge.DirBlockedBy},
			}, nil
		case 3:
			return []forge.IssueDependency{
				{Number: 10, Title: "Blocker 1", State: forge.StateOpen, Direction: forge.DirBlockedBy},
				{Number: 11, Title: "Blocker 2", State: forge.StateOpen, Direction: forge.DirBlockedBy},
			}, nil
		default:
			return nil, nil
		}
	}

	buf, err := runCmd("issue", "list", "--blocked",
		"--forge", "github.com", "--repo", "test/repo",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if strings.Contains(out, "No blockers") {
		t.Errorf("should NOT contain unblocked issue, got: %s", out)
	}
	if !strings.Contains(out, "Has open blocker") {
		t.Errorf("should contain issue with open blocker, got: %s", out)
	}
	if !strings.Contains(out, "Has two open blockers") {
		t.Errorf("should contain issue with two open blockers, got: %s", out)
	}
}

func TestIssueListUnblockedAndBlockedMutuallyExclusive(t *testing.T) {
	_ = setupForgeTest(t)

	_, err := runCmd("issue", "list",
		"--unblocked", "--blocked",
		"--forge", "github.com", "--repo", "test/repo",
	)
	if err == nil {
		t.Fatal("expected error when both --unblocked and --blocked are passed")
	}

	out := err.Error()
	if !strings.Contains(out, "mutually exclusive") {
		t.Errorf("should mention mutual exclusivity, got: %s", out)
	}
}

func TestIssueListFieldsBlocked(t *testing.T) {
	fk := setupForgeTest(t)
	fk.IssueSvc.Issues = []forge.Issue{
		{Number: 1, Title: "Unblocked", State: "open"},
		{Number: 2, Title: "One blocker", State: "open"},
		{Number: 3, Title: "Two blockers", State: "open"},
	}

	fk.RelationSvc.BlockedByFn = func(ctx context.Context, number int) ([]forge.IssueDependency, error) {
		switch number {
		case 1:
			return nil, nil
		case 2:
			return []forge.IssueDependency{
				{Number: 10, Title: "Blocker", State: forge.StateOpen, Direction: forge.DirBlockedBy},
			}, nil
		case 3:
			return []forge.IssueDependency{
				{Number: 10, Title: "Blocker 1", State: forge.StateOpen, Direction: forge.DirBlockedBy},
				{Number: 11, Title: "Blocker 2", State: forge.StateOpen, Direction: forge.DirBlockedBy},
			}, nil
		default:
			return nil, nil
		}
	}

	buf, err := runCmd("issue", "list",
		"--fields", "blocked",
		"--forge", "github.com", "--repo", "test/repo",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "none") {
		t.Errorf("should show 'none' for unblocked issue, got: %s", out)
	}
	if !strings.Contains(out, "\"1\"") {
		t.Errorf("should show count '1' for issue with one blocker, got: %s", out)
	}
	if !strings.Contains(out, "\"2\"") {
		t.Errorf("should show count '2' for issue with two blockers, got: %s", out)
	}
}

func TestIssueListFieldsBlockedDefaultHidden(t *testing.T) {
	fk := setupForgeTest(t)
	fk.IssueSvc.Issues = []forge.Issue{
		{Number: 1, Title: "Test", State: "open"},
	}

	buf, err := runCmd("issue", "list",
		"--forge", "github.com", "--repo", "test/repo",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if strings.Contains(out, "blocked") {
		t.Errorf("blocked column should be hidden by default, got: %s", out)
	}
}

func TestIssueListUnblockedWithStateFilter(t *testing.T) {
	fk := setupForgeTest(t)
	// Use ListFn to simulate server-side state filtering.
	fk.IssueSvc.ListFn = func(ctx context.Context, opts forge.IssueListOptions) ([]forge.Issue, *forge.ListMeta, error) {
		all := []forge.Issue{
			{Number: 1, Title: "Open unblocked", State: "open"},
			{Number: 2, Title: "Closed unblocked", State: "closed"},
			{Number: 3, Title: "Open blocked", State: "open"},
		}
		var filtered []forge.Issue
		for _, i := range all {
			if opts.State == "all" || i.State == opts.State {
				filtered = append(filtered, i)
			}
		}
		return filtered, &forge.ListMeta{Total: len(filtered), Count: len(filtered)}, nil
	}

	fk.RelationSvc.BlockedByFn = func(ctx context.Context, number int) ([]forge.IssueDependency, error) {
		if number == 3 {
			return []forge.IssueDependency{
				{Number: 10, Title: "Blocker", State: forge.StateOpen, Direction: forge.DirBlockedBy},
			}, nil
		}
		return nil, nil
	}

	// --unblocked with --state closed should only show #2.
	buf, err := runCmd("issue", "list",
		"--unblocked", "--state", "closed",
		"--forge", "github.com", "--repo", "test/repo",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if strings.Contains(out, "Open unblocked") {
		t.Errorf("should NOT contain open issue when filtering by closed, got: %s", out)
	}
	if !strings.Contains(out, "Closed unblocked") {
		t.Errorf("should contain closed unblocked issue, got: %s", out)
	}
	if strings.Contains(out, "Open blocked") {
		t.Errorf("should NOT contain blocked issue, got: %s", out)
	}
}

func TestIssueListUnblockedWithLabelFilter(t *testing.T) {
	fk := setupForgeTest(t)
	fk.IssueSvc.Issues = []forge.Issue{
		{Number: 1, Title: "Bug unblocked", State: "open", Labels: []forge.Label{{Name: "bug", Scope: "kind"}}},
		{Number: 2, Title: "Bug blocked", State: "open", Labels: []forge.Label{{Name: "bug", Scope: "kind"}}},
	}

	fk.RelationSvc.BlockedByFn = func(ctx context.Context, number int) ([]forge.IssueDependency, error) {
		if number == 2 {
			return []forge.IssueDependency{
				{Number: 10, Title: "Blocker", State: forge.StateOpen, Direction: forge.DirBlockedBy},
			}, nil
		}
		return nil, nil
	}

	buf, err := runCmd("issue", "list",
		"--unblocked", "--label", "kind:bug",
		"--forge", "github.com", "--repo", "test/repo",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Bug unblocked") {
		t.Errorf("should contain unblocked bug, got: %s", out)
	}
	if strings.Contains(out, "Bug blocked") {
		t.Errorf("should NOT contain blocked bug, got: %s", out)
	}

	// Verify the label filter was passed through.
	opts := fk.IssueSvc.LastListOpts
	if len(opts.Labels) != 1 || opts.Labels[0] != "kind:bug" {
		t.Errorf("label filter should be preserved, got: %v", opts.Labels)
	}
}

func TestIssueListBlockedByError(t *testing.T) {
	fk := setupForgeTest(t)
	fk.IssueSvc.Issues = []forge.Issue{
		{Number: 1, Title: "Test", State: "open"},
	}
	fk.RelationSvc.BlockedByFn = func(ctx context.Context, number int) ([]forge.IssueDependency, error) {
		return nil, forge.NewBaseError("failed to fetch blockers", "Retry later")
	}

	_, err := runCmd("issue", "list", "--unblocked",
		"--forge", "github.com", "--repo", "test/repo",
	)
	if err == nil {
		t.Fatal("expected error when BlockedBy fails")
	}
	if !strings.Contains(err.Error(), "failed to fetch blockers") {
		t.Errorf("should propagate BlockedBy error, got: %s", err.Error())
	}
}
