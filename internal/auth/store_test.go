package auth_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tnikic/anvil/internal/auth"
)

func TestSetAndGet(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "creds.json")
	store := auth.NewStore(path)

	err := store.Set("github.com", "ghp_test123")
	if err != nil {
		t.Fatalf("Set: %v", err)
	}

	tok, ok := store.Get("github.com")
	if !ok {
		t.Fatal("Get: expected token to exist")
	}
	if tok != "ghp_test123" {
		t.Errorf("Get: got %q, want %q", tok, "ghp_test123")
	}
}

func TestGetMissing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "creds.json")
	store := auth.NewStore(path)

	_, ok := store.Get("github.com")
	if ok {
		t.Error("Get: expected false for missing host")
	}
}

func TestUnset(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "creds.json")
	store := auth.NewStore(path)

	if err := store.Set("github.com", "ghp_test123"); err != nil {
		t.Fatal(err)
	}

	if err := store.Unset("github.com"); err != nil {
		t.Fatalf("Unset: %v", err)
	}

	_, ok := store.Get("github.com")
	if ok {
		t.Error("Get: expected false after Unset")
	}
}

func TestUnsetMissing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "creds.json")
	store := auth.NewStore(path)

	// Unsetting a non-existent entry should not error — it's a no-op.
	err := store.Unset("nonexistent.example.com")
	if err != nil {
		t.Fatalf("Unset missing: %v", err)
	}
}

func TestList(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "creds.json")
	store := auth.NewStore(path)

	if err := store.Set("github.com", "ghp_aaa"); err != nil {
		t.Fatal(err)
	}
	if err := store.Set("gitlab.com", "glpat-bbb"); err != nil {
		t.Fatal(err)
	}

	entries := store.List()
	if len(entries) != 2 {
		t.Fatalf("List: expected 2 entries, got %d", len(entries))
	}

	hosts := make(map[string]string)
	for _, e := range entries {
		hosts[e.Host] = e.Token
	}

	if hosts["github.com"] != "ghp_aaa" {
		t.Errorf("List: github.com token mismatch")
	}
	if hosts["gitlab.com"] != "glpat-bbb" {
		t.Errorf("List: gitlab.com token mismatch")
	}
}

func TestListEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "creds.json")
	store := auth.NewStore(path)

	entries := store.List()
	if len(entries) != 0 {
		t.Errorf("List: expected 0 entries, got %d", len(entries))
	}
}

func TestOverwrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "creds.json")
	store := auth.NewStore(path)

	if err := store.Set("github.com", "old_token"); err != nil {
		t.Fatal(err)
	}
	if err := store.Set("github.com", "new_token"); err != nil {
		t.Fatal(err)
	}

	tok, ok := store.Get("github.com")
	if !ok {
		t.Fatal("Get: expected token to exist")
	}
	if tok != "new_token" {
		t.Errorf("Get: got %q, want %q", tok, "new_token")
	}
}

func TestPersistence(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "creds.json")
	store := auth.NewStore(path)

	if err := store.Set("github.com", "ghp_persist"); err != nil {
		t.Fatal(err)
	}

	// Create a new store pointing to the same file.
	store2 := auth.NewStore(path)
	tok, ok := store2.Get("github.com")
	if !ok {
		t.Fatal("Get: expected token to persist across store instances")
	}
	if tok != "ghp_persist" {
		t.Errorf("Get: got %q, want %q", tok, "ghp_persist")
	}
}

func TestFileCreatedWithRestrictedPermissions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "creds.json")
	store := auth.NewStore(path)

	if err := store.Set("github.com", "ghp_test"); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}

	// On Unix, the file should be readable only by the owner.
	if info.Mode().Perm()&0o077 != 0 {
		t.Errorf("file permissions too open: %o", info.Mode().Perm())
	}
}

func TestStorePathDefault(t *testing.T) {
	path := auth.DefaultStorePath()
	home, _ := os.UserHomeDir()
	if home != "" {
		// Path should contain .cache/anvil/credentials.json
		if !filepath.IsAbs(path) {
			t.Errorf("DefaultStorePath should be absolute, got: %s", path)
		}
	}
}

func TestConcurrentSet(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "creds.json")
	store := auth.NewStore(path)

	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(i int) {
			host := "host" + string(rune('0'+i))
			_ = store.Set(host, "token")
			done <- true
		}(i)
	}
	for i := 0; i < 10; i++ {
		<-done
	}

	entries := store.List()
	if len(entries) != 10 {
		t.Errorf("expected 10 entries after concurrent writes, got %d", len(entries))
	}
}
