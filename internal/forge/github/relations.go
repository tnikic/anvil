package github

import (
	"context"
	"fmt"

	gh "github.com/google/go-github/v69/github"
	"github.com/tnikic/anvil/internal/forge"
)

// relationReader implements the read side of forge.RelationService for GitHub.
type relationReader struct {
	forge *Forge
}

func (r *relationReader) BlockedBy(ctx context.Context, number int) ([]forge.IssueDependency, error) {
	u := fmt.Sprintf("repos/%s/%s/issues/%d/dependencies/blocked_by", r.forge.owner, r.forge.repo, number)
	req, err := r.forge.client.NewRequest("GET", u, nil)
	if err != nil {
		return nil, r.forge.translateError("", err)
	}
	var ghIssues []*gh.Issue
	_, err = r.forge.client.Do(ctx, req, &ghIssues)
	if err != nil {
		return nil, r.forge.translateError(fmt.Sprintf("issue #%d blocked-by", number), err)
	}
	return mapIssueDeps(ghIssues, forge.DirBlockedBy), nil
}

func (r *relationReader) Blocking(ctx context.Context, number int) ([]forge.IssueDependency, error) {
	u := fmt.Sprintf("repos/%s/%s/issues/%d/dependencies/blocking", r.forge.owner, r.forge.repo, number)
	req, err := r.forge.client.NewRequest("GET", u, nil)
	if err != nil {
		return nil, r.forge.translateError("", err)
	}
	var ghIssues []*gh.Issue
	_, err = r.forge.client.Do(ctx, req, &ghIssues)
	if err != nil {
		return nil, r.forge.translateError(fmt.Sprintf("issue #%d blocking", number), err)
	}
	return mapIssueDeps(ghIssues, forge.DirBlocks), nil
}

func (r *relationReader) Children(ctx context.Context, number int) ([]forge.IssueDependency, error) {
	u := fmt.Sprintf("repos/%s/%s/issues/%d/sub_issues", r.forge.owner, r.forge.repo, number)
	req, err := r.forge.client.NewRequest("GET", u, nil)
	if err != nil {
		return nil, r.forge.translateError("", err)
	}
	var ghIssues []*gh.Issue
	_, err = r.forge.client.Do(ctx, req, &ghIssues)
	if err != nil {
		return nil, r.forge.translateError(fmt.Sprintf("issue #%d children", number), err)
	}
	return mapIssueDeps(ghIssues, forge.DirChild), nil
}

func (r *relationReader) Parent(ctx context.Context, number int) (*forge.IssueDependency, error) {
	u := fmt.Sprintf("repos/%s/%s/issues/%d/parent", r.forge.owner, r.forge.repo, number)
	req, err := r.forge.client.NewRequest("GET", u, nil)
	if err != nil {
		return nil, r.forge.translateError("", err)
	}
	var ghIssue gh.Issue
	_, err = r.forge.client.Do(ctx, req, &ghIssue)
	if err != nil {
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
	u := fmt.Sprintf("repos/%s/%s/issues/%d/dependencies/blocked_by", f.owner, f.repo, target)
	req, err := f.client.NewRequest("POST", u, &dependencyRequest{IssueNumber: number})
	if err != nil {
		return f.translateError("", err)
	}
	_, err = f.client.Do(ctx, req, nil)
	if err != nil {
		return f.translateError(fmt.Sprintf("add blocks #%d → #%d", number, target), err)
	}
	return nil
}

func (f *Forge) removeBlocks(ctx context.Context, number, target int) error {
	u := fmt.Sprintf("repos/%s/%s/issues/%d/dependencies/blocked_by", f.owner, f.repo, target)
	req, err := f.client.NewRequest("DELETE", u, &dependencyRequest{IssueNumber: number})
	if err != nil {
		return f.translateError("", err)
	}
	_, err = f.client.Do(ctx, req, nil)
	if err != nil {
		return f.translateError(fmt.Sprintf("remove blocks #%d → #%d", number, target), err)
	}
	return nil
}

func (f *Forge) addParentOf(ctx context.Context, number, child int) error {
	u := fmt.Sprintf("repos/%s/%s/issues/%d/sub_issues", f.owner, f.repo, number)
	req, err := f.client.NewRequest("POST", u, &subIssueRequest{SubIssueID: child})
	if err != nil {
		return f.translateError("", err)
	}
	_, err = f.client.Do(ctx, req, nil)
	if err != nil {
		return f.translateError(fmt.Sprintf("add parent #%d → #%d", number, child), err)
	}
	return nil
}

func (f *Forge) removeParentOf(ctx context.Context, number, child int) error {
	u := fmt.Sprintf("repos/%s/%s/issues/%d/sub_issues", f.owner, f.repo, number)
	req, err := f.client.NewRequest("DELETE", u, &subIssueRequest{SubIssueID: child})
	if err != nil {
		return f.translateError("", err)
	}
	_, err = f.client.Do(ctx, req, nil)
	if err != nil {
		return f.translateError(fmt.Sprintf("remove parent #%d → #%d", number, child), err)
	}
	return nil
}

// ---- helpers ----

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

// dependencyRequest is the JSON body for adding/removing a blocking dependency.
type dependencyRequest struct {
	IssueNumber int `json:"issue_number"`
}

// subIssueRequest is the JSON body for adding/removing a sub-issue.
type subIssueRequest struct {
	SubIssueID int `json:"sub_issue_id"`
}
