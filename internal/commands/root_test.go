package commands_test

import (
	"bytes"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/tnikic/anvil/internal/commands"
	"github.com/tnikic/anvil/internal/forge"
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
	// Error is returned unprinted — main.go is the sole owner of error formatting.
	if err == nil {
		t.Fatal("expected error when outside git repo")
	}

	var se forge.StructuredError
	if errors.As(err, &se) {
		help := se.Help()
		if !strings.Contains(help, "--forge") {
			t.Errorf("help should suggest --forge flag, got: %s", help)
		}
	} else {
		t.Errorf("expected StructuredError, got: %v", err)
	}
}

func TestFlagOverrides(t *testing.T) {
	cmd := commands.NewRoot()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--forge", "gitlab.com", "--repo", "my/project"})

	err := cmd.Execute()
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
