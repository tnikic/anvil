// Package github implements the forge.Forge interface backed by google/go-github.
// It maps GitHub REST API responses to the normalized domain types defined in
// the parent forge package, translating GitHub's scoped-label naming convention
// (scope:name) to the two-argument model used at the CLI boundary.
package github

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	gh "github.com/google/go-github/v90/github"
	"github.com/tnikic/anvil/internal/forge"
)

// compile-time check
var _ forge.Forge = (*Forge)(nil)

// Forge is a GitHub implementation of the forge.Forge interface.
type Forge struct {
	client *gh.Client
	host   string
	owner  string
	repo   string

	issueSvc    *issueService
	labelSvc    *labelService
	prSvc       *prService
	relationSvc *forge.RelationGuard
	commentSvc  *commentService
}

// New creates a new GitHub Forge adapter.
// host is the forge host (e.g., "github.com", or "http://127.0.0.1:8080" for testing).
// owner and repo identify the repository.
// httpClient is used for authentication — attach a token via an
// oauth2.StaticTokenSource transport or set the Authorization header.
// For GitHub Enterprise, host must be the enterprise domain and New will
// configure the API base URL accordingly.
func New(host, owner, repo string, httpClient *http.Client) *Forge {
	c, err := newGHClient(host, httpClient)
	if err != nil {
		// Configuration error — invalid enterprise URLs. This is a programmer
		// error akin to url.Parse failures in the old code, which were silently
		// discarded. Panic to fail fast.
		panic("github.New: " + err.Error())
	}
	f := &Forge{client: c, host: host, owner: owner, repo: repo}
	f.issueSvc = &issueService{forge: f}
	f.labelSvc = &labelService{forge: f}
	f.prSvc = &prService{forge: f}
	f.relationSvc = newRelationGuard(f)
	f.commentSvc = &commentService{forge: f}
	return f
}

// newGHClient creates a go-github Client, configuring enterprise URLs when
// host is not the public github.com.
func newGHClient(host string, httpClient *http.Client) (*gh.Client, error) {
	if host == "" || host == "github.com" {
		return gh.NewClient(gh.WithHTTPClient(httpClient))
	}
	scheme, cleanHost := forge.ParseHost(host)
	baseURL := scheme + "://" + cleanHost + "/api/v3/"
	uploadURL := scheme + "://" + cleanHost + "/api/uploads/"
	return gh.NewClient(
		gh.WithHTTPClient(httpClient),
		gh.WithEnterpriseURLs(baseURL, uploadURL),
	)
}

// Issues returns the IssueService for this forge.
func (f *Forge) Issues() forge.IssueService { return f.issueSvc }

// Labels returns the LabelService for this forge.
func (f *Forge) Labels() forge.LabelService { return f.labelSvc }

// PRs returns the PRService for this forge.
func (f *Forge) PRs() forge.PRService { return f.prSvc }

// Relations returns the RelationService for this forge.
func (f *Forge) Relations() forge.RelationService { return f.relationSvc }

// Comments returns the CommentService for this forge.
func (f *Forge) Comments() forge.CommentService { return f.commentSvc }

// CurrentUser returns the login of the authenticated user via GET /user.
func (f *Forge) CurrentUser(ctx context.Context) (string, error) {
	user, _, err := f.client.Users.Get(ctx, "")
	if err != nil {
		return "", f.translateError("", err)
	}
	if user == nil || user.Login == nil {
		return "", forge.NewBaseError(
			"could not determine current user",
			"Verify that your token is valid",
		)
	}
	return *user.Login, nil
}

// ---- Error translation ----

// translateError converts a GitHub API or network error into a user-facing
// error message with an actionable help hint.
// resource describes what was being accessed (e.g., "issue #42", "PR #10").
// When empty, a generic message is used.
func (f *Forge) translateError(resource string, err error) error {
	// Check for go-github rate limit errors first (they wrap before ErrorResponse)
	var rateLimitErr *gh.RateLimitError
	if errors.As(err, &rateLimitErr) {
		msg := fmt.Sprintf("rate limit exceeded for %s", f.host)
		help := "Retry after rate limit window resets"
		if rateLimitErr.Rate.Reset.After(time.Now()) {
			help = fmt.Sprintf("Retry after %s", rateLimitErr.Rate.Reset.Format(time.RFC3339))
		}
		return forge.NewBaseError(msg, help)
	}

	// Check for GitHub error response
	if ghErr, ok := err.(*gh.ErrorResponse); ok {
		if fe := forge.TranslateHTTPError(ghErr.Response.StatusCode, f.host, f.owner, f.repo, resource); fe != nil {
			return fe
		}
		return err
	}

	// Network / context errors
	if ne := forge.TranslateNetworkError(err, f.host); ne != nil {
		return ne
	}
	return err
}

// ---- Issue mapping ----

func mapIssue(ghIssue *gh.Issue) *forge.Issue {
	if ghIssue == nil {
		return nil
	}
	issue := &forge.Issue{
		Number:    ghIssue.GetNumber(),
		Title:     ghIssue.GetTitle(),
		State:     ghIssue.GetState(),
		Body:      ghIssue.GetBody(),
		Author:    ghIssue.GetUser().GetLogin(),
		CreatedAt: ghIssue.GetCreatedAt().Time,
		UpdatedAt: ghIssue.GetUpdatedAt().Time,
		URL:       ghIssue.GetHTMLURL(),
	}
	for _, l := range ghIssue.Labels {
		scope, name := forge.ParseLabelScope(l.GetName(), ":")
		issue.Labels = append(issue.Labels, forge.Label{
			Name:        name,
			Scope:       scope,
			Color:       l.GetColor(),
			Description: l.GetDescription(),
		})
	}
	return issue
}

func mapIssues(ghIssues []*gh.Issue) []forge.Issue {
	out := make([]forge.Issue, 0, len(ghIssues))
	for _, i := range ghIssues {
		if m := mapIssue(i); m != nil {
			out = append(out, *m)
		}
	}
	return out
}

// ---- Label mapping ----

func mapLabel(ghLabel *gh.Label) forge.Label {
	scope, name := forge.ParseLabelScope(ghLabel.GetName(), ":")
	return forge.Label{
		Name:        name,
		Scope:       scope,
		Color:       ghLabel.GetColor(),
		Description: ghLabel.GetDescription(),
	}
}

func mapLabels(ghLabels []*gh.Label) []forge.Label {
	out := make([]forge.Label, 0, len(ghLabels))
	for _, l := range ghLabels {
		out = append(out, mapLabel(l))
	}
	return out
}

// ---- PR mapping ----

func mapPR(ghPR *gh.PullRequest) *forge.PR {
	if ghPR == nil {
		return nil
	}
	pr := &forge.PR{
		Number:    ghPR.GetNumber(),
		Title:     ghPR.GetTitle(),
		State:     ghPR.GetState(),
		Body:      ghPR.GetBody(),
		BaseRef:   ghPR.GetBase().GetRef(),
		HeadRef:   ghPR.GetHead().GetRef(),
		Author:    ghPR.GetUser().GetLogin(),
		CreatedAt: ghPR.GetCreatedAt().Time,
		UpdatedAt: ghPR.GetUpdatedAt().Time,
		URL:       ghPR.GetHTMLURL(),
	}
	if ghPR.GetDraft() {
		pr.Extras = map[string]any{"draft": true}
	}
	return pr
}

func mapPRs(ghPRs []*gh.PullRequest) []forge.PR {
	out := make([]forge.PR, 0, len(ghPRs))
	for _, p := range ghPRs {
		if m := mapPR(p); m != nil {
			out = append(out, *m)
		}
	}
	return out
}

// ---- Issue service ----

type issueService struct {
	forge *Forge
}

func (s *issueService) List(ctx context.Context, opts forge.IssueListOptions) ([]forge.Issue, *forge.ListMeta, error) {
	perPage, limit := forge.ListPerPage(opts.Limit)

	ghOpts := &gh.IssueListByRepoOptions{
		State:     opts.State,
		Labels:    opts.Labels,
		Sort:      opts.Sort,
		Direction: opts.Direction,
		ListOptions: gh.ListOptions{
			PerPage: perPage,
		},
	}

	allIssues, err := forge.Paginate(limit, func(page int) (forge.Page[forge.Issue], error) {
		ghOpts.ListOptions.Page = page
		ghIssues, resp, err := s.forge.client.Issues.ListByRepo(ctx, s.forge.owner, s.forge.repo, ghOpts)
		if err != nil {
			return forge.Page[forge.Issue]{}, s.forge.translateError("", err)
		}
		return forge.Page[forge.Issue]{Items: mapIssues(ghIssues), NextPage: resp.NextPage}, nil
	})
	if err != nil {
		return nil, nil, err
	}

	return allIssues, forge.NewListMeta(len(allIssues)), nil
}

func (s *issueService) Get(ctx context.Context, opts forge.IssueGetOptions) (*forge.Issue, error) {
	ghIssue, _, err := s.forge.client.Issues.Get(ctx, s.forge.owner, s.forge.repo, opts.Number)
	if err != nil {
		return nil, s.forge.translateError(fmt.Sprintf("issue #%d", opts.Number), err)
	}
	return mapIssue(ghIssue), nil
}

func (s *issueService) Create(ctx context.Context, opts forge.IssueCreateOptions) (*forge.Issue, error) {
	req := gh.CreateIssueRequest{
		Title: forge.StringVal(opts.Title),
	}
	if opts.Body != nil {
		req.Body = opts.Body
	}
	if len(opts.Labels) > 0 {
		req.Labels = opts.Labels
	}
	if len(opts.Assignees) > 0 {
		req.Assignees = opts.Assignees
	}
	ghIssue, _, err := s.forge.client.Issues.Create(ctx, s.forge.owner, s.forge.repo, req)
	if err != nil {
		return nil, s.forge.translateError("", err)
	}
	return mapIssue(ghIssue), nil
}

func (s *issueService) Update(ctx context.Context, opts forge.IssueUpdateOptions) (*forge.Issue, error) {
	req := gh.UpdateIssueRequest{
		Title: opts.Title,
		Body:  opts.Body,
		State: opts.State,
	}
	if len(opts.Assignees) > 0 {
		req.Assignees = opts.Assignees
	}

	// Handle label operations.
	// If --label (replace-all) is set, use it directly.
	// If --add-label/--remove-label are used, fetch current labels, compute
	// the new set, and patch with the combined list.
	hasIncremental := len(opts.AddLabels) > 0 || len(opts.RemoveLabels) > 0
	if len(opts.Labels) > 0 {
		req.Labels = opts.Labels
	} else if hasIncremental {
		// Fetch current issue to get existing labels.
		current, _, err := s.forge.client.Issues.Get(ctx, s.forge.owner, s.forge.repo, opts.Number)
		if err != nil {
			return nil, s.forge.translateError(fmt.Sprintf("issue #%d", opts.Number), err)
		}

		// Build the current label name set.
		labelSet := make(map[string]bool)
		for _, l := range current.Labels {
			labelSet[l.GetName()] = true
		}

		// Apply removals (idempotent no-op for absent labels).
		for _, name := range opts.RemoveLabels {
			delete(labelSet, name)
		}

		// Apply additions (idempotent).
		for _, name := range opts.AddLabels {
			labelSet[name] = true
		}

		// Convert map keys to a slice.
		allLabels := make([]string, 0, len(labelSet))
		for name := range labelSet {
			allLabels = append(allLabels, name)
		}
		req.Labels = allLabels
	}

	ghIssue, _, err := s.forge.client.Issues.Update(ctx, s.forge.owner, s.forge.repo, opts.Number, req)
	if err != nil {
		return nil, s.forge.translateError(fmt.Sprintf("issue #%d", opts.Number), err)
	}
	return mapIssue(ghIssue), nil
}

func (s *issueService) Close(ctx context.Context, opts forge.IssueCloseOptions) (*forge.Issue, error) {
	state := "closed"
	req := gh.UpdateIssueRequest{State: &state}
	ghIssue, _, err := s.forge.client.Issues.Update(ctx, s.forge.owner, s.forge.repo, opts.Number, req)
	if err != nil {
		return nil, s.forge.translateError(fmt.Sprintf("issue #%d", opts.Number), err)
	}
	// Idempotent: if already closed, go-github doesn't error
	return mapIssue(ghIssue), nil
}

func (s *issueService) Reopen(ctx context.Context, opts forge.IssueReopenOptions) (*forge.Issue, error) {
	state := "open"
	stateReason := "reopened"
	req := gh.UpdateIssueRequest{State: &state, StateReason: &stateReason}
	ghIssue, _, err := s.forge.client.Issues.Update(ctx, s.forge.owner, s.forge.repo, opts.Number, req)
	if err != nil {
		return nil, s.forge.translateError(fmt.Sprintf("issue #%d", opts.Number), err)
	}
	return mapIssue(ghIssue), nil
}

// ---- Label service ----

type labelService struct {
	forge *Forge
}

func (s *labelService) List(ctx context.Context, opts forge.LabelListOptions) ([]forge.Label, error) {
	perPage, limit := forge.ListPerPage(opts.Limit)

	ghOpts := &gh.ListOptions{
		PerPage: perPage,
	}

	return forge.Paginate(limit, func(page int) (forge.Page[forge.Label], error) {
		ghOpts.Page = page
		ghLabels, resp, err := s.forge.client.Issues.ListLabels(ctx, s.forge.owner, s.forge.repo, ghOpts)
		if err != nil {
			return forge.Page[forge.Label]{}, s.forge.translateError("", err)
		}
		return forge.Page[forge.Label]{Items: mapLabels(ghLabels), NextPage: resp.NextPage}, nil
	})
}

func (s *labelService) Create(ctx context.Context, opts forge.LabelCreateOptions) (*forge.Label, error) {
	fullName := forge.LabelFullName(forge.StringVal(opts.Scope), opts.Name, ":")
	req := gh.CreateIssueLabelRequest{
		Name:        fullName,
		Color:       opts.Color,
		Description: opts.Description,
	}
	created, _, err := s.forge.client.Issues.CreateLabel(ctx, s.forge.owner, s.forge.repo, req)
	if err != nil {
		return nil, s.forge.translateError("", err)
	}
	l := mapLabel(created)
	return &l, nil
}

func (s *labelService) Update(ctx context.Context, opts forge.LabelUpdateOptions) (*forge.Label, error) {
	oldFullName := forge.LabelFullName(opts.Scope, opts.Name, ":")
	req := gh.UpdateIssueLabelRequest{
		Color:       opts.Color,
		Description: opts.Description,
	}
	if opts.NewName != nil || opts.NewScope != nil {
		name := opts.Name
		scope := opts.Scope
		if opts.NewName != nil {
			name = *opts.NewName
		}
		if opts.NewScope != nil {
			scope = *opts.NewScope
		}
		newFullName := forge.LabelFullName(scope, name, ":")
		req.NewName = &newFullName
	}
	updated, _, err := s.forge.client.Issues.UpdateLabel(ctx, s.forge.owner, s.forge.repo, oldFullName, req)
	if err != nil {
		return nil, s.forge.translateError(fmt.Sprintf("label %q", oldFullName), err)
	}
	l := mapLabel(updated)
	return &l, nil
}

func (s *labelService) Delete(ctx context.Context, opts forge.LabelDeleteOptions) error {
	fullName := forge.LabelFullName(opts.Scope, opts.Name, ":")
	_, err := s.forge.client.Issues.DeleteLabel(ctx, s.forge.owner, s.forge.repo, fullName)
	if err != nil {
		return s.forge.translateError(fmt.Sprintf("label %q", fullName), err)
	}
	return nil
}

// ---- PR service ----

type prService struct {
	forge *Forge
}

func (s *prService) List(ctx context.Context, opts forge.PRListOptions) ([]forge.PR, *forge.ListMeta, error) {
	perPage, limit := forge.ListPerPage(opts.Limit)

	ghOpts := &gh.PullRequestListOptions{
		State:     opts.State,
		Sort:      opts.Sort,
		Direction: opts.Direction,
		ListOptions: gh.ListOptions{
			PerPage: perPage,
		},
	}

	allPRs, err := forge.Paginate(limit, func(page int) (forge.Page[forge.PR], error) {
		ghOpts.Page = page
		ghPRs, resp, err := s.forge.client.PullRequests.List(ctx, s.forge.owner, s.forge.repo, ghOpts)
		if err != nil {
			return forge.Page[forge.PR]{}, s.forge.translateError("", err)
		}
		return forge.Page[forge.PR]{Items: mapPRs(ghPRs), NextPage: resp.NextPage}, nil
	})
	if err != nil {
		return nil, nil, err
	}

	return allPRs, forge.NewListMeta(len(allPRs)), nil
}

func (s *prService) Get(ctx context.Context, opts forge.PRGetOptions) (*forge.PR, error) {
	ghPR, _, err := s.forge.client.PullRequests.Get(ctx, s.forge.owner, s.forge.repo, opts.Number)
	if err != nil {
		return nil, s.forge.translateError(fmt.Sprintf("PR #%d", opts.Number), err)
	}
	pr := mapPR(ghPR)

	// Fetch reviews (best-effort — don't fail the view if this errors)
	reviews, _, revErr := s.forge.client.PullRequests.ListReviews(ctx, s.forge.owner, s.forge.repo, opts.Number, nil)
	if revErr == nil {
		seen := make(map[string]string) // login → latest state
		for _, r := range reviews {
			if r.GetState() != "" {
				seen[r.GetUser().GetLogin()] = r.GetState()
			}
		}
		for login, state := range seen {
			pr.Reviewers = append(pr.Reviewers, forge.ReviewState{Login: login, State: state})
		}
	}

	// Fetch check runs for the head SHA (best-effort)
	if ghPR.GetHead().GetSHA() != "" {
		checks, _, checkErr := s.forge.client.Checks.ListCheckRunsForRef(ctx, s.forge.owner, s.forge.repo, ghPR.GetHead().GetSHA(), nil)
		if checkErr == nil && checks.GetTotal() > 0 {
			total := checks.GetTotal()
			passed := 0
			for _, c := range checks.CheckRuns {
				if c.GetConclusion() == "success" || c.GetConclusion() == "neutral" || c.GetConclusion() == "skipped" {
					passed++
				}
			}
			pr.Checks = &forge.CheckSummary{Passed: passed, Total: total}
		}
	}

	return pr, nil
}

func (s *prService) Create(ctx context.Context, opts forge.PRCreateOptions) (*forge.PR, error) {
	req := gh.CreatePullRequest{
		Head: forge.StringVal(opts.HeadRef),
		Base: forge.StringVal(opts.BaseRef),
	}
	if opts.Title != nil {
		req.Title = opts.Title
	}
	if opts.Body != nil {
		req.Body = opts.Body
	}
	if opts.Draft != nil {
		req.Draft = opts.Draft
	}
	ghPR, _, err := s.forge.client.PullRequests.Create(ctx, s.forge.owner, s.forge.repo, req)
	if err != nil {
		return nil, s.forge.translateError("", err)
	}
	return mapPR(ghPR), nil
}

func (s *prService) Merge(ctx context.Context, opts forge.PRMergeOptions) (*forge.PR, error) {
	commitMsg := ""
	if opts.Title != nil {
		commitMsg = *opts.Title
	}
	_, _, err := s.forge.client.PullRequests.Merge(ctx, s.forge.owner, s.forge.repo, opts.Number, commitMsg, nil)
	if err != nil {
		return nil, s.forge.translateError(fmt.Sprintf("PR #%d", opts.Number), err)
	}
	// After merge, fetch the PR to return its final state (handles race conditions)
	ghPR, _, err := s.forge.client.PullRequests.Get(ctx, s.forge.owner, s.forge.repo, opts.Number)
	if err != nil {
		// If we can't fetch it, return a synthesized merged PR
		return &forge.PR{Number: opts.Number, State: forge.StateMerged}, nil
	}
	return mapPR(ghPR), nil
}

func (s *prService) Update(ctx context.Context, opts forge.PRUpdateOptions) (*forge.PR, error) {
	req := &gh.PullRequest{}
	if opts.Title != nil {
		req.Title = opts.Title
	}
	ghPR, _, err := s.forge.client.PullRequests.Edit(ctx, s.forge.owner, s.forge.repo, opts.Number, req)
	if err != nil {
		return nil, s.forge.translateError(fmt.Sprintf("PR #%d", opts.Number), err)
	}
	return mapPR(ghPR), nil
}

func (s *prService) Close(ctx context.Context, opts forge.PRCloseOptions) (*forge.PR, error) {
	state := "closed"
	req := &gh.PullRequest{State: &state}
	ghPR, _, err := s.forge.client.PullRequests.Edit(ctx, s.forge.owner, s.forge.repo, opts.Number, req)
	if err != nil {
		return nil, s.forge.translateError(fmt.Sprintf("PR #%d", opts.Number), err)
	}
	return mapPR(ghPR), nil
}

// ---- Comment service ----

type commentService struct {
	forge *Forge
}

func mapComment(ghComment *gh.IssueComment) *forge.Comment {
	if ghComment == nil {
		return nil
	}
	c := &forge.Comment{
		ID:        int(ghComment.GetID()),
		Body:      ghComment.GetBody(),
		Author:    ghComment.GetUser().GetLogin(),
		System:    false,
		CreatedAt: ghComment.GetCreatedAt().Time,
		UpdatedAt: ghComment.GetUpdatedAt().Time,
		URL:       ghComment.GetHTMLURL(),
	}
	// Populate reactions summary.
	rx := ghComment.GetReactions()
	if rx != nil {
		c.Reactions = map[string]int{
			"+1":       rx.GetPlusOne(),
			"-1":       rx.GetMinusOne(),
			"laugh":    rx.GetLaugh(),
			"hooray":   rx.GetHooray(),
			"confused": rx.GetConfused(),
			"heart":    rx.GetHeart(),
			"rocket":   rx.GetRocket(),
			"eyes":     rx.GetEyes(),
		}
	}
	return c
}

func mapComments(ghComments []*gh.IssueComment) []forge.Comment {
	out := make([]forge.Comment, 0, len(ghComments))
	for _, c := range ghComments {
		if m := mapComment(c); m != nil {
			out = append(out, *m)
		}
	}
	return out
}

func (s *commentService) List(ctx context.Context, opts forge.CommentListOptions) ([]forge.Comment, error) {
	ghOpts := &gh.IssueListCommentsOptions{
		ListOptions: gh.ListOptions{PerPage: 100},
	}

	return forge.Paginate(0, func(page int) (forge.Page[forge.Comment], error) {
		ghOpts.Page = page
		ghComments, resp, err := s.forge.client.Issues.ListComments(
			ctx, s.forge.owner, s.forge.repo, opts.IssueNumber, ghOpts,
		)
		if err != nil {
			return forge.Page[forge.Comment]{}, s.forge.translateError(fmt.Sprintf("comments for issue #%d", opts.IssueNumber), err)
		}
		return forge.Page[forge.Comment]{Items: mapComments(ghComments), NextPage: resp.NextPage}, nil
	})
}

func (s *commentService) Get(ctx context.Context, opts forge.CommentGetOptions) (*forge.Comment, error) {
	ghComment, _, err := s.forge.client.Issues.GetComment(
		ctx, s.forge.owner, s.forge.repo, int64(opts.CommentID),
	)
	if err != nil {
		return nil, s.forge.translateError(fmt.Sprintf("comment #%d on issue #%d", opts.CommentID, opts.IssueNumber), err)
	}
	return mapComment(ghComment), nil
}

func (s *commentService) Create(ctx context.Context, opts forge.CommentCreateOptions) (*forge.Comment, error) {
	req := &gh.IssueComment{Body: &opts.Body}
	ghComment, _, err := s.forge.client.Issues.CreateComment(
		ctx, s.forge.owner, s.forge.repo, opts.IssueNumber, req,
	)
	if err != nil {
		return nil, s.forge.translateError(fmt.Sprintf("issue #%d", opts.IssueNumber), err)
	}
	return mapComment(ghComment), nil
}

func (s *commentService) Update(ctx context.Context, opts forge.CommentUpdateOptions) (*forge.Comment, error) {
	req := &gh.IssueComment{Body: &opts.Body}
	ghComment, _, err := s.forge.client.Issues.EditComment(
		ctx, s.forge.owner, s.forge.repo, int64(opts.CommentID), req,
	)
	if err != nil {
		return nil, s.forge.translateError(fmt.Sprintf("comment #%d", opts.CommentID), err)
	}
	return mapComment(ghComment), nil
}

func (s *commentService) Delete(ctx context.Context, opts forge.CommentDeleteOptions) error {
	_, err := s.forge.client.Issues.DeleteComment(
		ctx, s.forge.owner, s.forge.repo, int64(opts.CommentID),
	)
	if err != nil {
		return s.forge.translateError(fmt.Sprintf("comment #%d", opts.CommentID), err)
	}
	return nil
}
