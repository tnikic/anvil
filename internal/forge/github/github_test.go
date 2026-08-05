package github_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	gh "github.com/google/go-github/v90/github"
	"github.com/tnikic/anvil/internal/forge"
	githubadapter "github.com/tnikic/anvil/internal/forge/github"
)

func respondJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// ---- Interface compliance ----

func TestGitHubForgeSatisfiesInterface(t *testing.T) {
	var _ forge.Forge = (*githubadapter.Forge)(nil)
}

// ---- Scope parsing ----

func TestParseLabelScopeScoped(t *testing.T) {
	scope, name := forge.ParseLabelScope("kind:bug", ":")
	if scope != "kind" {
		t.Errorf("scope = %q, want %q", scope, "kind")
	}
	if name != "bug" {
		t.Errorf("name = %q, want %q", name, "bug")
	}
}

func TestParseLabelScopeUnscoped(t *testing.T) {
	scope, name := forge.ParseLabelScope("good-first-issue", ":")
	if scope != "" {
		t.Errorf("scope should be empty, got %q", scope)
	}
	if name != "good-first-issue" {
		t.Errorf("name = %q, want %q", name, "good-first-issue")
	}
}

func TestParseLabelScopeMultipleColons(t *testing.T) {
	// Only split on first colon
	scope, name := forge.ParseLabelScope("a:b:c", ":")
	if scope != "a" {
		t.Errorf("scope = %q, want %q", scope, "a")
	}
	if name != "b:c" {
		t.Errorf("name = %q, want %q", name, "b:c")
	}
}

func TestLabelFullNameScoped(t *testing.T) {
	full := forge.LabelFullName("kind", "bug", ":")
	if full != "kind:bug" {
		t.Errorf("full = %q, want %q", full, "kind:bug")
	}
}

func TestLabelFullNameUnscoped(t *testing.T) {
	full := forge.LabelFullName("", "good-first-issue", ":")
	if full != "good-first-issue" {
		t.Errorf("full = %q, want %q", full, "good-first-issue")
	}
}

func TestLabelRoundTrip(t *testing.T) {
	tests := []struct {
		scope, name string
	}{
		{"kind", "bug"},
		{"priority", "high"},
		{"", "good-first-issue"},
		{"team", "frontend"},
	}
	for _, tt := range tests {
		full := forge.LabelFullName(tt.scope, tt.name, ":")
		scope, name := forge.ParseLabelScope(full, ":")
		if scope != tt.scope {
			t.Errorf("round-trip %q/%q: scope = %q, want %q", tt.scope, tt.name, scope, tt.scope)
		}
		if name != tt.name {
			t.Errorf("round-trip %q/%q: name = %q, want %q", tt.scope, tt.name, name, tt.name)
		}
	}
}

// ---- Issue mapping ----

func TestIssueListMapping(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		issues := []*gh.Issue{
			{
				Number:    gh.Ptr(1),
				Title:     gh.Ptr("Fix login timeout"),
				State:     gh.Ptr("open"),
				Body:      gh.Ptr("Users are logged out after 5 minutes of inactivity."),
				User:      &gh.User{Login: gh.Ptr("alice")},
				HTMLURL:   gh.Ptr("https://github.com/owner/repo/issues/1"),
				CreatedAt: &gh.Timestamp{},
				UpdatedAt: &gh.Timestamp{},
				Labels: []*gh.Label{
					{Name: "kind:bug", Color: "ff0000", Description: gh.Ptr("A bug")},
					{Name: "good-first-issue", Color: "7057ff", Description: gh.Ptr("Good for newcomers")},
				},
			},
		}
		respondJSON(w, http.StatusOK, issues)
	}))
	defer srv.Close()

	f := githubadapter.New(srv.URL, "owner", "repo", srv.Client())

	issues, meta, err := f.Issues().List(context.Background(), forge.IssueListOptions{State: "open"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(issues))
	}
	if issues[0].Number != 1 {
		t.Errorf("number = %d, want 1", issues[0].Number)
	}
	if issues[0].Title != "Fix login timeout" {
		t.Errorf("title = %q", issues[0].Title)
	}
	if issues[0].State != "open" {
		t.Errorf("state = %q", issues[0].State)
	}
	if issues[0].Author != "alice" {
		t.Errorf("author = %q", issues[0].Author)
	}
	if len(issues[0].Labels) != 2 {
		t.Fatalf("expected 2 labels, got %d", len(issues[0].Labels))
	}
	// First label: kind:bug -> scope=kind, name=bug
	if issues[0].Labels[0].Scope != "kind" || issues[0].Labels[0].Name != "bug" {
		t.Errorf("label 0: scope=%q name=%q", issues[0].Labels[0].Scope, issues[0].Labels[0].Name)
	}
	// Second label: unscoped
	if issues[0].Labels[1].Scope != "" || issues[0].Labels[1].Name != "good-first-issue" {
		t.Errorf("label 1: scope=%q name=%q", issues[0].Labels[1].Scope, issues[0].Labels[1].Name)
	}
	if meta.Count != 1 {
		t.Errorf("meta.Count = %d, want 1", meta.Count)
	}
}

func TestIssueGetMapping(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		issue := &gh.Issue{
			Number:    gh.Ptr(42),
			Title:     gh.Ptr("Test Issue"),
			State:     gh.Ptr("closed"),
			Body:      gh.Ptr("Full body content"),
			User:      &gh.User{Login: gh.Ptr("bob")},
			HTMLURL:   gh.Ptr("https://github.com/owner/repo/issues/42"),
			CreatedAt: &gh.Timestamp{},
			UpdatedAt: &gh.Timestamp{},
		}
		respondJSON(w, http.StatusOK, issue)
	}))
	defer srv.Close()

	f := githubadapter.New(srv.URL, "owner", "repo", srv.Client())

	issue, err := f.Issues().Get(context.Background(), forge.IssueGetOptions{Number: 42})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if issue.Number != 42 {
		t.Errorf("number = %d", issue.Number)
	}
	if issue.Title != "Test Issue" {
		t.Errorf("title = %q", issue.Title)
	}
	if issue.State != "closed" {
		t.Errorf("state = %q", issue.State)
	}
	if issue.Body != "Full body content" {
		t.Errorf("body = %q", issue.Body)
	}
	if issue.Author != "bob" {
		t.Errorf("author = %q", issue.Author)
	}
}

func TestIssueCreateMapping(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		issue := &gh.Issue{
			Number:    gh.Ptr(99),
			Title:     gh.Ptr("Created Issue"),
			State:     gh.Ptr("open"),
			User:      &gh.User{Login: gh.Ptr("alice")},
			HTMLURL:   gh.Ptr("https://github.com/owner/repo/issues/99"),
			CreatedAt: &gh.Timestamp{},
			UpdatedAt: &gh.Timestamp{},
		}
		respondJSON(w, http.StatusCreated, issue)
	}))
	defer srv.Close()

	f := githubadapter.New(srv.URL, "owner", "repo", srv.Client())

	created, err := f.Issues().Create(context.Background(), forge.IssueCreateOptions{
		Title: forge.String("Created Issue"),
		Body:  forge.String("Body text"),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if created.Number != 99 {
		t.Errorf("number = %d", created.Number)
	}
	if created.Title != "Created Issue" {
		t.Errorf("title = %q", created.Title)
	}
}

func TestIssueCloseMapping(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		issue := &gh.Issue{
			Number: gh.Ptr(42),
			State:  gh.Ptr("closed"),
		}
		respondJSON(w, http.StatusOK, issue)
	}))
	defer srv.Close()

	f := githubadapter.New(srv.URL, "owner", "repo", srv.Client())

	closed, err := f.Issues().Close(context.Background(), forge.IssueCloseOptions{Number: 42})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if closed.State != "closed" {
		t.Errorf("state = %q, want closed", closed.State)
	}
}

func TestIssueReopenMapping(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		issue := &gh.Issue{
			Number: gh.Ptr(42),
			State:  gh.Ptr("open"),
		}
		respondJSON(w, http.StatusOK, issue)
	}))
	defer srv.Close()

	f := githubadapter.New(srv.URL, "owner", "repo", srv.Client())

	reopened, err := f.Issues().Reopen(context.Background(), forge.IssueReopenOptions{Number: 42})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reopened.State != "open" {
		t.Errorf("state = %q, want open", reopened.State)
	}
}

// ---- Label mapping ----

func TestLabelListMapping(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		labels := []*gh.Label{
			{Name: "kind:bug", Color: "d73a4a", Description: gh.Ptr("Something broken")},
			{Name: "priority:high", Color: "ff0000", Description: gh.Ptr("Urgent")},
			{Name: "good-first-issue", Color: "7057ff", Description: gh.Ptr("Good for newcomers")},
		}
		respondJSON(w, http.StatusOK, labels)
	}))
	defer srv.Close()

	f := githubadapter.New(srv.URL, "owner", "repo", srv.Client())

	labels, err := f.Labels().List(context.Background(), forge.LabelListOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(labels) != 3 {
		t.Fatalf("expected 3 labels, got %d", len(labels))
	}

	// Scoped label
	if labels[0].Scope != "kind" || labels[0].Name != "bug" {
		t.Errorf("label 0: scope=%q name=%q", labels[0].Scope, labels[0].Name)
	}
	if labels[0].Color != "d73a4a" || labels[0].Description != "Something broken" {
		t.Errorf("label 0: color=%q desc=%q", labels[0].Color, labels[0].Description)
	}

	// Unscoped label
	if labels[2].Scope != "" || labels[2].Name != "good-first-issue" {
		t.Errorf("label 2: scope=%q name=%q", labels[2].Scope, labels[2].Name)
	}
}

func TestLabelCreateMapping(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// The adapter sends the full name "kind:bug" to GitHub
		label := &gh.Label{
			Name:        "kind:bug",
			Color:       "d73a4a",
			Description: gh.Ptr("A bug"),
		}
		respondJSON(w, http.StatusCreated, label)
	}))
	defer srv.Close()

	f := githubadapter.New(srv.URL, "owner", "repo", srv.Client())

	created, err := f.Labels().Create(context.Background(), forge.LabelCreateOptions{
		Scope:       forge.String("kind"),
		Name:        "bug",
		Color:       forge.String("d73a4a"),
		Description: forge.String("A bug"),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if created.Scope != "kind" || created.Name != "bug" {
		t.Errorf("created: scope=%q name=%q", created.Scope, created.Name)
	}
}

func TestLabelCreateUnscopedMapping(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		label := &gh.Label{
			Name:  "enhancement",
			Color: "0052cc",
		}
		respondJSON(w, http.StatusCreated, label)
	}))
	defer srv.Close()

	f := githubadapter.New(srv.URL, "owner", "repo", srv.Client())

	created, err := f.Labels().Create(context.Background(), forge.LabelCreateOptions{
		Name:  "enhancement",
		Color: forge.String("0052cc"),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if created.Scope != "" || created.Name != "enhancement" {
		t.Errorf("created: scope=%q name=%q", created.Scope, created.Name)
	}
}

func TestLabelUpdateMapping(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		label := &gh.Label{
			Name:        "kind:bug",
			Color:       "ff0000",
			Description: gh.Ptr("Updated description"),
		}
		respondJSON(w, http.StatusOK, label)
	}))
	defer srv.Close()

	f := githubadapter.New(srv.URL, "owner", "repo", srv.Client())

	updated, err := f.Labels().Update(context.Background(), forge.LabelUpdateOptions{
		Scope:       "kind",
		Name:        "bug",
		Color:       forge.String("ff0000"),
		Description: forge.String("Updated description"),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated.Scope != "kind" || updated.Name != "bug" {
		t.Errorf("updated: scope=%q name=%q", updated.Scope, updated.Name)
	}
	if updated.Color != "ff0000" {
		t.Errorf("updated color = %q", updated.Color)
	}
}

func TestLabelDeleteMapping(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	f := githubadapter.New(srv.URL, "owner", "repo", srv.Client())

	err := f.Labels().Delete(context.Background(), forge.LabelDeleteOptions{
		Scope: "kind",
		Name:  "bug",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ---- PR mapping ----

func TestPRListMapping(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		prs := []*gh.PullRequest{
			{
				Number:    gh.Ptr(10),
				Title:     gh.Ptr("[auth:1/2] Add OAuth"),
				State:     gh.Ptr("open"),
				User:      &gh.User{Login: gh.Ptr("dev")},
				HTMLURL:   gh.Ptr("https://github.com/owner/repo/pull/10"),
				Head:      &gh.PullRequestBranch{Ref: gh.Ptr("feat/auth")},
				Base:      &gh.PullRequestBranch{Ref: gh.Ptr("main")},
				Draft:     gh.Ptr(false),
				CreatedAt: &gh.Timestamp{},
				UpdatedAt: &gh.Timestamp{},
			},
		}
		respondJSON(w, http.StatusOK, prs)
	}))
	defer srv.Close()

	f := githubadapter.New(srv.URL, "owner", "repo", srv.Client())

	prs, meta, err := f.PRs().List(context.Background(), forge.PRListOptions{State: "open"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(prs) != 1 {
		t.Fatalf("expected 1 PR, got %d", len(prs))
	}
	if prs[0].Number != 10 {
		t.Errorf("number = %d", prs[0].Number)
	}
	if prs[0].Title != "[auth:1/2] Add OAuth" {
		t.Errorf("title = %q", prs[0].Title)
	}
	if prs[0].State != "open" {
		t.Errorf("state = %q", prs[0].State)
	}
	if prs[0].BaseRef != "main" || prs[0].HeadRef != "feat/auth" {
		t.Errorf("base=%q head=%q", prs[0].BaseRef, prs[0].HeadRef)
	}
	if prs[0].Author != "dev" {
		t.Errorf("author = %q", prs[0].Author)
	}
	if meta.Count != 1 {
		t.Errorf("meta.Count = %d", meta.Count)
	}
}

func TestPRGetMapping(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pr := &gh.PullRequest{
			Number:    gh.Ptr(10),
			Title:     gh.Ptr("[auth:1/2] Add OAuth"),
			State:     gh.Ptr("open"),
			Body:      gh.Ptr("Implements OAuth flow."),
			User:      &gh.User{Login: gh.Ptr("dev")},
			HTMLURL:   gh.Ptr("https://github.com/owner/repo/pull/10"),
			Head:      &gh.PullRequestBranch{Ref: gh.Ptr("feat/auth")},
			Base:      &gh.PullRequestBranch{Ref: gh.Ptr("main")},
			Draft:     gh.Ptr(true),
			CreatedAt: &gh.Timestamp{},
			UpdatedAt: &gh.Timestamp{},
		}
		respondJSON(w, http.StatusOK, pr)
	}))
	defer srv.Close()

	f := githubadapter.New(srv.URL, "owner", "repo", srv.Client())

	pr, err := f.PRs().Get(context.Background(), forge.PRGetOptions{Number: 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pr.Number != 10 {
		t.Errorf("number = %d", pr.Number)
	}
	if pr.Body != "Implements OAuth flow." {
		t.Errorf("body = %q", pr.Body)
	}
	// Draft should be in Extras
	if pr.Extras == nil || pr.Extras["draft"] != true {
		t.Errorf("draft should be true in Extras, got %v", pr.Extras)
	}
}

func TestPRCreateMapping(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pr := &gh.PullRequest{
			Number:    gh.Ptr(11),
			Title:     gh.Ptr("New Feature"),
			State:     gh.Ptr("open"),
			User:      &gh.User{Login: gh.Ptr("dev")},
			HTMLURL:   gh.Ptr("https://github.com/owner/repo/pull/11"),
			Head:      &gh.PullRequestBranch{Ref: gh.Ptr("feature-branch")},
			Base:      &gh.PullRequestBranch{Ref: gh.Ptr("main")},
			CreatedAt: &gh.Timestamp{},
			UpdatedAt: &gh.Timestamp{},
		}
		respondJSON(w, http.StatusCreated, pr)
	}))
	defer srv.Close()

	f := githubadapter.New(srv.URL, "owner", "repo", srv.Client())

	created, err := f.PRs().Create(context.Background(), forge.PRCreateOptions{
		Title:   forge.String("New Feature"),
		HeadRef: forge.String("feature-branch"),
		BaseRef: forge.String("main"),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if created.Number != 11 {
		t.Errorf("number = %d", created.Number)
	}
	if created.HeadRef != "feature-branch" {
		t.Errorf("head = %q", created.HeadRef)
	}
}

func TestPRMergeMapping(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if strings.Contains(r.URL.Path, "/merge") {
			result := &gh.PullRequestMergeResult{
				Merged:  gh.Ptr(true),
				Message: gh.Ptr("Pull Request successfully merged"),
			}
			respondJSON(w, http.StatusOK, result)
			return
		}
		// PR Get after merge
		pr := &gh.PullRequest{
			Number:  gh.Ptr(10),
			State:   gh.Ptr("merged"),
			Head:    &gh.PullRequestBranch{Ref: gh.Ptr("feature")},
			Base:    &gh.PullRequestBranch{Ref: gh.Ptr("main")},
			User:    &gh.User{Login: gh.Ptr("dev")},
			HTMLURL: gh.Ptr("https://github.com/owner/repo/pull/10"),
		}
		respondJSON(w, http.StatusOK, pr)
	}))
	defer srv.Close()

	f := githubadapter.New(srv.URL, "owner", "repo", srv.Client())

	merged, err := f.PRs().Merge(context.Background(), forge.PRMergeOptions{Number: 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if merged.Number != 10 {
		t.Errorf("number = %d", merged.Number)
	}
	if merged.State != "merged" {
		t.Errorf("state = %q, want merged", merged.State)
	}
}

// ---- Error translation ----

func TestErrorTranslation401(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"message": "Bad credentials"})
	}))
	defer srv.Close()

	f := githubadapter.New(srv.URL, "owner", "repo", srv.Client())

	_, _, err := f.Issues().List(context.Background(), forge.IssueListOptions{})
	if err == nil {
		t.Fatal("expected error for 401")
	}
	errStr := err.Error()
	if !strings.Contains(errStr, "authentication failed") {
		t.Errorf("error should mention authentication: %s", errStr)
	}
	if !strings.Contains(errStr, "401") {
		t.Errorf("error should include status code 401: %s", errStr)
	}
	// Help hint is on the StructuredError interface, not Error()
	var se forge.StructuredError
	if errors.As(err, &se) {
		if !strings.Contains(se.Help(), "auth set") {
			t.Errorf("help should suggest auth set command: %s", se.Help())
		}
	}
}

func TestErrorTranslation404(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"message": "Not Found"})
	}))
	defer srv.Close()

	f := githubadapter.New(srv.URL, "owner", "repo", srv.Client())

	_, err := f.Issues().Get(context.Background(), forge.IssueGetOptions{Number: 999})
	if err == nil {
		t.Fatal("expected error for 404")
	}
	errStr := err.Error()
	if !strings.Contains(errStr, "not found") && !strings.Contains(errStr, "404") {
		t.Errorf("error should mention not found or 404: %s", errStr)
	}
}

func TestErrorTranslationRateLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-RateLimit-Remaining", "0")
		w.Header().Set("X-RateLimit-Reset", "2025-01-01T00:00:00Z")
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"message": "API rate limit exceeded",
		})
	}))
	defer srv.Close()

	f := githubadapter.New(srv.URL, "owner", "repo", srv.Client())

	_, _, err := f.Issues().List(context.Background(), forge.IssueListOptions{})
	if err == nil {
		t.Fatal("expected error for rate limit")
	}
	errStr := err.Error()
	if !strings.Contains(errStr, "rate limit") {
		t.Errorf("error should mention rate limit: %s", errStr)
	}
	// Help hint is on the StructuredError interface, not Error()
	var se forge.StructuredError
	if !errors.As(err, &se) || se.Help() == "" {
		t.Errorf("error should have a help hint via StructuredError: %s", errStr)
	}
}

// ---- Pagination ----

func TestIssueListPagination(t *testing.T) {
	page := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page++
		issues := []*gh.Issue{
			{Number: gh.Ptr(page * 10), Title: gh.Ptr("Issue"), State: gh.Ptr("open")},
		}

		// Set Link header for pagination
		if page < 3 {
			w.Header().Set("Link", `<https://api.github.com/resource?page=2>; rel="next"`)
		}
		respondJSON(w, http.StatusOK, issues)
	}))
	defer srv.Close()

	f := githubadapter.New(srv.URL, "owner", "repo", srv.Client())

	issues, _, err := f.Issues().List(context.Background(), forge.IssueListOptions{Limit: 50})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// go-github uses the Link header to determine next page; our test server
	// sends a link header so pagination should work
	if len(issues) < 1 {
		t.Errorf("expected at least 1 issue, got %d", len(issues))
	}
}

// ---- Label service full test ----

func TestLabelServiceAllMethods(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		label := &gh.Label{
			Name:        "kind:feature",
			Color:       "0052cc",
			Description: gh.Ptr("New feature"),
		}
		respondJSON(w, http.StatusCreated, label)
	}))
	defer srv.Close()

	f := githubadapter.New(srv.URL, "owner", "repo", srv.Client())

	// Create
	created, err := f.Labels().Create(context.Background(), forge.LabelCreateOptions{
		Scope:       forge.String("kind"),
		Name:        "feature",
		Color:       forge.String("0052cc"),
		Description: forge.String("New feature"),
	})
	if err != nil {
		t.Fatalf("create error: %v", err)
	}
	if created.Scope != "kind" || created.Name != "feature" {
		t.Errorf("created: scope=%q name=%q", created.Scope, created.Name)
	}
}

// ---- Relation reads ----

func TestRelationBlockedBy(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		issues := []*gh.Issue{
			{
				Number:  gh.Ptr(2),
				Title:   gh.Ptr("Blocking issue"),
				State:   gh.Ptr("open"),
				User:    &gh.User{Login: gh.Ptr("alice")},
				HTMLURL: gh.Ptr("https://github.com/owner/repo/issues/2"),
			},
			{
				Number:  gh.Ptr(3),
				Title:   gh.Ptr("Another blocker"),
				State:   gh.Ptr("closed"),
				User:    &gh.User{Login: gh.Ptr("bob")},
				HTMLURL: gh.Ptr("https://github.com/owner/repo/issues/3"),
			},
		}
		respondJSON(w, http.StatusOK, issues)
	}))
	defer srv.Close()

	f := githubadapter.New(srv.URL, "owner", "repo", srv.Client())

	deps, err := f.Relations().BlockedBy(context.Background(), 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(deps) != 2 {
		t.Fatalf("expected 2 dependencies, got %d", len(deps))
	}
	if deps[0].Number != 2 {
		t.Errorf("dep[0].Number = %d, want 2", deps[0].Number)
	}
	if deps[0].Title != "Blocking issue" {
		t.Errorf("dep[0].Title = %q", deps[0].Title)
	}
	if deps[0].State != "open" {
		t.Errorf("dep[0].State = %q", deps[0].State)
	}
	if deps[0].Direction != forge.DirBlockedBy {
		t.Errorf("dep[0].Direction = %q, want %q", deps[0].Direction, forge.DirBlockedBy)
	}
	if deps[1].Number != 3 {
		t.Errorf("dep[1].Number = %d, want 3", deps[1].Number)
	}
	if deps[1].State != "closed" {
		t.Errorf("dep[1].State = %q", deps[1].State)
	}
}

func TestRelationBlocking(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		issues := []*gh.Issue{
			{
				Number:  gh.Ptr(5),
				Title:   gh.Ptr("Blocked by me"),
				State:   gh.Ptr("open"),
				User:    &gh.User{Login: gh.Ptr("carol")},
				HTMLURL: gh.Ptr("https://github.com/owner/repo/issues/5"),
			},
		}
		respondJSON(w, http.StatusOK, issues)
	}))
	defer srv.Close()

	f := githubadapter.New(srv.URL, "owner", "repo", srv.Client())

	deps, err := f.Relations().Blocking(context.Background(), 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(deps) != 1 {
		t.Fatalf("expected 1 dependency, got %d", len(deps))
	}
	if deps[0].Number != 5 {
		t.Errorf("dep[0].Number = %d, want 5", deps[0].Number)
	}
	if deps[0].Direction != forge.DirBlocks {
		t.Errorf("dep[0].Direction = %q, want %q", deps[0].Direction, forge.DirBlocks)
	}
}

func TestRelationChildren(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// SubIssue is the same shape as Issue; respond with regular issues.
		issues := []*gh.Issue{
			{
				Number:  gh.Ptr(10),
				Title:   gh.Ptr("Child task"),
				State:   gh.Ptr("open"),
				User:    &gh.User{Login: gh.Ptr("dev")},
				HTMLURL: gh.Ptr("https://github.com/owner/repo/issues/10"),
			},
		}
		respondJSON(w, http.StatusOK, issues)
	}))
	defer srv.Close()

	f := githubadapter.New(srv.URL, "owner", "repo", srv.Client())

	deps, err := f.Relations().Children(context.Background(), 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(deps) != 1 {
		t.Fatalf("expected 1 child, got %d", len(deps))
	}
	if deps[0].Number != 10 {
		t.Errorf("dep[0].Number = %d, want 10", deps[0].Number)
	}
	if deps[0].Title != "Child task" {
		t.Errorf("dep[0].Title = %q", deps[0].Title)
	}
	if deps[0].Direction != forge.DirChild {
		t.Errorf("dep[0].Direction = %q, want %q", deps[0].Direction, forge.DirChild)
	}
}

func TestRelationParentExists(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		issue := &gh.Issue{
			Number:  gh.Ptr(42),
			Title:   gh.Ptr("Parent issue"),
			State:   gh.Ptr("open"),
			User:    &gh.User{Login: gh.Ptr("admin")},
			HTMLURL: gh.Ptr("https://github.com/owner/repo/issues/42"),
		}
		respondJSON(w, http.StatusOK, issue)
	}))
	defer srv.Close()

	f := githubadapter.New(srv.URL, "owner", "repo", srv.Client())

	dep, err := f.Relations().Parent(context.Background(), 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dep == nil {
		t.Fatal("expected parent dependency, got nil")
	}
	if dep.Number != 42 {
		t.Errorf("dep.Number = %d, want 42", dep.Number)
	}
	if dep.Title != "Parent issue" {
		t.Errorf("dep.Title = %q", dep.Title)
	}
	if dep.Direction != forge.DirParent {
		t.Errorf("dep.Direction = %q, want %q", dep.Direction, forge.DirParent)
	}
}

func TestRelationParentNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"message": "Not Found"})
	}))
	defer srv.Close()

	f := githubadapter.New(srv.URL, "owner", "repo", srv.Client())

	dep, err := f.Relations().Parent(context.Background(), 10)
	if err != nil {
		t.Fatalf("unexpected error for no-parent case: %v", err)
	}
	if dep != nil {
		t.Errorf("expected nil dependency for no parent, got %+v", dep)
	}
}

func TestRelationEmpty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		respondJSON(w, http.StatusOK, []*gh.Issue{})
	}))
	defer srv.Close()

	f := githubadapter.New(srv.URL, "owner", "repo", srv.Client())

	// All list methods should return empty slices, not errors.
	deps, err := f.Relations().BlockedBy(context.Background(), 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(deps) != 0 {
		t.Errorf("expected 0 deps, got %d", len(deps))
	}
}

// ---- Context cancellation ----

func TestContextCancellation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Hang forever
		select {}
	}))
	defer srv.Close()

	f := githubadapter.New(srv.URL, "owner", "repo", srv.Client())

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, _, err := f.Issues().List(ctx, forge.IssueListOptions{})
	if err == nil {
		t.Fatal("expected error for canceled context")
	}
}
