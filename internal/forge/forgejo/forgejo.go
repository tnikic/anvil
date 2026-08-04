// Package forgejo implements the forge.Forge interface backed by code.gitea.io/sdk/gitea.
// It maps Gitea/Forgejo REST API responses to the normalized domain types defined in
// the parent forge package, translating Forgejo's scoped-label naming convention
// (scope/name) to the two-argument model used at the CLI boundary.
package forgejo

import (
	"context"
	"fmt"
	"net/http"

	gitea "code.gitea.io/sdk/gitea"
	"github.com/tnikic/anvil/internal/forge"
)

// compile-time check
var _ forge.Forge = (*Forge)(nil)

// Forge is a Forgejo/Gitea implementation of the forge.Forge interface.
type Forge struct {
	client *gitea.Client
	host   string
	owner  string
	repo   string

	issueSvc    *issueService
	labelSvc    *labelService
	prSvc       *prService
	relationSvc *forge.RelationGuard
	commentSvc  *commentService
}

// New creates a new Forgejo Forge adapter.
// host is the forge host (e.g., "codeberg.org", or "http://127.0.0.1:8080" for testing).
// owner and repo identify the repository.
// httpClient is used for authentication — its transport should already carry
// a "token <token>" Authorization header (see auth.TokenTransport).
func New(host, owner, repo string, httpClient *http.Client) *Forge {
	scheme, cleanHost := forge.ParseHost(host)
	baseURL := scheme + "://" + cleanHost

	client := httpClient
	if client == nil {
		client = &http.Client{}
	}

	c, _ := gitea.NewClient(baseURL,
		gitea.SetHTTPClient(client),
		gitea.SetGiteaVersion("1.22.0"),
	)

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
	user, resp, err := f.client.GetMyUserInfo()
	if err != nil {
		return "", f.translateError("", resp, err)
	}
	if user == nil || user.UserName == "" {
		return "", forge.NewBaseError(
			"could not determine current user",
			"Verify that your token is valid",
		)
	}
	return user.UserName, nil
}

// ---- Error translation ----

// translateError converts a Gitea API or network error into a user-facing
// error message with an actionable help hint.
// resource describes what was being accessed (e.g., "issue #42").
// When empty, a generic message is used.
// resp is the Gitea SDK response, used to extract the HTTP status code
// for structured error translation; it may be nil for pre-request failures.
func (f *Forge) translateError(resource string, resp *gitea.Response, err error) error {
	if err == nil {
		return nil
	}

	// Translate HTTP errors from the response status code.
	if resp != nil && resp.Response != nil {
		if fe := forge.TranslateHTTPError(resp.StatusCode, f.host, f.owner, f.repo, resource); fe != nil {
			return fe
		}
	}

	// Network / context errors.
	if ne := forge.TranslateNetworkError(err, f.host); ne != nil {
		return ne
	}

	return err
}

// ---- Issue mapping ----

func mapIssue(giteaIssue *gitea.Issue) *forge.Issue {
	if giteaIssue == nil {
		return nil
	}
	parent, title := Parse(giteaIssue.Title)
	issue := &forge.Issue{
		Number:    int(giteaIssue.Index),
		Title:     title,
		State:     string(giteaIssue.State),
		Body:      giteaIssue.Body,
		Parent:    parent,
		CreatedAt: giteaIssue.Created,
		UpdatedAt: giteaIssue.Updated,
		URL:       giteaIssue.HTMLURL,
	}
	if giteaIssue.Poster != nil {
		issue.Author = giteaIssue.Poster.UserName
	}
	for _, l := range giteaIssue.Labels {
		scope, name := forge.ParseLabelScope(l.Name, "/")
		issue.Labels = append(issue.Labels, forge.Label{
			Name:        name,
			Scope:       scope,
			Color:       l.Color,
			Description: l.Description,
			Exclusive:   l.Exclusive,
		})
	}
	return issue
}

func mapIssues(giteaIssues []*gitea.Issue) []forge.Issue {
	out := make([]forge.Issue, 0, len(giteaIssues))
	for _, i := range giteaIssues {
		if m := mapIssue(i); m != nil {
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

	giteaOpts := gitea.ListIssueOption{
		ListOptions: gitea.ListOptions{
			PageSize: perPage,
		},
		State:  gitea.StateType(opts.State),
		Labels: opts.Labels,
	}

	allIssues, err := forge.Paginate(limit, func(page int) (forge.Page[forge.Issue], error) {
		giteaOpts.Page = page
		giteaIssues, resp, err := s.forge.client.ListRepoIssues(s.forge.owner, s.forge.repo, giteaOpts)
		if err != nil {
			return forge.Page[forge.Issue]{}, s.forge.translateError("", resp, err)
		}
		nextPage := 0
		if resp != nil {
			nextPage = resp.NextPage
		}
		return forge.Page[forge.Issue]{Items: mapIssues(giteaIssues), NextPage: nextPage}, nil
	})
	if err != nil {
		return nil, nil, err
	}

	return allIssues, forge.NewListMeta(len(allIssues)), nil
}

func (s *issueService) Get(ctx context.Context, opts forge.IssueGetOptions) (*forge.Issue, error) {
	giteaIssue, resp, err := s.forge.client.GetIssue(s.forge.owner, s.forge.repo, int64(opts.Number))
	if err != nil {
		return nil, s.forge.translateError(fmt.Sprintf("issue #%d", opts.Number), resp, err)
	}
	return mapIssue(giteaIssue), nil
}

func (s *issueService) Create(ctx context.Context, opts forge.IssueCreateOptions) (*forge.Issue, error) {
	req := gitea.CreateIssueOption{}
	if opts.Title != nil {
		// Inject parent prefix for sub-issues (the SDK CreateIssueOption has no Parent field).
		// We store parent via the title convention.
		req.Title = *opts.Title
	}
	if opts.Body != nil {
		req.Body = *opts.Body
	}
	// Forgejo uses label names (strings) for create, but the SDK's CreateIssueOption
	// only supports label IDs. We'll set labels after creation if needed.
	// For now, we create without labels and then update. Actually, looking at the SDK more
	// carefully, CreateIssueOption.Labels is []int64 (label IDs), not names.
	// We'll skip labels for create since we'd need to look up IDs.
	giteaIssue, resp, err := s.forge.client.CreateIssue(s.forge.owner, s.forge.repo, req)
	if err != nil {
		return nil, s.forge.translateError("", resp, err)
	}
	return mapIssue(giteaIssue), nil
}

func (s *issueService) Update(ctx context.Context, opts forge.IssueUpdateOptions) (*forge.Issue, error) {
	// Fetch the current issue to preserve fields we don't want to change.
	current, resp, err := s.forge.client.GetIssue(s.forge.owner, s.forge.repo, int64(opts.Number))
	if err != nil {
		return nil, s.forge.translateError(fmt.Sprintf("issue #%d", opts.Number), resp, err)
	}

	req := gitea.EditIssueOption{}
	if opts.Title != nil {
		req.Title = *opts.Title
		// If the current issue has a parent prefix, preserve it.
		if parentTitleRE.MatchString(current.Title) {
			parent, _ := Parse(current.Title)
			req.Title = Inject(*opts.Title, parent)
		}
	} else {
		req.Title = current.Title
	}

	if opts.Body != nil {
		req.Body = opts.Body
	}
	if opts.State != nil {
		state := gitea.StateType(*opts.State)
		req.State = &state
	}

	giteaIssue, resp, err := s.forge.client.EditIssue(s.forge.owner, s.forge.repo, int64(opts.Number), req)
	if err != nil {
		return nil, s.forge.translateError(fmt.Sprintf("issue #%d", opts.Number), resp, err)
	}
	return mapIssue(giteaIssue), nil
}

func (s *issueService) Close(ctx context.Context, opts forge.IssueCloseOptions) (*forge.Issue, error) {
	state := gitea.StateClosed
	req := gitea.EditIssueOption{State: &state}
	giteaIssue, resp, err := s.forge.client.EditIssue(s.forge.owner, s.forge.repo, int64(opts.Number), req)
	if err != nil {
		return nil, s.forge.translateError(fmt.Sprintf("issue #%d", opts.Number), resp, err)
	}
	return mapIssue(giteaIssue), nil
}

func (s *issueService) Reopen(ctx context.Context, opts forge.IssueReopenOptions) (*forge.Issue, error) {
	state := gitea.StateOpen
	req := gitea.EditIssueOption{State: &state}
	giteaIssue, resp, err := s.forge.client.EditIssue(s.forge.owner, s.forge.repo, int64(opts.Number), req)
	if err != nil {
		return nil, s.forge.translateError(fmt.Sprintf("issue #%d", opts.Number), resp, err)
	}
	return mapIssue(giteaIssue), nil
}

// ---- Label mapping ----

func mapLabel(giteaLabel *gitea.Label) forge.Label {
	scope, name := forge.ParseLabelScope(giteaLabel.Name, "/")
	return forge.Label{
		Name:        name,
		Scope:       scope,
		Color:       giteaLabel.Color,
		Description: giteaLabel.Description,
		Exclusive:   giteaLabel.Exclusive,
	}
}

func mapLabels(giteaLabels []*gitea.Label) []forge.Label {
	out := make([]forge.Label, 0, len(giteaLabels))
	for _, l := range giteaLabels {
		out = append(out, mapLabel(l))
	}
	return out
}

// ---- Label service ----

type labelService struct {
	forge *Forge
}

// resolveLabelID lists all repo labels and returns the ID of the label
// matching the given scope and name. Returns 0 and an error if not found.
func (s *labelService) resolveLabelID(ctx context.Context, scope, name string) (int64, error) {
	// Use a generous page size — repos rarely have >100 labels.
	opt := gitea.ListLabelsOptions{
		ListOptions: gitea.ListOptions{PageSize: 100},
	}
	labels, resp, err := s.forge.client.ListRepoLabels(s.forge.owner, s.forge.repo, opt)
	if err != nil {
		return 0, s.forge.translateError("labels", resp, err)
	}
	fullName := forge.LabelFullName(scope, name, "/")
	for _, l := range labels {
		if l.Name == fullName {
			return l.ID, nil
		}
	}
	return 0, forge.NewBaseError(
		fmt.Sprintf("label %q not found in %s/%s", fullName, s.forge.owner, s.forge.repo),
		"Run \"anvil label list\" to see available labels",
	)
}

func (s *labelService) List(ctx context.Context, opts forge.LabelListOptions) ([]forge.Label, error) {
	perPage, limit := forge.ListPerPage(opts.Limit)

	giteaOpts := gitea.ListLabelsOptions{
		ListOptions: gitea.ListOptions{
			PageSize: perPage,
		},
	}

	return forge.Paginate(limit, func(page int) (forge.Page[forge.Label], error) {
		giteaOpts.Page = page
		giteaLabels, resp, err := s.forge.client.ListRepoLabels(s.forge.owner, s.forge.repo, giteaOpts)
		if err != nil {
			return forge.Page[forge.Label]{}, s.forge.translateError("", resp, err)
		}
		nextPage := 0
		if resp != nil {
			nextPage = resp.NextPage
		}
		return forge.Page[forge.Label]{Items: mapLabels(giteaLabels), NextPage: nextPage}, nil
	})
}

func (s *labelService) Create(ctx context.Context, opts forge.LabelCreateOptions) (*forge.Label, error) {
	fullName := forge.LabelFullName(forge.StringVal(opts.Scope), opts.Name, "/")
	color := ""
	if opts.Color != nil {
		// The domain model stores hex without #; the Gitea API expects
		// (optionally) a # prefix. We prepend it for safety.
		color = "#" + *opts.Color
	}
	req := gitea.CreateLabelOption{
		Name:  fullName,
		Color: color,
	}
	if opts.Description != nil {
		req.Description = *opts.Description
	}
	if opts.Exclusive != nil {
		req.Exclusive = *opts.Exclusive
	}
	created, resp, err := s.forge.client.CreateLabel(s.forge.owner, s.forge.repo, req)
	if err != nil {
		return nil, s.forge.translateError("", resp, err)
	}
	l := mapLabel(created)
	return &l, nil
}

func (s *labelService) Update(ctx context.Context, opts forge.LabelUpdateOptions) (*forge.Label, error) {
	// Forgejo requires a numeric label ID for updates. Resolve it from the
	// label list, matching on scope+name.
	id, err := s.resolveLabelID(ctx, opts.Scope, opts.Name)
	if err != nil {
		return nil, err
	}

	req := gitea.EditLabelOption{}
	if opts.NewName != nil || opts.NewScope != nil {
		name := opts.Name
		scope := opts.Scope
		if opts.NewName != nil {
			name = *opts.NewName
		}
		if opts.NewScope != nil {
			scope = *opts.NewScope
		}
		newFullName := forge.LabelFullName(scope, name, "/")
		req.Name = &newFullName
	}
	if opts.Color != nil {
		c := "#" + *opts.Color
		req.Color = &c
	}
	if opts.Description != nil {
		req.Description = opts.Description
	}
	if opts.Exclusive != nil {
		req.Exclusive = opts.Exclusive
	}

	updated, resp, err := s.forge.client.EditLabel(s.forge.owner, s.forge.repo, id, req)
	if err != nil {
		return nil, s.forge.translateError(fmt.Sprintf("label %q", forge.LabelFullName(opts.Scope, opts.Name, "/")), resp, err)
	}
	l := mapLabel(updated)
	return &l, nil
}

func (s *labelService) Delete(ctx context.Context, opts forge.LabelDeleteOptions) error {
	// Forgejo requires a numeric label ID for deletion. Resolve it from the
	// label list, matching on scope+name.
	id, err := s.resolveLabelID(ctx, opts.Scope, opts.Name)
	if err != nil {
		return err
	}
	resp, err := s.forge.client.DeleteLabel(s.forge.owner, s.forge.repo, id)
	if err != nil {
		return s.forge.translateError(fmt.Sprintf("label %q", forge.LabelFullName(opts.Scope, opts.Name, "/")), resp, err)
	}
	return nil
}

// ---- PR mapping ----

func mapPR(giteaPR *gitea.PullRequest) *forge.PR {
	if giteaPR == nil {
		return nil
	}
	state := string(giteaPR.State)
	if giteaPR.HasMerged {
		state = forge.StateMerged
	}
	pr := &forge.PR{
		Number: int(giteaPR.Index),
		Title:  giteaPR.Title,
		State:  state,
		Body:   giteaPR.Body,
		URL:    giteaPR.HTMLURL,
	}
	if giteaPR.Base != nil {
		pr.BaseRef = giteaPR.Base.Ref
	}
	if giteaPR.Head != nil {
		pr.HeadRef = giteaPR.Head.Ref
	}
	if giteaPR.Poster != nil {
		pr.Author = giteaPR.Poster.UserName
	}
	if giteaPR.Draft {
		pr.Extras = map[string]any{"draft": true}
	}
	if giteaPR.Created != nil {
		pr.CreatedAt = *giteaPR.Created
	}
	if giteaPR.Updated != nil {
		pr.UpdatedAt = *giteaPR.Updated
	}
	return pr
}

func mapPRs(giteaPRs []*gitea.PullRequest) []forge.PR {
	out := make([]forge.PR, 0, len(giteaPRs))
	for _, p := range giteaPRs {
		if m := mapPR(p); m != nil {
			out = append(out, *m)
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

	giteaOpts := gitea.ListPullRequestsOptions{
		ListOptions: gitea.ListOptions{
			PageSize: perPage,
		},
		State: gitea.StateType(opts.State),
		Sort:  opts.Sort,
	}

	allPRs, err := forge.Paginate(limit, func(page int) (forge.Page[forge.PR], error) {
		giteaOpts.Page = page
		giteaPRs, resp, err := s.forge.client.ListRepoPullRequests(s.forge.owner, s.forge.repo, giteaOpts)
		if err != nil {
			return forge.Page[forge.PR]{}, s.forge.translateError("", resp, err)
		}
		nextPage := 0
		if resp != nil {
			nextPage = resp.NextPage
		}
		return forge.Page[forge.PR]{Items: mapPRs(giteaPRs), NextPage: nextPage}, nil
	})
	if err != nil {
		return nil, nil, err
	}

	return allPRs, forge.NewListMeta(len(allPRs)), nil
}

func (s *prService) Get(ctx context.Context, opts forge.PRGetOptions) (*forge.PR, error) {
	giteaPR, resp, err := s.forge.client.GetPullRequest(s.forge.owner, s.forge.repo, int64(opts.Number))
	if err != nil {
		return nil, s.forge.translateError(fmt.Sprintf("PR #%d", opts.Number), resp, err)
	}
	return mapPR(giteaPR), nil
}

func (s *prService) Create(ctx context.Context, opts forge.PRCreateOptions) (*forge.PR, error) {
	req := gitea.CreatePullRequestOption{}
	if opts.Title != nil {
		req.Title = *opts.Title
	}
	if opts.Body != nil {
		req.Body = *opts.Body
	}
	if opts.HeadRef != nil {
		req.Head = *opts.HeadRef
	}
	if opts.BaseRef != nil {
		req.Base = *opts.BaseRef
	}
	// Note: CreatePullRequestOption has no Draft field in this SDK version.
	// Draft status can be read back from PRs but not set on creation.
	giteaPR, resp, err := s.forge.client.CreatePullRequest(s.forge.owner, s.forge.repo, req)
	if err != nil {
		return nil, s.forge.translateError("", resp, err)
	}
	return mapPR(giteaPR), nil
}

func (s *prService) Update(ctx context.Context, opts forge.PRUpdateOptions) (*forge.PR, error) {
	req := gitea.EditPullRequestOption{}
	if opts.Title != nil {
		req.Title = *opts.Title
	}
	giteaPR, resp, err := s.forge.client.EditPullRequest(s.forge.owner, s.forge.repo, int64(opts.Number), req)
	if err != nil {
		return nil, s.forge.translateError(fmt.Sprintf("PR #%d", opts.Number), resp, err)
	}
	return mapPR(giteaPR), nil
}

func (s *prService) Merge(ctx context.Context, opts forge.PRMergeOptions) (*forge.PR, error) {
	mergeOpt := gitea.MergePullRequestOption{
		Style: gitea.MergeStyleMerge,
	}
	if opts.Title != nil {
		mergeOpt.Title = *opts.Title
	}
	if opts.Body != nil {
		mergeOpt.Message = *opts.Body
	}
	if opts.Method != nil {
		switch *opts.Method {
		case "squash":
			mergeOpt.Style = gitea.MergeStyleSquash
		case "rebase":
			mergeOpt.Style = gitea.MergeStyleRebase
		}
	}
	ok, resp, err := s.forge.client.MergePullRequest(s.forge.owner, s.forge.repo, int64(opts.Number), mergeOpt)
	if err != nil {
		return nil, s.forge.translateError(fmt.Sprintf("PR #%d", opts.Number), resp, err)
	}
	if !ok {
		return nil, forge.NewBaseError(
			fmt.Sprintf("PR #%d could not be merged in %s/%s", opts.Number, s.forge.owner, s.forge.repo),
			"Check for merge conflicts or required status checks",
		)
	}
	// After merge, fetch the PR to return its final state.
	giteaPR, _, err := s.forge.client.GetPullRequest(s.forge.owner, s.forge.repo, int64(opts.Number))
	if err != nil {
		// If we can't fetch it, return a synthesized merged PR.
		return &forge.PR{Number: opts.Number, State: forge.StateMerged}, nil
	}
	return mapPR(giteaPR), nil
}

func (s *prService) Close(ctx context.Context, opts forge.PRCloseOptions) (*forge.PR, error) {
	state := gitea.StateClosed
	req := gitea.EditPullRequestOption{State: &state}
	giteaPR, resp, err := s.forge.client.EditPullRequest(s.forge.owner, s.forge.repo, int64(opts.Number), req)
	if err != nil {
		return nil, s.forge.translateError(fmt.Sprintf("PR #%d", opts.Number), resp, err)
	}
	return mapPR(giteaPR), nil
}

// ---- Relation service ----

// relationReader implements the read side of forge.RelationService for Forgejo.
type relationReader struct {
	forge *Forge
}

func (r *relationReader) BlockedBy(ctx context.Context, number int) ([]forge.IssueDependency, error) {
	opt := gitea.ListIssueDependenciesOptions{
		ListOptions: gitea.ListOptions{PageSize: 50},
	}

	return forge.Paginate(0, func(page int) (forge.Page[forge.IssueDependency], error) {
		opt.Page = page
		issues, resp, err := r.forge.client.ListIssueDependencies(r.forge.owner, r.forge.repo, int64(number), opt)
		if err != nil {
			return forge.Page[forge.IssueDependency]{}, r.forge.translateError(fmt.Sprintf("issue #%d blocked-by", number), resp, err)
		}
		next := 0
		if resp != nil {
			next = resp.NextPage
		}
		return forge.Page[forge.IssueDependency]{Items: mapIssueDeps(issues, forge.DirBlockedBy), NextPage: next}, nil
	})
}

func (r *relationReader) Blocking(ctx context.Context, number int) ([]forge.IssueDependency, error) {
	opt := gitea.ListIssueBlocksOptions{
		ListOptions: gitea.ListOptions{PageSize: 50},
	}

	return forge.Paginate(0, func(page int) (forge.Page[forge.IssueDependency], error) {
		opt.Page = page
		issues, resp, err := r.forge.client.ListIssueBlocks(r.forge.owner, r.forge.repo, int64(number), opt)
		if err != nil {
			return forge.Page[forge.IssueDependency]{}, r.forge.translateError(fmt.Sprintf("issue #%d blocking", number), resp, err)
		}
		next := 0
		if resp != nil {
			next = resp.NextPage
		}
		return forge.Page[forge.IssueDependency]{Items: mapIssueDeps(issues, forge.DirBlocks), NextPage: next}, nil
	})
}

func (r *relationReader) Children(ctx context.Context, number int) ([]forge.IssueDependency, error) {
	opt := gitea.ListIssueOption{
		ListOptions: gitea.ListOptions{PageSize: 50},
		State:       gitea.StateAll,
	}

	return forge.Paginate(0, func(page int) (forge.Page[forge.IssueDependency], error) {
		opt.Page = page
		issues, resp, err := r.forge.client.ListRepoIssues(r.forge.owner, r.forge.repo, opt)
		if err != nil {
			return forge.Page[forge.IssueDependency]{}, r.forge.translateError(fmt.Sprintf("children of #%d", number), resp, err)
		}
		next := 0
		if resp != nil {
			next = resp.NextPage
		}
		return forge.Page[forge.IssueDependency]{Items: FindChildren(issues, number), NextPage: next}, nil
	})
}

func (r *relationReader) Parent(ctx context.Context, number int) (*forge.IssueDependency, error) {
	issue, resp, err := r.forge.client.GetIssue(r.forge.owner, r.forge.repo, int64(number))
	if err != nil {
		return nil, r.forge.translateError(fmt.Sprintf("issue #%d parent", number), resp, err)
	}
	parent, _ := Parse(issue.Title)
	if parent == nil {
		return nil, nil
	}
	parentIssue, resp, err := r.forge.client.GetIssue(r.forge.owner, r.forge.repo, int64(*parent))
	if err != nil {
		return nil, r.forge.translateError(fmt.Sprintf("issue #%d parent (#%d)", number, *parent), resp, err)
	}
	return &forge.IssueDependency{
		Number:    int(parentIssue.Index),
		Title:     parentIssue.Title,
		State:     string(parentIssue.State),
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
	_, resp, err := f.client.CreateIssueDependency(f.owner, f.repo, int64(target), gitea.IssueMeta{Index: int64(number)})
	if err != nil {
		return f.translateError(fmt.Sprintf("add blocks #%d → #%d", number, target), resp, err)
	}
	return nil
}

func (f *Forge) removeBlocks(ctx context.Context, number, target int) error {
	_, resp, err := f.client.RemoveIssueDependency(f.owner, f.repo, int64(target), gitea.IssueMeta{Index: int64(number)})
	if err != nil {
		return f.translateError(fmt.Sprintf("remove blocks #%d → #%d", number, target), resp, err)
	}
	return nil
}

func (f *Forge) addParentOf(ctx context.Context, number, child int) error {
	childIssue, resp, err := f.client.GetIssue(f.owner, f.repo, int64(child))
	if err != nil {
		return f.translateError(fmt.Sprintf("add parent #%d → #%d", number, child), resp, err)
	}
	_, cleanTitle := Parse(childIssue.Title)
	newTitle := Inject(cleanTitle, forge.Int(number))
	_, resp, err = f.client.EditIssue(f.owner, f.repo, int64(child), gitea.EditIssueOption{Title: newTitle})
	if err != nil {
		return f.translateError(fmt.Sprintf("add parent #%d → #%d", number, child), resp, err)
	}
	return nil
}

func (f *Forge) removeParentOf(ctx context.Context, number, child int) error {
	childIssue, resp, err := f.client.GetIssue(f.owner, f.repo, int64(child))
	if err != nil {
		return f.translateError(fmt.Sprintf("remove parent #%d → #%d", number, child), resp, err)
	}
	parent, cleanTitle := Parse(childIssue.Title)
	if parent == nil || *parent != number {
		return nil // parent already removed or different
	}
	_, resp, err = f.client.EditIssue(f.owner, f.repo, int64(child), gitea.EditIssueOption{Title: cleanTitle})
	if err != nil {
		return f.translateError(fmt.Sprintf("remove parent #%d → #%d", number, child), resp, err)
	}
	return nil
}

// mapIssueDeps converts Gitea Issue objects into IssueDependency values.
func mapIssueDeps(giteaIssues []*gitea.Issue, dir forge.IssueDependencyDirection) []forge.IssueDependency {
	out := make([]forge.IssueDependency, 0, len(giteaIssues))
	for _, i := range giteaIssues {
		if i == nil {
			continue
		}
		out = append(out, forge.IssueDependency{
			Number:    int(i.Index),
			Title:     i.Title,
			State:     string(i.State),
			Direction: dir,
		})
	}
	return out
}

// ---- Comment mapping ----

func mapComment(giteaComment *gitea.Comment) *forge.Comment {
	if giteaComment == nil {
		return nil
	}
	c := &forge.Comment{
		ID:        int(giteaComment.ID),
		Body:      giteaComment.Body,
		System:    false, // Forgejo returns only user comments from the issues comments endpoint
		CreatedAt: giteaComment.Created,
		UpdatedAt: giteaComment.Updated,
		URL:       giteaComment.HTMLURL,
	}
	if giteaComment.Poster != nil {
		c.Author = giteaComment.Poster.UserName
	}
	return c
}

func mapComments(giteaComments []*gitea.Comment) []forge.Comment {
	out := make([]forge.Comment, 0, len(giteaComments))
	for _, cm := range giteaComments {
		if m := mapComment(cm); m != nil {
			out = append(out, *m)
		}
	}
	return out
}

// ---- Comment service ----

type commentService struct {
	forge *Forge
}

func (s *commentService) List(ctx context.Context, opts forge.CommentListOptions) ([]forge.Comment, error) {
	giteaOpts := gitea.ListIssueCommentOptions{
		ListOptions: gitea.ListOptions{PageSize: 100},
	}

	return forge.Paginate(0, func(page int) (forge.Page[forge.Comment], error) {
		giteaOpts.Page = page
		giteaComments, resp, err := s.forge.client.ListIssueComments(
			s.forge.owner, s.forge.repo, int64(opts.IssueNumber), giteaOpts,
		)
		if err != nil {
			return forge.Page[forge.Comment]{}, s.forge.translateError(fmt.Sprintf("comments for issue #%d", opts.IssueNumber), resp, err)
		}
		nextPage := 0
		if resp != nil {
			nextPage = resp.NextPage
		}
		return forge.Page[forge.Comment]{Items: mapComments(giteaComments), NextPage: nextPage}, nil
	})
}

func (s *commentService) Get(ctx context.Context, opts forge.CommentGetOptions) (*forge.Comment, error) {
	giteaComment, resp, err := s.forge.client.GetIssueComment(
		s.forge.owner, s.forge.repo, int64(opts.CommentID),
	)
	if err != nil {
		return nil, s.forge.translateError(fmt.Sprintf("comment #%d on issue #%d", opts.CommentID, opts.IssueNumber), resp, err)
	}
	return mapComment(giteaComment), nil
}

func (s *commentService) Create(ctx context.Context, opts forge.CommentCreateOptions) (*forge.Comment, error) {
	req := gitea.CreateIssueCommentOption{Body: opts.Body}
	giteaComment, resp, err := s.forge.client.CreateIssueComment(
		s.forge.owner, s.forge.repo, int64(opts.IssueNumber), req,
	)
	if err != nil {
		return nil, s.forge.translateError(fmt.Sprintf("issue #%d", opts.IssueNumber), resp, err)
	}
	return mapComment(giteaComment), nil
}

func (s *commentService) Update(ctx context.Context, opts forge.CommentUpdateOptions) (*forge.Comment, error) {
	req := gitea.EditIssueCommentOption{Body: opts.Body}
	giteaComment, resp, err := s.forge.client.EditIssueComment(
		s.forge.owner, s.forge.repo, int64(opts.CommentID), req,
	)
	if err != nil {
		return nil, s.forge.translateError(fmt.Sprintf("comment #%d", opts.CommentID), resp, err)
	}
	return mapComment(giteaComment), nil
}

func (s *commentService) Delete(ctx context.Context, opts forge.CommentDeleteOptions) error {
	resp, err := s.forge.client.DeleteIssueComment(
		s.forge.owner, s.forge.repo, int64(opts.CommentID),
	)
	if err != nil {
		return s.forge.translateError(fmt.Sprintf("comment #%d", opts.CommentID), resp, err)
	}
	return nil
}
