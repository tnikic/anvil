package forge

import (
	"context"
	"fmt"
)

// IssueHints holds aggregated relationship and comment hints for an issue view.
// Each field is a human-readable string suitable for display, or empty if the
// corresponding data is unavailable or empty.
type IssueHints struct {
	CommentsHint  string
	BlockedByHint string
	BlockingHint  string
	ChildrenHint  string
	ParentHint    string
}

// EnrichIssueHints fetches relationship and comment hints for an issue.
// All hint fetches are best-effort: errors are silently ignored and
// missing/empty data results in an empty hint string.
//
// This function replaces the orchestration that was previously done
// inline in the "issue view" command handler, which had to call 5
// service methods and handle errors for each. Callers that need hints
// for an issue view can call this once instead of wiring up each
// service individually.
func EnrichIssueHints(ctx context.Context, f Forge, number int) IssueHints {
	var hints IssueHints

	if comments, err := f.Comments().List(ctx, CommentListOptions{IssueNumber: number}); err == nil && len(comments) > 0 {
		hints.CommentsHint = fmt.Sprintf("%d — use 'anvil issue comment list %d'", len(comments), number)
	}

	rel := f.Relations()
	if blockedBy, err := rel.BlockedBy(ctx, number); err == nil && len(blockedBy) > 0 {
		hints.BlockedByHint = fmt.Sprintf("%d — use 'anvil issue blocked-by %d'", len(blockedBy), number)
	}
	if blocking, err := rel.Blocking(ctx, number); err == nil && len(blocking) > 0 {
		hints.BlockingHint = fmt.Sprintf("%d — use 'anvil issue blocking %d'", len(blocking), number)
	}
	if children, err := rel.Children(ctx, number); err == nil && len(children) > 0 {
		hints.ChildrenHint = fmt.Sprintf("%d — use 'anvil issue children %d'", len(children), number)
	}
	if parent, err := rel.Parent(ctx, number); err == nil && parent != nil {
		hints.ParentHint = fmt.Sprintf("%d — use 'anvil issue parent %d'", parent.Number, number)
	}

	return hints
}
