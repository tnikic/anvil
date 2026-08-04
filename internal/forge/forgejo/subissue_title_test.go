package forgejo

import (
	"testing"

	gitea "code.gitea.io/sdk/gitea"
	"github.com/tnikic/anvil/internal/forge"
)

func TestParseNoParent(t *testing.T) {
	parent, clean := Parse("Regular issue title")
	if parent != nil {
		t.Errorf("parent = %v, want nil", *parent)
	}
	if clean != "Regular issue title" {
		t.Errorf("clean = %q, want %q", clean, "Regular issue title")
	}
}

func TestParseWithParent(t *testing.T) {
	parent, clean := Parse("[parent:42] Sub-task for login")
	if parent == nil {
		t.Fatal("parent = nil, want 42")
	}
	if *parent != 42 {
		t.Errorf("parent = %d, want 42", *parent)
	}
	if clean != "Sub-task for login" {
		t.Errorf("clean = %q, want %q", clean, "Sub-task for login")
	}
}

func TestParseWithParentNoSpace(t *testing.T) {
	parent, clean := Parse("[parent:1]Sub-task no space")
	if parent == nil {
		t.Fatal("parent = nil, want 1")
	}
	if *parent != 1 {
		t.Errorf("parent = %d, want 1", *parent)
	}
	if clean != "Sub-task no space" {
		t.Errorf("clean = %q, want %q", clean, "Sub-task no space")
	}
}

func TestParseEmptyTitle(t *testing.T) {
	parent, clean := Parse("")
	if parent != nil {
		t.Errorf("parent = %v, want nil", *parent)
	}
	if clean != "" {
		t.Errorf("clean = %q, want empty", clean)
	}
}

func TestParseOnlyParentPrefix(t *testing.T) {
	parent, clean := Parse("[parent:7]")
	if parent == nil {
		t.Fatal("parent = nil, want 7")
	}
	if *parent != 7 {
		t.Errorf("parent = %d, want 7", *parent)
	}
	if clean != "" {
		t.Errorf("clean = %q, want empty", clean)
	}
}

func TestParseInvalidPrefix(t *testing.T) {
	// Missing closing bracket — should not match.
	parent, clean := Parse("[parent:42 Some text")
	if parent != nil {
		t.Errorf("parent = %v, want nil", *parent)
	}
	if clean != "[parent:42 Some text" {
		t.Errorf("clean = %q, want unchanged", clean)
	}
}

func TestParseNonNumericParent(t *testing.T) {
	// Non-numeric parent number — should not match.
	parent, clean := Parse("[parent:abc] Task")
	if parent != nil {
		t.Errorf("parent = %v, want nil", *parent)
	}
	if clean != "[parent:abc] Task" {
		t.Errorf("clean = %q, want unchanged", clean)
	}
}

func TestInjectNilParent(t *testing.T) {
	result := Inject("Some title", nil)
	if result != "Some title" {
		t.Errorf("result = %q, want %q", result, "Some title")
	}
}

func TestInjectAddsPrefix(t *testing.T) {
	result := Inject("Sub-task", forge.Int(42))
	if result != "[parent:42] Sub-task" {
		t.Errorf("result = %q, want %q", result, "[parent:42] Sub-task")
	}
}

func TestInjectReplacesExistingPrefix(t *testing.T) {
	// If title already has a parent prefix, Inject replaces it.
	result := Inject("[parent:1] Old title", forge.Int(42))
	if result != "[parent:42] Old title" {
		t.Errorf("result = %q, want %q", result, "[parent:42] Old title")
	}
}

func TestInjectEmptyTitle(t *testing.T) {
	result := Inject("", forge.Int(99))
	if result != "[parent:99] " {
		t.Errorf("result = %q, want %q", result, "[parent:99] ")
	}
}

func TestStripRemovesPrefix(t *testing.T) {
	result := Strip("[parent:42] Sub-task")
	if result != "Sub-task" {
		t.Errorf("result = %q, want %q", result, "Sub-task")
	}
}

func TestStripNoPrefix(t *testing.T) {
	result := Strip("Regular title")
	if result != "Regular title" {
		t.Errorf("result = %q, want %q", result, "Regular title")
	}
}

func TestStripEmptyTitle(t *testing.T) {
	result := Strip("")
	if result != "" {
		t.Errorf("result = %q, want empty", result)
	}
}

func TestFindChildrenMatchesParent(t *testing.T) {
	issues := []*gitea.Issue{
		{Index: 2, Title: "[parent:1] Child A", State: "open"},
		{Index: 3, Title: "[parent:1] Child B", State: "closed"},
		{Index: 4, Title: "Not a child", State: "open"},
		{Index: 5, Title: "[parent:2] Other parent", State: "open"},
	}
	deps := FindChildren(issues, 1)
	if len(deps) != 2 {
		t.Fatalf("len(deps) = %d, want 2", len(deps))
	}
	if deps[0].Number != 2 || deps[0].Title != "Child A" || deps[0].Direction != forge.DirChild {
		t.Errorf("dep 0: %+v", deps[0])
	}
	if deps[1].Number != 3 || deps[1].Title != "Child B" || deps[1].State != "closed" {
		t.Errorf("dep 1: %+v", deps[1])
	}
}

func TestFindChildrenNoMatch(t *testing.T) {
	issues := []*gitea.Issue{
		{Index: 2, Title: "Not a child", State: "open"},
	}
	deps := FindChildren(issues, 1)
	if len(deps) != 0 {
		t.Errorf("len(deps) = %d, want 0", len(deps))
	}
}

func TestFindChildrenEmptyList(t *testing.T) {
	deps := FindChildren(nil, 1)
	if len(deps) != 0 {
		t.Errorf("len(deps) = %d, want 0", len(deps))
	}
}

func TestFindChildrenWithNilEntries(t *testing.T) {
	issues := []*gitea.Issue{
		nil,
		{Index: 2, Title: "[parent:1] Child", State: "open"},
		nil,
	}
	deps := FindChildren(issues, 1)
	if len(deps) != 1 {
		t.Fatalf("len(deps) = %d, want 1", len(deps))
	}
	if deps[0].Number != 2 {
		t.Errorf("dep number = %d, want 2", deps[0].Number)
	}
}

func TestRoundTrip(t *testing.T) {
	// Parse then Inject should produce a title that Parse can re-parse.
	original := "[parent:7] Original task"
	parent, clean := Parse(original)
	if parent == nil || *parent != 7 {
		t.Fatalf("parse: parent = %v", parent)
	}
	reinjected := Inject(clean, forge.Int(7))
	if reinjected != original {
		t.Errorf("reinjected = %q, want %q", reinjected, original)
	}

	// Re-parse to confirm.
	parent2, clean2 := Parse(reinjected)
	if parent2 == nil || *parent2 != 7 {
		t.Fatalf("re-parse: parent = %v", parent2)
	}
	if clean2 != "Original task" {
		t.Errorf("re-parse: clean = %q, want %q", clean2, "Original task")
	}
}
