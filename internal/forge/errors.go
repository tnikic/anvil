package forge

import (
	"fmt"
	"net/http"
	"strings"
)

// StructuredError is an error that carries a user-facing message and an
// optional help hint. All architectural layers — adapters, commands, auth —
// return errors satisfying this interface so that main.go can format output
// with a single type-assertion.
type StructuredError interface {
	error
	// Message returns the user-facing error message.
	Message() string
	// Help returns an optional corrective action hint, or "".
	Help() string
}

// BaseError is a reusable implementation of StructuredError.
// Use NewBaseError to construct one.
type BaseError struct {
	msg  string
	help string
}

// NewBaseError creates a BaseError with a message and optional help hint.
func NewBaseError(msg, help string) *BaseError {
	return &BaseError{msg: msg, help: help}
}

func (e *BaseError) Error() string   { return e.msg }
func (e *BaseError) Message() string { return e.msg }
func (e *BaseError) Help() string    { return e.help }

// TranslateHTTPError converts an HTTP status code from a forge API response
// into a StructuredError with a user-facing message and help hint.
// Returns nil if the status code doesn't map to a known error, so the
// caller can fall back to passing through the original error.
func TranslateHTTPError(statusCode int, host, owner, repo, resource string) error {
	switch statusCode {
	case http.StatusUnauthorized:
		return NewBaseError(
			fmt.Sprintf("authentication failed for %s — token rejected (401)", host),
			fmt.Sprintf("Run \"anvil auth set %s <token>\"", host),
		)
	case http.StatusNotFound:
		if resource != "" {
			return NewBaseError(
				fmt.Sprintf("%s not found in %s/%s", resource, owner, repo),
				"Run \"anvil issue list\" to see available items",
			)
		}
		return NewBaseError(
			"not found (404)",
			"Run \"anvil issue list\" to see available items",
		)
	case http.StatusForbidden:
		return NewBaseError(
			fmt.Sprintf("access denied for %s (403)", host),
			"Check that your token has the required scopes",
		)
	case http.StatusTooManyRequests:
		return NewBaseError(
			fmt.Sprintf("rate limit exceeded for %s", host),
			"Retry after rate limit window resets",
		)
	default:
		return nil
	}
}

// TranslateNetworkError checks whether err is a network-level error and
// returns a StructuredError with a user-facing message. Returns nil for
// nil errors and non-network errors so the caller can pass through the
// original error when it doesn't match.
func TranslateNetworkError(err error, host string) error {
	if err == nil {
		return nil
	}
	msg := err.Error()
	// Common DNS / dial / network failure patterns.
	if strings.Contains(msg, "no such host") ||
		strings.Contains(msg, "connection refused") ||
		strings.Contains(msg, "i/o timeout") ||
		strings.Contains(msg, "server misbehaving") ||
		strings.Contains(msg, "dial tcp") ||
		strings.Contains(msg, "lookup") {
		return NewBaseError(
			fmt.Sprintf("cannot reach %s — network error", host),
			"Check your network connection and the host name",
		)
	}
	return nil
}
