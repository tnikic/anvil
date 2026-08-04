// Package auth manages authentication token storage for anvil.
// Tokens are stored in a single JSON file at $XDG_CACHE_HOME/anvil/credentials.json,
// keyed by host. The file is the single source of truth — no environment variables.
package auth

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// Entry represents a single stored credential.
type Entry struct {
	Host  string
	Token string
}

// Store provides concurrent-safe read/write access to the credentials file.
// The zero value is not usable; use NewStore to create one.
type Store struct {
	path string
	mu   sync.Mutex
}

// NewStore creates a Store backed by the file at the given path.
// The parent directory must exist; the file is created on first write.
func NewStore(path string) *Store {
	return &Store{path: path}
}

// DefaultStorePath returns the default path to the credentials file.
// Uses $XDG_CACHE_HOME if set, otherwise ~/.cache.
func DefaultStorePath() string {
	cacheDir := os.Getenv("XDG_CACHE_HOME")
	if cacheDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			// Fallback for environments without a home directory.
			cacheDir = ".cache"
		} else {
			cacheDir = filepath.Join(home, ".cache")
		}
	}
	return filepath.Join(cacheDir, "anvil", "credentials.json")
}

// Get returns the token for the given host and true if it exists.
func (s *Store) Get(host string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := s.read()
	if err != nil {
		return "", false
	}
	tok, ok := data[host]
	return tok, ok
}

// Set stores a token for the given host. Creates the file and parent
// directories if they don't exist. File is created with 0600 permissions.
func (s *Store) Set(host, token string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := s.read()
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read credentials: %w", err)
	}
	if data == nil {
		data = make(map[string]string)
	}
	data[host] = token
	return s.write(data)
}

// Unset removes the token for the given host. It is a no-op if the host
// does not exist. Returns an error only if writing the file fails.
func (s *Store) Unset(host string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := s.read()
	if err != nil {
		if os.IsNotExist(err) {
			return nil // nothing to remove
		}
		return fmt.Errorf("read credentials: %w", err)
	}
	if _, ok := data[host]; !ok {
		return nil // no-op
	}
	delete(data, host)
	return s.write(data)
}

// List returns all stored credential entries.
func (s *Store) List() []Entry {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := s.read()
	if err != nil {
		return nil
	}
	entries := make([]Entry, 0, len(data))
	for host, token := range data {
		entries = append(entries, Entry{Host: host, Token: token})
	}
	return entries
}

// read reads and parses the credentials file. Returns an empty map if the
// file does not exist. The caller must hold s.mu.
func (s *Store) read() (map[string]string, error) {
	raw, err := os.ReadFile(s.path)
	if err != nil {
		return nil, err
	}
	var data map[string]string
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, fmt.Errorf("parse credentials: %w", err)
	}
	return data, nil
}

// write writes the credentials map to disk. Creates parent directories and
// the file with 0600 permissions. The caller must hold s.mu.
func (s *Store) write(data map[string]string) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	raw, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal credentials: %w", err)
	}

	// Write to a temp file and rename for atomicity.
	tmp, err := os.CreateTemp(filepath.Dir(s.path), ".cred-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmp.Name()
	defer func() { _ = os.Remove(tmpPath) }() // best-effort cleanup if rename fails

	if _, err := tmp.Write(raw); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}

	if err := os.Chmod(tmpPath, 0o600); err != nil {
		return fmt.Errorf("chmod temp file: %w", err)
	}

	if err := os.Rename(tmpPath, s.path); err != nil {
		return fmt.Errorf("rename temp file: %w", err)
	}

	return nil
}
