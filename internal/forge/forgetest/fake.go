package forgetest

import (
	"context"

	"github.com/tnikic/anvil/internal/forge"
)

// FakeForge is a test double implementing forge.Forge.
// Each service field is a state-based fake that can be pre-populated
// with data. Individual methods can be overridden by setting ...Fn fields
// on each service.
type FakeForge struct {
	IssueSvc    *FakeIssueService
	LabelSvc    *FakeLabelService
	PRSvc       *FakePRService
	RelationSvc *FakeRelationService
	CommentSvc  *FakeCommentService

	// CurrentUserFn overrides CurrentUser behaviour. If nil, returns "test-user".
	CurrentUserFn func(ctx context.Context) (string, error)
}

var _ forge.Forge = (*FakeForge)(nil)

// NewFakeForge creates a FakeForge with all five services initialized.
// Callers can then populate Issues, Labels, PRs slices as needed.
func NewFakeForge() *FakeForge {
	return &FakeForge{
		IssueSvc:    &FakeIssueService{},
		LabelSvc:    &FakeLabelService{},
		PRSvc:       &FakePRService{},
		RelationSvc: &FakeRelationService{},
		CommentSvc:  &FakeCommentService{},
	}
}

// NewFakeForgeIssueOnly creates a FakeForge with the Issue and Relation
// services initialized. Label, PR, and Comment services are nil — callers must
// set them before they are accessed.
func NewFakeForgeIssueOnly() *FakeForge {
	return &FakeForge{
		IssueSvc:    &FakeIssueService{},
		RelationSvc: &FakeRelationService{},
	}
}

func (f *FakeForge) Issues() forge.IssueService       { return f.IssueSvc }
func (f *FakeForge) Labels() forge.LabelService       { return f.LabelSvc }
func (f *FakeForge) PRs() forge.PRService             { return f.PRSvc }
func (f *FakeForge) Relations() forge.RelationService { return f.RelationSvc }
func (f *FakeForge) Comments() forge.CommentService   { return f.CommentSvc }

func (f *FakeForge) CurrentUser(ctx context.Context) (string, error) {
	if f.CurrentUserFn != nil {
		return f.CurrentUserFn(ctx)
	}
	return "test-user", nil
}
