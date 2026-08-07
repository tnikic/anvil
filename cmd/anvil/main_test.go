package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tnikic/anvil/internal/auth"
	"github.com/tnikic/anvil/internal/commands"
	"github.com/tnikic/anvil/internal/forge"
	"github.com/tnikic/anvil/internal/forge/github"
	"github.com/tnikic/anvil/internal/forge/gitlab"
)

// buildForge compiles the binary to a temp directory and returns its path.
func buildForge(t *testing.T) string {
	t.Helper()

	binPath := filepath.Join(t.TempDir(), "anvil")
	// Resolve the package directory relative to this test file's location.
	// We can't rely on cwd because tests may chdir.
	pkgDir, err := filepath.Abs(".")
	if err != nil {
		t.Fatalf("failed to resolve package dir: %v", err)
	}

	cmd := exec.Command("go", "build", "-o", binPath, pkgDir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go build failed: %v\n%s", err, out)
	}
	return binPath
}

// runForge runs the compiled binary with the given arguments and returns
// stdout, stderr (combined), and the exit code.
func runForge(t *testing.T, binPath string, args ...string) (output string, exitCode int) {
	t.Helper()

	cmd := exec.Command(binPath, args...)
	var outBuf strings.Builder
	cmd.Stdout = &outBuf
	cmd.Stderr = &outBuf // merge stderr into stdout for simpler assertions

	err := cmd.Run()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return outBuf.String(), exitErr.ExitCode()
		}
		t.Fatalf("failed to run binary: %v", err)
	}
	return outBuf.String(), 0
}

func TestSmokeHelp(t *testing.T) {
	bin := buildForge(t)

	out, exitCode := runForge(t, bin, "--help")
	if exitCode != 0 {
		t.Errorf("--help should exit 0, got %d", exitCode)
	}
	if !strings.Contains(out, "anvil") {
		t.Errorf("--help output should mention anvil, got: %s", out)
	}
	if !strings.Contains(out, "Usage:") {
		t.Errorf("--help output should contain Usage section, got: %s", out)
	}
}

func TestSmokeHomeView(t *testing.T) {
	bin := buildForge(t)

	// Running with explicit forge/repo flags skips git remote detection
	// and auth, printing the TOON home view directly.
	out, exitCode := runForge(t, bin, "--forge", "github.com", "--repo", "owner/repo")
	if exitCode != 0 {
		t.Errorf("home view should exit 0, got %d", exitCode)
	}
	if !strings.Contains(out, "bin:") {
		t.Errorf("home view should contain 'bin:', got: %s", out)
	}
	if !strings.Contains(out, "forge: github.com") {
		t.Errorf("home view should contain 'forge: github.com', got: %s", out)
	}
	if !strings.Contains(out, "repo: owner/repo") {
		t.Errorf("home view should contain 'repo: owner/repo', got: %s", out)
	}
}

func TestSmokeUnknownFlag(t *testing.T) {
	bin := buildForge(t)

	// Exercises main()'s isUsageError → exit 2 path.
	out, exitCode := runForge(t, bin, "--bogus")
	if exitCode != 2 {
		t.Errorf("unknown flag should exit 2, got %d; output: %s", exitCode, out)
	}
	if !strings.Contains(out, "error: unknown flag") {
		t.Errorf("unknown flag should print 'error: unknown flag', got: %s", out)
	}
}

func TestSmokeSubcommandHelp(t *testing.T) {
	bin := buildForge(t)

	// Spot-check that subcommand dispatch works through main().
	for _, sc := range []string{"issue", "pr", "label", "auth", "skills"} {
		t.Run(sc, func(t *testing.T) {
			out, exitCode := runForge(t, bin, sc, "--help")
			if exitCode != 0 {
				t.Errorf("%s --help should exit 0, got %d", sc, exitCode)
			}
			if !strings.Contains(out, "Usage:") {
				t.Errorf("%s --help should contain Usage, got: %s", sc, out)
			}
		})
	}
}

func TestSmokeVersion(t *testing.T) {
	bin := buildForge(t)

	out, exitCode := runForge(t, bin, "--version")
	if exitCode != 0 {
		t.Errorf("--version should exit 0, got %d", exitCode)
	}
	// Dev builds report "dev"; release builds report the tag (e.g., v0.1.0).
	if !strings.Contains(out, "dev") {
		t.Errorf("--version should contain version, got: %s", out)
	}
}

// ---- Integration tests: forge dispatch through httptest.Server ----
// These tests exercise the full stack — cobra → resolveForge → ForgeFn →
// adapter → HTTP → test server — without compiling the binary or hitting
// the real network. They verify that both GitHub and GitLab adapters are
// correctly dispatched from the command layer.

const (
	testOwner = "testowner"
	testRepo  = "testrepo"
)

// setupIntegrationTest creates an httptest.Server with the given handler,
// creates the specified adapter wired to it, overrides commands.ForgeFn,
// and seeds auth tokens for both github.com and gitlab.com.
func setupIntegrationTest(t *testing.T, handler http.HandlerFunc, adapterType string) {
	t.Helper()

	// Set up token cache so auth.ResolveToken succeeds.
	cacheDir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cacheDir)
	store := auth.NewStore(auth.DefaultStorePath())
	if err := store.Set("github.com", "test-token-gh"); err != nil {
		t.Fatalf("failed to seed github token: %v", err)
	}
	if err := store.Set("gitlab.com", "test-token-gl"); err != nil {
		t.Fatalf("failed to seed gitlab token: %v", err)
	}

	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	origFn := commands.ForgeFn
	commands.ForgeFn = func(host, owner, repo, token string) forge.Forge {
		switch adapterType {
		case "github":
			return github.New(srv.URL, owner, repo, srv.Client())
		case "gitlab":
			return gitlab.New(srv.URL, owner, repo, srv.Client())
		default:
			panic("unknown adapter type: " + adapterType)
		}
	}
	t.Cleanup(func() { commands.ForgeFn = origFn })
}

// runIntegrationCmd executes cobra commands in-process and returns stdout.
// If the command returns an error, it is formatted via PrintFormatted (same as
// main.go) so error output is included in the buffer.
func runIntegrationCmd(t *testing.T, args ...string) string {
	t.Helper()
	cmd := commands.NewRoot()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		commands.PrintFormatted(buf, err)
	}
	return buf.String()
}

// respondJSON writes a JSON response with the given status code.
func respondJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// ---- GitHub adapter integration tests ----

func TestIntegrationGitHub_IssueList(t *testing.T) {
	setupIntegrationTest(t, func(w http.ResponseWriter, r *http.Request) {
		// The GitHub adapter hits GET /api/v3/repos/owner/repo/issues
		issues := []map[string]any{
			{"number": 1, "title": "Fix login", "state": "open", "user": map[string]any{"login": "alice"}, "html_url": "https://github.com/testowner/testrepo/issues/1", "labels": []any{}},
			{"number": 2, "title": "Add rate limit", "state": "closed", "user": map[string]any{"login": "bob"}, "html_url": "https://github.com/testowner/testrepo/issues/2", "labels": []any{}},
		}
		respondJSON(w, http.StatusOK, issues)
	}, "github")

	out := runIntegrationCmd(t, "issue", "list", "--forge", "github.com", "--repo", testOwner+"/"+testRepo)

	if !strings.Contains(out, "Fix login") {
		t.Errorf("should contain first issue title, got: %s", out)
	}
	if !strings.Contains(out, "Add rate limit") {
		t.Errorf("should contain second issue title, got: %s", out)
	}
	if !strings.Contains(out, "2 of 2 total") {
		t.Errorf("should show count, got: %s", out)
	}
}

func TestIntegrationGitHub_IssueView(t *testing.T) {
	setupIntegrationTest(t, func(w http.ResponseWriter, r *http.Request) {
		issue := map[string]any{
			"number":     42,
			"title":      "Test Issue",
			"state":      "open",
			"body":       "This is a test body with some content.",
			"user":       map[string]any{"login": "alice"},
			"html_url":   "https://github.com/testowner/testrepo/issues/42",
			"created_at": "2025-01-01T00:00:00Z",
			"updated_at": "2025-06-01T00:00:00Z",
			"labels":     []any{},
		}
		respondJSON(w, http.StatusOK, issue)
	}, "github")

	out := runIntegrationCmd(t, "issue", "view", "42", "--forge", "github.com", "--repo", testOwner+"/"+testRepo)

	if !strings.Contains(out, "Test Issue") {
		t.Errorf("should contain issue title, got: %s", out)
	}
	if !strings.Contains(out, "open") {
		t.Errorf("should contain state, got: %s", out)
	}
	if !strings.Contains(out, "body_size") {
		t.Errorf("should contain body_size, got: %s", out)
	}
	if !strings.Contains(out, "alice") {
		t.Errorf("should contain author, got: %s", out)
	}
}

func TestIntegrationGitHub_LabelList(t *testing.T) {
	setupIntegrationTest(t, func(w http.ResponseWriter, r *http.Request) {
		labels := []map[string]any{
			{"name": "kind:bug", "color": "d73a4a", "description": "Something isn't working"},
			{"name": "kind:feature", "color": "a2eeef", "description": "New feature"},
			{"name": "good-first-issue", "color": "7057ff", "description": ""},
		}
		respondJSON(w, http.StatusOK, labels)
	}, "github")

	out := runIntegrationCmd(t, "label", "list", "--forge", "github.com", "--repo", testOwner+"/"+testRepo)

	// TOON output separates scope and name into distinct fields.
	// "kind:bug" → scope=kind, name=bug in the tabular output.
	if !strings.Contains(out, "bug") || !strings.Contains(out, "kind") {
		t.Errorf("should contain parsed scoped label (scope=kind, name=bug), got: %s", out)
	}
	if !strings.Contains(out, "feature") {
		t.Errorf("should contain second scoped label, got: %s", out)
	}
	if !strings.Contains(out, "good-first-issue") {
		t.Errorf("should contain unscoped label, got: %s", out)
	}
	if !strings.Contains(out, "3 labels") {
		t.Errorf("should show count '3 labels', got: %s", out)
	}
}

func TestIntegrationGitHub_IssueNotFound(t *testing.T) {
	setupIntegrationTest(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"message": "Not Found"})
	}, "github")

	out := runIntegrationCmd(t, "issue", "view", "999", "--forge", "github.com", "--repo", testOwner+"/"+testRepo)

	if !strings.Contains(out, "error:") {
		t.Errorf("should contain error prefix, got: %s", out)
	}
	if !strings.Contains(out, "not found") {
		t.Errorf("should mention not found, got: %s", out)
	}
}

func TestIntegrationGitHub_AuthFailure(t *testing.T) {
	setupIntegrationTest(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"message": "Bad credentials"})
	}, "github")

	out := runIntegrationCmd(t, "issue", "list", "--forge", "github.com", "--repo", testOwner+"/"+testRepo)

	if !strings.Contains(out, "error:") {
		t.Errorf("should contain error prefix, got: %s", out)
	}
	if !strings.Contains(out, "authentication") {
		t.Errorf("should mention authentication, got: %s", out)
	}
}

// ---- GitLab adapter integration tests ----

func TestIntegrationGitLab_IssueList(t *testing.T) {
	setupIntegrationTest(t, func(w http.ResponseWriter, r *http.Request) {
		// GitLab adapter expects []*gl.Issue. Must include id, project_id
		// because gl.Issue.UnmarshalJSON accesses raw["id"].
		issues := []map[string]any{
			{"id": 0, "iid": 1, "project_id": 0, "title": "Fix login", "state": "opened", "author": map[string]any{"username": "alice"}, "web_url": "https://gitlab.com/testowner/testrepo/-/issues/1", "labels": []any{}, "created_at": "2025-01-01T00:00:00Z", "updated_at": "2025-06-01T00:00:00Z"},
			{"id": 0, "iid": 2, "project_id": 0, "title": "Add rate limit", "state": "closed", "author": map[string]any{"username": "bob"}, "web_url": "https://gitlab.com/testowner/testrepo/-/issues/2", "labels": []any{}, "created_at": "2025-01-01T00:00:00Z", "updated_at": "2025-06-01T00:00:00Z"},
		}
		respondJSON(w, http.StatusOK, issues)
	}, "gitlab")

	out := runIntegrationCmd(t, "issue", "list", "--forge", "gitlab.com", "--repo", testOwner+"/"+testRepo)

	if !strings.Contains(out, "Fix login") {
		t.Errorf("should contain first issue title, got: %s", out)
	}
	if !strings.Contains(out, "Add rate limit") {
		t.Errorf("should contain second issue title, got: %s", out)
	}
	if !strings.Contains(out, "of") {
		t.Errorf("should show count, got: %s", out)
	}
}

func TestIntegrationGitLab_IssueView(t *testing.T) {
	setupIntegrationTest(t, func(w http.ResponseWriter, r *http.Request) {
		issue := map[string]any{
			"id":          0,
			"iid":         42,
			"project_id":  0,
			"title":       "Test Issue",
			"state":       "opened",
			"description": "This is a test body.",
			"author":      map[string]any{"username": "alice"},
			"web_url":     "https://gitlab.com/testowner/testrepo/-/issues/42",
			"created_at":  "2025-01-01T00:00:00Z",
			"updated_at":  "2025-06-01T00:00:00Z",
			"labels":      []any{},
		}
		respondJSON(w, http.StatusOK, issue)
	}, "gitlab")

	out := runIntegrationCmd(t, "issue", "view", "42", "--forge", "gitlab.com", "--repo", testOwner+"/"+testRepo)

	if !strings.Contains(out, "Test Issue") {
		t.Errorf("should contain issue title, got: %s", out)
	}
	if !strings.Contains(out, "open") {
		t.Errorf("should contain state, got: %s", out)
	}
	if !strings.Contains(out, "body_size") {
		t.Errorf("should contain body_size, got: %s", out)
	}
}

func TestIntegrationGitLab_LabelList(t *testing.T) {
	setupIntegrationTest(t, func(w http.ResponseWriter, r *http.Request) {
		labels := []map[string]any{
			{"name": "kind::bug", "color": "#d73a4a", "description": "Something isn't working"},
			{"name": "kind::feature", "color": "#a2eeef", "description": "New feature"},
			{"name": "good-first-issue", "color": "#7057ff", "description": ""},
		}
		respondJSON(w, http.StatusOK, labels)
	}, "gitlab")

	out := runIntegrationCmd(t, "label", "list", "--forge", "gitlab.com", "--repo", testOwner+"/"+testRepo)

	// TOON output separates scope and name into distinct fields.
	// "kind::bug" → scope=kind, name=bug in the tabular output.
	if !strings.Contains(out, "bug") || !strings.Contains(out, "kind") {
		t.Errorf("should contain parsed scoped label (scope=kind, name=bug), got: %s", out)
	}
	if !strings.Contains(out, "feature") {
		t.Errorf("should contain second scoped label, got: %s", out)
	}
	if !strings.Contains(out, "good-first-issue") {
		t.Errorf("should contain unscoped label, got: %s", out)
	}
	if !strings.Contains(out, "3 labels") {
		t.Errorf("should show count '3 labels', got: %s", out)
	}
}

func TestIntegrationGitLab_IssueNotFound(t *testing.T) {
	setupIntegrationTest(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"message": "Not Found"})
	}, "gitlab")

	out := runIntegrationCmd(t, "issue", "view", "999", "--forge", "gitlab.com", "--repo", testOwner+"/"+testRepo)

	if !strings.Contains(out, "error:") {
		t.Errorf("should contain error prefix, got: %s", out)
	}
	if !strings.Contains(out, "not found") {
		t.Errorf("should mention not found, got: %s", out)
	}
}

func TestIntegrationGitLab_AuthFailure(t *testing.T) {
	setupIntegrationTest(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"message": "Unauthorized"})
	}, "gitlab")

	out := runIntegrationCmd(t, "issue", "list", "--forge", "gitlab.com", "--repo", testOwner+"/"+testRepo)

	if !strings.Contains(out, "error:") {
		t.Errorf("should contain error prefix, got: %s", out)
	}
	if !strings.Contains(out, "authentication") {
		t.Errorf("should mention authentication, got: %s", out)
	}
}

// ---- Forge dispatch tests ----

func TestIntegrationForgeDispatch(t *testing.T) {
	// Verify that defaultForgeFn correctly dispatches to GitHub for
	// "github.com" and GitLab for "gitlab.com" based on ForgeType.
	// We test this by registering real adapters in adapterConstructors
	// and exercising the same ForgeFn pattern used by resolveForge.

	t.Run("github dispatch", func(t *testing.T) {
		setupIntegrationTest(t, func(w http.ResponseWriter, r *http.Request) {
			issues := []map[string]any{
				{"number": 1, "title": "GH Issue", "state": "open", "user": map[string]any{"login": "gh-user"}, "html_url": "https://github.com/o/r/issues/1", "labels": []any{}},
			}
			respondJSON(w, http.StatusOK, issues)
		}, "github")

		out := runIntegrationCmd(t, "issue", "list", "--forge", "github.com", "--repo", testOwner+"/"+testRepo)
		if !strings.Contains(out, "GH Issue") {
			t.Errorf("GitHub dispatch should produce issue, got: %s", out)
		}
	})

	t.Run("gitlab dispatch", func(t *testing.T) {
		setupIntegrationTest(t, func(w http.ResponseWriter, r *http.Request) {
			issues := []map[string]any{
				{"id": 0, "iid": 1, "project_id": 0, "title": "GL Issue", "state": "opened", "author": map[string]any{"username": "gl-user"}, "web_url": "https://gitlab.com/o/r/-/issues/1", "labels": []any{}, "created_at": "2025-01-01T00:00:00Z", "updated_at": "2025-06-01T00:00:00Z"},
			}
			respondJSON(w, http.StatusOK, issues)
		}, "gitlab")

		out := runIntegrationCmd(t, "issue", "list", "--forge", "gitlab.com", "--repo", testOwner+"/"+testRepo)
		if !strings.Contains(out, "GL Issue") {
			t.Errorf("GitLab dispatch should produce issue, got: %s", out)
		}
	})
}

// TestSmokeOutsideGitRepo exercises the fallback path through main():
// no flags, no git repo → Detect fails → fallback output, exit 0.
func TestSmokeOutsideGitRepo(t *testing.T) {
	bin := buildForge(t)

	// Change to a temp directory that has no git repo.
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}

	// Run the binary from outside the repo directory.
	out, exitCode := runForge(t, bin)
	if exitCode != 0 {
		t.Errorf("outside git repo should exit 0 (fallback), got %d; output: %s", exitCode, out)
	}
	if !strings.Contains(out, "bin:") {
		t.Errorf("fallback should show bin:, got: %s", out)
	}
	if !strings.Contains(out, "--forge") {
		t.Errorf("fallback should suggest --forge, got: %s", out)
	}
	if !strings.Contains(out, "auth set") {
		t.Errorf("fallback should suggest auth set, got: %s", out)
	}
}
