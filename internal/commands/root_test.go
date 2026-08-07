package commands_test

import (
	"bytes"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/tnikic/anvil/internal/commands"
	"github.com/tnikic/anvil/internal/forge"
	"github.com/tnikic/anvil/internal/forge/forgetest"
)

func TestHomeView(t *testing.T) {
	cmd := commands.NewRoot()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "bin:") {
		t.Errorf("home view should contain 'bin:', got: %s", out)
	}
	if !strings.Contains(out, "description:") {
		t.Errorf("home view should contain 'description:', got: %s", out)
	}
}

func TestOutsideGitRepo(t *testing.T) {
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatal(err)
	}

	cmd := commands.NewRoot()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	err = cmd.Execute()
	// Fallback: no error, shows identity + auth/targeting guidance.
	if err != nil {
		t.Fatalf("unexpected error outside git repo (should show fallback): %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "bin:") {
		t.Errorf("fallback should contain 'bin:', got: %s", out)
	}
	if !strings.Contains(out, "description:") {
		t.Errorf("fallback should contain 'description:', got: %s", out)
	}
	if !strings.Contains(out, "help[") {
		t.Errorf("fallback should contain help hints, got: %s", out)
	}
	if !strings.Contains(out, "--forge") {
		t.Errorf("fallback help should mention --forge, got: %s", out)
	}
}

func TestFlagOverrides(t *testing.T) {
	cmd := commands.NewRoot()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--forge", "gitlab.com", "--repo", "my/project"})

	err := cmd.Execute()
	// Auth will fail without setup, but forge/repo should still appear.
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "forge: gitlab.com") {
		t.Errorf("output should contain 'forge: gitlab.com', got: %s", out)
	}
	if !strings.Contains(out, "repo: my/project") {
		t.Errorf("output should contain 'repo: my/project', got: %s", out)
	}
	// Without auth, should show fallback hints.
	if !strings.Contains(out, "help[") {
		t.Errorf("output should contain fallback help hints, got: %s", out)
	}
}

// ---- Dashboard tests with live data ----

func TestDashboardWithIssuesAndPRs(t *testing.T) {
	fk := forgetest.Setup(t)
	fk.IssueSvc.Issues = []forge.Issue{
		{Number: 1, Title: "Fix login timeout", State: forge.StateOpen, Author: "alice"},
		{Number: 2, Title: "Add rate limiting", State: forge.StateOpen, Author: "bob"},
	}
	fk.PRSvc.PRs = []forge.PR{
		{Number: 100, Title: "Refactor auth", State: forge.StateOpen, Author: "carol"},
	}

	cmd := commands.NewRoot()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "forge: github.com") {
		t.Errorf("should contain forge: %s", out)
	}
	if !strings.Contains(out, "issues") {
		t.Errorf("should contain issues section: %s", out)
	}
	if !strings.Contains(out, "Fix login timeout") {
		t.Errorf("should contain issue title: %s", out)
	}
	if !strings.Contains(out, "prs") {
		t.Errorf("should contain PRs section: %s", out)
	}
	if !strings.Contains(out, "Refactor auth") {
		t.Errorf("should contain PR title: %s", out)
	}
	if !strings.Contains(out, "help[") {
		t.Errorf("should contain help hints: %s", out)
	}
}

func TestDashboardCountAggregates(t *testing.T) {
	fk := forgetest.Setup(t)
	// 5 issues, only 3 shown
	fk.IssueSvc.Issues = []forge.Issue{
		{Number: 1, Title: "A", State: forge.StateOpen, Author: "a"},
		{Number: 2, Title: "B", State: forge.StateOpen, Author: "b"},
		{Number: 3, Title: "C", State: forge.StateOpen, Author: "c"},
		{Number: 4, Title: "D", State: forge.StateOpen, Author: "d"},
		{Number: 5, Title: "E", State: forge.StateOpen, Author: "e"},
	}
	fk.PRSvc.PRs = []forge.PR{
		{Number: 10, Title: "PR1", State: forge.StateOpen, Author: "x"},
		{Number: 11, Title: "PR2", State: forge.StateOpen, Author: "y"},
		{Number: 12, Title: "PR3", State: forge.StateOpen, Author: "z"},
		{Number: 13, Title: "PR4", State: forge.StateOpen, Author: "w"},
	}

	cmd := commands.NewRoot()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	// Count aggregates
	if !strings.Contains(out, "3 of 5 total") {
		t.Errorf("issues count should show '3 of 5 total': %s", out)
	}
	if !strings.Contains(out, "3 of 4 total") {
		t.Errorf("PRs count should show '3 of 4 total': %s", out)
	}
	// Contextual hints should mention totals > 3
	if !strings.Contains(out, "for all 5 open issues") {
		t.Errorf("help should mention 'all 5 open issues': %s", out)
	}
	if !strings.Contains(out, "for all 4 open PRs") {
		t.Errorf("help should mention 'all 4 open PRs': %s", out)
	}
}

func TestDashboardEmptyRepo(t *testing.T) {
	forgetest.Setup(t)

	cmd := commands.NewRoot()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "issues: 0 open") {
		t.Errorf("should show 'issues: 0 open': %s", out)
	}
	if !strings.Contains(out, "prs: 0 open") {
		t.Errorf("should show 'prs: 0 open': %s", out)
	}
	// Should still have help hints
	if !strings.Contains(out, "help[") {
		t.Errorf("should contain help hints: %s", out)
	}
}

func TestDashboardForgeDetectionFailure(t *testing.T) {
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatal(err)
	}

	cmd := commands.NewRoot()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	err = cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error on forge detection failure: %v", err)
	}

	out := buf.String()
	// Should show identity (bin + description)
	if !strings.Contains(out, "bin:") {
		t.Errorf("should contain 'bin:' in fallback: %s", out)
	}
	if !strings.Contains(out, "description:") {
		t.Errorf("should contain 'description:' in fallback: %s", out)
	}
	// Should show auth/targeting guidance
	if !strings.Contains(out, "--forge") {
		t.Errorf("should mention --forge flag in fallback: %s", out)
	}
	if !strings.Contains(out, "auth set") {
		t.Errorf("should mention 'auth set' in fallback: %s", out)
	}
	// Should not show forge/repo since detection failed
	if strings.Contains(out, "forge:") {
		t.Errorf("should NOT contain forge when detection fails: %s", out)
	}
}

func TestSubcommandHelp(t *testing.T) {
	subcommands := []string{"issue", "label", "pr", "auth", "skills"}
	for _, sc := range subcommands {
		t.Run(sc, func(t *testing.T) {
			cmd := commands.NewRoot()
			buf := new(bytes.Buffer)
			cmd.SetOut(buf)
			cmd.SetErr(buf)
			cmd.SetArgs([]string{sc, "--help"})

			err := cmd.Execute()
			if err != nil {
				t.Fatalf("unexpected error for %s --help: %v", sc, err)
			}

			out := buf.String()
			if out == "" {
				t.Errorf("%s --help output should not be empty", sc)
			}
			// Subcommand help should not be the root help
			if strings.Contains(out, "anvil [flags]") {
				t.Errorf("%s --help should show subcommand help, not root help. got: %s", sc, out)
			}
		})
	}
}

func TestRootHelp(t *testing.T) {
	cmd := commands.NewRoot()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--help"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if out == "" {
		t.Error("help output should not be empty")
	}
	if !strings.Contains(out, "anvil") && !strings.Contains(out, "AXI") {
		t.Errorf("help output should mention the tool, got: %s", out)
	}
}

// ---- Unknown flag tests ----

func TestUnknownFlagOnRoot(t *testing.T) {
	cmd := commands.NewRoot()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--stat"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for unknown flag")
	}

	msg := err.Error()
	if !strings.Contains(msg, "unknown flag") {
		t.Errorf("error should contain unknown flag message, got: %s", msg)
	}
	if !strings.Contains(msg, "\"anvil\"") {
		t.Errorf("error should contain command path, got: %s", msg)
	}
}

func TestUnknownFlagOnSubcommand(t *testing.T) {
	cmd := commands.NewRoot()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"issue", "list", "--stat"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for unknown flag")
	}

	msg := err.Error()
	if !strings.Contains(msg, "unknown flag") {
		t.Errorf("error should contain unknown flag message, got: %s", msg)
	}
	// The command path should be "anvil issue list", not just "anvil"
	if !strings.Contains(msg, "\"anvil issue list\"") {
		t.Errorf("error should contain subcommand path, got: %s", msg)
	}
}

func TestUnknownFlagInlinesSubcommandFlags(t *testing.T) {
	cmd := commands.NewRoot()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"issue", "list", "--stat"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for unknown flag")
	}

	// Help is accessible via the StructuredError interface, not Error().
	// The error message contains the flag info; the help contains valid flags.
	var se forge.StructuredError
	if errors.As(err, &se) {
		help := se.Help()
		if !strings.Contains(help, "--state") {
			t.Errorf("should show --state flag (issue list specific), got: %s", help)
		}
		if !strings.Contains(help, "--label") {
			t.Errorf("should show --label flag, got: %s", help)
		}
	}
}

func TestUnknownFlagOnLabelCreate(t *testing.T) {
	cmd := commands.NewRoot()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"label", "create", "--bogus"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for unknown flag")
	}

	msg := err.Error()
	if !strings.Contains(msg, "\"anvil label create\"") {
		t.Errorf("should show label create path, got: %s", msg)
	}
}

func TestHelpAlwaysPassesThrough(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"root help", []string{"--help"}},
		{"issue list help", []string{"issue", "list", "--help"}},
		{"pr view help", []string{"pr", "view", "--help"}},
		{"label create help", []string{"label", "create", "--help"}},
		{"auth status help", []string{"auth", "status", "--help"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := commands.NewRoot()
			buf := new(bytes.Buffer)
			cmd.SetOut(buf)
			cmd.SetErr(buf)
			cmd.SetArgs(tt.args)

			err := cmd.Execute()
			if err != nil {
				t.Fatalf("--help should never be rejected: %v", err)
			}
			out := buf.String()
			if out == "" {
				t.Error("--help should produce non-empty output")
			}
			// Help should not contain error text
			if strings.Contains(out, "error: unknown flag") {
				t.Errorf("--help output should not contain error, got: %s", out)
			}
		})
	}
}
