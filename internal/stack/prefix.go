// Package stack implements PR stack tracking — parsing, ordering, renumbering,
// and diagnosing linear chains of dependent PRs identified by title prefixes.
//
// Pure functions (ParsePrefix, FormatPrefix, CleanTitle, Populate, SortKey, Sort,
// ComputeDepends, DeriveName, CollectOpen, DiagnoseBroken) require no dependencies
// and are tested directly. Orchestration that requires forge API access lives on
// Tracker, which takes a narrow PRUpdater interface.
package stack

import (
	"fmt"
	"regexp"
	"strconv"
)

// prefixRe matches a stack prefix like "[auth:2/3]" at the start of a title.
var prefixRe = regexp.MustCompile(`^\[([a-zA-Z][a-zA-Z0-9_.-]*):(\d+)/(\d+)\]\s`)

// Prefix holds the parsed components of a stack title prefix.
type Prefix struct {
	Name  string // stack name (e.g., "auth")
	Pos   int    // position in the stack (1-indexed)
	Total int    // total PRs in the stack at time of prefix
}

// ParsePrefix extracts stack info from a PR title.
// Returns the Prefix and whether a prefix was found.
//
//	"[auth:2/3] Add OAuth" → Prefix{Name: "auth", Pos: 2, Total: 3}, true
//	"Fix login"           → Prefix{}, false
func ParsePrefix(title string) (Prefix, bool) {
	m := prefixRe.FindStringSubmatch(title)
	if m == nil {
		return Prefix{}, false
	}
	pos, _ := strconv.Atoi(m[2])
	total, _ := strconv.Atoi(m[3])
	return Prefix{Name: m[1], Pos: pos, Total: total}, true
}

// FormatPrefix builds a stack prefix string from its components.
//
//	FormatPrefix(Prefix{Name: "auth", Pos: 2, Total: 3}) → "[auth:2/3]"
func FormatPrefix(p Prefix) string {
	return fmt.Sprintf("[%s:%d/%d]", p.Name, p.Pos, p.Total)
}

// CleanTitle strips the stack prefix from a title, returning the clean title.
// Returns the title unchanged if no prefix is present.
func CleanTitle(title string) string {
	loc := prefixRe.FindStringIndex(title)
	if loc == nil {
		return title
	}
	return title[loc[1]:]
}
