package stack_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/tnikic/anvil/internal/forge"
	"github.com/tnikic/anvil/internal/stack"
)

// ---- Prefix tests ----

func TestParsePrefixValid(t *testing.T) {
	tests := []struct {
		title string
		want  stack.Prefix
	}{
		{"[auth:1/2] Add OAuth", stack.Prefix{Name: "auth", Pos: 1, Total: 2}},
		{"[auth:2/3] Add token refresh", stack.Prefix{Name: "auth", Pos: 2, Total: 3}},
		{"[my-stack:1/1] Single PR", stack.Prefix{Name: "my-stack", Pos: 1, Total: 1}},
		{"[a:10/42] Large numbers", stack.Prefix{Name: "a", Pos: 10, Total: 42}},
		{"[x.y_z:5/9] Weird name", stack.Prefix{Name: "x.y_z", Pos: 5, Total: 9}},
	}
	for _, tt := range tests {
		p, ok := stack.ParsePrefix(tt.title)
		if !ok {
			t.Errorf("ParsePrefix(%q): expected ok=true", tt.title)
			continue
		}
		if p != tt.want {
			t.Errorf("ParsePrefix(%q): got %+v, want %+v", tt.title, p, tt.want)
		}
	}
}

func TestParsePrefixInvalid(t *testing.T) {
	titles := []string{
		"",
		"No prefix here",
		"[auth:]",             // missing pos/total
		"[auth:1/] Add OAuth", // missing total
		"[auth:/2] Add OAuth", // missing pos
		"[auth:1/2",           // no closing bracket
		"prefix [auth:1/2] not at start",

		"[] fixed",
		"[:1/2] no name",
		"[*bad:1/2] special char", // * is not in [a-zA-Z]
	}
	for _, title := range titles {
		p, ok := stack.ParsePrefix(title)
		if ok {
			t.Errorf("ParsePrefix(%q): expected ok=false, got %+v", title, p)
		}
	}
}

func TestFormatPrefix(t *testing.T) {
	got := stack.FormatPrefix(stack.Prefix{Name: "auth", Pos: 2, Total: 3})
	want := "[auth:2/3]"
	if got != want {
		t.Errorf("FormatPrefix: got %q, want %q", got, want)
	}
}

func TestPrefixRoundTrip(t *testing.T) {
	original := "[fix:3/5] Some PR title"
	p, ok := stack.ParsePrefix(original)
	if !ok {
		t.Fatal("expected prefix to parse")
	}
	clean := stack.CleanTitle(original)
	if clean != "Some PR title" {
		t.Errorf("CleanTitle: got %q, want %q", clean, "Some PR title")
	}
	rebuilt := stack.FormatPrefix(p) + " " + clean
	if rebuilt != original {
		t.Errorf("round-trip: got %q, want %q", rebuilt, original)
	}
}

func TestCleanTitleNoPrefix(t *testing.T) {
	title := "Just a normal PR title"
	got := stack.CleanTitle(title)
	if got != title {
		t.Errorf("CleanTitle: got %q, want %q", got, title)
	}
}

// ---- Order tests ----

func TestPopulate(t *testing.T) {
	prs := []forge.PR{
		{Number: 1, Title: "[auth:1/2] Add OAuth"},
		{Number: 2, Title: "No stack"},
		{Number: 3, Title: "[auth:2/2] Add refresh"},
	}
	stack.Populate(prs)

	if prs[0].Stack != "auth" {
		t.Errorf("pr[0].Stack = %q, want auth", prs[0].Stack)
	}
	if prs[1].Stack != "" {
		t.Errorf("pr[1].Stack = %q, want empty", prs[1].Stack)
	}
	if prs[2].Stack != "auth" {
		t.Errorf("pr[2].Stack = %q, want auth", prs[2].Stack)
	}
}

func TestSortKey(t *testing.T) {
	pr1 := forge.PR{Number: 5, Title: "[auth:1/2] First", Stack: "auth"}
	pr2 := forge.PR{Number: 10, Title: "No stack", Stack: ""}
	pr3 := forge.PR{Number: 3, Title: "[auth:2/2] Second", Stack: "auth"}

	// Unstacked sorts before stacked.
	k2 := stack.SortKey(pr2)
	k1 := stack.SortKey(pr1)
	if k2 > k1 {
		t.Errorf("unstacked key should sort before stacked key: %q > %q", k2, k1)
	}
	// Within stack, sort by position.
	k3 := stack.SortKey(pr3)
	if k1 > k3 {
		t.Errorf("pos 1 should sort before pos 2: %q > %q", k1, k3)
	}
}

func TestSort(t *testing.T) {
	prs := []forge.PR{
		{Number: 3, Title: "[auth:3/3] Top", Stack: "auth", HeadRef: "feat/top", BaseRef: "feat/mid"},
		{Number: 1, Title: "Standalone fix", Stack: "", HeadRef: "fix", BaseRef: "main"},
		{Number: 2, Title: "[auth:2/3] Mid", Stack: "auth", HeadRef: "feat/mid", BaseRef: "feat/base"},
	}
	stack.Populate(prs) // re-set Stack from titles
	stack.Sort(prs)

	if prs[0].Number != 1 {
		t.Errorf("unstacked PR should be first, got #%d", prs[0].Number)
	}
	if prs[1].Number != 2 || prs[2].Number != 3 {
		t.Errorf("stacked PRs should be in position order, got #%d then #%d", prs[1].Number, prs[2].Number)
	}
}

func TestComputeDepends(t *testing.T) {
	prs := []forge.PR{
		{Number: 1, Title: "[auth:1/3] Base", HeadRef: "feat/base", BaseRef: "main"},
		{Number: 2, Title: "[auth:2/3] Mid", HeadRef: "feat/mid", BaseRef: "feat/base"},
		{Number: 3, Title: "[auth:3/3] Top", HeadRef: "feat/top", BaseRef: "feat/mid"},
	}
	stack.ComputeDepends(prs)

	// PR #1 (base): depends on nothing, depended on by #2
	if len(prs[0].DependsOn) != 0 {
		t.Errorf("base should have no depends_on, got %v", prs[0].DependsOn)
	}
	if len(prs[0].DependedOnBy) != 1 || prs[0].DependedOnBy[0] != 2 {
		t.Errorf("base should be depended on by #2, got %v", prs[0].DependedOnBy)
	}

	// PR #2 (mid): depends on #1, depended on by #3
	if len(prs[1].DependsOn) != 1 || prs[1].DependsOn[0] != 1 {
		t.Errorf("mid should depend on #1, got %v", prs[1].DependsOn)
	}
	if len(prs[1].DependedOnBy) != 1 || prs[1].DependedOnBy[0] != 3 {
		t.Errorf("mid should be depended on by #3, got %v", prs[1].DependedOnBy)
	}

	// PR #3 (top): depends on #2, depended on by nothing
	if len(prs[2].DependsOn) != 1 || prs[2].DependsOn[0] != 2 {
		t.Errorf("top should depend on #2, got %v", prs[2].DependsOn)
	}
	if len(prs[2].DependedOnBy) != 0 {
		t.Errorf("top should have no depended_on_by, got %v", prs[2].DependedOnBy)
	}
}

func TestComputeDependsDisjoint(t *testing.T) {
	prs := []forge.PR{
		{Number: 1, Title: "PR A", HeadRef: "feat/a", BaseRef: "main"},
		{Number: 2, Title: "PR B", HeadRef: "feat/b", BaseRef: "main"},
	}
	stack.ComputeDepends(prs)

	for i := range prs {
		if len(prs[i].DependsOn) != 0 {
			t.Errorf("disjoint PR #%d should have no depends_on", prs[i].Number)
		}
		if len(prs[i].DependedOnBy) != 0 {
			t.Errorf("disjoint PR #%d should have no depended_on_by", prs[i].Number)
		}
	}
}

func TestDeriveName(t *testing.T) {
	tests := []struct {
		branch string
		want   string
	}{
		{"feat/auth", "auth"},
		{"fix/bug-42", "bug-42"},
		{"user/tnikic/feature", "feature"},
		{"main", ""},
		{"fix-branch", ""},
		{"a/", ""},
		{"", ""},
	}
	for _, tt := range tests {
		got := stack.DeriveName(tt.branch)
		if got != tt.want {
			t.Errorf("DeriveName(%q) = %q, want %q", tt.branch, got, tt.want)
		}
	}
}

// ---- CollectOpen tests ----

func TestCollectOpen(t *testing.T) {
	prs := []forge.PR{
		{Number: 1, Title: "[auth:2/2] Top", State: forge.StateOpen},
		{Number: 2, Title: "[auth:1/2] Base", State: forge.StateOpen},
		{Number: 3, Title: "Unrelated", State: forge.StateOpen},
		{Number: 4, Title: "[auth:1/1] Closed one", State: forge.StateClosed},
		{Number: 5, Title: "[other:1/1] Other stack", State: forge.StateOpen},
	}
	collected := stack.CollectOpen("auth", prs)

	if len(collected) != 2 {
		t.Fatalf("expected 2 open auth PRs, got %d", len(collected))
	}
	// Should be sorted by position.
	if collected[0].Number != 2 {
		t.Errorf("first should be #2 (pos 1), got #%d", collected[0].Number)
	}
	if collected[1].Number != 1 {
		t.Errorf("second should be #1 (pos 2), got #%d", collected[1].Number)
	}
}

func TestCollectOpenNone(t *testing.T) {
	prs := []forge.PR{
		{Number: 1, Title: "[other:1/1] Other", State: forge.StateOpen},
	}
	collected := stack.CollectOpen("auth", prs)
	if len(collected) != 0 {
		t.Errorf("expected no results, got %d", len(collected))
	}
}

// ---- Tracker tests ----

type fakeUpdater struct {
	updateFn func(ctx context.Context, opts forge.PRUpdateOptions) (*forge.PR, error)
	calls    []forge.PRUpdateOptions
}

func (f *fakeUpdater) Update(ctx context.Context, opts forge.PRUpdateOptions) (*forge.PR, error) {
	f.calls = append(f.calls, opts)
	if f.updateFn != nil {
		return f.updateFn(ctx, opts)
	}
	return &forge.PR{Number: opts.Number, Title: *opts.Title}, nil
}

func TestRenumber(t *testing.T) {
	updater := &fakeUpdater{}
	tracker := stack.NewTracker(updater)

	// Titles need renumbering: [auth:1/3] → [auth:1/2], [auth:3/3] → [auth:2/2]
	prs := []forge.PR{
		{Number: 1, Title: "[auth:1/3] Add OAuth"},
		{Number: 2, Title: "[auth:3/3] Add refresh"},
	}

	err := tracker.Renumber(context.Background(), prs, "auth")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(updater.calls) != 2 {
		t.Fatalf("expected 2 update calls, got %d", len(updater.calls))
	}
	if *updater.calls[0].Title != "[auth:1/2] Add OAuth" {
		t.Errorf("PR #1 title: %q", *updater.calls[0].Title)
	}
	if *updater.calls[1].Title != "[auth:2/2] Add refresh" {
		t.Errorf("PR #2 title: %q", *updater.calls[1].Title)
	}
}

func TestRenumberChangesPositions(t *testing.T) {
	updater := &fakeUpdater{}
	tracker := stack.NewTracker(updater)

	// Simulate a merge: stack goes from 3 PRs to 2 remaining.
	prs := []forge.PR{
		{Number: 2, Title: "[auth:2/3] Mid"},
		{Number: 3, Title: "[auth:3/3] Top"},
	}

	err := tracker.Renumber(context.Background(), prs, "auth")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if *updater.calls[0].Title != "[auth:1/2] Mid" {
		t.Errorf("PR #2 should be renumbered to [auth:1/2], got %q", *updater.calls[0].Title)
	}
	if *updater.calls[1].Title != "[auth:2/2] Top" {
		t.Errorf("PR #3 should be renumbered to [auth:2/2], got %q", *updater.calls[1].Title)
	}
}

func TestRenumberSkipsUnchanged(t *testing.T) {
	updater := &fakeUpdater{}
	tracker := stack.NewTracker(updater)

	prs := []forge.PR{
		{Number: 1, Title: "[auth:1/1] Only one"},
	}

	err := tracker.Renumber(context.Background(), prs, "auth")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(updater.calls) != 0 {
		t.Errorf("expected no update calls (title already correct), got %d", len(updater.calls))
	}
}

func TestRenumberError(t *testing.T) {
	updater := &fakeUpdater{
		updateFn: func(ctx context.Context, opts forge.PRUpdateOptions) (*forge.PR, error) {
			return nil, errors.New("api error")
		},
	}
	tracker := stack.NewTracker(updater)

	prs := []forge.PR{
		{Number: 1, Title: "[auth:1/2] First"},
	}

	err := tracker.Renumber(context.Background(), prs, "auth")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "updating PR #1") {
		t.Errorf("error should mention PR number, got: %v", err)
	}
}

// ---- DiagnoseBroken tests ----

func TestDiagnoseBrokenClean(t *testing.T) {
	allPRs := []forge.PR{
		{Number: 1, Title: "[auth:1/2] Base", State: forge.StateOpen, HeadRef: "feat/base", BaseRef: "main"},
		{Number: 2, Title: "[auth:2/2] Top", State: forge.StateOpen, HeadRef: "feat/top", BaseRef: "feat/base"},
	}
	openPRs := []forge.PR{
		{Number: 1, Title: "[auth:1/2] Base", State: forge.StateOpen, HeadRef: "feat/base", BaseRef: "main"},
		{Number: 2, Title: "[auth:2/2] Top", State: forge.StateOpen, HeadRef: "feat/top", BaseRef: "feat/base"},
	}
	diags := stack.DiagnoseBroken(openPRs, allPRs)
	if len(diags) != 0 {
		t.Errorf("expected no diagnostics for clean stack, got: %v", diags)
	}
}

func TestDiagnoseBrokenMiddleClosed(t *testing.T) {
	allPRs := []forge.PR{
		{Number: 1, Title: "[auth:1/3] Base", State: forge.StateOpen, HeadRef: "feat/base", BaseRef: "main"},
		{Number: 2, Title: "[auth:2/3] Mid", State: forge.StateClosed, HeadRef: "feat/mid", BaseRef: "feat/base"},
		{Number: 3, Title: "[auth:3/3] Top", State: forge.StateOpen, HeadRef: "feat/top", BaseRef: "feat/mid"},
	}
	openPRs := []forge.PR{
		{Number: 1, Title: "[auth:1/3] Base", State: forge.StateOpen, HeadRef: "feat/base", BaseRef: "main"},
		{Number: 3, Title: "[auth:3/3] Top", State: forge.StateOpen, HeadRef: "feat/top", BaseRef: "feat/mid"},
	}
	diags := stack.DiagnoseBroken(openPRs, allPRs)
	if len(diags) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d", len(diags))
	}
	if !strings.Contains(diags[0], "broken") {
		t.Errorf("diagnostic should mention broken: %s", diags[0])
	}
	if !strings.Contains(diags[0], "closed without merging") {
		t.Errorf("diagnostic should mention closed without merging: %s", diags[0])
	}
}

func TestDiagnoseBrokenMergedOk(t *testing.T) {
	// Merged PRs between open ones are fine.
	allPRs := []forge.PR{
		{Number: 1, Title: "[auth:1/3] Base", State: forge.StateMerged, HeadRef: "feat/base", BaseRef: "main"},
		{Number: 2, Title: "[auth:2/3] Mid", State: forge.StateOpen, HeadRef: "feat/mid", BaseRef: "feat/base"},
		{Number: 3, Title: "[auth:3/3] Top", State: forge.StateOpen, HeadRef: "feat/top", BaseRef: "feat/mid"},
	}
	openPRs := []forge.PR{
		{Number: 2, Title: "[auth:2/3] Mid", State: forge.StateOpen, HeadRef: "feat/mid", BaseRef: "feat/base"},
		{Number: 3, Title: "[auth:3/3] Top", State: forge.StateOpen, HeadRef: "feat/top", BaseRef: "feat/mid"},
	}
	diags := stack.DiagnoseBroken(openPRs, allPRs)
	if len(diags) != 0 {
		t.Errorf("merged PR below open range should not trigger diagnostic: %v", diags)
	}
}

func TestDiagnoseBrokenClosedOutsideRange(t *testing.T) {
	// Closed PRs before the first open or after the last open are fine.
	allPRs := []forge.PR{
		{Number: 1, Title: "[auth:1/4] Closed early", State: forge.StateClosed, HeadRef: "feat/early", BaseRef: "main"},
		{Number: 2, Title: "[auth:2/4] Base open", State: forge.StateOpen, HeadRef: "feat/base", BaseRef: "main"},
		{Number: 3, Title: "[auth:3/4] Top open", State: forge.StateOpen, HeadRef: "feat/top", BaseRef: "feat/base"},
		{Number: 4, Title: "[auth:4/4] Closed late", State: forge.StateClosed, HeadRef: "feat/late", BaseRef: "feat/top"},
	}
	openPRs := []forge.PR{
		{Number: 2, Title: "[auth:2/4] Base open", State: forge.StateOpen, HeadRef: "feat/base", BaseRef: "main"},
		{Number: 3, Title: "[auth:3/4] Top open", State: forge.StateOpen, HeadRef: "feat/top", BaseRef: "feat/base"},
	}
	diags := stack.DiagnoseBroken(openPRs, allPRs)
	// #1 is before first open, #4 is after last open — no gap in active range.
	if len(diags) != 0 {
		t.Errorf("closed PRs outside open range should not trigger diagnostic: %v", diags)
	}
}

func TestDiagnoseBrokenSingleStack(t *testing.T) {
	// Single-PR stack — can't have a broken middle.
	allPRs := []forge.PR{
		{Number: 1, Title: "[auth:1/1] Only", State: forge.StateOpen, HeadRef: "feat/base", BaseRef: "main"},
	}
	openPRs := []forge.PR{
		{Number: 1, Title: "[auth:1/1] Only", State: forge.StateOpen, HeadRef: "feat/base", BaseRef: "main"},
	}
	diags := stack.DiagnoseBroken(openPRs, allPRs)
	if len(diags) != 0 {
		t.Errorf("single-PR stack should not trigger diagnostic: %v", diags)
	}
}

func TestDiagnoseBrokenMultipleStacks(t *testing.T) {
	// fix stack has a closed (#2) between two open ones (#1, #3).
	// feat stack is clean.
	allPRs := []forge.PR{
		{Number: 1, Title: "[fix:1/3] Base", State: forge.StateOpen},
		{Number: 2, Title: "[fix:2/3] Mid", State: forge.StateClosed},
		{Number: 3, Title: "[fix:3/3] Top", State: forge.StateOpen},
		{Number: 4, Title: "[feat:1/1] Only", State: forge.StateOpen},
	}
	openPRs := []forge.PR{
		{Number: 1, Title: "[fix:1/3] Base", State: forge.StateOpen},
		{Number: 3, Title: "[fix:3/3] Top", State: forge.StateOpen},
		{Number: 4, Title: "[feat:1/1] Only", State: forge.StateOpen},
	}
	diags := stack.DiagnoseBroken(openPRs, allPRs)
	if len(diags) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d", len(diags))
	}
	if !strings.Contains(diags[0], "fix") {
		t.Errorf("diagnostic should mention broken stack 'fix': %s", diags[0])
	}
}
