package auth

import (
	"fmt"
	"os"
	"strings"
)

// TokenError is a structured error returned when a token is not found for a host.
// It satisfies forge.StructuredError so main.go can format output uniformly.
type TokenError struct {
	Host string
}

func (e *TokenError) Error() string {
	return fmt.Sprintf("authentication failed for %s — no token configured", e.Host)
}

// Message returns the user-facing error message.
func (e *TokenError) Message() string { return e.Error() }

// Help returns the suggested corrective command.
func (e *TokenError) Help() string {
	return fmt.Sprintf("Run \"anvil auth set %s <token>\"", e.Host)
}

// ResolveToken returns the token for the given host from the default store,
// or a TokenError if not found.
func ResolveToken(host string) (string, error) {
	store := NewStore(DefaultStorePath())
	tok, ok := store.Get(host)
	if !ok {
		return "", &TokenError{Host: host}
	}
	return tok, nil
}

// CollapseHome replaces the user's home directory prefix in path with "~".
// Returns the original path if the home directory cannot be determined.
func CollapseHome(path string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	if strings.HasPrefix(path, home+string(os.PathSeparator)) {
		return "~" + path[len(home):]
	}
	return path
}
