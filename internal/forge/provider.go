package forge

import "context"

// Forge is the central facade interface composed of service interfaces.
// Implementations provide access to a specific forge (GitHub, GitLab, Forgejo).
type Forge interface {
	Issues() IssueService
	Labels() LabelService
	PRs() PRService
	Relations() RelationService
	Comments() CommentService

	// CurrentUser returns the login of the currently authenticated user.
	// Returns an error if the user is not authenticated or the lookup fails.
	CurrentUser(ctx context.Context) (string, error)
}

// IssueService defines operations for issue management.
type IssueService interface {
	List(ctx context.Context, opts IssueListOptions) ([]Issue, *ListMeta, error)
	Get(ctx context.Context, opts IssueGetOptions) (*Issue, error)
	Create(ctx context.Context, opts IssueCreateOptions) (*Issue, error)
	Update(ctx context.Context, opts IssueUpdateOptions) (*Issue, error)
	Close(ctx context.Context, opts IssueCloseOptions) (*Issue, error)
	Reopen(ctx context.Context, opts IssueReopenOptions) (*Issue, error)
}

// LabelService defines operations for label management.
type LabelService interface {
	List(ctx context.Context, opts LabelListOptions) ([]Label, error)
	Create(ctx context.Context, opts LabelCreateOptions) (*Label, error)
	Update(ctx context.Context, opts LabelUpdateOptions) (*Label, error)
	Delete(ctx context.Context, opts LabelDeleteOptions) error
}

// PRService defines operations for pull request management.
type PRService interface {
	List(ctx context.Context, opts PRListOptions) ([]PR, *ListMeta, error)
	Get(ctx context.Context, opts PRGetOptions) (*PR, error)
	Create(ctx context.Context, opts PRCreateOptions) (*PR, error)
	Update(ctx context.Context, opts PRUpdateOptions) (*PR, error)
	Merge(ctx context.Context, opts PRMergeOptions) (*PR, error)
	Close(ctx context.Context, opts PRCloseOptions) (*PR, error)
}

// ---- Issue options ----

// IssueListOptions holds parameters for listing issues.
type IssueListOptions struct {
	State     string   // "open", "closed", "all"
	Labels    []string // filter by label names
	Sort      string   // "created", "updated", "comments"
	Direction string   // "asc", "desc"
	Limit     int      // max results per page
	Page      int      // page number (1-indexed)
}

// IssueGetOptions holds parameters for fetching a single issue.
type IssueGetOptions struct {
	Number int // issue number
}

// IssueCreateOptions holds parameters for creating an issue.
type IssueCreateOptions struct {
	Title     *string  `json:"title,omitempty"` // required
	Body      *string  `json:"body,omitempty"`
	Labels    []string `json:"labels,omitempty"`
	Assignees []string `json:"assignees,omitempty"`
}

// IssueUpdateOptions holds parameters for updating an issue.
type IssueUpdateOptions struct {
	Number       int      // issue number
	Title        *string  `json:"title,omitempty"`
	Body         *string  `json:"body,omitempty"`
	State        *string  `json:"state,omitempty"`         // "open" or "closed"
	Labels       []string `json:"labels,omitempty"`        // replace-all: set exact label set
	AddLabels    []string `json:"add_labels,omitempty"`    // incremental: add these labels
	RemoveLabels []string `json:"remove_labels,omitempty"` // incremental: remove these labels
	Assignees    []string `json:"assignees,omitempty"`
}

// IssueCloseOptions holds parameters for closing an issue.
type IssueCloseOptions struct {
	Number int // issue number
}

// IssueReopenOptions holds parameters for reopening an issue.
type IssueReopenOptions struct {
	Number int // issue number
}

// RelationService defines operations for issue relationship queries and mutations.
type RelationService interface {
	BlockedBy(ctx context.Context, number int) ([]IssueDependency, error)
	Blocking(ctx context.Context, number int) ([]IssueDependency, error)
	Children(ctx context.Context, number int) ([]IssueDependency, error)
	Parent(ctx context.Context, number int) (*IssueDependency, error)

	// AddBlocks makes `number` block `target`. Idempotent: no-op if already present.
	AddBlocks(ctx context.Context, number int, target int) error
	// RemoveBlocks removes the "number blocks target" relationship. Idempotent: no-op if absent.
	RemoveBlocks(ctx context.Context, number int, target int) error
	// AddParentOf makes `number` the parent of `child`. Idempotent: no-op if already parent.
	AddParentOf(ctx context.Context, number int, child int) error
	// RemoveParentOf removes the parent/child relationship between `number` and `child`. Idempotent: no-op if absent.
	RemoveParentOf(ctx context.Context, number int, child int) error
}

// ---- Label options ----

// LabelListOptions holds parameters for listing labels.
type LabelListOptions struct {
	Limit int // max results per page
	Page  int // page number (1-indexed)
}

// LabelCreateOptions holds parameters for creating a label.
type LabelCreateOptions struct {
	Scope       *string `json:"scope,omitempty"` // scope prefix (nil for unscoped)
	Name        string  `json:"name"`            // label name (required)
	Color       *string `json:"color,omitempty"` // hex color without #
	Description *string `json:"description,omitempty"`
	Exclusive   *bool   `json:"exclusive,omitempty"` // only one label allowed per scope
}

// LabelUpdateOptions holds parameters for updating a label.
type LabelUpdateOptions struct {
	Scope       string  // current scope
	Name        string  // current name
	NewName     *string `json:"new_name,omitempty"`
	NewScope    *string `json:"new_scope,omitempty"`
	Color       *string `json:"color,omitempty"`
	Description *string `json:"description,omitempty"`
	Exclusive   *bool   `json:"exclusive,omitempty"`
}

// LabelDeleteOptions holds parameters for deleting a label.
type LabelDeleteOptions struct {
	Scope string // scope prefix (empty for unscoped)
	Name  string // label name
}

// ---- PR options ----

// PRListOptions holds parameters for listing pull requests.
type PRListOptions struct {
	State     string // "open", "closed", "merged", "all"
	Sort      string // "created", "updated"
	Direction string // "asc", "desc"
	Limit     int    // max results per page
	Page      int    // page number (1-indexed)
}

// PRGetOptions holds parameters for fetching a single pull request.
type PRGetOptions struct {
	Number int // PR number
}

// PRCreateOptions holds parameters for creating a pull request.
type PRCreateOptions struct {
	Title   *string `json:"title,omitempty"` // required
	Body    *string `json:"body,omitempty"`
	HeadRef *string `json:"head,omitempty"`  // source branch (default: current)
	BaseRef *string `json:"base,omitempty"`  // target branch (default: repo default)
	Stack   *string `json:"stack,omitempty"` // stack name; auto-derived from branch if nil
	Draft   *bool   `json:"draft,omitempty"`
}

// PRMergeOptions holds parameters for merging a pull request.
type PRMergeOptions struct {
	Number int     // PR number
	Method *string `json:"merge_method,omitempty"` // "merge", "squash", "rebase"
	Title  *string `json:"title,omitempty"`
	Body   *string `json:"body,omitempty"`
}

// PRUpdateOptions holds parameters for updating a pull request.
type PRUpdateOptions struct {
	Number int     // PR number
	Title  *string `json:"title,omitempty"`
}

// PRCloseOptions holds parameters for closing a pull request.
type PRCloseOptions struct {
	Number int // PR number
}

// ---- Comment service ----

// CommentService defines operations for issue comment management.
type CommentService interface {
	List(ctx context.Context, opts CommentListOptions) ([]Comment, error)
	Get(ctx context.Context, opts CommentGetOptions) (*Comment, error)
	Create(ctx context.Context, opts CommentCreateOptions) (*Comment, error)
	Update(ctx context.Context, opts CommentUpdateOptions) (*Comment, error)
	Delete(ctx context.Context, opts CommentDeleteOptions) error
}

// CommentListOptions holds parameters for listing comments on an issue.
type CommentListOptions struct {
	IssueNumber   int  // issue number (required)
	IncludeSystem bool // include system-generated notes (GitLab)
}

// CommentGetOptions holds parameters for fetching a single comment.
type CommentGetOptions struct {
	IssueNumber int // issue number
	CommentID   int // comment ID
}

// CommentCreateOptions holds parameters for creating a comment.
type CommentCreateOptions struct {
	IssueNumber int    // issue number
	Body        string // comment body (markdown)
}

// CommentUpdateOptions holds parameters for updating a comment.
type CommentUpdateOptions struct {
	IssueNumber int    // issue number
	CommentID   int    // comment ID
	Body        string // new body (markdown)
}

// CommentDeleteOptions holds parameters for deleting a comment.
type CommentDeleteOptions struct {
	IssueNumber int // issue number
	CommentID   int // comment ID
}
