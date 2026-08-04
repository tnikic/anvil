package forgetest

import (
	"context"
	"fmt"
	"time"

	"github.com/tnikic/anvil/internal/forge"
)

// ---- Fake IssueService ----

// FakeIssueService is a state-based fake of forge.IssueService.
// Issues stored in the Issues slice behave like real forge data.
// Individual methods can be overridden by setting their ...Fn fields.
type FakeIssueService struct {
	Issues  []forge.Issue
	ListErr error
	GetErr  error

	// Fn overrides — if set, the function is called instead of the state-based behaviour.
	ListFn   func(ctx context.Context, opts forge.IssueListOptions) ([]forge.Issue, *forge.ListMeta, error)
	GetFn    func(ctx context.Context, opts forge.IssueGetOptions) (*forge.Issue, error)
	CreateFn func(ctx context.Context, opts forge.IssueCreateOptions) (*forge.Issue, error)
	UpdateFn func(ctx context.Context, opts forge.IssueUpdateOptions) (*forge.Issue, error)
	CloseFn  func(ctx context.Context, opts forge.IssueCloseOptions) (*forge.Issue, error)
	ReopenFn func(ctx context.Context, opts forge.IssueReopenOptions) (*forge.Issue, error)

	// Last*Opts capture the most recent call arguments for assertion.
	LastListOpts   forge.IssueListOptions
	LastGetOpts    forge.IssueGetOptions
	LastCreateOpts forge.IssueCreateOptions
	LastUpdateOpts forge.IssueUpdateOptions
	LastCloseOpts  forge.IssueCloseOptions
	LastReopenOpts forge.IssueReopenOptions

	// AddLabelCalls records each call to add labels (slice of label names per call).
	AddLabelCalls [][]string
	// RemoveLabelCalls records each call to remove labels (slice of label names per call).
	RemoveLabelCalls [][]string
}

var _ forge.IssueService = (*FakeIssueService)(nil)

func (s *FakeIssueService) List(ctx context.Context, opts forge.IssueListOptions) ([]forge.Issue, *forge.ListMeta, error) {
	s.LastListOpts = opts
	if s.ListFn != nil {
		return s.ListFn(ctx, opts)
	}
	if s.ListErr != nil {
		return nil, nil, s.ListErr
	}
	issues := s.Issues
	if issues == nil {
		issues = []forge.Issue{}
	}
	meta := &forge.ListMeta{Total: len(issues), Count: len(issues)}
	return issues, meta, nil
}

func (s *FakeIssueService) Get(ctx context.Context, opts forge.IssueGetOptions) (*forge.Issue, error) {
	s.LastGetOpts = opts
	if s.GetFn != nil {
		return s.GetFn(ctx, opts)
	}
	if s.GetErr != nil {
		return nil, s.GetErr
	}
	for i, iss := range s.Issues {
		if iss.Number == opts.Number {
			return &s.Issues[i], nil
		}
	}
	return nil, forge.NewBaseError("not found", "Run \"anvil issue list\"")
}

func (s *FakeIssueService) Create(ctx context.Context, opts forge.IssueCreateOptions) (*forge.Issue, error) {
	s.LastCreateOpts = opts
	if s.CreateFn != nil {
		return s.CreateFn(ctx, opts)
	}
	number := 42
	if len(s.Issues) > 0 {
		number = s.Issues[len(s.Issues)-1].Number + 1
	}
	title := ""
	if opts.Title != nil {
		title = *opts.Title
	}
	issue := forge.Issue{
		Number:    number,
		Title:     title,
		State:     "open",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		URL:       fmt.Sprintf("https://github.com/test/repo/issues/%d", number),
		Labels:    parseLabels(opts.Labels),
	}
	s.Issues = append(s.Issues, issue)
	return &issue, nil
}

func (s *FakeIssueService) Update(ctx context.Context, opts forge.IssueUpdateOptions) (*forge.Issue, error) {
	s.LastUpdateOpts = opts
	if s.UpdateFn != nil {
		return s.UpdateFn(ctx, opts)
	}
	for i, iss := range s.Issues {
		if iss.Number == opts.Number {
			if opts.Title != nil {
				s.Issues[i].Title = *opts.Title
			}
			if opts.State != nil {
				s.Issues[i].State = *opts.State
			}
			if len(opts.Labels) > 0 {
				s.Issues[i].Labels = parseLabels(opts.Labels)
			}
			if len(opts.AddLabels) > 0 {
				s.AddLabelCalls = append(s.AddLabelCalls, opts.AddLabels)
				for _, l := range opts.AddLabels {
					scope, name := splitLabel(l)
					label := forge.Label{Scope: scope, Name: name}
					if !hasLabel(s.Issues[i].Labels, label) {
						s.Issues[i].Labels = append(s.Issues[i].Labels, label)
					}
				}
			}
			if len(opts.RemoveLabels) > 0 {
				s.RemoveLabelCalls = append(s.RemoveLabelCalls, opts.RemoveLabels)
				for _, l := range opts.RemoveLabels {
					scope, name := splitLabel(l)
					s.Issues[i].Labels = removeLabel(s.Issues[i].Labels, forge.Label{Scope: scope, Name: name})
				}
			}
			return &s.Issues[i], nil
		}
	}
	return nil, forge.NewBaseError("not found", "Run \"anvil issue list\"")
}

// hasLabel checks whether a label exists in the slice (matched by Scope and Name).
func hasLabel(labels []forge.Label, target forge.Label) bool {
	for _, l := range labels {
		if l.Scope == target.Scope && l.Name == target.Name {
			return true
		}
	}
	return false
}

// removeLabel removes a label from the slice (matched by Scope and Name).
// If the label doesn't exist, returns the slice unchanged (idempotent no-op).
func removeLabel(labels []forge.Label, target forge.Label) []forge.Label {
	for i, l := range labels {
		if l.Scope == target.Scope && l.Name == target.Name {
			return append(labels[:i], labels[i+1:]...)
		}
	}
	return labels
}

func (s *FakeIssueService) Close(ctx context.Context, opts forge.IssueCloseOptions) (*forge.Issue, error) {
	s.LastCloseOpts = opts
	if s.CloseFn != nil {
		return s.CloseFn(ctx, opts)
	}
	for i, iss := range s.Issues {
		if iss.Number == opts.Number {
			s.Issues[i].State = "closed"
			return &s.Issues[i], nil
		}
	}
	return &forge.Issue{Number: opts.Number, State: "closed"}, nil
}

func (s *FakeIssueService) Reopen(ctx context.Context, opts forge.IssueReopenOptions) (*forge.Issue, error) {
	s.LastReopenOpts = opts
	if s.ReopenFn != nil {
		return s.ReopenFn(ctx, opts)
	}
	for i, iss := range s.Issues {
		if iss.Number == opts.Number {
			s.Issues[i].State = "open"
			return &s.Issues[i], nil
		}
	}
	return &forge.Issue{Number: opts.Number, State: "open"}, nil
}

// ---- Fake LabelService ----

// FakeLabelService is a state-based fake of forge.LabelService.
type FakeLabelService struct {
	Labels  []forge.Label
	ListErr error

	ListFn   func(ctx context.Context, opts forge.LabelListOptions) ([]forge.Label, error)
	CreateFn func(ctx context.Context, opts forge.LabelCreateOptions) (*forge.Label, error)
	UpdateFn func(ctx context.Context, opts forge.LabelUpdateOptions) (*forge.Label, error)
	DeleteFn func(ctx context.Context, opts forge.LabelDeleteOptions) error

	LastListOpts   forge.LabelListOptions
	LastCreateOpts forge.LabelCreateOptions
	LastUpdateOpts forge.LabelUpdateOptions
	LastDeleteOpts forge.LabelDeleteOptions
}

var _ forge.LabelService = (*FakeLabelService)(nil)

func (s *FakeLabelService) List(ctx context.Context, opts forge.LabelListOptions) ([]forge.Label, error) {
	s.LastListOpts = opts
	if s.ListFn != nil {
		return s.ListFn(ctx, opts)
	}
	if s.ListErr != nil {
		return nil, s.ListErr
	}
	if s.Labels == nil {
		return []forge.Label{}, nil
	}
	return s.Labels, nil
}

func (s *FakeLabelService) Create(ctx context.Context, opts forge.LabelCreateOptions) (*forge.Label, error) {
	s.LastCreateOpts = opts
	if s.CreateFn != nil {
		return s.CreateFn(ctx, opts)
	}
	label := forge.Label{
		Name:        opts.Name,
		Scope:       forge.StringVal(opts.Scope),
		Color:       forge.StringVal(opts.Color),
		Description: forge.StringVal(opts.Description),
		Exclusive:   ptrBool(opts.Exclusive),
	}
	s.Labels = append(s.Labels, label)
	return &label, nil
}

func (s *FakeLabelService) Update(ctx context.Context, opts forge.LabelUpdateOptions) (*forge.Label, error) {
	s.LastUpdateOpts = opts
	if s.UpdateFn != nil {
		return s.UpdateFn(ctx, opts)
	}
	for i, l := range s.Labels {
		if l.Name == opts.Name && l.Scope == opts.Scope {
			if opts.NewName != nil {
				s.Labels[i].Name = *opts.NewName
			}
			if opts.NewScope != nil {
				s.Labels[i].Scope = *opts.NewScope
			}
			if opts.Color != nil {
				s.Labels[i].Color = *opts.Color
			}
			if opts.Description != nil {
				s.Labels[i].Description = *opts.Description
			}
			if opts.Exclusive != nil {
				s.Labels[i].Exclusive = *opts.Exclusive
			}
			return &s.Labels[i], nil
		}
	}
	return nil, forge.NewBaseError("label not found", "Run \"anvil label list\"")
}

func (s *FakeLabelService) Delete(ctx context.Context, opts forge.LabelDeleteOptions) error {
	s.LastDeleteOpts = opts
	if s.DeleteFn != nil {
		return s.DeleteFn(ctx, opts)
	}
	for i, l := range s.Labels {
		if l.Name == opts.Name && l.Scope == opts.Scope {
			s.Labels = append(s.Labels[:i], s.Labels[i+1:]...)
			return nil
		}
	}
	return nil
}

// ---- Fake PRService ----

// FakePRService is a state-based fake of forge.PRService.
type FakePRService struct {
	PRs     []forge.PR
	ListErr error
	GetErr  error

	ListFn   func(ctx context.Context, opts forge.PRListOptions) ([]forge.PR, *forge.ListMeta, error)
	GetFn    func(ctx context.Context, opts forge.PRGetOptions) (*forge.PR, error)
	CreateFn func(ctx context.Context, opts forge.PRCreateOptions) (*forge.PR, error)
	UpdateFn func(ctx context.Context, opts forge.PRUpdateOptions) (*forge.PR, error)
	MergeFn  func(ctx context.Context, opts forge.PRMergeOptions) (*forge.PR, error)
	CloseFn  func(ctx context.Context, opts forge.PRCloseOptions) (*forge.PR, error)

	LastListOpts   forge.PRListOptions
	LastGetOpts    forge.PRGetOptions
	LastCreateOpts forge.PRCreateOptions
	LastMergeOpts  forge.PRMergeOptions
	LastUpdateOpts []forge.PRUpdateOptions

	NextNumber int
}

var _ forge.PRService = (*FakePRService)(nil)

func (s *FakePRService) List(ctx context.Context, opts forge.PRListOptions) ([]forge.PR, *forge.ListMeta, error) {
	s.LastListOpts = opts
	if s.ListFn != nil {
		return s.ListFn(ctx, opts)
	}
	if s.ListErr != nil {
		return nil, nil, s.ListErr
	}
	prs := s.PRs
	if prs == nil {
		prs = []forge.PR{}
	}
	if opts.State != "" && opts.State != "all" {
		filtered := make([]forge.PR, 0, len(prs))
		for _, p := range prs {
			if opts.State == "merged" {
				if p.State == "merged" {
					filtered = append(filtered, p)
				}
			} else if p.State == opts.State {
				filtered = append(filtered, p)
			}
		}
		prs = filtered
	}
	meta := &forge.ListMeta{Total: len(prs), Count: len(prs)}
	return prs, meta, nil
}

func (s *FakePRService) Get(ctx context.Context, opts forge.PRGetOptions) (*forge.PR, error) {
	s.LastGetOpts = opts
	if s.GetFn != nil {
		return s.GetFn(ctx, opts)
	}
	if s.GetErr != nil {
		return nil, s.GetErr
	}
	for i, p := range s.PRs {
		if p.Number == opts.Number {
			return &s.PRs[i], nil
		}
	}
	return nil, forge.NewBaseError("not found", "Run \"anvil pr list\"")
}

func (s *FakePRService) Create(ctx context.Context, opts forge.PRCreateOptions) (*forge.PR, error) {
	s.LastCreateOpts = opts
	if s.CreateFn != nil {
		return s.CreateFn(ctx, opts)
	}
	if s.NextNumber == 0 {
		s.NextNumber = 100
	}
	num := s.NextNumber
	s.NextNumber++

	title := ""
	if opts.Title != nil {
		title = *opts.Title
	}
	headRef := ""
	if opts.HeadRef != nil {
		headRef = *opts.HeadRef
	}
	baseRef := ""
	if opts.BaseRef != nil {
		baseRef = *opts.BaseRef
	}
	pr := forge.PR{
		Number:  num,
		Title:   title,
		State:   "open",
		HeadRef: headRef,
		BaseRef: baseRef,
		Author:  "test-user",
		URL:     fmt.Sprintf("https://github.com/test/repo/pull/%d", num),
	}
	if opts.Draft != nil && *opts.Draft {
		pr.Extras = map[string]any{"draft": true}
	}
	s.PRs = append(s.PRs, pr)
	return &pr, nil
}

func (s *FakePRService) Update(ctx context.Context, opts forge.PRUpdateOptions) (*forge.PR, error) {
	s.LastUpdateOpts = append(s.LastUpdateOpts, opts)
	if s.UpdateFn != nil {
		return s.UpdateFn(ctx, opts)
	}
	for i, p := range s.PRs {
		if p.Number == opts.Number {
			if opts.Title != nil {
				s.PRs[i].Title = *opts.Title
			}
			return &s.PRs[i], nil
		}
	}
	return nil, forge.NewBaseError("not found", "Run \"anvil pr list\"")
}

func (s *FakePRService) Merge(ctx context.Context, opts forge.PRMergeOptions) (*forge.PR, error) {
	s.LastMergeOpts = opts
	if s.MergeFn != nil {
		return s.MergeFn(ctx, opts)
	}
	for i, p := range s.PRs {
		if p.Number == opts.Number {
			s.PRs[i].State = "merged"
			return &s.PRs[i], nil
		}
	}
	return nil, forge.NewBaseError("not found", "Run \"anvil pr list\"")
}

func (s *FakePRService) Close(ctx context.Context, opts forge.PRCloseOptions) (*forge.PR, error) {
	if s.CloseFn != nil {
		return s.CloseFn(ctx, opts)
	}
	for i, p := range s.PRs {
		if p.Number == opts.Number {
			s.PRs[i].State = "closed"
			return &s.PRs[i], nil
		}
	}
	return nil, forge.NewBaseError("not found", "Run \"anvil pr list\"")
}

// ---- helpers ----

func ptrBool(b *bool) bool {
	if b == nil {
		return false
	}
	return *b
}

func parseLabels(names []string) []forge.Label {
	var labels []forge.Label
	for _, n := range names {
		// Simple colon-split: "kind:bug" → scope=kind, name=bug
		scope, name := splitLabel(n)
		labels = append(labels, forge.Label{Name: name, Scope: scope})
	}
	return labels
}

func splitLabel(raw string) (scope, name string) {
	for i := 0; i < len(raw); i++ {
		if raw[i] == ':' {
			return raw[:i], raw[i+1:]
		}
	}
	return "", raw
}

// ---- Fake CommentService ----

// FakeCommentService is a state-based fake of forge.CommentService.
// Comments stored in the Comments slice behave like real forge data.
// Individual methods can be overridden by setting their ...Fn fields.
type FakeCommentService struct {
	Comments  []forge.Comment
	ListErr   error
	GetErr    error
	DeleteErr error

	ListFn   func(ctx context.Context, opts forge.CommentListOptions) ([]forge.Comment, error)
	GetFn    func(ctx context.Context, opts forge.CommentGetOptions) (*forge.Comment, error)
	CreateFn func(ctx context.Context, opts forge.CommentCreateOptions) (*forge.Comment, error)
	UpdateFn func(ctx context.Context, opts forge.CommentUpdateOptions) (*forge.Comment, error)
	DeleteFn func(ctx context.Context, opts forge.CommentDeleteOptions) error

	LastListOpts   forge.CommentListOptions
	LastGetOpts    forge.CommentGetOptions
	LastCreateOpts forge.CommentCreateOptions
	LastUpdateOpts forge.CommentUpdateOptions
	LastDeleteOpts forge.CommentDeleteOptions

	// nextID is an auto-increment counter for comment IDs.
	nextID int
}

var _ forge.CommentService = (*FakeCommentService)(nil)

func (s *FakeCommentService) List(ctx context.Context, opts forge.CommentListOptions) ([]forge.Comment, error) {
	s.LastListOpts = opts
	if s.ListFn != nil {
		return s.ListFn(ctx, opts)
	}
	if s.ListErr != nil {
		return nil, s.ListErr
	}
	if !opts.IncludeSystem {
		filtered := make([]forge.Comment, 0, len(s.Comments))
		for _, c := range s.Comments {
			if !c.System {
				filtered = append(filtered, c)
			}
		}
		return filtered, nil
	}
	return s.Comments, nil
}

func (s *FakeCommentService) Get(ctx context.Context, opts forge.CommentGetOptions) (*forge.Comment, error) {
	s.LastGetOpts = opts
	if s.GetFn != nil {
		return s.GetFn(ctx, opts)
	}
	if s.GetErr != nil {
		return nil, s.GetErr
	}
	for i, c := range s.Comments {
		if c.ID == opts.CommentID {
			return &s.Comments[i], nil
		}
	}
	return nil, forge.NewBaseError("comment not found", "Run \"anvil issue comment list <number>\"")
}

func (s *FakeCommentService) Create(ctx context.Context, opts forge.CommentCreateOptions) (*forge.Comment, error) {
	s.LastCreateOpts = opts
	if s.CreateFn != nil {
		return s.CreateFn(ctx, opts)
	}
	s.nextID++
	if s.nextID == 0 {
		s.nextID = 1
	}
	c := forge.Comment{
		ID:        s.nextID,
		Body:      opts.Body,
		Author:    "test-user",
		System:    false,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		URL:       fmt.Sprintf("https://github.com/test/repo/issues/%d#comment-%d", opts.IssueNumber, s.nextID),
	}
	s.Comments = append(s.Comments, c)
	return &s.Comments[len(s.Comments)-1], nil
}

func (s *FakeCommentService) Update(ctx context.Context, opts forge.CommentUpdateOptions) (*forge.Comment, error) {
	s.LastUpdateOpts = opts
	if s.UpdateFn != nil {
		return s.UpdateFn(ctx, opts)
	}
	for i, c := range s.Comments {
		if c.ID == opts.CommentID {
			s.Comments[i].Body = opts.Body
			s.Comments[i].UpdatedAt = time.Now()
			return &s.Comments[i], nil
		}
	}
	return nil, forge.NewBaseError("comment not found", "Run \"anvil issue comment list <number>\"")
}

func (s *FakeCommentService) Delete(ctx context.Context, opts forge.CommentDeleteOptions) error {
	s.LastDeleteOpts = opts
	if s.DeleteFn != nil {
		return s.DeleteFn(ctx, opts)
	}
	if s.DeleteErr != nil {
		return s.DeleteErr
	}
	for i, c := range s.Comments {
		if c.ID == opts.CommentID {
			s.Comments = append(s.Comments[:i], s.Comments[i+1:]...)
			return nil
		}
	}
	return nil
}
