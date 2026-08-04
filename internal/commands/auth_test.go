package commands_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tnikic/anvil/internal/commands"
)

func setCacheEnv(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", dir)
	return dir
}

func TestAuthStatusEmpty(t *testing.T) {
	setCacheEnv(t)

	cmd := commands.NewRoot()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"auth", "status"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "No credentials configured") {
		t.Errorf("should show no credentials message, got: %s", out)
	}
}

func TestAuthSetAndStatus(t *testing.T) {
	setCacheEnv(t)

	// Set a token
	cmd := commands.NewRoot()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"auth", "set", "github.com", "ghp_test123"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("auth set: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "token stored") {
		t.Errorf("should confirm token stored, got: %s", out)
	}
	if !strings.Contains(out, "github.com") {
		t.Errorf("should show host, got: %s", out)
	}
	if !strings.Contains(out, "github") {
		t.Errorf("should show inferred forge type, got: %s", out)
	}

	// Now check status
	buf2 := new(bytes.Buffer)
	cmd2 := commands.NewRoot()
	cmd2.SetOut(buf2)
	cmd2.SetArgs([]string{"auth", "status"})

	err = cmd2.Execute()
	if err != nil {
		t.Fatalf("auth status: %v", err)
	}

	out2 := buf2.String()
	if !strings.Contains(out2, "github.com") {
		t.Errorf("status should show github.com, got: %s", out2)
	}
	if !strings.Contains(out2, "github") {
		t.Errorf("status should show forge type 'github', got: %s", out2)
	}
}

func TestAuthSetAndUnset(t *testing.T) {
	setCacheEnv(t)

	// Set
	cmd := commands.NewRoot()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"auth", "set", "gitlab.com", "glpat-abc"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("auth set: %v", err)
	}

	// Unset
	cmd2 := commands.NewRoot()
	buf2 := new(bytes.Buffer)
	cmd2.SetOut(buf2)
	cmd2.SetArgs([]string{"auth", "unset", "gitlab.com"})
	if err := cmd2.Execute(); err != nil {
		t.Fatalf("auth unset: %v", err)
	}

	out := buf2.String()
	if !strings.Contains(out, "token removed for gitlab.com") {
		t.Errorf("should confirm removal, got: %s", out)
	}

	// Status should be empty now
	cmd3 := commands.NewRoot()
	buf3 := new(bytes.Buffer)
	cmd3.SetOut(buf3)
	cmd3.SetArgs([]string{"auth", "status"})
	if err := cmd3.Execute(); err != nil {
		t.Fatalf("auth status: %v", err)
	}
	out3 := buf3.String()
	if !strings.Contains(out3, "No credentials configured") {
		t.Errorf("status should show empty after unset, got: %s", out3)
	}
}

func TestAuthUnsetMissing(t *testing.T) {
	setCacheEnv(t)

	cmd := commands.NewRoot()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"auth", "unset", "nonexistent.example.com"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unset missing should not error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "token removed") {
		t.Errorf("should confirm removal (no-op), got: %s", out)
	}
}

func TestAuthSetMultipleForgeTypes(t *testing.T) {
	setCacheEnv(t)

	hosts := []struct {
		host, token, wantForge string
	}{
		{"github.com", "ghp_1", "github"},
		{"gitlab.com", "glpat-2", "gitlab"},
		{"codeberg.org", "cb_3", "forgejo"},
	}

	for _, h := range hosts {
		cmd := commands.NewRoot()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetArgs([]string{"auth", "set", h.host, h.token})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("auth set %s: %v", h.host, err)
		}
	}

	// Status should show all three
	cmd := commands.NewRoot()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"auth", "status"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("auth status: %v", err)
	}

	out := buf.String()
	for _, h := range hosts {
		if !strings.Contains(out, h.host) {
			t.Errorf("status should contain host %s, got: %s", h.host, out)
		}
		if !strings.Contains(out, h.wantForge) {
			t.Errorf("status should contain forge %s for %s, got: %s", h.wantForge, h.host, out)
		}
	}
}

func TestAuthStatusPathCollapsesHome(t *testing.T) {
	dir := setCacheEnv(t)

	// Set a token
	cmd := commands.NewRoot()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"auth", "set", "github.com", "ghp_x"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("auth set: %v", err)
	}

	// Check that the path in status uses ~ if under home
	cmd2 := commands.NewRoot()
	buf2 := new(bytes.Buffer)
	cmd2.SetOut(buf2)
	cmd2.SetArgs([]string{"auth", "status"})
	if err := cmd2.Execute(); err != nil {
		t.Fatalf("auth status: %v", err)
	}

	out := buf2.String()
	expectedPath := filepath.Join(dir, "anvil", "credentials.json")
	if !strings.Contains(out, expectedPath) {
		// If the path isn't shown fully, it might be collapsed with ~
		home, _ := os.UserHomeDir()
		if home != "" && strings.HasPrefix(dir, home) {
			collapsed := "~" + dir[len(home):] + "/anvil/credentials.json"
			if !strings.Contains(out, collapsed) {
				t.Errorf("status should show path, expected %q or %q, got: %s", expectedPath, collapsed, out)
			}
		} else {
			t.Errorf("status should show path %q, got: %s", expectedPath, out)
		}
	}
}

func TestAuthSetRequiresArgs(t *testing.T) {
	setCacheEnv(t)

	tests := []struct {
		name string
		args []string
	}{
		{"no args", []string{"auth", "set"}},
		{"host only", []string{"auth", "set", "github.com"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := commands.NewRoot()
			buf := new(bytes.Buffer)
			cmd.SetOut(buf)
			cmd.SetErr(buf)
			cmd.SetArgs(tt.args)

			err := cmd.Execute()
			if err == nil {
				t.Errorf("expected error for %q, got nil", tt.name)
			}
		})
	}
}

func TestAuthUnsetRequiresArgs(t *testing.T) {
	setCacheEnv(t)

	cmd := commands.NewRoot()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"auth", "unset"})

	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for missing args")
	}
}
