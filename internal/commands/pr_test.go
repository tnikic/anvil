package commands_test

import (
	"strings"
	"testing"

	"github.com/tnikic/anvil/internal/forge"
	"github.com/tnikic/anvil/internal/forge/forgetest"
)

// ---- Setup with PR service ----

func setupForgeTestWithPR(t *testing.T) (*forgetest.FakePRService, *forgetest.FakeForge) {
	t.Helper()
	fk := forgetest.Setup(t)
	return fk.PRSvc, fk
}

// ---- Tests ----

func TestPRListDefaultOutput(t *testing.T) {
	prSvc, fk := setupForgeTestWithPR(t)
	_ = fk
	prSvc.PRs = []forge.PR{
		{Number: 1, Title: "Fix login", State: "open", BaseRef: "main", HeadRef: "login-fix"},
		{Number: 2, Title: "Add rate limit", State: "open", BaseRef: "main", HeadRef: "rate-limit"},
	}

	buf, err := runCmd("pr", "list", "--forge", "github.com", "--repo", "test/repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Fix login") {
		t.Errorf("should contain first PR title, got: %s", out)
	}
	if !strings.Contains(out, "Add rate limit") {
		t.Errorf("should contain second PR title, got: %s", out)
	}
	if !strings.Contains(out, "prs") {
		t.Errorf("should contain 'prs' key, got: %s", out)
	}
	if !strings.Contains(out, "2 of 2 total") {
		t.Errorf("should show count aggregate, got: %s", out)
	}
}

func TestPRListStackSorting(t *testing.T) {
	prSvc, fk := setupForgeTestWithPR(t)
	_ = fk
	prSvc.PRs = []forge.PR{
		{Number: 3, Title: "[auth:2/2] Add token refresh", State: "open", BaseRef: "feat/auth", HeadRef: "token-refresh"},
		{Number: 1, Title: "Standalone fix", State: "open", BaseRef: "main", HeadRef: "fix"},
		{Number: 2, Title: "[auth:1/2] Add OAuth", State: "open", BaseRef: "main", HeadRef: "feat/auth"},
	}

	buf, err := runCmd("pr", "list", "--forge", "github.com", "--repo", "test/repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()

	// Unstacked PR (#1) should appear before stacked PRs
	idxStandalone := strings.Index(out, "Standalone fix")
	idxAuth1 := strings.Index(out, "[auth:1/2]")
	idxAuth2 := strings.Index(out, "[auth:2/2]")

	if idxStandalone == -1 || idxAuth1 == -1 || idxAuth2 == -1 {
		t.Fatalf("missing expected PRs in output: %s", out)
	}
	if idxStandalone > idxAuth1 {
		t.Errorf("unstacked PR should appear before stacked PRs")
	}
	if idxAuth1 > idxAuth2 {
		t.Errorf("auth:1/2 should appear before auth:2/2 within stack")
	}
}

func TestPRListStackColumn(t *testing.T) {
	prSvc, fk := setupForgeTestWithPR(t)
	_ = fk
	prSvc.PRs = []forge.PR{
		{Number: 1, Title: "[auth:1/1] Add OAuth", State: "open", BaseRef: "main", HeadRef: "feat/auth"},
		{Number: 2, Title: "No stack", State: "open", BaseRef: "main", HeadRef: "fix"},
	}

	buf, err := runCmd("pr", "list", "--forge", "github.com", "--repo", "test/repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	// The stacked PR should show "auth" as its stack value
	if !strings.Contains(out, "auth") {
		t.Errorf("should contain stack name 'auth' for stacked PR, got: %s", out)
	}
}

func TestPRListStateAll(t *testing.T) {
	prSvc, fk := setupForgeTestWithPR(t)
	_ = fk
	prSvc.PRs = []forge.PR{
		{Number: 1, Title: "Open PR", State: "open", BaseRef: "main", HeadRef: "feat"},
		{Number: 2, Title: "Merged PR", State: "merged", BaseRef: "main", HeadRef: "done"},
		{Number: 3, Title: "Closed PR", State: "closed", BaseRef: "main", HeadRef: "wontdo"},
	}

	buf, err := runCmd("pr", "list", "--forge", "github.com", "--repo", "test/repo", "--state", "all")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Merged PR") {
		t.Errorf("should show merged PR with --state all, got: %s", out)
	}
	if !strings.Contains(out, "Closed PR") {
		t.Errorf("should show closed PR with --state all, got: %s", out)
	}
}

func TestPRListFields(t *testing.T) {
	prSvc, fk := setupForgeTestWithPR(t)
	_ = fk
	prSvc.PRs = []forge.PR{
		{Number: 1, Title: "Test PR", State: "open", Author: "alice", BaseRef: "main", HeadRef: "feat"},
	}

	buf, err := runCmd("pr", "list",
		"--forge", "github.com", "--repo", "test/repo",
		"--fields", "number,stack,title,state,author,created",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Just verify it doesn't crash — the extended fields may be empty
	out := buf.String()
	if !strings.Contains(out, "1 of 1 total") {
		t.Errorf("should show count, got: %s", out)
	}
}

func TestPRView(t *testing.T) {
	prSvc, fk := setupForgeTestWithPR(t)
	_ = fk
	prSvc.PRs = []forge.PR{
		{
			Number:  10,
			Title:   "Add OAuth",
			State:   "open",
			Body:    "Implements OAuth flow.",
			BaseRef: "main",
			HeadRef: "feat/auth",
			Author:  "dev",
			URL:     "https://github.com/test/repo/pull/10",
		},
	}

	buf, err := runCmd("pr", "view", "10", "--forge", "github.com", "--repo", "test/repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Add OAuth") {
		t.Errorf("should contain PR title, got: %s", out)
	}
	if !strings.Contains(out, "number:") {
		t.Errorf("should contain 'number:', got: %s", out)
	}
	if !strings.Contains(out, "depends_on") {
		t.Errorf("should contain 'depends_on', got: %s", out)
	}
	if !strings.Contains(out, "depended_on_by") {
		t.Errorf("should contain 'depended_on_by', got: %s", out)
	}
	if !strings.Contains(out, "base_ref:") {
		t.Errorf("should contain 'base_ref:', got: %s", out)
	}
	if !strings.Contains(out, "head_ref:") {
		t.Errorf("should contain 'head_ref:', got: %s", out)
	}
}

func TestPRViewDeps(t *testing.T) {
	prSvc, fk := setupForgeTestWithPR(t)
	_ = fk
	prSvc.PRs = []forge.PR{
		{Number: 9, Title: "Base PR", State: "merged", BaseRef: "main", HeadRef: "feat/base"},
		{Number: 10, Title: "Middle PR", State: "open", BaseRef: "feat/base", HeadRef: "feat/middle"},
		{Number: 11, Title: "Top PR", State: "open", BaseRef: "feat/middle", HeadRef: "feat/top"},
	}

	buf, err := runCmd("pr", "view", "10", "--forge", "github.com", "--repo", "test/repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	// depends_on should show #9 (head_ref feat/base == pr 10's base_ref)
	if !strings.Contains(out, "Base PR") {
		t.Errorf("depends_on should contain Base PR (#9), got: %s", out)
	}
	// depended_on_by should show #11 (base_ref feat/middle == pr 10's head_ref)
	if !strings.Contains(out, "Top PR") {
		t.Errorf("depended_on_by should contain Top PR (#11), got: %s", out)
	}
}

func TestPRViewStackedTitle(t *testing.T) {
	prSvc, fk := setupForgeTestWithPR(t)
	_ = fk
	prSvc.PRs = []forge.PR{
		{Number: 10, Title: "[auth:2/3] Middle PR", State: "open", BaseRef: "feat/auth-1", HeadRef: "feat/auth-2"},
		{Number: 9, Title: "[auth:1/3] Base PR", State: "open", BaseRef: "main", HeadRef: "feat/auth-1"},
		{Number: 11, Title: "[auth:3/3] Top PR", State: "open", BaseRef: "feat/auth-2", HeadRef: "feat/auth-3"},
	}

	buf, err := runCmd("pr", "view", "10", "--forge", "github.com", "--repo", "test/repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "stack:") || !strings.Contains(out, "auth") {
		t.Errorf("should show stack name 'auth', got: %s", out)
	}
}

func TestPRViewDraft(t *testing.T) {
	prSvc, fk := setupForgeTestWithPR(t)
	_ = fk
	prSvc.PRs = []forge.PR{
		{
			Number:  10,
			Title:   "WIP feature",
			State:   "open",
			BaseRef: "main",
			HeadRef: "wip",
			Author:  "dev",
			URL:     "https://github.com/test/repo/pull/10",
			Extras:  map[string]any{"draft": true},
		},
	}

	buf, err := runCmd("pr", "view", "10", "--forge", "github.com", "--repo", "test/repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "draft:") {
		t.Errorf("should show draft status for draft PRs, got: %s", out)
	}
}

func TestPRViewTruncation(t *testing.T) {
	prSvc, fk := setupForgeTestWithPR(t)
	_ = fk
	body := strings.Repeat("x", 600)
	prSvc.PRs = []forge.PR{
		{
			Number:  10,
			Title:   "Long body PR",
			State:   "open",
			Body:    body,
			BaseRef: "main",
			HeadRef: "feat",
			Author:  "dev",
			URL:     "https://github.com/test/repo/pull/10",
		},
	}

	buf, err := runCmd("pr", "view", "10", "--forge", "github.com", "--repo", "test/repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if strings.Contains(out, body) {
		t.Error("body should be truncated without --full")
	}
	if !strings.Contains(out, "600") {
		t.Errorf("should show body_size 600, got: %s", out)
	}
	if !strings.Contains(out, "--full") {
		t.Errorf("should show --full hint, got: %s", out)
	}
}

func TestPRViewFull(t *testing.T) {
	prSvc, fk := setupForgeTestWithPR(t)
	_ = fk
	body := strings.Repeat("x", 600)
	prSvc.PRs = []forge.PR{
		{
			Number:  10,
			Title:   "Long body PR",
			State:   "open",
			Body:    body,
			BaseRef: "main",
			HeadRef: "feat",
			Author:  "dev",
			URL:     "https://github.com/test/repo/pull/10",
		},
	}

	buf, err := runCmd("pr", "view", "10", "--full", "--forge", "github.com", "--repo", "test/repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, body) {
		t.Error("body should not be truncated with --full")
	}
}

func TestPRViewInvalidNumber(t *testing.T) {
	_, _ = setupForgeTestWithPR(t)

	_, err := runCmd("pr", "view", "notanumber", "--forge", "github.com", "--repo", "test/repo")
	if err == nil {
		t.Fatal("expected error for invalid number")
	}
}

func TestPRListDefaultState(t *testing.T) {
	prSvc, fk := setupForgeTestWithPR(t)
	_ = fk
	prSvc.PRs = []forge.PR{
		{Number: 1, Title: "Open", State: "open", BaseRef: "main", HeadRef: "feat"},
		{Number: 2, Title: "Merged", State: "merged", BaseRef: "main", HeadRef: "done"},
	}

	buf, err := runCmd("pr", "list", "--forge", "github.com", "--repo", "test/repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if strings.Contains(out, "Merged") {
		t.Errorf("default --state open should exclude merged PRs, got: %s", out)
	}
	if !strings.Contains(out, "Open") {
		t.Errorf("should show open PRs by default, got: %s", out)
	}
}

func TestPRSubcommandHelp(t *testing.T) {
	_, _ = setupForgeTestWithPR(t)

	subs := []string{"list", "view", "create", "merge"}
	for _, sub := range subs {
		t.Run(sub, func(t *testing.T) {
			buf, err := runCmd("pr", sub, "--help")
			if err != nil {
				t.Fatalf("unexpected error for pr %s --help: %v", sub, err)
			}
			out := buf.String()
			if out == "" {
				t.Errorf("help output for pr %s should not be empty", sub)
			}
		})
	}
}

// ---- PR create tests ----

func TestPRCreateUnstacked(t *testing.T) {
	prSvc, _ := setupForgeTestWithPR(t)

	buf, err := runCmd("pr", "create",
		"--forge", "github.com", "--repo", "test/repo",
		"--title", "Fix bug",
		"--head", "fix-branch",
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

	// The created PR should not have a stack prefix
	if prSvc.LastCreateOpts.Title != nil && strings.Contains(*prSvc.LastCreateOpts.Title, "[") {
		t.Errorf("unstacked PR should not have stack prefix, got: %s", *prSvc.LastCreateOpts.Title)
	}

	// Verify the PR was added to the list
	if len(prSvc.PRs) != 1 {
		t.Fatalf("expected 1 PR in fake store, got %d", len(prSvc.PRs))
	}
	if prSvc.PRs[0].Title != "Fix bug" {
		t.Errorf("stored title = %q, want %q", prSvc.PRs[0].Title, "Fix bug")
	}
}

func TestPRCreateWithStackFlag(t *testing.T) {
	prSvc, _ := setupForgeTestWithPR(t)

	buf, err := runCmd("pr", "create",
		"--forge", "github.com", "--repo", "test/repo",
		"--title", "Add OAuth",
		"--head", "my-branch",
		"--stack", "auth",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Add OAuth") {
		t.Errorf("should contain title, got: %s", out)
	}

	// Title should have stack prefix [auth:1/1]
	expectedPrefix := "[auth:1/1]"
	if !strings.Contains(out, expectedPrefix) {
		t.Errorf("output should contain %q, got: %s", expectedPrefix, out)
	}

	// Verify stored title
	if len(prSvc.PRs) != 1 {
		t.Fatalf("expected 1 PR, got %d", len(prSvc.PRs))
	}
	if !strings.HasPrefix(prSvc.PRs[0].Title, "[auth:1/1]") {
		t.Errorf("stored title should have stack prefix, got: %s", prSvc.PRs[0].Title)
	}
}

func TestPRCreateSecondInStack(t *testing.T) {
	prSvc, _ := setupForgeTestWithPR(t)

	// Pre-populate with one existing stacked PR
	prSvc.PRs = []forge.PR{
		{Number: 1, Title: "[auth:1/1] Add OAuth", State: "open", BaseRef: "main", HeadRef: "feat/auth"},
	}

	buf, err := runCmd("pr", "create",
		"--forge", "github.com", "--repo", "test/repo",
		"--title", "Add token refresh",
		"--head", "token-refresh",
		"--stack", "auth",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()

	// New PR should be [auth:2/2]
	if !strings.Contains(out, "[auth:2/2]") {
		t.Errorf("new PR should be [auth:2/2], got: %s", out)
	}

	// Existing PR should be updated to [auth:1/2]
	updated := false
	for _, p := range prSvc.PRs {
		if p.Number == 1 && strings.HasPrefix(p.Title, "[auth:1/2]") {
			updated = true
		}
	}
	if !updated {
		t.Error("existing PR #1 should have been renumbered to [auth:1/2]")
		for _, p := range prSvc.PRs {
			t.Logf("PR #%d: %s", p.Number, p.Title)
		}
	}
}

func TestPRCreateStackAutoDerived(t *testing.T) {
	prSvc, _ := setupForgeTestWithPR(t)

	buf, err := runCmd("pr", "create",
		"--forge", "github.com", "--repo", "test/repo",
		"--title", "Add OAuth",
		"--head", "feat/auth",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	// Stack name should be auto-derived from branch "feat/auth" → "auth"
	if !strings.Contains(out, "[auth:1/1]") {
		t.Errorf("should auto-derive stack 'auth' from branch, got: %s", out)
	}

	_ = prSvc
}

func TestPRCreateAutoDerivedNoStack(t *testing.T) {
	prSvc, _ := setupForgeTestWithPR(t)

	buf, err := runCmd("pr", "create",
		"--forge", "github.com", "--repo", "test/repo",
		"--title", "Fix",
		"--head", "fix-branch",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// fix-branch has no '/' so no stack auto-derived
	if prSvc.LastCreateOpts.Title != nil && strings.Contains(*prSvc.LastCreateOpts.Title, "[") {
		t.Errorf("branch without '/' should not auto-derive stack, got: %s", *prSvc.LastCreateOpts.Title)
	}
	_ = buf
}

func TestPRCreateMissingTitle(t *testing.T) {
	_, _ = setupForgeTestWithPR(t)

	_, err := runCmd("pr", "create",
		"--forge", "github.com", "--repo", "test/repo",
		"--head", "my-branch",
	)
	if err == nil {
		t.Fatal("expected error for missing --title")
	}
}

func TestPRCreateDraft(t *testing.T) {
	prSvc, _ := setupForgeTestWithPR(t)

	_, err := runCmd("pr", "create",
		"--forge", "github.com", "--repo", "test/repo",
		"--title", "WIP",
		"--head", "wip-branch",
		"--draft",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if prSvc.LastCreateOpts.Draft == nil || !*prSvc.LastCreateOpts.Draft {
		t.Error("draft should be true")
	}
}

// ---- PR merge tests ----

func TestPRMergeUnstacked(t *testing.T) {
	prSvc, _ := setupForgeTestWithPR(t)
	prSvc.PRs = []forge.PR{
		{Number: 51, Title: "Fix bug", State: "open", BaseRef: "main", HeadRef: "fix"},
	}

	buf, err := runCmd("pr", "merge", "51",
		"--forge", "github.com", "--repo", "test/repo",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "merged") {
		t.Errorf("should contain 'merged', got: %s", out)
	}

	// PR should now be merged
	got := false
	for _, p := range prSvc.PRs {
		if p.Number == 51 && p.State == "merged" {
			got = true
		}
	}
	if !got {
		t.Error("PR #51 should be merged")
	}
}

func TestPRMergeStackedRenumbersRemaining(t *testing.T) {
	prSvc, _ := setupForgeTestWithPR(t)
	prSvc.NextNumber = 200 // ensure create doesn't clash
	prSvc.PRs = []forge.PR{
		{Number: 51, Title: "[auth:1/2] Add OAuth", State: "open", BaseRef: "main", HeadRef: "feat/auth"},
		{Number: 52, Title: "[auth:2/2] Add token refresh", State: "open", BaseRef: "feat/auth", HeadRef: "token-refresh"},
	}

	buf, err := runCmd("pr", "merge", "51",
		"--forge", "github.com", "--repo", "test/repo",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "merged") {
		t.Errorf("should contain 'merged', got: %s", out)
	}

	// PR #51 should be merged (retains its title prefix [auth:1/2])
	for _, p := range prSvc.PRs {
		if p.Number == 51 {
			if p.State != "merged" {
				t.Errorf("PR #51 should be merged, got state: %s", p.State)
			}
			if !strings.HasPrefix(p.Title, "[auth:1/2]") {
				t.Errorf("merged PR should retain its title prefix, got: %s", p.Title)
			}
		}
	}

	// PR #52 should be renumbered to [auth:1/1]
	for _, p := range prSvc.PRs {
		if p.Number == 52 {
			if !strings.HasPrefix(p.Title, "[auth:1/1]") {
				t.Errorf("PR #52 should be renumbered to [auth:1/1], got: %s", p.Title)
			}
		}
	}
}

func TestPRMergeStackedLastPR(t *testing.T) {
	prSvc, _ := setupForgeTestWithPR(t)
	prSvc.PRs = []forge.PR{
		{Number: 51, Title: "[auth:1/1] Only PR", State: "open", BaseRef: "main", HeadRef: "feat/auth"},
	}

	buf, err := runCmd("pr", "merge", "51",
		"--forge", "github.com", "--repo", "test/repo",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "merged") {
		t.Errorf("should contain 'merged', got: %s", out)
	}

	// PR #51 should be merged
	for _, p := range prSvc.PRs {
		if p.Number == 51 && p.State != "merged" {
			t.Errorf("PR #51 should be merged, got: %s", p.State)
		}
	}

	// No renumbering needed (no remaining open PRs in stack)
	if len(prSvc.LastUpdateOpts) > 0 {
		t.Logf("update calls (should be none since no remaining PRs): %v", prSvc.LastUpdateOpts)
	}
}

func TestPRMergeInvalidNumber(t *testing.T) {
	_, _ = setupForgeTestWithPR(t)

	_, err := runCmd("pr", "merge", "notanumber",
		"--forge", "github.com", "--repo", "test/repo",
	)
	if err == nil {
		t.Fatal("expected error for invalid number")
	}
}

// ---- Broken stack detection tests ----

func TestPRListDetectsBrokenStack(t *testing.T) {
	prSvc, _ := setupForgeTestWithPR(t)
	prSvc.PRs = []forge.PR{
		{Number: 1, Title: "[auth:1/3] Base PR", State: "open", BaseRef: "main", HeadRef: "feat/auth"},
		{Number: 2, Title: "[auth:2/3] Middle PR", State: "closed", BaseRef: "feat/auth", HeadRef: "feat/middle"},
		{Number: 3, Title: "[auth:3/3] Top PR", State: "open", BaseRef: "feat/middle", HeadRef: "feat/top"},
	}

	buf, err := runCmd("pr", "list", "--forge", "github.com", "--repo", "test/repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	// Should contain a diagnostic about the broken stack
	if !strings.Contains(out, "diagnostic") {
		t.Errorf("should contain 'diagnostic' for broken stack, got: %s", out)
	}
	if !strings.Contains(out, "broken") {
		t.Errorf("should contain 'broken', got: %s", out)
	}
	if !strings.Contains(out, "closed without merging") {
		t.Errorf("should mention closed without merging, got: %s", out)
	}
	if !strings.Contains(out, "rebase") {
		t.Errorf("should suggest rebase --onto, got: %s", out)
	}
}

func TestPRListNoBrokenStackWhenOnlyMerged(t *testing.T) {
	prSvc, _ := setupForgeTestWithPR(t)
	prSvc.PRs = []forge.PR{
		{Number: 1, Title: "[auth:1/3] Base PR", State: "merged", BaseRef: "main", HeadRef: "feat/auth"},
		{Number: 2, Title: "[auth:2/3] Middle PR", State: "open", BaseRef: "feat/auth", HeadRef: "feat/middle"},
		{Number: 3, Title: "[auth:3/3] Top PR", State: "open", BaseRef: "feat/middle", HeadRef: "feat/top"},
	}

	buf, err := runCmd("pr", "list", "--forge", "github.com", "--repo", "test/repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	// Merged PRs are normal — no broken stack diagnostic
	if strings.Contains(out, "closed without merging") {
		t.Errorf("should NOT contain broken stack diagnostic for merged PRs, got: %s", out)
	}
}

func TestPRListNoBrokenStackWhenClean(t *testing.T) {
	prSvc, _ := setupForgeTestWithPR(t)
	prSvc.PRs = []forge.PR{
		{Number: 1, Title: "[auth:1/2] Base PR", State: "open", BaseRef: "main", HeadRef: "feat/auth"},
		{Number: 2, Title: "[auth:2/2] Top PR", State: "open", BaseRef: "feat/auth", HeadRef: "feat/top"},
	}

	buf, err := runCmd("pr", "list", "--forge", "github.com", "--repo", "test/repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if strings.Contains(out, "broken") {
		t.Errorf("should NOT contain 'broken' for clean stack, got: %s", out)
	}
}
