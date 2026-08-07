package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tnikic/anvil/internal/forge"
)

// ---- Hook data types ----

// hookEntry represents a single hook entry in JSON hook files.
// Used by Claude Code and Codex.
type hookEntry struct {
	Matcher string `json:"matcher"`
	Type    string `json:"type"`
	Command string `json:"command"`
	Timeout int    `json:"timeout"`
}

// hooksFile is the top-level structure for Claude/Codex hook files.
type hooksFile struct {
	Hooks struct {
		SessionStart []hookEntry `json:"SessionStart"`
	} `json:"hooks"`
}

// hookStatus describes the state of a single agent's hook installation.
type hookStatus struct {
	Agent string // "Claude Code", "Codex", "OpenCode"
	State string // "current", "stale", "missing"
	Path  string // file path
}

// anvilManagedMarker is the comment marker identifying anvil-managed files.
const anvilManagedMarker = "anvil-managed"

// ---- Command definition ----

func newSetupCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Set up agent integrations",
		Long:  "Set up agent harness integrations: hooks, skills, and more.",
	}
	cmd.AddCommand(newSetupHooksCmd())
	setFlagErrorFunc(cmd)
	return cmd
}

func newSetupHooksCmd() *cobra.Command {
	var (
		check     bool
		uninstall bool
	)

	cmd := &cobra.Command{
		Use:          "hooks",
		Short:        "Install SessionStart hooks for AI agent harnesses",
		SilenceUsage: true,
		Long: `Install SessionStart hooks that inject ambient forge context at
session start. Supports Claude Code, Codex, and OpenCode.

Without flags, installs hooks for all three agents. Hooks are idempotent:
re-running updates stale paths and is a silent no-op when current.

Use --check to verify hook status without making changes.
Use --uninstall to remove all anvil-managed hooks.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			w := cmd.OutOrStdout()

			switch {
			case uninstall:
				return runHooksUninstall(w)
			case check:
				return runHooksCheck(w)
			default:
				return runHooksInstall(w)
			}
		},
	}

	cmd.Flags().BoolVar(&check, "check", false, "Verify hooks are current without modifying")
	cmd.Flags().BoolVar(&uninstall, "uninstall", false, "Remove all anvil-managed hooks")

	setFlagErrorFunc(cmd)
	return cmd
}

// ---- Path resolution ----

// hookCommand returns the command string to use in hook entries.
// Uses the bare binary name if it resolves in PATH to the current executable;
// otherwise falls back to the absolute path with home collapsed to ~.
func hookCommand() string {
	exe, err := os.Executable()
	if err != nil {
		return "anvil"
	}

	base := filepath.Base(exe)
	looked, err := exec.LookPath(base)
	if err == nil && looked == exe {
		return base
	}
	return collapseHome(filepath.Clean(exe))
}

// hookPaths returns the file paths for each agent's hook configuration.
func hookPaths() (claudePath, codexPath, codexConfigPath, opencodePath string, err error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", "", "", "", fmt.Errorf("cannot determine home directory: %w", err)
	}

	claudePath = filepath.Join(home, ".claude", "settings.json")
	codexPath = filepath.Join(home, ".codex", "hooks.json")
	codexConfigPath = filepath.Join(home, ".codex", "config.toml")
	opencodePath = filepath.Join(home, ".config", "opencode", "plugins", "axi-anvil.js")
	return claudePath, codexPath, codexConfigPath, opencodePath, nil
}

// ---- Install ----

func runHooksInstall(w io.Writer) error {
	cmd := hookCommand()

	claudePath, codexPath, codexConfigPath, opencodePath, err := hookPaths()
	if err != nil {
		return err
	}

	// Claude Code
	if err := installClaudeHook(claudePath, cmd); err != nil {
		return err
	}

	// Codex
	if err := installCodexHook(codexPath, codexConfigPath, cmd); err != nil {
		return err
	}

	// OpenCode
	if err := installOpenCodeHook(opencodePath, cmd); err != nil {
		return err
	}

	_, _ = fmt.Fprintf(w, "hooks:\n")
	_, _ = fmt.Fprintf(w, "  status: installed\n")
	_, _ = fmt.Fprintf(w, "  integrations: Claude Code, Codex, OpenCode\n")
	_, _ = fmt.Fprintf(w, "  binary: %s\n", cmd)

	return nil
}

// installClaudeHook ensures a SessionStart hook entry exists in Claude Code's settings.json.
func installClaudeHook(path, cmd string) error {
	hooks, err := readHooksFile(path)
	if err != nil {
		return err
	}

	// Remove legacy anvil entries.
	if hooks != nil {
		hooks.Hooks.SessionStart = removeAnvilEntries(hooks.Hooks.SessionStart, cmd)
	}

	entry := hookEntry{Matcher: "", Type: "command", Command: cmd, Timeout: 10}
	if hooks == nil {
		hooks = &hooksFile{}
	}
	hooks.Hooks.SessionStart = upsertHookEntry(hooks.Hooks.SessionStart, entry)

	return writeHooksFile(path, hooks)
}

// installCodexHook ensures hooks are installed for Codex.
func installCodexHook(hooksPath, configPath, cmd string) error {
	// Write hooks.json.
	hooks, err := readHooksFile(hooksPath)
	if err != nil {
		return err
	}

	if hooks != nil {
		hooks.Hooks.SessionStart = removeAnvilEntries(hooks.Hooks.SessionStart, cmd)
	}

	entry := hookEntry{Matcher: "", Type: "command", Command: cmd, Timeout: 10}
	if hooks == nil {
		hooks = &hooksFile{}
	}
	hooks.Hooks.SessionStart = upsertHookEntry(hooks.Hooks.SessionStart, entry)

	if err := writeHooksFile(hooksPath, hooks); err != nil {
		return err
	}

	// Ensure hooks feature is enabled in config.toml.
	return ensureCodexConfig(configPath)
}

// installOpenCodeHook writes the anvil JS plugin for OpenCode.
func installOpenCodeHook(path, cmd string) error {
	plugin := fmt.Sprintf(`// %s — do not edit; managed by anvil setup hooks

const SESSION_KEY = "anvil_session_context";

export async function experimental_chat_system_transform({ session_id, system }) {
  if (session_id === SESSION_KEY) return system;

  const { execSync } = await import("node:child_process");
  try {
    const ctx = execSync("%s", {
      encoding: "utf-8",
      timeout: 10000,
      stdio: ["ignore", "pipe", "pipe"],
    });
    SESSION_KEY = session_id;
    return ctx + "\n" + system;
  } catch {
    return system;
  }
}
`, anvilManagedMarker, cmd)

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("cannot create plugin directory %s: %w", dir, err)
	}

	// Check if existing file is not anvil-managed.
	existing, err := os.ReadFile(path)
	if err == nil && !strings.Contains(string(existing), anvilManagedMarker) {
		return fmt.Errorf("%s exists but is not anvil-managed; refusing to overwrite", collapseHome(path))
	}

	return os.WriteFile(path, []byte(plugin), 0o644)
}

// ---- Uninstall ----

func runHooksUninstall(w io.Writer) error {
	claudePath, codexPath, codexConfigPath, opencodePath, err := hookPaths()
	if err != nil {
		return err
	}

	_ = removeAnvilHookEntries(claudePath)
	_ = removeAnvilHookEntries(codexPath)
	_ = removeCodexConfigHook(codexConfigPath)
	_ = removeOpenCodePlugin(opencodePath)

	_, _ = fmt.Fprintf(w, "hooks:\n")
	_, _ = fmt.Fprintf(w, "  status: uninstalled\n")
	return nil
}

func removeAnvilHookEntries(path string) error {
	hooks, err := readHooksFile(path)
	if err != nil {
		return err
	}
	if hooks == nil {
		return nil
	}

	cmd := hookCommand()
	hooks.Hooks.SessionStart = removeAnvilEntries(hooks.Hooks.SessionStart, cmd)

	if len(hooks.Hooks.SessionStart) == 0 {
		return os.Remove(path)
	}

	return writeHooksFile(path, hooks)
}

func removeCodexConfigHook(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	if !strings.Contains(string(data), anvilManagedMarker) {
		return nil // not managed by anvil, leave alone
	}

	return os.Remove(path)
}

func removeOpenCodePlugin(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	if !strings.Contains(string(data), anvilManagedMarker) {
		return nil // not managed by anvil, leave alone
	}

	return os.Remove(path)
}

// ---- Check ----

func runHooksCheck(w io.Writer) error {
	cmd := hookCommand()

	claudePath, codexPath, codexConfigPath, opencodePath, err := hookPaths()
	if err != nil {
		return err
	}

	statuses := []hookStatus{
		checkClaudeHook(claudePath, cmd),
		checkCodexHook(codexPath, codexConfigPath, cmd),
		checkOpenCodeHook(opencodePath, cmd),
	}

	current := 0
	_, _ = fmt.Fprintf(w, "hooks:\n")
	for _, s := range statuses {
		key := strings.ToLower(strings.ReplaceAll(s.Agent, " ", "_"))
		_, _ = fmt.Fprintf(w, "  %s:\n", key)
		_, _ = fmt.Fprintf(w, "    status: %s\n", s.State)
		_, _ = fmt.Fprintf(w, "    path: %s\n", collapseHome(s.Path))
		if s.State == "current" {
			current++
		}
	}
	_, _ = fmt.Fprintf(w, "  summary: %d of %d current\n", current, len(statuses))

	if current == len(statuses) {
		return nil
	}
	return forge.NewBaseError(
		fmt.Sprintf("hooks not current: %d of %d", current, len(statuses)),
		"Run `anvil setup hooks` to install or repair",
	)
}

func checkClaudeHook(path, cmd string) hookStatus {
	status := hookStatus{Agent: "Claude Code", Path: path, State: "missing"}

	hooks, err := readHooksFile(path)
	if err != nil || hooks == nil {
		return status
	}

	for _, h := range hooks.Hooks.SessionStart {
		if h.Type == "command" && h.Command == cmd {
			status.State = "current"
			return status
		}
		if h.Type == "command" {
			status.State = "stale"
		}
	}
	return status
}

func checkCodexHook(hooksPath, configPath, cmd string) hookStatus {
	status := hookStatus{Agent: "Codex", Path: hooksPath, State: "missing"}

	hooks, err := readHooksFile(hooksPath)
	if err != nil || hooks == nil {
		return status
	}

	found := false
	for _, h := range hooks.Hooks.SessionStart {
		if h.Type == "command" && h.Command == cmd {
			found = true
			status.State = "current"
		} else if h.Type == "command" && status.State != "current" {
			status.State = "stale"
		}
	}
	if !found && status.State != "stale" {
		status.State = "missing"
	}

	// Also check config.toml for hooks feature.
	if status.State == "current" || status.State == "stale" {
		data, configErr := os.ReadFile(configPath)
		if configErr != nil || !strings.Contains(string(data), "hooks = true") {
			status.State = "stale"
		}
	}

	return status
}

func checkOpenCodeHook(path, cmd string) hookStatus {
	status := hookStatus{Agent: "OpenCode", Path: path, State: "missing"}

	data, err := os.ReadFile(path)
	if err != nil {
		return status
	}

	if !strings.Contains(string(data), anvilManagedMarker) {
		return status
	}

	if strings.Contains(string(data), cmd) {
		status.State = "current"
	} else {
		status.State = "stale"
	}
	return status
}

// ---- JSON file helpers ----

// readHooksFile reads and unmarshals a hooks JSON file.
// Returns nil, nil if the file does not exist.
func readHooksFile(path string) (*hooksFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var hf hooksFile
	if err := json.Unmarshal(data, &hf); err != nil {
		return nil, fmt.Errorf("invalid hooks file %s: %w", path, err)
	}
	return &hf, nil
}

// writeHooksFile writes the hooks file, creating parent directories as needed.
func writeHooksFile(path string, hf *hooksFile) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("cannot create directory %s: %w", dir, err)
	}

	data, err := json.MarshalIndent(hf, "", "  ")
	if err != nil {
		return fmt.Errorf("cannot marshal hooks: %w", err)
	}
	data = append(data, '\n')

	return os.WriteFile(path, data, 0o644)
}

// upsertHookEntry appends a hook entry to the slice. Callers must
// call removeAnvilEntries first to clean up stale entries, so we
// never duplicate anvil entries.
func upsertHookEntry(entries []hookEntry, entry hookEntry) []hookEntry {
	return append(entries, entry)
}

// removeAnvilEntries removes all entries whose command points to an anvil binary.
// Matches by exact command string, base name, or the literal "anvil".
func removeAnvilEntries(entries []hookEntry, cmd string) []hookEntry {
	base := filepath.Base(cmd)

	var filtered []hookEntry
	for _, e := range entries {
		if e.Command == cmd || e.Command == "anvil" || filepath.Base(e.Command) == base {
			continue
		}
		filtered = append(filtered, e)
	}
	return filtered
}

// ensureCodexConfig ensures the Codex config.toml has hooks enabled
// and is marked as anvil-managed.
func ensureCodexConfig(path string) error {
	existing, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	content := string(existing)

	// If already has hooks = true, we're done.
	if strings.Contains(content, "[features]") && strings.Contains(content, "hooks = true") {
		return nil
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("cannot create directory %s: %w", dir, err)
	}

	if strings.Contains(content, "[features]") {
		// Append hooks = true after [features] line.
		lines := strings.Split(content, "\n")
		var result []string
		for _, line := range lines {
			result = append(result, line)
			if strings.TrimSpace(line) == "[features]" {
				result = append(result, "# "+anvilManagedMarker)
				result = append(result, "hooks = true")
			}
		}
		content = strings.Join(result, "\n")
	} else {
		if len(content) > 0 && content[len(content)-1] != '\n' {
			content += "\n"
		}
		content += fmt.Sprintf("\n# %s\n[features]\nhooks = true\n", anvilManagedMarker)
	}

	return os.WriteFile(path, []byte(content), 0o644)
}
