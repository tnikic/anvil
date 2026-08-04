package forge_test

import (
	"fmt"
	"testing"

	"github.com/tnikic/anvil/internal/forge"
)

func TestParseRemoteHTTPS(t *testing.T) {
	forgeHost, repo, err := forge.ParseRemote("https://github.com/tnikic/anvil.git")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if forgeHost != "github.com" {
		t.Errorf("expected forge 'github.com', got '%s'", forgeHost)
	}
	if repo != "tnikic/anvil" {
		t.Errorf("expected repo 'tnikic/anvil', got '%s'", repo)
	}
}

func TestParseRemoteSSH(t *testing.T) {
	forgeHost, repo, err := forge.ParseRemote("git@gitlab.com:mygroup/myproject.git")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if forgeHost != "gitlab.com" {
		t.Errorf("expected forge 'gitlab.com', got '%s'", forgeHost)
	}
	if repo != "mygroup/myproject" {
		t.Errorf("expected repo 'mygroup/myproject', got '%s'", repo)
	}
}

func TestParseRemoteNoGitSuffix(t *testing.T) {
	forgeHost, repo, err := forge.ParseRemote("https://github.com/owner/repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if forgeHost != "github.com" {
		t.Errorf("expected forge 'github.com', got '%s'", forgeHost)
	}
	if repo != "owner/repo" {
		t.Errorf("expected repo 'owner/repo', got '%s'", repo)
	}
}

func TestParseRemoteInvalid(t *testing.T) {
	_, _, err := forge.ParseRemote("not-a-valid-url")
	if err == nil {
		t.Error("expected error for invalid remote URL")
	}
}

// --- Detect tests ---

func TestDetectBothFlagsProvided(t *testing.T) {
	orig := forge.RemoteFn
	t.Cleanup(func() { forge.RemoteFn = orig })
	// Stub returns something that would parse, but should not be called.
	forge.RemoteFn = func() (string, error) {
		t.Error("RemoteFn should not be called when both flags are provided")
		return "", nil
	}

	forgeHost, repo, err := forge.Detect("gitlab.com", "group/project")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if forgeHost != "gitlab.com" {
		t.Errorf("expected forge 'gitlab.com', got '%s'", forgeHost)
	}
	if repo != "group/project" {
		t.Errorf("expected repo 'group/project', got '%s'", repo)
	}
}

func TestDetectNoFlagsAutoDetect(t *testing.T) {
	orig := forge.RemoteFn
	t.Cleanup(func() { forge.RemoteFn = orig })
	forge.RemoteFn = func() (string, error) {
		return "https://github.com/owner/repo.git", nil
	}

	forgeHost, repo, err := forge.Detect("", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if forgeHost != "github.com" {
		t.Errorf("expected forge 'github.com', got '%s'", forgeHost)
	}
	if repo != "owner/repo" {
		t.Errorf("expected repo 'owner/repo', got '%s'", repo)
	}
}

func TestDetectForgeFlagOnly(t *testing.T) {
	orig := forge.RemoteFn
	t.Cleanup(func() { forge.RemoteFn = orig })
	forge.RemoteFn = func() (string, error) {
		return "https://github.com/owner/repo.git", nil
	}

	forgeHost, repo, err := forge.Detect("gitlab.com", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// forge flag overrides auto-detected forge
	if forgeHost != "gitlab.com" {
		t.Errorf("expected forge 'gitlab.com', got '%s'", forgeHost)
	}
	// repo comes from auto-detection
	if repo != "owner/repo" {
		t.Errorf("expected repo 'owner/repo', got '%s'", repo)
	}
}

func TestDetectRepoFlagOnly(t *testing.T) {
	orig := forge.RemoteFn
	t.Cleanup(func() { forge.RemoteFn = orig })
	forge.RemoteFn = func() (string, error) {
		return "https://github.com/owner/repo.git", nil
	}

	forgeHost, repo, err := forge.Detect("", "mygroup/myproject")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// forge comes from auto-detection
	if forgeHost != "github.com" {
		t.Errorf("expected forge 'github.com', got '%s'", forgeHost)
	}
	// repo flag overrides auto-detected repo
	if repo != "mygroup/myproject" {
		t.Errorf("expected repo 'mygroup/myproject', got '%s'", repo)
	}
}

func TestDetectRemoteFnError(t *testing.T) {
	orig := forge.RemoteFn
	t.Cleanup(func() { forge.RemoteFn = orig })
	forge.RemoteFn = func() (string, error) {
		return "", fmt.Errorf("no git repo")
	}

	_, _, err := forge.Detect("", "")
	if err == nil {
		t.Error("expected error when RemoteFn fails")
	}
}

func TestDetectRemoteFnInvalidURL(t *testing.T) {
	orig := forge.RemoteFn
	t.Cleanup(func() { forge.RemoteFn = orig })
	forge.RemoteFn = func() (string, error) {
		return "not-a-remote-url", nil
	}

	_, _, err := forge.Detect("", "")
	if err == nil {
		t.Error("expected error for invalid remote URL from RemoteFn")
	}
}
