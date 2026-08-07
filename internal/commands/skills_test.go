package commands_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tnikic/anvil/internal/commands"
)

func TestSkillsList(t *testing.T) {
	cmd := commands.NewRoot()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"skills", "list"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("skills list should not error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "skills[") {
		t.Errorf("should contain TOON table header, got: %s", out)
	}
	if !strings.Contains(out, "SKILL.md") {
		t.Errorf("should list SKILL.md, got: %s", out)
	}
	if !strings.Contains(out, "count: 1 total") {
		t.Errorf("should show count with total, got: %s", out)
	}
}

func TestSkillsInstall(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	cmd := commands.NewRoot()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"skills", "install"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("skills install should not error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "installed:") {
		t.Errorf("should report installed, got: %s", out)
	}
	if !strings.Contains(out, "files: 1") {
		t.Errorf("should report files: 1, got: %s", out)
	}

	// Verify file was created.
	skillPath := filepath.Join(tmpHome, ".agents", "skills", "anvil", "SKILL.md")
	data, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatalf("SKILL.md should exist at %s: %v", skillPath, err)
	}
	if !strings.Contains(string(data), "name: anvil") {
		t.Errorf("SKILL.md should contain frontmatter, got: %s", string(data))
	}
}

func TestSkillsUpdate(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	// Install first.
	cmd := commands.NewRoot()
	cmd.SetArgs([]string{"skills", "install"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	// Update.
	cmd2 := commands.NewRoot()
	buf := new(bytes.Buffer)
	cmd2.SetOut(buf)
	cmd2.SetErr(buf)
	cmd2.SetArgs([]string{"skills", "update"})
	if err := cmd2.Execute(); err != nil {
		t.Fatalf("skills update should not error: %v", err)
	}
	if !strings.Contains(buf.String(), "updated:") {
		t.Errorf("should report updated, got: %s", buf.String())
	}
}

func TestSkillsUninstall(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	// Uninstall when not installed → no-op.
	cmd := commands.NewRoot()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"skills", "uninstall"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	// No-op: uninstalling when not installed returns nil silently.
	// (The "no-op: not installed" message was moved to main.go's error formatting;
	//  here we just verify success.)

	// Install then uninstall.
	cmd2 := commands.NewRoot()
	cmd2.SetArgs([]string{"skills", "install"})
	if err := cmd2.Execute(); err != nil {
		t.Fatal(err)
	}

	cmd3 := commands.NewRoot()
	buf3 := new(bytes.Buffer)
	cmd3.SetOut(buf3)
	cmd3.SetErr(buf3)
	cmd3.SetArgs([]string{"skills", "uninstall"})
	if err := cmd3.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf3.String(), "removed:") {
		t.Errorf("should report removed, got: %s", buf3.String())
	}

	// Verify directory is gone.
	skillDir := filepath.Join(tmpHome, ".agents", "skills", "anvil")
	if _, err := os.Stat(skillDir); !os.IsNotExist(err) {
		t.Errorf("skills directory should be removed")
	}
}

func TestSkillsStatus(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	// Status before install.
	cmd := commands.NewRoot()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"skills", "status"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "installed: false") {
		t.Errorf("should report not installed, got: %s", buf.String())
	}

	// Install.
	cmd2 := commands.NewRoot()
	cmd2.SetArgs([]string{"skills", "install"})
	if err := cmd2.Execute(); err != nil {
		t.Fatal(err)
	}

	// Status after install.
	cmd3 := commands.NewRoot()
	buf3 := new(bytes.Buffer)
	cmd3.SetOut(buf3)
	cmd3.SetErr(buf3)
	cmd3.SetArgs([]string{"skills", "status"})
	if err := cmd3.Execute(); err != nil {
		t.Fatal(err)
	}
	out := buf3.String()
	if !strings.Contains(out, "installed: true") {
		t.Errorf("should report installed, got: %s", out)
	}
	if !strings.Contains(out, "name: anvil") {
		t.Errorf("should report name, got: %s", out)
	}
	if !strings.Contains(out, "path:") {
		t.Errorf("should report path, got: %s", out)
	}
}

func TestSkillsHelp(t *testing.T) {
	cmd := commands.NewRoot()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"skills", "--help"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if buf.String() == "" {
		t.Error("skills --help should not be empty")
	}
}

func TestSkillsCheck(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	// --check should pass when embedded skill matches generated.
	cmd := commands.NewRoot()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"skills", "status", "--check"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("--check should pass when skill is current: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "status: current") {
		t.Errorf("should show 'status: current', got: %s", out)
	}
}

func TestSkillsUpdateRegenerates(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	// Run update directly (no prior install).
	cmd := commands.NewRoot()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"skills", "update"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("skills update should not error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "updated:") {
		t.Errorf("should report updated, got: %s", out)
	}
	if !strings.Contains(out, "skill:") {
		t.Errorf("should show staleness note, got: %s", out)
	}

	// Verify file was created.
	skillPath := filepath.Join(tmpHome, ".agents", "skills", "anvil", "SKILL.md")
	data, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatalf("SKILL.md should exist: %v", err)
	}
	if !strings.Contains(string(data), "name: anvil") {
		t.Errorf("SKILL.md should contain frontmatter, got: %s", string(data))
	}
}

func TestSkillsInstallStaleNote(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	cmd := commands.NewRoot()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"skills", "install"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("skills install should not error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "installed:") {
		t.Errorf("should report installed, got: %s", out)
	}
	if !strings.Contains(out, "skill:") {
		t.Errorf("should show staleness note, got: %s", out)
	}
	if !strings.Contains(out, "current") && !strings.Contains(out, "stale") {
		t.Errorf("should show current or stale, got: %s", out)
	}
}
