package gitlab

import (
	"context"
	"fmt"

	"github.com/tnikic/anvil/internal/forge"
	gl "gitlab.com/gitlab-org/api/client-go"
)

// relationReader implements the read side of forge.RelationService for GitLab.
type relationReader struct {
	forge *Forge
}

func (r *relationReader) BlockedBy(ctx context.Context, number int) ([]forge.IssueDependency, error) {
	relations, _, err := r.forge.client.IssueLinks.ListIssueRelations(
		r.forge.owner+"/"+r.forge.repo, int64(number),
		gl.WithContext(ctx),
	)
	if err != nil {
		return nil, r.forge.translateError(fmt.Sprintf("issue #%d blocked-by", number), err)
	}

	var deps []forge.IssueDependency
	for _, rel := range relations {
		if rel.LinkType == "is_blocked_by" {
			deps = append(deps, forge.IssueDependency{
				Number:    int(rel.IID),
				Title:     rel.Title,
				State:     normalizeState(rel.State),
				Direction: forge.DirBlockedBy,
			})
		}
	}
	return deps, nil
}

func (r *relationReader) Blocking(ctx context.Context, number int) ([]forge.IssueDependency, error) {
	relations, _, err := r.forge.client.IssueLinks.ListIssueRelations(
		r.forge.owner+"/"+r.forge.repo, int64(number),
		gl.WithContext(ctx),
	)
	if err != nil {
		return nil, r.forge.translateError(fmt.Sprintf("issue #%d blocking", number), err)
	}

	var deps []forge.IssueDependency
	for _, rel := range relations {
		if rel.LinkType == "blocks" {
			deps = append(deps, forge.IssueDependency{
				Number:    int(rel.IID),
				Title:     rel.Title,
				State:     normalizeState(rel.State),
				Direction: forge.DirBlocks,
			})
		}
	}
	return deps, nil
}

func (r *relationReader) Children(ctx context.Context, number int) ([]forge.IssueDependency, error) {
	// GitLab doesn't have a native sub-issues API; scan issue links for
	// parent/child relationships. Return empty on error (best-effort).
	relations, _, err := r.forge.client.IssueLinks.ListIssueRelations(
		r.forge.owner+"/"+r.forge.repo, int64(number),
		gl.WithContext(ctx),
	)
	if err != nil {
		return []forge.IssueDependency{}, nil //nolint:nilerr
	}

	var deps []forge.IssueDependency
	for _, rel := range relations {
		if rel.LinkType == "parent" {
			deps = append(deps, forge.IssueDependency{
				Number:    int(rel.IID),
				Title:     rel.Title,
				State:     normalizeState(rel.State),
				Direction: forge.DirChild,
			})
		}
	}
	return deps, nil
}

func (r *relationReader) Parent(ctx context.Context, number int) (*forge.IssueDependency, error) {
	// Scan issue links for "child" link type (indicating this issue is a child
	// of the linked issue). Return nil when no parent is found.
	relations, _, err := r.forge.client.IssueLinks.ListIssueRelations(
		r.forge.owner+"/"+r.forge.repo, int64(number),
		gl.WithContext(ctx),
	)
	if err != nil {
		return nil, nil //nolint:nilerr
	}

	for _, rel := range relations {
		if rel.LinkType == "child" {
			return &forge.IssueDependency{
				Number:    int(rel.IID),
				Title:     rel.Title,
				State:     normalizeState(rel.State),
				Direction: forge.DirParent,
			}, nil
		}
	}
	return nil, nil
}

// newRelationGuard creates a RelationGuard that wraps the reader methods
// (above) and uses the forge-specific mutation functions below for the
// actual API calls. The guard handles all idempotency checks.
func newRelationGuard(f *Forge) *forge.RelationGuard {
	reader := &relationReader{forge: f}
	return forge.NewRelationGuard(
		reader,
		func(ctx context.Context, number, target int) error { return f.addBlocks(ctx, number, target) },
		func(ctx context.Context, number, target int) error { return f.removeBlocks(ctx, number, target) },
		func(ctx context.Context, number, child int) error { return f.addParentOf(ctx, number, child) },
		func(ctx context.Context, number, child int) error { return f.removeParentOf(ctx, number, child) },
	)
}

// ---- raw mutation functions (no idempotency checks) ----

func (f *Forge) addBlocks(ctx context.Context, number, target int) error {
	_, _, err := f.client.IssueLinks.CreateIssueLink(
		f.owner+"/"+f.repo, int64(number),
		&gl.CreateIssueLinkOptions{
			TargetIssueIID: forge.String(fmt.Sprintf("%d", target)),
			LinkType:       forge.String("blocks"),
		},
		gl.WithContext(ctx),
	)
	if err != nil {
		return f.translateError(fmt.Sprintf("add blocks #%d → #%d", number, target), err)
	}
	return nil
}

func (f *Forge) removeBlocks(ctx context.Context, number, target int) error {
	// Find the link ID from the source issue's relations.
	relations, _, err := f.client.IssueLinks.ListIssueRelations(
		f.owner+"/"+f.repo, int64(number),
		gl.WithContext(ctx),
	)
	if err != nil {
		return f.translateError(fmt.Sprintf("remove blocks #%d → #%d", number, target), err)
	}
	linkID := findIssueLinkID(relations, "blocks", target)
	if linkID == 0 {
		return nil // link not found, no-op
	}

	_, _, err = f.client.IssueLinks.DeleteIssueLink(
		f.owner+"/"+f.repo, int64(number), linkID,
		gl.WithContext(ctx),
	)
	if err != nil {
		return f.translateError(fmt.Sprintf("remove blocks #%d → #%d", number, target), err)
	}
	return nil
}

func (f *Forge) addParentOf(ctx context.Context, number, child int) error {
	_, _, err := f.client.IssueLinks.CreateIssueLink(
		f.owner+"/"+f.repo, int64(number),
		&gl.CreateIssueLinkOptions{
			TargetIssueIID: forge.String(fmt.Sprintf("%d", child)),
			LinkType:       forge.String("parent"),
		},
		gl.WithContext(ctx),
	)
	if err != nil {
		return f.translateError(fmt.Sprintf("add parent #%d → #%d", number, child), err)
	}
	return nil
}

func (f *Forge) removeParentOf(ctx context.Context, number, child int) error {
	// Find the parent link from the parent issue's relations.
	relations, _, err := f.client.IssueLinks.ListIssueRelations(
		f.owner+"/"+f.repo, int64(number),
		gl.WithContext(ctx),
	)
	if err != nil {
		return f.translateError(fmt.Sprintf("remove parent #%d → #%d", number, child), err)
	}
	linkID := findIssueLinkID(relations, "parent", child)
	if linkID == 0 {
		return nil // link not found, no-op
	}

	_, _, err = f.client.IssueLinks.DeleteIssueLink(
		f.owner+"/"+f.repo, int64(number), linkID,
		gl.WithContext(ctx),
	)
	if err != nil {
		return f.translateError(fmt.Sprintf("remove parent #%d → #%d", number, child), err)
	}
	return nil
}

// findIssueLinkID finds the ID of a link with the given linkType and target IID.
// Returns 0 if not found.
func findIssueLinkID(relations []*gl.IssueRelation, linkType string, targetIID int) int64 {
	for _, r := range relations {
		if r.LinkType == linkType && int(r.IID) == targetIID {
			return r.IssueLinkID
		}
	}
	return 0
}
