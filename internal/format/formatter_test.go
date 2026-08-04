package format_test

import (
	"strings"
	"testing"

	"github.com/tnikic/anvil/internal/format"
)

// ---- Issue List ----

func TestFormatIssueList(t *testing.T) {
	issues := []format.IssueRow{
		{Number: 1, Title: "Fix login timeout", State: "open"},
		{Number: 2, Title: "Add rate limiting", State: "closed"},
	}
	out := format.IssueList(issues, 2, 10)
	if out == "" {
		t.Fatal("expected non-empty output")
	}
	if !strings.Contains(out, "issues") {
		t.Errorf("output should contain 'issues', got: %s", out)
	}
	if !strings.Contains(out, "Fix login timeout") {
		t.Errorf("output should contain issue title, got: %s", out)
	}
	if !strings.Contains(out, "count:") {
		t.Errorf("output should contain 'count:', got: %s", out)
	}
	if !strings.Contains(out, "2 of 10 total") {
		t.Errorf("output should contain '2 of 10 total', got: %s", out)
	}
	// Default fields: only number, title, state — no author or labels in header
	if strings.Contains(out, "author") {
		t.Errorf("default output should NOT contain 'author' without --fields, got: %s", out)
	}
	if strings.Contains(out, "labels") && !strings.Contains(out, "2 labels") {
		t.Errorf("default output should NOT contain 'labels' field without --fields, got: %s", out)
	}
}

func TestFormatIssueListEmpty(t *testing.T) {
	out := format.IssueList(nil, 0, 0)
	if out == "" {
		t.Fatal("expected non-empty output for empty list")
	}
	if !strings.Contains(out, "issues") {
		t.Errorf("output should contain 'issues', got: %s", out)
	}
	if !strings.Contains(out, "0 of 0 total") {
		t.Errorf("output should contain '0 of 0 total', got: %s", out)
	}
}

func TestFormatIssueListWithFields(t *testing.T) {
	issues := []format.IssueRow{
		{Number: 1, Title: "Test", State: "open", Author: "alice", Labels: "bug, enhancement"},
	}
	out := format.IssueList(issues, 1, 5)
	if !strings.Contains(out, "1 of 5 total") {
		t.Errorf("output should show count, got: %s", out)
	}
	// When data is populated, actual values should appear in output
	if !strings.Contains(out, "alice") {
		t.Errorf("output should contain author value 'alice', got: %s", out)
	}
	if !strings.Contains(out, "bug, enhancement") {
		t.Errorf("output should contain labels value 'bug, enhancement', got: %s", out)
	}
	// Column headers should still appear
	if !strings.Contains(out, "author") {
		t.Errorf("output should contain 'author' column header, got: %s", out)
	}
	if !strings.Contains(out, "labels") {
		t.Errorf("output should contain 'labels' column header, got: %s", out)
	}
}

func TestFormatIssueListWithFieldsEmptyValues(t *testing.T) {
	// When fields are empty, omitempty should suppress them in output
	issues := []format.IssueRow{
		{Number: 1, Title: "Test", State: "open"},
	}
	out := format.IssueList(issues, 1, 5)
	if !strings.Contains(out, "1 of 5 total") {
		t.Errorf("output should show count, got: %s", out)
	}
	// The column header 'author' may appear in the TOON header, but no author data value
	// With omitempty on empty strings, the author field should not appear as a data line
	if strings.Contains(out, "author:") && !strings.Contains(out, "author: alice") {
		t.Error("output should contain 'author: alice' or no 'author:' at all")
	}
}

// ---- Issue List Blocked Field ----

func TestFormatIssueListWithBlockedField(t *testing.T) {
	issues := []format.IssueRow{
		{Number: 1, Title: "Unblocked", State: "open", Blocked: "none"},
		{Number: 2, Title: "One blocker", State: "open", Blocked: "1"},
		{Number: 3, Title: "Two blockers", State: "open", Blocked: "2"},
	}
	out := format.IssueList(issues, 3, 3)
	if out == "" {
		t.Fatal("expected non-empty output")
	}
	if !strings.Contains(out, "none") {
		t.Errorf("output should contain 'none' for unblocked issue, got: %s", out)
	}
	if !strings.Contains(out, "\"1\"") {
		t.Errorf("output should contain '1' for single blocker, got: %s", out)
	}
	if !strings.Contains(out, "\"2\"") {
		t.Errorf("output should contain '2' for two blockers, got: %s", out)
	}
	if !strings.Contains(out, "blocked") {
		t.Errorf("output should contain 'blocked' column header when data present, got: %s", out)
	}
}

func TestFormatIssueListBlockedFieldHiddenWhenEmpty(t *testing.T) {
	// When Blocked is empty string, omitempty should suppress it.
	issues := []format.IssueRow{
		{Number: 1, Title: "Test", State: "open"},
	}
	out := format.IssueList(issues, 1, 1)
	if strings.Contains(out, "blocked") {
		t.Errorf("output should NOT contain 'blocked' when field is empty, got: %s", out)
	}
}

// ---- Issue View ----

func TestFormatIssueView(t *testing.T) {
	issue := &format.IssueDetail{
		Number:    1,
		Title:     "Fix login timeout",
		State:     "open",
		Body:      "This is the issue body.",
		BodySize:  len("This is the issue body."),
		Author:    "user1",
		CreatedAt: "2025-01-15T10:30:00Z",
		UpdatedAt: "2025-01-16T08:00:00Z",
		URL:       "https://github.com/owner/repo/issues/1",
	}
	out := format.IssueView(issue, false)
	if out == "" {
		t.Fatal("expected non-empty output")
	}
	if !strings.Contains(out, "number:") {
		t.Errorf("output should contain 'number:', got: %s", out)
	}
	if !strings.Contains(out, "Fix login timeout") {
		t.Errorf("output should contain title, got: %s", out)
	}
	if !strings.Contains(out, "open") {
		t.Errorf("output should contain state, got: %s", out)
	}
}

func TestFormatIssueViewBodyTruncation(t *testing.T) {
	longBody := strings.Repeat("a", 1000)
	issue := &format.IssueDetail{
		Number:    1,
		Title:     "Test",
		State:     "open",
		Body:      longBody,
		BodySize:  len(longBody),
		Author:    "user",
		CreatedAt: "2025-01-01T00:00:00Z",
		UpdatedAt: "2025-01-01T00:00:00Z",
		URL:       "https://github.com/owner/repo/issues/1",
	}
	// Without --full: body should be truncated
	out := format.IssueView(issue, false)
	if !strings.Contains(out, "body_size: 1000") {
		t.Errorf("should show body_size, got: %s", out)
	}
	// The body field should be truncated to 500 chars
	// We can't easily check the exact string, but the output should not contain the full 1000 chars
	if strings.Contains(out, longBody) {
		t.Error("body should be truncated when full=false")
	}

	// With --full: full body
	outFull := format.IssueView(issue, true)
	if !strings.Contains(outFull, longBody) {
		t.Error("body should not be truncated when full=true")
	}
}

// ---- Label List ----

func TestFormatLabelList(t *testing.T) {
	labels := []format.LabelRow{
		{Name: "bug", Scope: "kind", Color: "ff0000", Description: "A bug"},
		{Name: "good-first-issue", Scope: "", Color: "7057ff", Description: "Good for newcomers"},
	}
	out := format.LabelList(labels)
	if out == "" {
		t.Fatal("expected non-empty output")
	}
	if !strings.Contains(out, "labels") {
		t.Errorf("output should contain 'labels', got: %s", out)
	}
	if !strings.Contains(out, "bug") {
		t.Errorf("output should contain label name, got: %s", out)
	}
	if !strings.Contains(out, "kind") {
		t.Errorf("output should contain label scope, got: %s", out)
	}
	if !strings.Contains(out, "2 labels") {
		t.Errorf("output should contain count, got: %s", out)
	}
}

func TestFormatLabelListEmpty(t *testing.T) {
	out := format.LabelList(nil)
	if !strings.Contains(out, "0 labels") {
		t.Errorf("output should contain '0 labels', got: %s", out)
	}
}

// ---- PR List ----

func TestFormatPRList(t *testing.T) {
	prs := []format.PRRow{
		{Number: 10, Stack: "auth", Title: "[auth:1/2] Add OAuth", State: "open"},
		{Number: 11, Stack: "auth", Title: "[auth:2/2] Add token refresh", State: "open"},
	}
	out := format.PRList(prs, 2, 5)
	if out == "" {
		t.Fatal("expected non-empty output")
	}
	if !strings.Contains(out, "prs") {
		t.Errorf("output should contain 'prs', got: %s", out)
	}
	if !strings.Contains(out, "auth") {
		t.Errorf("output should contain stack name, got: %s", out)
	}
	if !strings.Contains(out, "2 of 5 total") {
		t.Errorf("output should contain count, got: %s", out)
	}
	// Default fields: no author in header
	if strings.Contains(out, "author") && !strings.Contains(out, "depended_on_by") {
		t.Errorf("default PR list should NOT contain 'author' without --fields, got: %s", out)
	}
}

// ---- PR View ----

func TestFormatPRView(t *testing.T) {
	pr := &format.PRDetail{
		Number:   10,
		Title:    "[auth:1/2] Add OAuth",
		State:    "open",
		Body:     "Implements OAuth flow.",
		BodySize: len("Implements OAuth flow."),
		BaseRef:  "main",
		HeadRef:  "feat/auth",
		Stack:    "auth",
		DependsOn: []format.DepPR{
			{Number: 9, Title: "Base PR", State: "merged"},
		},
		DependedOnBy: []format.DepPR{
			{Number: 11, Title: "Add token refresh", State: "open"},
		},
		Author:    "dev",
		CreatedAt: "2025-01-15T10:30:00Z",
		UpdatedAt: "2025-01-16T08:00:00Z",
		URL:       "https://github.com/owner/repo/pull/10",
	}
	out := format.PRView(pr, false)
	if out == "" {
		t.Fatal("expected non-empty output")
	}
	if !strings.Contains(out, "number:") {
		t.Errorf("output should contain 'number:', got: %s", out)
	}
	if !strings.Contains(out, "depends_on") {
		t.Errorf("output should contain 'depends_on', got: %s", out)
	}
	if !strings.Contains(out, "depended_on_by") {
		t.Errorf("output should contain 'depended_on_by', got: %s", out)
	}
	if !strings.Contains(out, "stack:") {
		t.Errorf("output should contain 'stack:', got: %s", out)
	}
	if !strings.Contains(out, "Base PR") {
		t.Errorf("output should contain dep PR title, got: %s", out)
	}
}

func TestFormatPRViewDraft(t *testing.T) {
	pr := &format.PRDetail{
		Number: 10,
		Title:  "WIP feature",
		State:  "open",
		Draft:  true,
		Author: "dev",
		URL:    "https://github.com/owner/repo/pull/10",
	}
	out := format.PRView(pr, false)
	if !strings.Contains(out, "draft:") {
		t.Errorf("output should contain 'draft:' for draft PRs, got: %s", out)
	}

	pr2 := &format.PRDetail{
		Number: 11,
		Title:  "Normal PR",
		State:  "open",
		Draft:  false,
		Author: "dev",
		URL:    "https://github.com/owner/repo/pull/11",
	}
	out2 := format.PRView(pr2, false)
	if strings.Contains(out2, "draft:") {
		t.Errorf("output should NOT contain 'draft:' for non-draft PRs, got: %s", out2)
	}
}

func TestFormatPRViewReviewers(t *testing.T) {
	pr := &format.PRDetail{
		Number: 10,
		Title:  "PR with reviewers",
		State:  "open",
		Reviewers: []format.ReviewerState{
			{Login: "alice", State: "APPROVED"},
			{Login: "bob", State: "CHANGES_REQUESTED"},
		},
		ChecksPassed: 2,
		ChecksTotal:  3,
		Author:       "dev",
		URL:          "https://github.com/owner/repo/pull/10",
	}
	out := format.PRView(pr, false)
	if !strings.Contains(out, "reviewers") {
		t.Errorf("output should contain 'reviewers', got: %s", out)
	}
	if !strings.Contains(out, "APPROVED") {
		t.Errorf("output should contain review state, got: %s", out)
	}
	if !strings.Contains(out, "checks_passed:") {
		t.Errorf("output should contain 'checks_passed:', got: %s", out)
	}
	if !strings.Contains(out, "checks_total:") {
		t.Errorf("output should contain 'checks_total:', got: %s", out)
	}
}

func TestFormatPRViewTruncation(t *testing.T) {
	longBody := strings.Repeat("x", 600)
	pr := &format.PRDetail{
		Number:   10,
		Title:    "Truncated PR",
		State:    "open",
		Body:     longBody,
		BodySize: len(longBody),
		Author:   "dev",
		URL:      "https://github.com/owner/repo/pull/10",
	}
	out := format.PRView(pr, false)
	if strings.Contains(out, longBody) {
		t.Error("body should be truncated without --full")
	}
	if !strings.Contains(out, "600") {
		t.Errorf("should show body_size 600, got: %s", out)
	}
	if !strings.Contains(out, "--full") {
		t.Errorf("should show --full hint, got: %s", out)
	}

	outFull := format.PRView(pr, true)
	if !strings.Contains(outFull, longBody) {
		t.Error("body should not be truncated with full=true")
	}
}

// ---- Auth Status ----

func TestFormatAuthStatus(t *testing.T) {
	hosts := []format.AuthRow{
		{Forge: "github", Host: "github.com", Source: "~/.cache/anvil/credentials.json"},
		{Forge: "gitlab", Host: "gitlab.com", Source: "~/.cache/anvil/credentials.json"},
	}
	out := format.AuthStatus(hosts)
	if out == "" {
		t.Fatal("expected non-empty output")
	}
	if !strings.Contains(out, "github") {
		t.Errorf("output should contain forge type, got: %s", out)
	}
	if !strings.Contains(out, "github.com") {
		t.Errorf("output should contain host, got: %s", out)
	}
}

func TestFormatAuthStatusEmpty(t *testing.T) {
	out := format.AuthStatus(nil)
	if !strings.Contains(out, "No credentials configured") {
		t.Errorf("output should mention no credentials, got: %s", out)
	}
}

// ---- Error ----

func TestFormatError(t *testing.T) {
	out := format.Error("Not authenticated", "Run `anvil auth set github.com <token>`")
	if out == "" {
		t.Fatal("expected non-empty output")
	}
	if !strings.Contains(out, "error:") {
		t.Errorf("output should contain 'error:', got: %s", out)
	}
	if !strings.Contains(out, "help:") {
		t.Errorf("output should contain 'help:', got: %s", out)
	}
	if !strings.Contains(out, "Not authenticated") {
		t.Errorf("output should contain error message, got: %s", out)
	}
}

func TestFormatErrorNoHelp(t *testing.T) {
	out := format.Error("Something went wrong", "")
	if !strings.Contains(out, "error:") {
		t.Errorf("output should contain 'error:', got: %s", out)
	}
	if strings.Contains(out, "help:") {
		t.Errorf("output should NOT contain 'help:' when no help provided, got: %s", out)
	}
}

// ---- Body Truncation ----

func TestTruncateBody(t *testing.T) {
	short := "Hello"
	trunc, size := format.TruncateBody(short, 500)
	if trunc != short {
		t.Errorf("short body should not be truncated, got: %s", trunc)
	}
	if size != len(short) {
		t.Errorf("size should be %d, got: %d", len(short), size)
	}

	long := strings.Repeat("abcdefghij", 100) // 1000 chars
	trunc, size = format.TruncateBody(long, 500)
	if size != len(long) {
		t.Errorf("size should be %d, got: %d", len(long), size)
	}
	if len(trunc) != 500 {
		t.Errorf("truncated body should be 500 chars, got: %d", len(trunc))
	}
	if trunc == long {
		t.Error("long body should be truncated")
	}
	// Should end with "..."
	if !strings.HasSuffix(trunc, "...") {
		t.Errorf("truncated body should end with '...', got: %s", trunc[len(trunc)-5:])
	}
}

func TestTruncateBodyZeroMax(t *testing.T) {
	body := "Hello"
	trunc, size := format.TruncateBody(body, 0)
	if trunc != body {
		t.Errorf("with maxLen=0, body should not be truncated, got: %s", trunc)
	}
	if size != len(body) {
		t.Errorf("size mismatch: got %d, want %d", size, len(body))
	}
}

// ---- Issue Create/Update Confirmation ----

func TestFormatIssueCreateConfirm(t *testing.T) {
	out := format.IssueCreateConfirm(42, "Fix bug", "https://github.com/owner/repo/issues/42", nil)
	if !strings.Contains(out, "created:") {
		t.Errorf("should contain 'created:', got: %s", out)
	}
	if !strings.Contains(out, "42") {
		t.Errorf("should contain issue number, got: %s", out)
	}
	if !strings.Contains(out, "Fix bug") {
		t.Errorf("should contain title, got: %s", out)
	}
	if !strings.Contains(out, "url:") {
		t.Errorf("should contain 'url:', got: %s", out)
	}
}

func TestFormatIssueCreateConfirmWithAutoCreatedLabels(t *testing.T) {
	auto := []format.AutoCreatedLabel{
		{Name: "kind:bug", Color: "333333"},
	}
	out := format.IssueCreateConfirm(42, "Fix bug", "https://github.com/owner/repo/issues/42", auto)
	if !strings.Contains(out, "auto_created_labels") {
		t.Errorf("should contain 'auto_created_labels', got: %s", out)
	}
	if !strings.Contains(out, "kind:bug") {
		t.Errorf("should contain auto-created label name, got: %s", out)
	}
	if !strings.Contains(out, "333333") {
		t.Errorf("should contain auto-created label color, got: %s", out)
	}
}

func TestFormatIssueUpdateConfirm(t *testing.T) {
	out := format.IssueUpdateConfirm(42, "Fix bug", "https://github.com/owner/repo/issues/42",
		[]string{"kind:bug", "priority:high"}, nil)
	if !strings.Contains(out, "updated:") {
		t.Errorf("should contain 'updated:', got: %s", out)
	}
	if !strings.Contains(out, "42") {
		t.Errorf("should contain issue number, got: %s", out)
	}
	if !strings.Contains(out, "Fix bug") {
		t.Errorf("should contain title, got: %s", out)
	}
	if !strings.Contains(out, "url:") {
		t.Errorf("should contain 'url:', got: %s", out)
	}
	if !strings.Contains(out, "kind:bug") {
		t.Errorf("should contain label kind:bug, got: %s", out)
	}
	if !strings.Contains(out, "priority:high") {
		t.Errorf("should contain label priority:high, got: %s", out)
	}
}

func TestFormatIssueUpdateConfirmNoLabels(t *testing.T) {
	out := format.IssueUpdateConfirm(42, "Fix bug", "https://github.com/owner/repo/issues/42", nil, nil)
	if !strings.Contains(out, "updated:") {
		t.Errorf("should contain 'updated:', got: %s", out)
	}
	// With omitempty and nil labels, the labels field should not appear.
	if strings.Contains(out, "labels:") {
		t.Errorf("should NOT contain 'labels:' when no labels, got: %s", out)
	}
}

func TestFormatIssueUpdateConfirmWithAutoCreatedLabels(t *testing.T) {
	auto := []format.AutoCreatedLabel{
		{Name: "kind:bug", Color: "333333"},
	}
	out := format.IssueUpdateConfirm(42, "Fix bug", "https://github.com/owner/repo/issues/42",
		nil, auto)
	if !strings.Contains(out, "auto_created_labels") {
		t.Errorf("should contain 'auto_created_labels', got: %s", out)
	}
	if !strings.Contains(out, "333333") {
		t.Errorf("should contain auto-created label color, got: %s", out)
	}
}

func TestFormatIssueCloseConfirm(t *testing.T) {
	out := format.IssueCloseConfirm(42)
	if !strings.Contains(out, "closed:") {
		t.Errorf("should contain 'closed:', got: %s", out)
	}
	if !strings.Contains(out, "42") {
		t.Errorf("should contain issue number, got: %s", out)
	}
}

// ---- PR Create/Merge Confirm ----

func TestFormatPRCreateConfirm(t *testing.T) {
	out := format.PRCreateConfirm(10, "Add OAuth", "https://github.com/owner/repo/pull/10")
	if !strings.Contains(out, "created:") {
		t.Errorf("should contain 'created:', got: %s", out)
	}
	if !strings.Contains(out, "10") {
		t.Errorf("should contain PR number, got: %s", out)
	}
}

func TestFormatPRMergeConfirm(t *testing.T) {
	out := format.PRMergeConfirm(10)
	if !strings.Contains(out, "merged:") {
		t.Errorf("should contain 'merged:', got: %s", out)
	}
}

// ---- Label Create/Update Confirm ----

func TestFormatLabelCreateConfirm(t *testing.T) {
	out := format.LabelCreateConfirm("bug", "kind")
	if !strings.Contains(out, "created:") {
		t.Errorf("should contain 'created:', got: %s", out)
	}
	if !strings.Contains(out, "bug") {
		t.Errorf("should contain label name, got: %s", out)
	}
}

func TestFormatLabelDeleteConfirm(t *testing.T) {
	out := format.LabelDeleteConfirm("bug", "kind")
	if !strings.Contains(out, "deleted:") {
		t.Errorf("should contain 'deleted:', got: %s", out)
	}
}

// ---- Diagnostic ----

func TestFormatDiagnostic(t *testing.T) {
	msg := `stack "auth" is broken: PR #2 (Mid) is closed without merging. Consider: anvil pr rebase --onto <target> --skip 2`
	out := format.Diagnostic(msg)
	if out == "" {
		t.Fatal("expected non-empty output")
	}
	if !strings.Contains(out, "diagnostic:") {
		t.Errorf("output should contain 'diagnostic:', got: %s", out)
	}
	if !strings.Contains(out, "broken") {
		t.Errorf("output should contain the diagnostic message, got: %s", out)
	}
	if !strings.Contains(out, "closed without merging") {
		t.Errorf("output should contain the diagnostic details, got: %s", out)
	}
}

// ---- Comment List ----

func TestFormatCommentList(t *testing.T) {
	comments := []format.CommentRow{
		{ID: 1, Author: "alice", Body: "First comment"},
		{ID: 2, Author: "bob", Body: "Second comment with a much longer body that should be truncated at 80 characters for display"},
	}
	out := format.CommentList(comments, 2, 5)
	if out == "" {
		t.Fatal("expected non-empty output")
	}
	if !strings.Contains(out, "comments") {
		t.Errorf("output should contain 'comments', got: %s", out)
	}
	if !strings.Contains(out, "2 of 5 comments") {
		t.Errorf("output should contain count '2 of 5 comments', got: %s", out)
	}
	if !strings.Contains(out, "alice") {
		t.Errorf("output should contain author, got: %s", out)
	}
	// Body longer than 80 chars should be truncated.
	if strings.Contains(out, "truncated at 80 characters for display") {
		t.Errorf("long body should be truncated at 80 chars, got full text: %s", out)
	}
}

func TestFormatCommentListEmpty(t *testing.T) {
	out := format.CommentList([]format.CommentRow{}, 0, 0)
	if out == "" {
		t.Fatal("expected non-empty output even when empty")
	}
	if !strings.Contains(out, "0 of 0 comments") {
		t.Errorf("should show 0 of 0 comments, got: %s", out)
	}
}

// ---- Comment View ----

func TestFormatCommentView(t *testing.T) {
	detail := &format.CommentDetail{
		ID:        1,
		Body:      "A comment body.",
		BodySize:  len("A comment body."),
		Author:    "alice",
		System:    false,
		CreatedAt: "2025-01-01T00:00:00Z",
		UpdatedAt: "2025-06-01T00:00:00Z",
		URL:       "https://github.com/test/repo/issues/42#comment-1",
	}
	out := format.CommentView(detail, false)
	if out == "" {
		t.Fatal("expected non-empty output")
	}
	if !strings.Contains(out, "id: 1") {
		t.Errorf("should contain id, got: %s", out)
	}
	if !strings.Contains(out, "body:") {
		t.Errorf("should contain body, got: %s", out)
	}
	if !strings.Contains(out, "author: alice") {
		t.Errorf("should contain author, got: %s", out)
	}
	if !strings.Contains(out, "url:") {
		t.Errorf("should contain url, got: %s", out)
	}
}

func TestFormatCommentViewTruncation(t *testing.T) {
	body := strings.Repeat("x", 600)
	detail := &format.CommentDetail{
		ID:       1,
		Body:     body,
		BodySize: 600,
		Author:   "alice",
		System:   false,
	}
	out := format.CommentView(detail, false)
	if strings.Contains(out, body) {
		t.Errorf("body should be truncated without --full, got full body")
	}
	if !strings.Contains(out, "Use --full to see the complete body") {
		t.Errorf("should show --full hint, got: %s", out)
	}
}

func TestFormatCommentViewFull(t *testing.T) {
	body := strings.Repeat("x", 600)
	detail := &format.CommentDetail{
		ID:       1,
		Body:     body,
		BodySize: 600,
		Author:   "alice",
	}
	out := format.CommentView(detail, true)
	if !strings.Contains(out, body) {
		t.Errorf("full body should be shown with --full")
	}
	if !strings.Contains(out, "body_size: 600") {
		t.Errorf("should show body_size, got: %s", out)
	}
}

// ---- Comment Confirm ----

func TestFormatCommentCreateConfirm(t *testing.T) {
	out := format.CommentCreateConfirm(42, 1, "https://github.com/test/repo/issues/42#comment-1")
	if out == "" {
		t.Fatal("expected non-empty output")
	}
	if !strings.Contains(out, "created: 1") {
		t.Errorf("should contain created ID, got: %s", out)
	}
	if !strings.Contains(out, "issue: 42") {
		t.Errorf("should contain issue number, got: %s", out)
	}
	if !strings.Contains(out, "https://github.com/test/repo/issues/42#comment-1") {
		t.Errorf("should contain url, got: %s", out)
	}
}

func TestFormatCommentUpdateConfirm(t *testing.T) {
	out := format.CommentUpdateConfirm(42, 1, "https://github.com/test/repo/issues/42#comment-1")
	if out == "" {
		t.Fatal("expected non-empty output")
	}
	if !strings.Contains(out, "updated: 1") {
		t.Errorf("should contain updated ID, got: %s", out)
	}
}

func TestFormatCommentDeleteConfirm(t *testing.T) {
	out := format.CommentDeleteConfirm(42, 1)
	if out == "" {
		t.Fatal("expected non-empty output")
	}
	if !strings.Contains(out, "deleted: 1") {
		t.Errorf("should contain deleted ID, got: %s", out)
	}
	if !strings.Contains(out, "issue: 42") {
		t.Errorf("should contain issue number, got: %s", out)
	}
}
