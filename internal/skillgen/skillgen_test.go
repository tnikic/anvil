package skillgen

import (
	"strings"
	"testing"
)

func TestRender(t *testing.T) {
	out, err := Render()
	if err != nil {
		t.Fatalf("Render() error: %v", err)
	}

	// Must contain YAML frontmatter.
	if !strings.Contains(out, "---\nname: anvil") {
		t.Errorf("output should have YAML frontmatter, got: %s", truncate(out, 200))
	}

	// Must contain description.
	if !strings.Contains(out, "description:") {
		t.Errorf("output should have description, got: %s", truncate(out, 200))
	}

	// Must contain commands.
	if !strings.Contains(out, "## Commands") {
		t.Errorf("output should have Commands section, got: %s", truncate(out, 200))
	}

	// Must contain tips.
	if !strings.Contains(out, "## Tips") {
		t.Errorf("output should have Tips section, got: %s", truncate(out, 200))
	}

	// Must contain Output format section.
	if !strings.Contains(out, "## Output format") {
		t.Errorf("output should have Output format section, got: %s", truncate(out, 200))
	}
}

func TestCheckMatch(t *testing.T) {
	out, err := Render()
	if err != nil {
		t.Fatal(err)
	}

	ok, diff, err := Check(out)
	if err != nil {
		t.Fatalf("Check() error: %v", err)
	}
	if !ok {
		t.Errorf("Check should pass when embedded matches generated, diff: %s", diff)
	}
}

func TestCheckDrift(t *testing.T) {
	out, err := Render()
	if err != nil {
		t.Fatal(err)
	}

	// Modify the output to simulate drift.
	modified := strings.Replace(out, "name: anvil", "name: different", 1)

	ok, _, err := Check(modified)
	if err != nil {
		t.Fatalf("Check() error: %v", err)
	}
	if ok {
		t.Error("Check should fail when embedded differs from generated")
	}
}

func TestCheckDriftReturnsDiff(t *testing.T) {
	out, err := Render()
	if err != nil {
		t.Fatal(err)
	}

	modified := strings.Replace(out, "name: anvil", "name: different", 1)

	ok, diff, err := Check(modified)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("should report drift")
	}
	if diff == "" {
		t.Error("should return non-empty diff on drift")
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
