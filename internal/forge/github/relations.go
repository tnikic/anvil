package github

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	gh "github.com/google/go-github/v90/github"
	"github.com/tnikic/anvil/internal/forge"
)

// relationReader implements the read side of forge.RelationService for GitHub.
type relationReader struct {
	forge *Forge
}

func (r *relationReader) BlockedBy(ctx context.Context, number int) ([]forge.IssueDependency, error) {
	ghIssues, _, err := r.forge.client.Issues.ListBlockedBy(ctx, r.forge.owner, r.forge.repo, int64(number), nil)
	if err != nil {
		return nil, r.forge.translateError(fmt.Sprintf("issue #%d blocked-by", number), err)
	}
	return mapIssueDeps(ghIssues, forge.DirBlockedBy), nil
}

func (r *relationReader) Blocking(ctx context.Context, number int) ([]forge.IssueDependency, error) {
	ghIssues, _, err := r.forge.client.Issues.ListBlocking(ctx, r.forge.owner, r.forge.repo, int64(number), nil)
	if err != nil {
		return nil, r.forge.translateError(fmt.Sprintf("issue #%d blocking", number), err)
	}
	return mapIssueDeps(ghIssues, forge.DirBlocks), nil
}

func (r *relationReader) Children(ctx context.Context, number int) ([]forge.IssueDependency, error) {
	subIssues, _, err := r.forge.client.SubIssue.ListByIssue(ctx, r.forge.owner, r.forge.repo, int64(number), nil)
	if err != nil {
		return nil, r.forge.translateError(fmt.Sprintf("issue #%d children", number), err)
	}
	ghIssues := make([]*gh.Issue, len(subIssues))
	for i, si := range subIssues {
		ghIssues[i] = (*gh.Issue)(si)
	}
	return mapIssueDeps(ghIssues, forge.DirChild), nil
}

func (r *relationReader) Parent(ctx context.Context, number int) (*forge.IssueDependency, error) {
	ghIssue, _, err := r.forge.client.SubIssue.GetParentIssue(ctx, r.forge.owner, r.forge.repo, int64(number))
	if err != nil {
		var ghErr *gh.ErrorResponse
		if errors.As(err, &ghErr) && ghErr.Response.StatusCode == http.StatusNotFound {
			return nil, nil // no parent — valid state
		}
		return nil, r.forge.translateError(fmt.Sprintf("issue #%d parent", number), err)
	}
	if ghIssue.GetNumber() == 0 {
		return nil, nil
	}
	return &forge.IssueDependency{
		Number:    ghIssue.GetNumber(),
		Title:     ghIssue.GetTitle(),
		State:     ghIssue.GetState(),
		Direction: forge.DirParent,
	}, nil
}

// newRelationGuard creates a RelationGuard that wraps the reader methods
// (above) and uses SDK-based mutation functions for the actual API calls.
// The guard handles all idempotency checks.
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

// ---- SDK-based mutation functions (no idempotency checks) ----

func (f *Forge) addBlocks(ctx context.Context, number, target int) error {
	id, err := f.issueNumberToID(ctx, number)
	if err != nil {
		return err
	}
	_, _, err = f.client.Issues.AddBlockedBy(ctx, f.owner, f.repo, int64(target), gh.IssueDependencyRequest{IssueID: id})
	if err != nil {
		return f.translateError(fmt.Sprintf("add blocks #%d → #%d", number, target), err)
	}
	return nil
}

func (f *Forge) removeBlocks(ctx context.Context, number, target int) error {
	id, err := f.issueNumberToID(ctx, number)
	if err != nil {
		return err
	}
	_, _, err = f.client.Issues.RemoveBlockedBy(ctx, f.owner, f.repo, int64(target), id)
	if err != nil {
		return f.translateError(fmt.Sprintf("remove blocks #%d → #%d", number, target), err)
	}
	return nil
}

func (f *Forge) addParentOf(ctx context.Context, number, child int) error {
	id, err := f.issueNumberToID(ctx, child)
	if err != nil {
		return err
	}
	_, _, err = f.client.SubIssue.Add(ctx, f.owner, f.repo, int64(number), gh.SubIssueRequest{SubIssueID: id})
	if err != nil {
		return f.translateError(fmt.Sprintf("add parent #%d → #%d", number, child), err)
	}
	return nil
}

func (f *Forge) removeParentOf(ctx context.Context, number, child int) error {
	id, err := f.issueNumberToID(ctx, child)
	if err != nil {
		return err
	}
	_, _, err = f.client.SubIssue.Remove(ctx, f.owner, f.repo, int64(number), gh.SubIssueRequest{SubIssueID: id})
	if err != nil {
		return f.translateError(fmt.Sprintf("remove parent #%d → #%d", number, child), err)
	}
	return nil
}

// ---- helpers ----

// issueNumberToID fetches an issue by its repo-scoped number and returns
// its global (database) ID required by the relation mutation endpoints.
func (f *Forge) issueNumberToID(ctx context.Context, number int) (int64, error) {
	ghIssue, _, err := f.client.Issues.Get(ctx, f.owner, f.repo, number)
	if err != nil {
		return 0, f.translateError(fmt.Sprintf("issue #%d", number), err)
	}
	return ghIssue.GetID(), nil
}

func mapIssueDeps(ghIssues []*gh.Issue, dir forge.IssueDependencyDirection) []forge.IssueDependency {
	out := make([]forge.IssueDependency, 0, len(ghIssues))
	for _, i := range ghIssues {
		if i == nil {
			continue
		}
		out = append(out, forge.IssueDependency{
			Number:    i.GetNumber(),
			Title:     i.GetTitle(),
			State:     i.GetState(),
			Direction: dir,
		})
	}
	return out
}
