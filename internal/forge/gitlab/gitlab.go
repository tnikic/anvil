// Package gitlab implements the forge.Forge interface backed by gitlab.com/gitlab-org/api/client-go.
// It maps GitLab REST API responses to the normalized domain types defined in
// the parent forge package, translating GitLab's scoped-label naming convention
// (scope::name) to the two-argument model used at the CLI boundary.
package gitlab

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/tnikic/anvil/internal/forge"
	gl "gitlab.com/gitlab-org/api/client-go"
)

// compile-time check
var _ forge.Forge = (*Forge)(nil)

// Forge is a GitLab implementation of the forge.Forge interface.
type Forge struct {
	client *gl.Client
	host   string
	owner  string
	repo   string

	issueSvc    *issueService
	labelSvc    *labelService
	prSvc       *prService
	relationSvc *forge.RelationGuard
	commentSvc  *commentService
}

// New creates a new GitLab Forge adapter.
// host is the forge host (e.g., "gitlab.com", or "http://127.0.0.1:8080" for testing).
// owner and repo identify the repository.
// httpClient is used for authentication — the transport should attach the
// authorization header (e.g., Bearer token).
// For self-hosted GitLab instances, the host must be the instance domain and
// New will configure the API base URL to https://<host>/api/v4/.
func New(host, owner, repo string, httpClient *http.Client) *Forge {
	opts := []gl.ClientOptionFunc{
		gl.WithHTTPClient(httpClient),
		gl.WithoutRetries(),
	}

	// Configure base URL for self-hosted GitLab instances.
	if host != "" && host != "gitlab.com" {
		scheme, cleanHost := forge.ParseHost(host)
		opts = append(opts, gl.WithBaseURL(scheme+"://"+cleanHost+"/api/v4/"))
	}

	c, _ := gl.NewClient("", opts...)

	f := &Forge{client: c, host: host, owner: owner, repo: repo}
	f.issueSvc = &issueService{forge: f}
	f.labelSvc = &labelService{forge: f}
	f.prSvc = &prService{forge: f}
	f.relationSvc = newRelationGuard(f)
	f.commentSvc = &commentService{forge: f}
	return f
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

// CurrentUser returns the username of the authenticated user via GET /user.
func (f *Forge) CurrentUser(ctx context.Context) (string, error) {
	user, _, err := f.client.Users.CurrentUser(gl.WithContext(ctx))
	if err != nil {
		return "", f.translateError("", err)
	}
	if user == nil || user.Username == "" {
		return "", forge.NewBaseError(
			"could not determine current user",
			"Verify that your token is valid",
		)
	}
	return user.Username, nil
}

// ---- Error translation ----

// translateError converts a GitLab API or network error into a user-facing
// error message with an actionable help hint.
// resource describes what was being accessed (e.g., "issue #42", "MR #10").
// When empty, a generic message is used.
func (f *Forge) translateError(resource string, err error) error {
	// GitLab returns a sentinel error for 404, not an ErrorResponse.
	if errors.Is(err, gl.ErrNotFound) {
		if resource != "" {
			return forge.NewBaseError(
				fmt.Sprintf("%s not found in %s/%s", resource, f.owner, f.repo),
				"Run \"anvil issue list\" to see available items",
			)
		}
		return forge.NewBaseError(
			"not found (404)",
			"Run \"anvil issue list\" to see available items",
		)
	}

	// Check for GitLab error response
	var glErr *gl.ErrorResponse
	if errors.As(err, &glErr) {
		if fe := forge.TranslateHTTPError(glErr.Response.StatusCode, f.host, f.owner, f.repo, resource); fe != nil {
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

// ---- Assignee resolution ----

// resolveAssigneeIDs looks up numeric user IDs from usernames via the GitLab
// Users API. Returns a StructuredError if any username cannot be resolved.
func (f *Forge) resolveAssigneeIDs(ctx context.Context, usernames []string) ([]int64, error) {
	ids := make([]int64, 0, len(usernames))
	for _, username := range usernames {
		users, _, err := f.client.Users.ListUsers(
			&gl.ListUsersOptions{Username: forge.String(username)},
			gl.WithContext(ctx),
		)
		if err != nil {
			return nil, NewAssigneeError(username, err)
		}
		if len(users) == 0 {
			return nil, NewAssigneeError(username,
				fmt.Errorf("user %q not found", username))
		}
		ids = append(ids, users[0].ID)
	}
	return ids, nil
}

// NewAssigneeError creates a StructuredError for a failed user lookup.
func NewAssigneeError(username string, cause error) error {
	return forge.NewBaseError(
		fmt.Sprintf("cannot assign user %q: %s", username, cause),
		fmt.Sprintf("Verify that %q is a valid username on this GitLab instance", username),
	)
}

// ---- Issue mapping ----

func mapIssue(glIssue *gl.Issue) *forge.Issue {
	if glIssue == nil {
		return nil
	}
	issue := &forge.Issue{
		Number: int(glIssue.IID),
		Title:  glIssue.Title,
		State:  normalizeState(glIssue.State),
		Body:   glIssue.Description,
		URL:    glIssue.WebURL,
	}
	if glIssue.Author != nil {
		issue.Author = glIssue.Author.Username
	}
	if glIssue.CreatedAt != nil {
		issue.CreatedAt = *glIssue.CreatedAt
	}
	if glIssue.UpdatedAt != nil {
		issue.UpdatedAt = *glIssue.UpdatedAt
	}
	for _, labelName := range glIssue.Labels {
		scope, name := forge.ParseLabelScope(labelName, "::")
		issue.Labels = append(issue.Labels, forge.Label{Name: name, Scope: scope})
	}
	return issue
}

func mapIssues(glIssues []*gl.Issue) []forge.Issue {
	out := make([]forge.Issue, 0, len(glIssues))
	for _, i := range glIssues {
		if m := mapIssue(i); m != nil {
			out = append(out, *m)
		}
	}
	return out
}

// normalizeState converts GitLab issue states ("opened", "closed") to
// the normalized form ("open", "closed").
func normalizeState(state string) string {
	if state == "opened" {
		return "open"
	}
	return state
}

// ---- Issue service ----

type issueService struct {
	forge *Forge
}

func (s *issueService) List(ctx context.Context, opts forge.IssueListOptions) ([]forge.Issue, *forge.ListMeta, error) {
	perPage, limit := forge.ListPerPage(opts.Limit)

	glOpts := &gl.ListProjectIssuesOptions{
		State:   stateForList(opts.State),
		OrderBy: forge.String(opts.Sort),
		Sort:    forge.String(opts.Direction),
		ListOptions: gl.ListOptions{
			PerPage: int64(perPage),
		},
	}
	if len(opts.Labels) > 0 {
		l := gl.LabelOptions(opts.Labels)
		glOpts.Labels = &l
	}

	allIssues, err := forge.Paginate(limit, func(page int) (forge.Page[forge.Issue], error) {
		glOpts.Page = int64(page)
		glIssues, resp, err := s.forge.client.Issues.ListProjectIssues(
			s.forge.owner+"/"+s.forge.repo, glOpts,
			gl.WithContext(ctx),
		)
		if err != nil {
			return forge.Page[forge.Issue]{}, s.forge.translateError("", err)
		}
		return forge.Page[forge.Issue]{Items: mapIssues(glIssues), NextPage: int(resp.NextPage)}, nil
	})
	if err != nil {
		return nil, nil, err
	}

	return allIssues, forge.NewListMeta(len(allIssues)), nil
}

func (s *issueService) Get(ctx context.Context, opts forge.IssueGetOptions) (*forge.Issue, error) {
	glIssue, _, err := s.forge.client.Issues.GetIssue(
		s.forge.owner+"/"+s.forge.repo, int64(opts.Number),
		gl.WithContext(ctx),
	)
	if err != nil {
		return nil, s.forge.translateError(fmt.Sprintf("issue #%d", opts.Number), err)
	}
	return mapIssue(glIssue), nil
}

func (s *issueService) Create(ctx context.Context, opts forge.IssueCreateOptions) (*forge.Issue, error) {
	req := &gl.CreateIssueOptions{}
	if opts.Title != nil {
		req.Title = opts.Title
	}
	if opts.Body != nil {
		req.Description = opts.Body
	}
	if len(opts.Labels) > 0 {
		l := gl.LabelOptions(opts.Labels)
		req.Labels = &l
	}
	if len(opts.Assignees) > 0 {
		ids, err := s.forge.resolveAssigneeIDs(ctx, opts.Assignees)
		if err != nil {
			return nil, err
		}
		req.AssigneeIDs = &ids
	}
	glIssue, _, err := s.forge.client.Issues.CreateIssue(
		s.forge.owner+"/"+s.forge.repo, req,
		gl.WithContext(ctx),
	)
	if err != nil {
		return nil, s.forge.translateError("", err)
	}
	return mapIssue(glIssue), nil
}

func (s *issueService) Update(ctx context.Context, opts forge.IssueUpdateOptions) (*forge.Issue, error) {
	req := &gl.UpdateIssueOptions{}
	if opts.Title != nil {
		req.Title = opts.Title
	}
	if opts.Body != nil {
		req.Description = opts.Body
	}
	if opts.State != nil {
		req.StateEvent = stateToEvent(*opts.State)
	}
	if len(opts.Labels) > 0 {
		l := gl.LabelOptions(opts.Labels)
		req.Labels = &l
	}
	if len(opts.AddLabels) > 0 {
		l := gl.LabelOptions(opts.AddLabels)
		req.AddLabels = &l
	}
	if len(opts.RemoveLabels) > 0 {
		l := gl.LabelOptions(opts.RemoveLabels)
		req.RemoveLabels = &l
	}
	if len(opts.Assignees) > 0 {
		ids, err := s.forge.resolveAssigneeIDs(ctx, opts.Assignees)
		if err != nil {
			return nil, err
		}
		req.AssigneeIDs = &ids
	}
	glIssue, _, err := s.forge.client.Issues.UpdateIssue(
		s.forge.owner+"/"+s.forge.repo, int64(opts.Number), req,
		gl.WithContext(ctx),
	)
	if err != nil {
		return nil, s.forge.translateError(fmt.Sprintf("issue #%d", opts.Number), err)
	}
	return mapIssue(glIssue), nil
}

func (s *issueService) Close(ctx context.Context, opts forge.IssueCloseOptions) (*forge.Issue, error) {
	req := &gl.UpdateIssueOptions{
		StateEvent: forge.String("close"),
	}
	glIssue, _, err := s.forge.client.Issues.UpdateIssue(
		s.forge.owner+"/"+s.forge.repo, int64(opts.Number), req,
		gl.WithContext(ctx),
	)
	if err != nil {
		return nil, s.forge.translateError(fmt.Sprintf("issue #%d", opts.Number), err)
	}
	// Idempotent: if already closed, GitLab doesn't error
	return mapIssue(glIssue), nil
}

func (s *issueService) Reopen(ctx context.Context, opts forge.IssueReopenOptions) (*forge.Issue, error) {
	req := &gl.UpdateIssueOptions{
		StateEvent: forge.String("reopen"),
	}
	glIssue, _, err := s.forge.client.Issues.UpdateIssue(
		s.forge.owner+"/"+s.forge.repo, int64(opts.Number), req,
		gl.WithContext(ctx),
	)
	if err != nil {
		return nil, s.forge.translateError(fmt.Sprintf("issue #%d", opts.Number), err)
	}
	return mapIssue(glIssue), nil
}

// stateToEvent maps a normalized state to a GitLab state_event value.
func stateToEvent(state string) *string {
	switch state {
	case "open":
		return forge.String("reopen")
	case "closed":
		return forge.String("close")
	default:
		return nil
	}
}

// ---- Label mapping ----

func mapLabel(glLabel *gl.Label) forge.Label {
	scope, name := forge.ParseLabelScope(glLabel.Name, "::")
	return forge.Label{
		Name:        name,
		Scope:       scope,
		Color:       glLabel.Color,
		Description: glLabel.Description,
	}
}

func mapLabels(glLabels []*gl.Label) []forge.Label {
	out := make([]forge.Label, 0, len(glLabels))
	for _, l := range glLabels {
		out = append(out, mapLabel(l))
	}
	return out
}

// ---- Label service ----

type labelService struct {
	forge *Forge
}

func (s *labelService) List(ctx context.Context, opts forge.LabelListOptions) ([]forge.Label, error) {
	perPage, limit := forge.ListPerPage(opts.Limit)

	glOpts := &gl.ListLabelsOptions{
		ListOptions: gl.ListOptions{
			PerPage: int64(perPage),
		},
	}

	return forge.Paginate(limit, func(page int) (forge.Page[forge.Label], error) {
		glOpts.Page = int64(page)
		glLabels, resp, err := s.forge.client.Labels.ListLabels(
			s.forge.owner+"/"+s.forge.repo, glOpts,
			gl.WithContext(ctx),
		)
		if err != nil {
			return forge.Page[forge.Label]{}, s.forge.translateError("", err)
		}
		return forge.Page[forge.Label]{Items: mapLabels(glLabels), NextPage: int(resp.NextPage)}, nil
	})
}

func (s *labelService) Create(ctx context.Context, opts forge.LabelCreateOptions) (*forge.Label, error) {
	fullName := forge.LabelFullName(forge.StringVal(opts.Scope), opts.Name, "::")
	glLabel, _, err := s.forge.client.Labels.CreateLabel(
		s.forge.owner+"/"+s.forge.repo,
		&gl.CreateLabelOptions{
			Name:        &fullName,
			Color:       opts.Color,
			Description: opts.Description,
		},
		gl.WithContext(ctx),
	)
	if err != nil {
		return nil, s.forge.translateError("", err)
	}
	l := mapLabel(glLabel)
	return &l, nil
}

func (s *labelService) Update(ctx context.Context, opts forge.LabelUpdateOptions) (*forge.Label, error) {
	oldFullName := forge.LabelFullName(opts.Scope, opts.Name, "::")
	var newName *string
	if opts.NewName != nil || opts.NewScope != nil {
		name := opts.Name
		scope := opts.Scope
		if opts.NewName != nil {
			name = *opts.NewName
		}
		if opts.NewScope != nil {
			scope = *opts.NewScope
		}
		full := forge.LabelFullName(scope, name, "::")
		newName = &full
	}
	glLabel, _, err := s.forge.client.Labels.UpdateLabel(
		s.forge.owner+"/"+s.forge.repo,
		nil,
		&gl.UpdateLabelOptions{
			Name:        &oldFullName,
			NewName:     newName,
			Color:       opts.Color,
			Description: opts.Description,
		},
		gl.WithContext(ctx),
	)
	if err != nil {
		return nil, s.forge.translateError(fmt.Sprintf("label %q", oldFullName), err)
	}
	l := mapLabel(glLabel)
	return &l, nil
}

func (s *labelService) Delete(ctx context.Context, opts forge.LabelDeleteOptions) error {
	fullName := forge.LabelFullName(opts.Scope, opts.Name, "::")
	_, err := s.forge.client.Labels.DeleteLabel(
		s.forge.owner+"/"+s.forge.repo,
		nil,
		&gl.DeleteLabelOptions{
			Name: &fullName,
		},
		gl.WithContext(ctx),
	)
	if err != nil {
		return s.forge.translateError(fmt.Sprintf("label %q", fullName), err)
	}
	return nil
}

// ---- MR mapping ----

// mapMR converts a GitLab BasicMergeRequest to a normalized forge.PR.
// GitLab's MergeRequest embeds BasicMergeRequest, so this works for both
// list results (BasicMergeRequest) and get/create/update results (MergeRequest).
func mapMR(glMR *gl.BasicMergeRequest) *forge.PR {
	if glMR == nil {
		return nil
	}
	pr := &forge.PR{
		Number:  int(glMR.IID),
		Title:   glMR.Title,
		State:   normalizeState(glMR.State),
		Body:    glMR.Description,
		HeadRef: glMR.SourceBranch,
		BaseRef: glMR.TargetBranch,
		URL:     glMR.WebURL,
	}
	if glMR.Author != nil {
		pr.Author = glMR.Author.Username
	}
	if glMR.CreatedAt != nil {
		pr.CreatedAt = *glMR.CreatedAt
	}
	if glMR.UpdatedAt != nil {
		pr.UpdatedAt = *glMR.UpdatedAt
	}
	if glMR.Draft {
		if pr.Extras == nil {
			pr.Extras = make(map[string]any)
		}
		pr.Extras["draft"] = true
	}
	return pr
}

func mapMRs(glMRs []*gl.BasicMergeRequest) []forge.PR {
	out := make([]forge.PR, 0, len(glMRs))
	for _, m := range glMRs {
		if pr := mapMR(m); pr != nil {
			out = append(out, *pr)
		}
	}
	return out
}

// ---- PR service ----

type prService struct {
	forge *Forge
}

func (s *prService) List(ctx context.Context, opts forge.PRListOptions) ([]forge.PR, *forge.ListMeta, error) {
	perPage, limit := forge.ListPerPage(opts.Limit)

	glOpts := &gl.ListProjectMergeRequestsOptions{
		State:   stateForList(opts.State),
		OrderBy: forge.String(opts.Sort),
		Sort:    forge.String(opts.Direction),
		ListOptions: gl.ListOptions{
			PerPage: int64(perPage),
		},
	}

	allPRs, err := forge.Paginate(limit, func(page int) (forge.Page[forge.PR], error) {
		glOpts.Page = int64(page)
		glMRs, resp, err := s.forge.client.MergeRequests.ListProjectMergeRequests(
			s.forge.owner+"/"+s.forge.repo, glOpts,
			gl.WithContext(ctx),
		)
		if err != nil {
			return forge.Page[forge.PR]{}, s.forge.translateError("", err)
		}
		return forge.Page[forge.PR]{Items: mapMRs(glMRs), NextPage: int(resp.NextPage)}, nil
	})
	if err != nil {
		return nil, nil, err
	}

	return allPRs, forge.NewListMeta(len(allPRs)), nil
}

func (s *prService) Get(ctx context.Context, opts forge.PRGetOptions) (*forge.PR, error) {
	glMR, _, err := s.forge.client.MergeRequests.GetMergeRequest(
		s.forge.owner+"/"+s.forge.repo, int64(opts.Number), nil,
		gl.WithContext(ctx),
	)
	if err != nil {
		return nil, s.forge.translateError(fmt.Sprintf("MR #%d", opts.Number), err)
	}
	pr := mapMR(&glMR.BasicMergeRequest)

	// Enrich with reviewers from approvals API (best-effort).
	approvals, _, revErr := s.forge.client.MergeRequests.GetMergeRequestApprovals(
		s.forge.owner+"/"+s.forge.repo, int64(opts.Number),
		gl.WithContext(ctx),
	)
	if revErr == nil {
		for _, approver := range approvals.ApprovedBy {
			if approver.User != nil {
				pr.Reviewers = append(pr.Reviewers, forge.ReviewState{
					Login: approver.User.Username,
					State: "APPROVED",
				})
			}
		}
	}

	// Enrich with CI pipeline status from head_pipeline (best-effort).
	// Only report terminal pipeline statuses; skip in-progress pipelines.
	if glMR.HeadPipeline != nil && isTerminalPipelineStatus(glMR.HeadPipeline.Status) {
		total := 1
		passed := 0
		if glMR.HeadPipeline.Status == "success" {
			passed = 1
		}
		pr.Checks = &forge.CheckSummary{Passed: passed, Total: total}
	}

	return pr, nil
}

func (s *prService) Create(ctx context.Context, opts forge.PRCreateOptions) (*forge.PR, error) {
	req := &gl.CreateMergeRequestOptions{}
	title := ""
	if opts.Title != nil {
		title = *opts.Title
	}
	// GitLab uses "Draft: " title prefix convention; the go-gitlab library
	// does not expose the draft boolean on create options in this version.
	if opts.Draft != nil && *opts.Draft && !strings.HasPrefix(title, "Draft:") {
		title = "Draft: " + title
	}
	if title != "" {
		req.Title = &title
	}
	if opts.Body != nil {
		req.Description = opts.Body
	}
	if opts.HeadRef != nil {
		req.SourceBranch = opts.HeadRef
	}
	if opts.BaseRef != nil {
		req.TargetBranch = opts.BaseRef
	}

	glMR, _, err := s.forge.client.MergeRequests.CreateMergeRequest(
		s.forge.owner+"/"+s.forge.repo, req,
		gl.WithContext(ctx),
	)
	if err != nil {
		return nil, s.forge.translateError("", err)
	}
	return mapMR(&glMR.BasicMergeRequest), nil
}

func (s *prService) Update(ctx context.Context, opts forge.PRUpdateOptions) (*forge.PR, error) {
	req := &gl.UpdateMergeRequestOptions{}
	if opts.Title != nil {
		req.Title = opts.Title
	}
	glMR, _, err := s.forge.client.MergeRequests.UpdateMergeRequest(
		s.forge.owner+"/"+s.forge.repo, int64(opts.Number), req,
		gl.WithContext(ctx),
	)
	if err != nil {
		return nil, s.forge.translateError(fmt.Sprintf("MR #%d", opts.Number), err)
	}
	return mapMR(&glMR.BasicMergeRequest), nil
}

func (s *prService) Merge(ctx context.Context, opts forge.PRMergeOptions) (*forge.PR, error) {
	req := &gl.AcceptMergeRequestOptions{}
	if opts.Title != nil {
		req.MergeCommitMessage = opts.Title
	}
	if opts.Body != nil {
		if req.MergeCommitMessage != nil {
			combined := *req.MergeCommitMessage + "\n\n" + *opts.Body
			req.MergeCommitMessage = &combined
		} else {
			req.MergeCommitMessage = opts.Body
		}
	}
	if opts.Method != nil && *opts.Method == "squash" {
		req.Squash = forge.Bool(true)
	}

	glMR, _, err := s.forge.client.MergeRequests.AcceptMergeRequest(
		s.forge.owner+"/"+s.forge.repo, int64(opts.Number), req,
		gl.WithContext(ctx),
	)
	if err != nil {
		return nil, s.forge.translateError(fmt.Sprintf("MR #%d", opts.Number), err)
	}
	return mapMR(&glMR.BasicMergeRequest), nil
}

func (s *prService) Close(ctx context.Context, opts forge.PRCloseOptions) (*forge.PR, error) {
	req := &gl.UpdateMergeRequestOptions{
		StateEvent: forge.String("close"),
	}
	glMR, _, err := s.forge.client.MergeRequests.UpdateMergeRequest(
		s.forge.owner+"/"+s.forge.repo, int64(opts.Number), req,
		gl.WithContext(ctx),
	)
	if err != nil {
		return nil, s.forge.translateError(fmt.Sprintf("MR #%d", opts.Number), err)
	}
	return mapMR(&glMR.BasicMergeRequest), nil
}

// ---- Comment service ----

type commentService struct {
	forge *Forge
}

func (s *commentService) mapComment(glNote *gl.Note) *forge.Comment {
	if glNote == nil {
		return nil
	}
	c := &forge.Comment{
		ID:     int(glNote.ID),
		Body:   glNote.Body,
		System: glNote.System,
		Author: glNote.Author.Username,
	}
	if glNote.CreatedAt != nil {
		c.CreatedAt = *glNote.CreatedAt
	}
	if glNote.UpdatedAt != nil {
		c.UpdatedAt = *glNote.UpdatedAt
	}
	// Build URL from the noteable type and IID.
	if glNote.NoteableType == "Issue" {
		c.URL = fmt.Sprintf("%s/%s/%s/-/issues/%d#note_%d",
			s.hostURL(), s.forge.owner, s.forge.repo, glNote.NoteableIID, glNote.ID)
	}
	return c
}

func (s *commentService) hostURL() string {
	if s.forge.host == "" || s.forge.host == "gitlab.com" {
		return "https://gitlab.com"
	}
	scheme := "https"
	host := s.forge.host
	if strings.HasPrefix(host, "http://") {
		scheme = "http"
		host = strings.TrimPrefix(host, "http://")
	}
	return scheme + "://" + host
}

func (s *commentService) mapComments(glNotes []*gl.Note) []forge.Comment {
	out := make([]forge.Comment, 0, len(glNotes))
	for _, n := range glNotes {
		if m := s.mapComment(n); m != nil {
			out = append(out, *m)
		}
	}
	return out
}

func (s *commentService) List(ctx context.Context, opts forge.CommentListOptions) ([]forge.Comment, error) {
	glOpts := &gl.ListIssueNotesOptions{
		Sort:    forge.String("asc"),
		OrderBy: forge.String("created_at"),
		ListOptions: gl.ListOptions{
			PerPage: 100,
		},
	}

	// Fetch all notes with pagination (GitLab defaults to 20).
	allNotes, err := forge.Paginate(0, func(page int) (forge.Page[*gl.Note], error) {
		glOpts.Page = int64(page)
		notes, resp, err := s.forge.client.Notes.ListIssueNotes(
			s.forge.owner+"/"+s.forge.repo, int64(opts.IssueNumber), glOpts,
			gl.WithContext(ctx),
		)
		if err != nil {
			return forge.Page[*gl.Note]{}, s.forge.translateError(fmt.Sprintf("notes for issue #%d", opts.IssueNumber), err)
		}
		return forge.Page[*gl.Note]{Items: notes, NextPage: int(resp.NextPage)}, nil
	})
	if err != nil {
		return nil, err
	}

	allComments := s.mapComments(allNotes)

	// Filter out system notes by default.
	if !opts.IncludeSystem {
		filtered := make([]forge.Comment, 0, len(allComments))
		for _, c := range allComments {
			if !c.System {
				filtered = append(filtered, c)
			}
		}
		return filtered, nil
	}
	return allComments, nil
}

func (s *commentService) Get(ctx context.Context, opts forge.CommentGetOptions) (*forge.Comment, error) {
	note, _, err := s.forge.client.Notes.GetIssueNote(
		s.forge.owner+"/"+s.forge.repo, int64(opts.IssueNumber), int64(opts.CommentID),
		gl.WithContext(ctx),
	)
	if err != nil {
		return nil, s.forge.translateError(fmt.Sprintf("note #%d on issue #%d", opts.CommentID, opts.IssueNumber), err)
	}
	return s.mapComment(note), nil
}

func (s *commentService) Create(ctx context.Context, opts forge.CommentCreateOptions) (*forge.Comment, error) {
	req := &gl.CreateIssueNoteOptions{Body: &opts.Body}
	note, _, err := s.forge.client.Notes.CreateIssueNote(
		s.forge.owner+"/"+s.forge.repo, int64(opts.IssueNumber), req,
		gl.WithContext(ctx),
	)
	if err != nil {
		return nil, s.forge.translateError(fmt.Sprintf("issue #%d", opts.IssueNumber), err)
	}
	return s.mapComment(note), nil
}

func (s *commentService) Update(ctx context.Context, opts forge.CommentUpdateOptions) (*forge.Comment, error) {
	req := &gl.UpdateIssueNoteOptions{Body: &opts.Body}
	note, _, err := s.forge.client.Notes.UpdateIssueNote(
		s.forge.owner+"/"+s.forge.repo, int64(opts.IssueNumber), int64(opts.CommentID), req,
		gl.WithContext(ctx),
	)
	if err != nil {
		return nil, s.forge.translateError(fmt.Sprintf("note #%d", opts.CommentID), err)
	}
	return s.mapComment(note), nil
}

func (s *commentService) Delete(ctx context.Context, opts forge.CommentDeleteOptions) error {
	_, err := s.forge.client.Notes.DeleteIssueNote(
		s.forge.owner+"/"+s.forge.repo, int64(opts.IssueNumber), int64(opts.CommentID),
		gl.WithContext(ctx),
	)
	if err != nil {
		return s.forge.translateError(fmt.Sprintf("note #%d", opts.CommentID), err)
	}
	return nil
}

// isTerminalPipelineStatus reports whether a GitLab pipeline status represents
// a completed pipeline (success, failed, canceled, skipped).
// Non-terminal statuses (running, pending, created, etc.) return false.
func isTerminalPipelineStatus(status string) bool {
	switch status {
	case "success", "failed", "canceled", "skipped":
		return true
	default:
		return false
	}
}

// stateForList converts a normalized state to the GitLab API list filter value.
// GitLab uses "opened" instead of "open".
func stateForList(state string) *string {
	if state == "" {
		return nil
	}
	if state == "open" {
		return forge.String("opened")
	}
	return forge.String(state)
}
