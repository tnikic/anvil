package commands_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tnikic/anvil/internal/commands"
)

func setHomeEnv(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	return dir
}

func TestSetupHooksInstall(t *testing.T) {
	tmpHome := setHomeEnv(t)

	cmd := commands.NewRoot()
	buf := new(strings.Builder)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"setup", "hooks"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "status: installed") {
		t.Errorf("should report installed, got: %s", out)
	}
	if !strings.Contains(out, "integrations: Claude Code, Codex, OpenCode") {
		t.Errorf("should list integrations, got: %s", out)
	}
	if !strings.Contains(out, "binary:") {
		t.Errorf("should report binary, got: %s", out)
	}

	// Verify Claude Code hooks file.
	claudePath := filepath.Join(tmpHome, ".claude", "settings.json")
	verifyHookFile(t, claudePath)

	// Verify Codex hooks file.
	codexPath := filepath.Join(tmpHome, ".codex", "hooks.json")
	verifyHookFile(t, codexPath)

	// Verify Codex config toml.
	codexConfigPath := filepath.Join(tmpHome, ".codex", "config.toml")
	configData, err := os.ReadFile(codexConfigPath)
	if err != nil {
		t.Fatalf("codex config.toml should exist: %v", err)
	}
	if !strings.Contains(string(configData), "[features]") {
		t.Errorf("config.toml should have [features] section")
	}
	if !strings.Contains(string(configData), "hooks = true") {
		t.Errorf("config.toml should have hooks = true")
	}

	// Verify OpenCode plugin.
	opencodePath := filepath.Join(tmpHome, ".config", "opencode", "plugins", "axi-anvil.js")
	pluginData, err := os.ReadFile(opencodePath)
	if err != nil {
		t.Fatalf("opencode plugin should exist: %v", err)
	}
	if !strings.Contains(string(pluginData), "anvil-managed") {
		t.Errorf("plugin should have managed marker")
	}
	if !strings.Contains(string(pluginData), "experimental_chat_system_transform") {
		t.Errorf("plugin should contain transform function")
	}
}

func verifyHookFile(t *testing.T, path string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("hook file %s should exist: %v", path, err)
	}

	var hf struct {
		Hooks struct {
			SessionStart []struct {
				Matcher string `json:"matcher"`
				Type    string `json:"type"`
				Command string `json:"command"`
				Timeout int    `json:"timeout"`
			} `json:"SessionStart"`
		} `json:"hooks"`
	}
	if err := json.Unmarshal(data, &hf); err != nil {
		t.Fatalf("hook file %s should be valid JSON: %v", path, err)
	}

	if len(hf.Hooks.SessionStart) == 0 {
		t.Fatalf("hook file %s should have at least one SessionStart entry", path)
	}

	entry := hf.Hooks.SessionStart[0]
	if entry.Type != "command" {
		t.Errorf("entry type should be 'command', got %q", entry.Type)
	}
	if entry.Command == "" {
		t.Errorf("entry command should not be empty")
	}
	if entry.Timeout != 10 {
		t.Errorf("entry timeout should be 10, got %d", entry.Timeout)
	}
}

func TestSetupHooksIdempotent(t *testing.T) {
	tmpHome := setHomeEnv(t)

	// First install.
	cmd1 := commands.NewRoot()
	cmd1.SetArgs([]string{"setup", "hooks"})
	if err := cmd1.Execute(); err != nil {
		t.Fatal(err)
	}

	// Second install should be silent no-op.
	cmd2 := commands.NewRoot()
	buf := new(strings.Builder)
	cmd2.SetOut(buf)
	cmd2.SetErr(buf)
	cmd2.SetArgs([]string{"setup", "hooks"})
	if err := cmd2.Execute(); err != nil {
		t.Fatal(err)
	}

	out := buf.String()
	if !strings.Contains(out, "status: installed") {
		t.Errorf("second run should still report installed, got: %s", out)
	}

	// Files should still exist and be valid.
	claudePath := filepath.Join(tmpHome, ".claude", "settings.json")
	verifyHookFile(t, claudePath)
}

func TestSetupHooksCheck(t *testing.T) {
	_ = setHomeEnv(t)

	// Check before install — all missing.
	cmd := commands.NewRoot()
	buf := new(strings.Builder)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"setup", "hooks", "--check"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("--check should fail when no hooks installed")
	}

	out := buf.String()
	if !strings.Contains(out, "hooks:") {
		t.Errorf("should show hooks section, got: %s", out)
	}
	if !strings.Contains(out, "status: missing") {
		t.Errorf("should show missing status, got: %s", out)
	}
	if !strings.Contains(out, "0 of 3 current") {
		t.Errorf("should show '0 of 3 current', got: %s", out)
	}
}

func TestSetupHooksCheckCurrent(t *testing.T) {
	_ = setHomeEnv(t)

	// Install first.
	cmd1 := commands.NewRoot()
	cmd1.SetArgs([]string{"setup", "hooks"})
	if err := cmd1.Execute(); err != nil {
		t.Fatal(err)
	}

	// Check should report all current.
	cmd2 := commands.NewRoot()
	buf := new(strings.Builder)
	cmd2.SetOut(buf)
	cmd2.SetErr(buf)
	cmd2.SetArgs([]string{"setup", "hooks", "--check"})

	err := cmd2.Execute()
	if err != nil {
		t.Fatalf("--check should succeed when all hooks current: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "3 of 3 current") {
		t.Errorf("should show '3 of 3 current', got: %s", out)
	}
	if !strings.Contains(out, "status: current") {
		t.Errorf("should show current status, got: %s", out)
	}
}

func TestSetupHooksUninstall(t *testing.T) {
	tmpHome := setHomeEnv(t)

	// Uninstall when nothing installed — no-op.
	cmd := commands.NewRoot()
	buf := new(strings.Builder)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"setup", "hooks", "--uninstall"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("uninstall should not error when nothing to remove: %v", err)
	}
	if !strings.Contains(buf.String(), "status: uninstalled") {
		t.Errorf("should report uninstalled, got: %s", buf.String())
	}

	// Install then uninstall.
	cmd1 := commands.NewRoot()
	cmd1.SetArgs([]string{"setup", "hooks"})
	if err := cmd1.Execute(); err != nil {
		t.Fatal(err)
	}

	cmd2 := commands.NewRoot()
	buf2 := new(strings.Builder)
	cmd2.SetOut(buf2)
	cmd2.SetErr(buf2)
	cmd2.SetArgs([]string{"setup", "hooks", "--uninstall"})
	if err := cmd2.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf2.String(), "status: uninstalled") {
		t.Errorf("should report uninstalled, got: %s", buf2.String())
	}

	// Verify files are removed.
	claudePath := filepath.Join(tmpHome, ".claude", "settings.json")
	if _, err := os.Stat(claudePath); !os.IsNotExist(err) {
		t.Errorf("claude hooks file should be removed")
	}
	codexPath := filepath.Join(tmpHome, ".codex", "hooks.json")
	if _, err := os.Stat(codexPath); !os.IsNotExist(err) {
		t.Errorf("codex hooks file should be removed")
	}
	opencodePath := filepath.Join(tmpHome, ".config", "opencode", "plugins", "axi-anvil.js")
	if _, err := os.Stat(opencodePath); !os.IsNotExist(err) {
		t.Errorf("opencode plugin should be removed")
	}
}

func TestSetupHooksUninstallPreservesOtherHooks(t *testing.T) {
	tmpHome := setHomeEnv(t)

	// Create a Claude settings.json with a non-anvil hook.
	claudePath := filepath.Join(tmpHome, ".claude", "settings.json")
	dir := filepath.Dir(claudePath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	otherHook := `{
  "hooks": {
    "SessionStart": [
      {"matcher": "", "type": "command", "command": "other-tool", "timeout": 5}
    ]
  }
}
`
	if err := os.WriteFile(claudePath, []byte(otherHook), 0o644); err != nil {
		t.Fatal(err)
	}

	// Install anvil hooks.
	cmd1 := commands.NewRoot()
	cmd1.SetArgs([]string{"setup", "hooks"})
	if err := cmd1.Execute(); err != nil {
		t.Fatal(err)
	}

	// Uninstall.
	cmd2 := commands.NewRoot()
	cmd2.SetArgs([]string{"setup", "hooks", "--uninstall"})
	if err := cmd2.Execute(); err != nil {
		t.Fatal(err)
	}

	// Verify the other hook was preserved.
	data, err := os.ReadFile(claudePath)
	if err != nil {
		t.Fatalf("claude hooks file should still exist: %v", err)
	}
	var hf struct {
		Hooks struct {
			SessionStart []struct {
				Command string `json:"command"`
			} `json:"SessionStart"`
		} `json:"hooks"`
	}
	if err := json.Unmarshal(data, &hf); err != nil {
		t.Fatal(err)
	}
	if len(hf.Hooks.SessionStart) != 1 {
		t.Fatalf("should have 1 remaining hook, got %d", len(hf.Hooks.SessionStart))
	}
	if hf.Hooks.SessionStart[0].Command != "other-tool" {
		t.Errorf("other hook should be preserved, got command: %s", hf.Hooks.SessionStart[0].Command)
	}
}

func TestSetupHooksHelp(t *testing.T) {
	cmd := commands.NewRoot()
	buf := new(strings.Builder)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"setup", "hooks", "--help"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("--help should not error: %v", err)
	}
	if buf.String() == "" {
		t.Error("--help should produce output")
	}
}

func TestSetupHelp(t *testing.T) {
	cmd := commands.NewRoot()
	buf := new(strings.Builder)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"setup", "--help"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("--help should not error: %v", err)
	}
	if buf.String() == "" {
		t.Error("--help should produce output")
	}
}
