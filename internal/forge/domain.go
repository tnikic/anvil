package forge

import "time"

// Issue and PR states.
const (
	StateOpen   = "open"
	StateClosed = "closed"
	StateMerged = "merged" // PR only
	StateAll    = "all"    // list filter only
)

// Issue represents an issue with normalized fields across forges.
// Fields present in ≥2 forges are normalized; forge-specific fields go in Extras.
type Issue struct {
	Number    int            `json:"number"`
	Title     string         `json:"title"`
	State     string         `json:"state"` // "open", "closed"
	Body      string         `json:"body"`
	Labels    []Label        `json:"labels"`
	Author    string         `json:"author"`
	Parent    *int           `json:"parent,omitempty"` // parent issue number, nil if none
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	URL       string         `json:"url"`
	Extras    map[string]any `json:"extras,omitempty"`
}

// Label represents a label with normalized fields.
// Scope (e.g., "kind" from GitHub's "kind:bug" or GitLab's "kind::bug")
// and Exclusive are normalized across GitHub, GitLab, and Forgejo.
type Label struct {
	Name        string         `json:"name"`
	Scope       string         `json:"scope,omitempty"` // empty for unscoped labels
	Color       string         `json:"color"`           // hex color without #
	Description string         `json:"description"`
	Exclusive   bool           `json:"exclusive"` // scoped labels: only one per scope
	Extras      map[string]any `json:"extras,omitempty"`
}

// ReviewState represents a single reviewer's review status on a PR.
type ReviewState struct {
	Login string `json:"login"`
	State string `json:"state"` // APPROVED, CHANGES_REQUESTED, COMMENTED, PENDING
}

// CheckSummary holds aggregate check/CI run status for a PR.
type CheckSummary struct {
	Passed int `json:"passed"`
	Total  int `json:"total"`
}

// PR represents a pull request / merge request with normalized fields.
type PR struct {
	Number       int            `json:"number"`
	Title        string         `json:"title"`
	State        string         `json:"state"` // "open", "closed", "merged"
	Body         string         `json:"body"`
	BaseRef      string         `json:"base_ref"`        // target branch
	HeadRef      string         `json:"head_ref"`        // source branch
	Stack        string         `json:"stack,omitempty"` // stack name extracted from title prefix
	DependsOn    []int          `json:"depends_on"`      // PR numbers this depends on
	DependedOnBy []int          `json:"depended_on_by"`  // PR numbers that depend on this
	Reviewers    []ReviewState  `json:"reviewers,omitempty"`
	Checks       *CheckSummary  `json:"checks,omitempty"`
	Author       string         `json:"author"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	URL          string         `json:"url"`
	Extras       map[string]any `json:"extras,omitempty"`
}

// Comment represents a comment on an issue with normalized fields.
type Comment struct {
	ID        int            `json:"id"`
	Body      string         `json:"body"`
	Author    string         `json:"author"`
	System    bool           `json:"system"` // true for system-generated notes (GitLab)
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	URL       string         `json:"url"`
	Reactions map[string]int `json:"reactions,omitempty"` // reaction name → count
	Extras    map[string]any `json:"extras,omitempty"`
}

// IssueDependencyDirection describes the direction of a relationship.
type IssueDependencyDirection string

const (
	DirBlocks    IssueDependencyDirection = "blocks"
	DirBlockedBy IssueDependencyDirection = "blocked_by"
	DirChild     IssueDependencyDirection = "child"
	DirParent    IssueDependencyDirection = "parent"
)

// IssueDependency represents a relationship between two issues.
// Direction indicates whether this issue blocks, is blocked by, is a child of,
// or is a parent of the issue being queried.
type IssueDependency struct {
	Number    int                      `json:"number"`
	Title     string                   `json:"title"`
	State     string                   `json:"state"` // "open", "closed"
	Direction IssueDependencyDirection `json:"direction"`
}

// ListMeta holds aggregate count information for list responses.
type ListMeta struct {
	Total int `json:"total"` // total number of items matching the query
	Count int `json:"count"` // number of items in this response
}
