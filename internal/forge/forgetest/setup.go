package forgetest

import (
	"testing"

	"github.com/tnikic/anvil/internal/auth"
	"github.com/tnikic/anvil/internal/commands"
	"github.com/tnikic/anvil/internal/forge"
)

// Setup configures the test environment for command tests:
//   - Creates a temp cache directory and seeds a test token for "github.com"
//   - Overrides commands.ForgeFn to return the given FakeForge
//   - Registers cleanup to restore the original ForgeFn
//
// Returns the FakeForge for the test to populate with data.
func Setup(t *testing.T) *FakeForge {
	t.Helper()

	cacheDir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cacheDir)
	store := auth.NewStore(auth.DefaultStorePath())
	if err := store.Set("github.com", "test-token"); err != nil {
		t.Fatalf("failed to set test token: %v", err)
	}

	fk := NewFakeForge()

	origFn := commands.ForgeFn
	commands.ForgeFn = func(host, owner, repo, token string) forge.Forge {
		return fk
	}
	t.Cleanup(func() { commands.ForgeFn = origFn })

	return fk
}
