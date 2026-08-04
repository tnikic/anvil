package forge

import "strings"

// String returns a pointer to the given string value.
// Useful for constructing options structs with optional fields.
func String(s string) *string { return &s }

// StringVal returns the string value pointed to by s, or "" if s is nil.
// This is the inverse of String — a self-documenting pair for optional
// string fields.
func StringVal(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// Int returns a pointer to the given int value.
// Useful for constructing options structs with optional fields.
func Int(i int) *int { return &i }

// Bool returns a pointer to the given bool value.
// Useful for constructing options structs with optional fields.
func Bool(b bool) *bool { return &b }

// ---- Label scope helpers (forge-agnostic, uses ":" separator) ----

// SplitLabel splits a label reference like "kind:bug" into scope and name.
// For labels without a colon, scope is empty and name is the full string.
// This is the CLI-boundary form that uses ":" as a universal separator.
func SplitLabel(raw string) (scope, name string) {
	return ParseLabelScope(raw, ":")
}

// BuildLabelName joins scope and name into a label reference like "kind:bug".
// If scope is empty, returns name unchanged.
// This is the CLI-boundary form that uses ":" as a universal separator.
func BuildLabelName(scope, name string) string {
	return LabelFullName(scope, name, ":")
}

// ---- Pagination ----

// Page holds a single page of results from a paginated API call.
type Page[T any] struct {
	Items    []T
	NextPage int // 0 means no more pages
}

// Paginate accumulates results across pages by calling fetch for each page.
// fetch receives the page number (1-indexed) and returns the items for that
// page along with the next page number. When NextPage is 0, pagination stops.
// limit caps the total number of items; 0 means no limit.
func Paginate[T any](limit int, fetch func(page int) (Page[T], error)) ([]T, error) {
	var all []T
	page := 1
	for {
		p, err := fetch(page)
		if err != nil {
			return nil, err
		}
		all = append(all, p.Items...)
		if p.NextPage == 0 {
			break
		}
		if limit > 0 && len(all) >= limit {
			break
		}
		page = p.NextPage
	}
	return all, nil
}

// ListPerPage computes the per-page count and effective limit for paginated
// list operations. If limit is 0, effectiveLimit defaults to 200.
func ListPerPage(limit int) (perPage, effectiveLimit int) {
	perPage = 30
	if limit > 0 && limit < perPage {
		perPage = limit
	}
	effectiveLimit = limit
	if effectiveLimit <= 0 {
		effectiveLimit = 200
	}
	return
}

// NewListMeta creates a ListMeta with count as both Count and Total.
func NewListMeta(count int) *ListMeta {
	return &ListMeta{Count: count, Total: count}
}

// ---- Label scope helpers (parameterized by separator) ----

// ParseLabelScope splits a scoped label name like "scope<sep>name" into
// (scope, name). sep is the forge-specific separator (":" for GitHub,
// "::" for GitLab). For labels without the separator, scope is empty
// and name is the full label name. Only the first occurrence of sep is
// used as the split point.
func ParseLabelScope(fullName, sep string) (scope, name string) {
	if idx := strings.Index(fullName, sep); idx >= 0 {
		return fullName[:idx], fullName[idx+len(sep):]
	}
	return "", fullName
}

// LabelFullName joins scope and name into a forge-style scoped label name.
// If scope is empty, returns name unchanged. Otherwise returns
// "scope<sep>name". sep is the forge-specific separator.
func LabelFullName(scope, name, sep string) string {
	if scope == "" {
		return name
	}
	return scope + sep + name
}

// ---- Host URL parsing ----

// ParseHost splits a host string into scheme and clean host.
// If no scheme prefix is present, defaults to "https".
// Used by all forge adapters during construction.
func ParseHost(host string) (scheme, cleanHost string) {
	scheme = "https"
	cleanHost = host
	if strings.HasPrefix(host, "http://") {
		scheme = "http"
		cleanHost = strings.TrimPrefix(host, "http://")
	} else if strings.HasPrefix(host, "https://") {
		cleanHost = strings.TrimPrefix(host, "https://")
	}
	return
}
