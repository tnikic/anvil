package forgejo_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	gitea "code.gitea.io/sdk/gitea"
	"github.com/tnikic/anvil/internal/forge"
	forgejoadapter "github.com/tnikic/anvil/internal/forge/forgejo"
)

func respondJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// ---- Interface compliance ----

func TestForgejoForgeSatisfiesInterface(t *testing.T) {
	var _ forge.Forge = (*forgejoadapter.Forge)(nil)
}

// ---- Scope parsing (Forgejo uses "/" separator) ----

func TestParseLabelScopeForgejoScoped(t *testing.T) {
	scope, name := forge.ParseLabelScope("kind/bug", "/")
	if scope != "kind" {
		t.Errorf("scope = %q, want %q", scope, "kind")
	}
	if name != "bug" {
		t.Errorf("name = %q, want %q", name, "bug")
	}
}

func TestParseLabelScopeForgejoUnscoped(t *testing.T) {
	scope, name := forge.ParseLabelScope("good-first-issue", "/")
	if scope != "" {
		t.Errorf("scope should be empty, got %q", scope)
	}
	if name != "good-first-issue" {
		t.Errorf("name = %q, want %q", name, "good-first-issue")
	}
}

func TestParseLabelScopeForgejoMultipleSlashes(t *testing.T) {
	// Only split on first slash
	scope, name := forge.ParseLabelScope("a/b/c", "/")
	if scope != "a" {
		t.Errorf("scope = %q, want %q", scope, "a")
	}
	if name != "b/c" {
		t.Errorf("name = %q, want %q", name, "b/c")
	}
}

func TestLabelFullNameForgejoScoped(t *testing.T) {
	full := forge.LabelFullName("kind", "bug", "/")
	if full != "kind/bug" {
		t.Errorf("full = %q, want %q", full, "kind/bug")
	}
}

func TestLabelFullNameForgejoUnscoped(t *testing.T) {
	full := forge.LabelFullName("", "good-first-issue", "/")
	if full != "good-first-issue" {
		t.Errorf("full = %q, want %q", full, "good-first-issue")
	}
}

// ---- Issue mapping ----

func TestIssueListMapping(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		issues := []*gitea.Issue{
			{
				Index:   1,
				Title:   "Fix login timeout",
				State:   "open",
				Body:    "Users are logged out after 5 minutes.",
				Poster:  &gitea.User{UserName: "alice"},
				HTMLURL: "https://codeberg.org/owner/repo/issues/1",
				Created: time.Now(),
				Updated: time.Now(),
				Labels: []*gitea.Label{
					{Name: "kind/bug", Color: "ff0000", Description: "A bug", Exclusive: true},
					{Name: "good-first-issue", Color: "7057ff", Description: "Good for newcomers"},
				},
			},
		}
		respondJSON(w, http.StatusOK, issues)
	}))
	defer srv.Close()

	f := forgejoadapter.New(srv.URL, "owner", "repo", srv.Client())

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
	if issues[0].Body != "Users are logged out after 5 minutes." {
		t.Errorf("body = %q", issues[0].Body)
	}
	if issues[0].Author != "alice" {
		t.Errorf("author = %q", issues[0].Author)
	}
	if issues[0].URL != "https://codeberg.org/owner/repo/issues/1" {
		t.Errorf("url = %q", issues[0].URL)
	}
	if len(issues[0].Labels) != 2 {
		t.Fatalf("expected 2 labels, got %d", len(issues[0].Labels))
	}
	// First label: kind/bug -> scope=kind, name=bug
	if issues[0].Labels[0].Scope != "kind" || issues[0].Labels[0].Name != "bug" {
		t.Errorf("label 0: scope=%q name=%q", issues[0].Labels[0].Scope, issues[0].Labels[0].Name)
	}
	if !issues[0].Labels[0].Exclusive {
		t.Errorf("label 0: expected exclusive=true")
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
		issue := &gitea.Issue{
			Index:   42,
			Title:   "Test Issue",
			State:   "closed",
			Body:    "Full body content",
			Poster:  &gitea.User{UserName: "bob"},
			HTMLURL: "https://codeberg.org/owner/repo/issues/42",
			Created: time.Now(),
			Updated: time.Now(),
		}
		respondJSON(w, http.StatusOK, issue)
	}))
	defer srv.Close()

	f := forgejoadapter.New(srv.URL, "owner", "repo", srv.Client())

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
		issue := &gitea.Issue{
			Index:   99,
			Title:   "Created Issue",
			State:   "open",
			Poster:  &gitea.User{UserName: "alice"},
			HTMLURL: "https://codeberg.org/owner/repo/issues/99",
			Created: time.Now(),
			Updated: time.Now(),
		}
		respondJSON(w, http.StatusCreated, issue)
	}))
	defer srv.Close()

	f := forgejoadapter.New(srv.URL, "owner", "repo", srv.Client())

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
		issue := &gitea.Issue{
			Index: 42,
			State: "closed",
		}
		respondJSON(w, http.StatusOK, issue)
	}))
	defer srv.Close()

	f := forgejoadapter.New(srv.URL, "owner", "repo", srv.Client())

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
		issue := &gitea.Issue{
			Index: 42,
			State: "open",
		}
		respondJSON(w, http.StatusOK, issue)
	}))
	defer srv.Close()

	f := forgejoadapter.New(srv.URL, "owner", "repo", srv.Client())

	reopened, err := f.Issues().Reopen(context.Background(), forge.IssueReopenOptions{Number: 42})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reopened.State != "open" {
		t.Errorf("state = %q, want open", reopened.State)
	}
}

// ---- Sub-issue title convention ----

func TestSubIssueParentParsing(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		issue := &gitea.Issue{
			Index:   2,
			Title:   "[parent:1] Sub-task for login",
			State:   "open",
			Poster:  &gitea.User{UserName: "bob"},
			HTMLURL: "https://codeberg.org/owner/repo/issues/2",
			Created: time.Now(),
			Updated: time.Now(),
		}
		respondJSON(w, http.StatusOK, issue)
	}))
	defer srv.Close()

	f := forgejoadapter.New(srv.URL, "owner", "repo", srv.Client())

	issue, err := f.Issues().Get(context.Background(), forge.IssueGetOptions{Number: 2})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if issue.Parent == nil {
		t.Fatal("expected Parent to be set")
	}
	if *issue.Parent != 1 {
		t.Errorf("parent = %d, want 1", *issue.Parent)
	}
	if issue.Title != "Sub-task for login" {
		t.Errorf("title = %q, want %q", issue.Title, "Sub-task for login")
	}
}

func TestSubIssueNoParentParsing(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		issue := &gitea.Issue{
			Index:   42,
			Title:   "Regular issue",
			State:   "open",
			Poster:  &gitea.User{UserName: "bob"},
			HTMLURL: "https://codeberg.org/owner/repo/issues/42",
			Created: time.Now(),
			Updated: time.Now(),
		}
		respondJSON(w, http.StatusOK, issue)
	}))
	defer srv.Close()

	f := forgejoadapter.New(srv.URL, "owner", "repo", srv.Client())

	issue, err := f.Issues().Get(context.Background(), forge.IssueGetOptions{Number: 42})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if issue.Parent != nil {
		t.Errorf("parent should be nil, got %d", *issue.Parent)
	}
	if issue.Title != "Regular issue" {
		t.Errorf("title = %q", issue.Title)
	}
}

func TestSubIssueParentWithSpace(t *testing.T) {
	// [parent:1] with no space before title
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		issue := &gitea.Issue{
			Index:   2,
			Title:   "[parent:1]Sub-task no space",
			State:   "open",
			Poster:  &gitea.User{UserName: "bob"},
			HTMLURL: "https://codeberg.org/owner/repo/issues/2",
			Created: time.Now(),
			Updated: time.Now(),
		}
		respondJSON(w, http.StatusOK, issue)
	}))
	defer srv.Close()

	f := forgejoadapter.New(srv.URL, "owner", "repo", srv.Client())

	issue, err := f.Issues().Get(context.Background(), forge.IssueGetOptions{Number: 2})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if issue.Parent == nil || *issue.Parent != 1 {
		t.Errorf("parent should be 1, got %v", issue.Parent)
	}
	if issue.Title != "Sub-task no space" {
		t.Errorf("title = %q, want %q", issue.Title, "Sub-task no space")
	}
}

// ---- Error translation ----

func TestErrorTranslation401(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"message": "unauthorized"})
	}))
	defer srv.Close()

	f := forgejoadapter.New(srv.URL, "owner", "repo", srv.Client())

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
	var se forge.StructuredError
	if !errors.As(err, &se) || !strings.Contains(se.Help(), "auth set") {
		t.Errorf("error should have a help hint via StructuredError: %s", errStr)
	}
}

func TestErrorTranslation404(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"message": "Not Found"})
	}))
	defer srv.Close()

	f := forgejoadapter.New(srv.URL, "owner", "repo", srv.Client())

	_, err := f.Issues().Get(context.Background(), forge.IssueGetOptions{Number: 999})
	if err == nil {
		t.Fatal("expected error for 404")
	}
	errStr := err.Error()
	if !strings.Contains(errStr, "not found") && !strings.Contains(errStr, "Not Found") {
		t.Errorf("error should mention not found: %s", errStr)
	}
	var se forge.StructuredError
	if !errors.As(err, &se) {
		t.Errorf("error should implement StructuredError: %T", err)
	}
}

func TestErrorTranslation403(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]string{"message": "Forbidden"})
	}))
	defer srv.Close()

	f := forgejoadapter.New(srv.URL, "owner", "repo", srv.Client())

	_, _, err := f.Issues().List(context.Background(), forge.IssueListOptions{})
	if err == nil {
		t.Fatal("expected error for 403")
	}
	errStr := err.Error()
	if !strings.Contains(errStr, "access denied") && !strings.Contains(errStr, "403") {
		t.Errorf("error should mention access denied or 403: %s", errStr)
	}
}

func TestErrorTranslation429(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_ = json.NewEncoder(w).Encode(map[string]string{"message": "Too Many Requests"})
	}))
	defer srv.Close()

	f := forgejoadapter.New(srv.URL, "owner", "repo", srv.Client())

	_, _, err := f.Issues().List(context.Background(), forge.IssueListOptions{})
	if err == nil {
		t.Fatal("expected error for 429")
	}
	errStr := err.Error()
	if !strings.Contains(errStr, "rate limit") {
		t.Errorf("error should mention rate limit: %s", errStr)
	}
}

// ---- Network error ----

func TestNetworkErrorNoSuchHost(t *testing.T) {
	f := forgejoadapter.New("http://no-such-host.invalid.local", "owner", "repo", &http.Client{})
	_, _, err := f.Issues().List(context.Background(), forge.IssueListOptions{})
	if err == nil {
		t.Fatal("expected error for no such host")
	}
	var se forge.StructuredError
	if !errors.As(err, &se) {
		t.Errorf("error should implement StructuredError, got %T: %v", err, err)
	}
}

// ---- CurrentUser ----

func TestCurrentUser(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := &gitea.User{
			UserName: "test-user",
		}
		respondJSON(w, http.StatusOK, user)
	}))
	defer srv.Close()

	f := forgejoadapter.New(srv.URL, "owner", "repo", srv.Client())

	login, err := f.CurrentUser(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if login != "test-user" {
		t.Errorf("login = %q, want %q", login, "test-user")
	}
}

// ---- Pagination ----

func TestIssueListPagination(t *testing.T) {
	page := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page++
		issues := []*gitea.Issue{
			{Index: int64(page * 10), Title: "Issue", State: "open"},
		}
		// Set Link header for pagination
		if page < 3 {
			w.Header().Set("Link", `<https://codeberg.org/api/v1/repos/owner/repo/issues?page=2>; rel="next"`)
		}
		respondJSON(w, http.StatusOK, issues)
	}))
	defer srv.Close()

	f := forgejoadapter.New(srv.URL, "owner", "repo", srv.Client())

	issues, _, err := f.Issues().List(context.Background(), forge.IssueListOptions{Limit: 50})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(issues) < 1 {
		t.Errorf("expected at least 1 issue, got %d", len(issues))
	}
}

// ---- Idempotent close/reopen ----

func TestCloseAlreadyClosed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		issue := &gitea.Issue{
			Index: 42,
			State: "closed",
		}
		respondJSON(w, http.StatusOK, issue)
	}))
	defer srv.Close()

	f := forgejoadapter.New(srv.URL, "owner", "repo", srv.Client())

	closed, err := f.Issues().Close(context.Background(), forge.IssueCloseOptions{Number: 42})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if closed.State != "closed" {
		t.Errorf("state = %q, want closed", closed.State)
	}
}

func TestReopenAlreadyOpen(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		issue := &gitea.Issue{
			Index: 42,
			State: "open",
		}
		respondJSON(w, http.StatusOK, issue)
	}))
	defer srv.Close()

	f := forgejoadapter.New(srv.URL, "owner", "repo", srv.Client())

	reopened, err := f.Issues().Reopen(context.Background(), forge.IssueReopenOptions{Number: 42})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reopened.State != "open" {
		t.Errorf("state = %q, want open", reopened.State)
	}
}

// ---- Label CRUD ----

func TestLabelListMapping(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		labels := []*gitea.Label{
			{ID: 1, Name: "kind/bug", Color: "ff0000", Description: "A bug", Exclusive: true},
			{ID: 2, Name: "good-first-issue", Color: "7057ff", Description: "Good for newcomers"},
		}
		respondJSON(w, http.StatusOK, labels)
	}))
	defer srv.Close()

	f := forgejoadapter.New(srv.URL, "owner", "repo", srv.Client())

	labels, err := f.Labels().List(context.Background(), forge.LabelListOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(labels) != 2 {
		t.Fatalf("expected 2 labels, got %d", len(labels))
	}
	// Scoped label: kind/bug -> scope=kind, name=bug
	if labels[0].Scope != "kind" || labels[0].Name != "bug" {
		t.Errorf("label 0: scope=%q name=%q, want scope=kind name=bug", labels[0].Scope, labels[0].Name)
	}
	if labels[0].Color != "ff0000" {
		t.Errorf("label 0: color=%q, want ff0000", labels[0].Color)
	}
	if labels[0].Description != "A bug" {
		t.Errorf("label 0: description=%q, want 'A bug'", labels[0].Description)
	}
	if !labels[0].Exclusive {
		t.Errorf("label 0: expected exclusive=true")
	}
	// Unscoped label
	if labels[1].Scope != "" || labels[1].Name != "good-first-issue" {
		t.Errorf("label 1: scope=%q name=%q, want scope=<empty> name=good-first-issue", labels[1].Scope, labels[1].Name)
	}
}

func TestLabelCreateScoped(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// The SDK sends a POST with the full name as "scope/name"
		label := &gitea.Label{
			ID:          1,
			Name:        "kind/bug",
			Color:       "ff0000",
			Description: "A bug",
			Exclusive:   true,
		}
		respondJSON(w, http.StatusCreated, label)
	}))
	defer srv.Close()

	f := forgejoadapter.New(srv.URL, "owner", "repo", srv.Client())

	created, err := f.Labels().Create(context.Background(), forge.LabelCreateOptions{
		Scope:       forge.String("kind"),
		Name:        "bug",
		Color:       forge.String("ff0000"),
		Description: forge.String("A bug"),
		Exclusive:   forge.Bool(true),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if created.Scope != "kind" || created.Name != "bug" {
		t.Errorf("scope=%q name=%q, want scope=kind name=bug", created.Scope, created.Name)
	}
	if created.Color != "ff0000" {
		t.Errorf("color=%q", created.Color)
	}
	if created.Description != "A bug" {
		t.Errorf("description=%q", created.Description)
	}
	if !created.Exclusive {
		t.Errorf("expected exclusive=true")
	}
}

func TestLabelCreateUnscoped(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		label := &gitea.Label{
			ID:    1,
			Name:  "good-first-issue",
			Color: "7057ff",
		}
		respondJSON(w, http.StatusCreated, label)
	}))
	defer srv.Close()

	f := forgejoadapter.New(srv.URL, "owner", "repo", srv.Client())

	created, err := f.Labels().Create(context.Background(), forge.LabelCreateOptions{
		Name:  "good-first-issue",
		Color: forge.String("7057ff"),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if created.Scope != "" || created.Name != "good-first-issue" {
		t.Errorf("scope=%q name=%q, want scope=<empty> name=good-first-issue", created.Scope, created.Name)
	}
}

func TestLabelCreateRoundTripScoped(t *testing.T) {
	// Create a scoped label and then list it — it should appear with
	// the same scope+name it was created with.
	var storedLabels []*gitea.Label
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			// Simulate storing the label
			newLabel := &gitea.Label{
				ID:    int64(len(storedLabels) + 1),
				Name:  "kind/bug",
				Color: "ff0000",
			}
			storedLabels = append(storedLabels, newLabel)
			respondJSON(w, http.StatusCreated, newLabel)
			return
		}
		// GET — list
		respondJSON(w, http.StatusOK, storedLabels)
	}))
	defer srv.Close()

	f := forgejoadapter.New(srv.URL, "owner", "repo", srv.Client())

	_, err := f.Labels().Create(context.Background(), forge.LabelCreateOptions{
		Scope: forge.String("kind"),
		Name:  "bug",
		Color: forge.String("ff0000"),
	})
	if err != nil {
		t.Fatalf("create error: %v", err)
	}

	labels, err := f.Labels().List(context.Background(), forge.LabelListOptions{})
	if err != nil {
		t.Fatalf("list error: %v", err)
	}
	if len(labels) != 1 {
		t.Fatalf("expected 1 label, got %d", len(labels))
	}
	if labels[0].Scope != "kind" || labels[0].Name != "bug" {
		t.Errorf("round-trip: scope=%q name=%q, want scope=kind name=bug", labels[0].Scope, labels[0].Name)
	}
}

func TestLabelCreateRoundTripUnscoped(t *testing.T) {
	var storedLabels []*gitea.Label
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			newLabel := &gitea.Label{
				ID:    int64(len(storedLabels) + 1),
				Name:  "good-first-issue",
				Color: "7057ff",
			}
			storedLabels = append(storedLabels, newLabel)
			respondJSON(w, http.StatusCreated, newLabel)
			return
		}
		respondJSON(w, http.StatusOK, storedLabels)
	}))
	defer srv.Close()

	f := forgejoadapter.New(srv.URL, "owner", "repo", srv.Client())

	_, err := f.Labels().Create(context.Background(), forge.LabelCreateOptions{
		Name:  "good-first-issue",
		Color: forge.String("7057ff"),
	})
	if err != nil {
		t.Fatalf("create error: %v", err)
	}

	labels, err := f.Labels().List(context.Background(), forge.LabelListOptions{})
	if err != nil {
		t.Fatalf("list error: %v", err)
	}
	if len(labels) != 1 {
		t.Fatalf("expected 1 label, got %d", len(labels))
	}
	if labels[0].Scope != "" || labels[0].Name != "good-first-issue" {
		t.Errorf("round-trip: scope=%q name=%q, want scope=<empty> name=good-first-issue", labels[0].Scope, labels[0].Name)
	}
}

func TestLabelUpdateResolvesID(t *testing.T) {
	// Update must first list labels to find the ID, then PATCH by ID.
	// We verify it hits the PATCH /labels/<id> endpoint.
	var patchURL string
	repoLabels := []*gitea.Label{
		{ID: 42, Name: "kind/bug", Color: "ff0000"},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/api/v1/repos/owner/repo/labels" {
			respondJSON(w, http.StatusOK, repoLabels)
			return
		}
		if r.Method == "PATCH" {
			patchURL = r.URL.Path
			updated := &gitea.Label{
				ID:    42,
				Name:  "kind/defect",
				Color: "0000ff",
			}
			respondJSON(w, http.StatusOK, updated)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	f := forgejoadapter.New(srv.URL, "owner", "repo", srv.Client())

	newName := "defect"
	newColor := "0000ff"
	updated, err := f.Labels().Update(context.Background(), forge.LabelUpdateOptions{
		Scope:   "kind",
		Name:    "bug",
		NewName: &newName,
		Color:   &newColor,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated.Scope != "kind" || updated.Name != "defect" {
		t.Errorf("updated: scope=%q name=%q, want scope=kind name=defect", updated.Scope, updated.Name)
	}
	if patchURL != "/api/v1/repos/owner/repo/labels/42" {
		t.Errorf("PATCH URL = %q, want /api/v1/repos/owner/repo/labels/42", patchURL)
	}
}

func TestLabelDeleteResolvesID(t *testing.T) {
	// Delete must first list labels to find the ID, then DELETE by ID.
	var deleteURL string
	repoLabels := []*gitea.Label{
		{ID: 99, Name: "kind/bug", Color: "ff0000"},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/api/v1/repos/owner/repo/labels" {
			respondJSON(w, http.StatusOK, repoLabels)
			return
		}
		if r.Method == "DELETE" {
			deleteURL = r.URL.Path
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	f := forgejoadapter.New(srv.URL, "owner", "repo", srv.Client())

	err := f.Labels().Delete(context.Background(), forge.LabelDeleteOptions{
		Scope: "kind",
		Name:  "bug",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if deleteURL != "/api/v1/repos/owner/repo/labels/99" {
		t.Errorf("DELETE URL = %q, want /api/v1/repos/owner/repo/labels/99", deleteURL)
	}
}

func TestLabelDeleteNotFound(t *testing.T) {
	// When the label doesn't exist in the list, Delete returns StructuredError.
	var repoLabels []*gitea.Label // empty — label not found
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/api/v1/repos/owner/repo/labels" {
			respondJSON(w, http.StatusOK, repoLabels)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	f := forgejoadapter.New(srv.URL, "owner", "repo", srv.Client())

	err := f.Labels().Delete(context.Background(), forge.LabelDeleteOptions{
		Scope: "kind",
		Name:  "nonexistent",
	})
	if err == nil {
		t.Fatal("expected error for nonexistent label")
	}
	var se forge.StructuredError
	if !errors.As(err, &se) {
		t.Errorf("error should implement StructuredError, got %T: %v", err, err)
	}
	if !strings.Contains(se.Message(), "not found") {
		t.Errorf("message should mention 'not found': %s", se.Message())
	}
}

func TestLabelUpdateNotFound(t *testing.T) {
	// When the label doesn't exist in the list, Update returns StructuredError.
	var repoLabels []*gitea.Label // empty — label not found
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/api/v1/repos/owner/repo/labels" {
			respondJSON(w, http.StatusOK, repoLabels)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	f := forgejoadapter.New(srv.URL, "owner", "repo", srv.Client())

	newName := "defect"
	_, err := f.Labels().Update(context.Background(), forge.LabelUpdateOptions{
		Scope:   "kind",
		Name:    "nonexistent",
		NewName: &newName,
	})
	if err == nil {
		t.Fatal("expected error for nonexistent label")
	}
	var se forge.StructuredError
	if !errors.As(err, &se) {
		t.Errorf("error should implement StructuredError, got %T: %v", err, err)
	}
	if !strings.Contains(se.Message(), "not found") {
		t.Errorf("message should mention 'not found': %s", se.Message())
	}
}

func TestLabelListErrorTranslation404(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"message": "Not Found"})
	}))
	defer srv.Close()

	f := forgejoadapter.New(srv.URL, "owner", "repo", srv.Client())

	_, err := f.Labels().List(context.Background(), forge.LabelListOptions{})
	if err == nil {
		t.Fatal("expected error for 404")
	}
	var se forge.StructuredError
	if !errors.As(err, &se) {
		t.Errorf("error should implement StructuredError, got %T: %v", err, err)
	}
	if !strings.Contains(se.Message(), "not found") {
		t.Errorf("message should mention 'not found': %s", se.Message())
	}
}

func TestLabelCreateErrorTranslation401(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"message": "unauthorized"})
	}))
	defer srv.Close()

	f := forgejoadapter.New(srv.URL, "owner", "repo", srv.Client())

	_, err := f.Labels().Create(context.Background(), forge.LabelCreateOptions{
		Name:  "test",
		Color: forge.String("ff0000"),
	})
	if err == nil {
		t.Fatal("expected error for 401")
	}
	var se forge.StructuredError
	if !errors.As(err, &se) {
		t.Errorf("error should implement StructuredError, got %T: %v", err, err)
	}
	if !strings.Contains(se.Message(), "authentication failed") {
		t.Errorf("message should mention 'authentication failed': %s", se.Message())
	}
}

// ---- PR mapping ----

func TestPRListMapping(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		prs := []*gitea.PullRequest{
			{
				Index:   10,
				Title:   "Add OAuth support",
				State:   "open",
				Body:    "Implements OAuth 2.0 flow.",
				Draft:   true,
				HTMLURL: "https://codeberg.org/owner/repo/pulls/10",
				Poster:  &gitea.User{UserName: "alice"},
				Base:    &gitea.PRBranchInfo{Ref: "main"},
				Head:    &gitea.PRBranchInfo{Ref: "feat/oauth"},
				Created: &time.Time{},
				Updated: &time.Time{},
			},
			{
				Index:     11,
				Title:     "Fix login timeout",
				State:     "closed",
				HasMerged: true,
				HTMLURL:   "https://codeberg.org/owner/repo/pulls/11",
				Poster:    &gitea.User{UserName: "bob"},
				Base:      &gitea.PRBranchInfo{Ref: "main"},
				Head:      &gitea.PRBranchInfo{Ref: "fix/timeout"},
				Created:   &time.Time{},
				Updated:   &time.Time{},
			},
		}
		respondJSON(w, http.StatusOK, prs)
	}))
	defer srv.Close()

	f := forgejoadapter.New(srv.URL, "owner", "repo", srv.Client())

	prs, meta, err := f.PRs().List(context.Background(), forge.PRListOptions{State: "all"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(prs) != 2 {
		t.Fatalf("expected 2 PRs, got %d", len(prs))
	}

	// First PR (draft, open)
	if prs[0].Number != 10 {
		t.Errorf("pr[0].Number = %d, want 10", prs[0].Number)
	}
	if prs[0].Title != "Add OAuth support" {
		t.Errorf("pr[0].Title = %q", prs[0].Title)
	}
	if prs[0].State != "open" {
		t.Errorf("pr[0].State = %q, want open", prs[0].State)
	}
	if prs[0].Body != "Implements OAuth 2.0 flow." {
		t.Errorf("pr[0].Body = %q", prs[0].Body)
	}
	if prs[0].BaseRef != "main" {
		t.Errorf("pr[0].BaseRef = %q, want main", prs[0].BaseRef)
	}
	if prs[0].HeadRef != "feat/oauth" {
		t.Errorf("pr[0].HeadRef = %q, want feat/oauth", prs[0].HeadRef)
	}
	if prs[0].Author != "alice" {
		t.Errorf("pr[0].Author = %q, want alice", prs[0].Author)
	}
	if prs[0].URL != "https://codeberg.org/owner/repo/pulls/10" {
		t.Errorf("pr[0].URL = %q", prs[0].URL)
	}
	if prs[0].Extras == nil || prs[0].Extras["draft"] != true {
		t.Errorf("pr[0].Extras[draft] should be true, got %v", prs[0].Extras)
	}

	// Second PR (merged — HasMerged=true maps to state=merged)
	if prs[1].State != "merged" {
		t.Errorf("pr[1].State = %q, want merged", prs[1].State)
	}
	if prs[1].Author != "bob" {
		t.Errorf("pr[1].Author = %q, want bob", prs[1].Author)
	}

	if meta.Count != 2 {
		t.Errorf("meta.Count = %d, want 2", meta.Count)
	}
}

func TestPRGetMapping(t *testing.T) {
	now := time.Now()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pr := &gitea.PullRequest{
			Index:   42,
			Title:   "Add OAuth support",
			State:   "open",
			Draft:   true,
			HTMLURL: "https://codeberg.org/owner/repo/pulls/42",
			Poster:  &gitea.User{UserName: "alice"},
			Base:    &gitea.PRBranchInfo{Ref: "main"},
			Head:    &gitea.PRBranchInfo{Ref: "feat/oauth"},
			Created: &now,
			Updated: &now,
		}
		respondJSON(w, http.StatusOK, pr)
	}))
	defer srv.Close()

	f := forgejoadapter.New(srv.URL, "owner", "repo", srv.Client())

	pr, err := f.PRs().Get(context.Background(), forge.PRGetOptions{Number: 42})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pr.Number != 42 {
		t.Errorf("Number = %d, want 42", pr.Number)
	}
	if pr.Title != "Add OAuth support" {
		t.Errorf("Title = %q", pr.Title)
	}
	if pr.State != "open" {
		t.Errorf("State = %q, want open", pr.State)
	}
	if pr.BaseRef != "main" {
		t.Errorf("BaseRef = %q, want main", pr.BaseRef)
	}
	if pr.HeadRef != "feat/oauth" {
		t.Errorf("HeadRef = %q, want feat/oauth", pr.HeadRef)
	}
	if pr.Author != "alice" {
		t.Errorf("Author = %q, want alice", pr.Author)
	}
	if pr.Extras == nil || pr.Extras["draft"] != true {
		t.Errorf("Extras[draft] should be true, got %v", pr.Extras)
	}
}

func TestPRCreateMapping(t *testing.T) {
	var capturedBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			_ = json.NewDecoder(r.Body).Decode(&capturedBody)
		}
		now := time.Now()
		pr := &gitea.PullRequest{
			Index:   99,
			Title:   "[auth:1/2] Add OAuth support",
			State:   "open",
			HTMLURL: "https://codeberg.org/owner/repo/pulls/99",
			Poster:  &gitea.User{UserName: "alice"},
			Base:    &gitea.PRBranchInfo{Ref: "main"},
			Head:    &gitea.PRBranchInfo{Ref: "feat/oauth"},
			Created: &now,
			Updated: &now,
		}
		respondJSON(w, http.StatusCreated, pr)
	}))
	defer srv.Close()

	f := forgejoadapter.New(srv.URL, "owner", "repo", srv.Client())

	created, err := f.PRs().Create(context.Background(), forge.PRCreateOptions{
		Title:   forge.String("[auth:1/2] Add OAuth support"),
		Body:    forge.String("Implements OAuth 2.0."),
		HeadRef: forge.String("feat/oauth"),
		BaseRef: forge.String("main"),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if created.Number != 99 {
		t.Errorf("Number = %d, want 99", created.Number)
	}
	// Stack prefix in title should be preserved (passed through as-is).
	if created.Title != "[auth:1/2] Add OAuth support" {
		t.Errorf("Title = %q, want [auth:1/2] Add OAuth support", created.Title)
	}
	// Verify the request body contains the stack prefix.
	if capturedBody["title"] != "[auth:1/2] Add OAuth support" {
		t.Errorf("captured title = %q, want [auth:1/2] Add OAuth support", capturedBody["title"])
	}
	if capturedBody["head"] != "feat/oauth" {
		t.Errorf("captured head = %q, want feat/oauth", capturedBody["head"])
	}
	if capturedBody["base"] != "main" {
		t.Errorf("captured base = %q, want main", capturedBody["base"])
	}
}

func TestPRUpdateMapping(t *testing.T) {
	var patchedTitle string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "PATCH" {
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			if t, ok := body["title"].(string); ok {
				patchedTitle = t
			}
		}
		now := time.Now()
		pr := &gitea.PullRequest{
			Index:   42,
			Title:   patchedTitle,
			State:   "open",
			HTMLURL: "https://codeberg.org/owner/repo/pulls/42",
			Poster:  &gitea.User{UserName: "alice"},
			Created: &now,
			Updated: &now,
		}
		respondJSON(w, http.StatusOK, pr)
	}))
	defer srv.Close()

	f := forgejoadapter.New(srv.URL, "owner", "repo", srv.Client())

	updated, err := f.PRs().Update(context.Background(), forge.PRUpdateOptions{
		Number: 42,
		Title:  forge.String("Updated PR title"),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated.Title != "Updated PR title" {
		t.Errorf("Title = %q, want Updated PR title", updated.Title)
	}
	if patchedTitle != "Updated PR title" {
		t.Errorf("patched title = %q", patchedTitle)
	}
}

func TestPRMergeMapping(t *testing.T) {
	mergeCalled := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/merge") && r.Method == http.MethodPost {
			mergeCalled = true
			w.WriteHeader(http.StatusOK)
			return
		}
		// GET after merge returns merged PR
		now := time.Now()
		pr := &gitea.PullRequest{
			Index:     42,
			Title:     "Add OAuth support",
			State:     "closed",
			HasMerged: true,
			HTMLURL:   "https://codeberg.org/owner/repo/pulls/42",
			Poster:    &gitea.User{UserName: "alice"},
			Base:      &gitea.PRBranchInfo{Ref: "main"},
			Head:      &gitea.PRBranchInfo{Ref: "feat/oauth"},
			Created:   &now,
			Updated:   &now,
		}
		respondJSON(w, http.StatusOK, pr)
	}))
	defer srv.Close()

	f := forgejoadapter.New(srv.URL, "owner", "repo", srv.Client())

	merged, err := f.PRs().Merge(context.Background(), forge.PRMergeOptions{
		Number: 42,
		Method: forge.String("merge"),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !mergeCalled {
		t.Error("merge was not called")
	}
	if merged.State != "merged" {
		t.Errorf("State = %q, want merged", merged.State)
	}
	if merged.Number != 42 {
		t.Errorf("Number = %d, want 42", merged.Number)
	}
}

func TestPRMergeFetchFails(t *testing.T) {
	// If the GET after merge fails, return a synthesized merged PR.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/merge") && r.Method == http.MethodPost {
			w.WriteHeader(http.StatusOK)
			return
		}
		// GET fails
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	f := forgejoadapter.New(srv.URL, "owner", "repo", srv.Client())

	merged, err := f.PRs().Merge(context.Background(), forge.PRMergeOptions{Number: 42})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if merged.State != "merged" {
		t.Errorf("State = %q, want merged", merged.State)
	}
	if merged.Number != 42 {
		t.Errorf("Number = %d, want 42", merged.Number)
	}
}

func TestPRMergeError(t *testing.T) {
	// Simulate a merge that cannot complete (e.g., conflict).
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/merge") && r.Method == http.MethodPost {
			w.WriteHeader(http.StatusConflict)
			_ = json.NewEncoder(w).Encode(map[string]string{"message": "merge conflict"})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	f := forgejoadapter.New(srv.URL, "owner", "repo", srv.Client())

	_, err := f.PRs().Merge(context.Background(), forge.PRMergeOptions{Number: 42})
	if err == nil {
		t.Fatal("expected error for merge conflict")
	}
	var se forge.StructuredError
	if !errors.As(err, &se) {
		t.Errorf("error should implement StructuredError, got %T: %v", err, err)
	}
	if !strings.Contains(se.Message(), "could not be merged") {
		t.Errorf("message should mention merge failure: %s", se.Message())
	}
}

func TestPRCloseMapping(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		now := time.Now()
		pr := &gitea.PullRequest{
			Index:   42,
			State:   "closed",
			HTMLURL: "https://codeberg.org/owner/repo/pulls/42",
			Poster:  &gitea.User{UserName: "alice"},
			Created: &now,
			Updated: &now,
		}
		respondJSON(w, http.StatusOK, pr)
	}))
	defer srv.Close()

	f := forgejoadapter.New(srv.URL, "owner", "repo", srv.Client())

	closed, err := f.PRs().Close(context.Background(), forge.PRCloseOptions{Number: 42})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if closed.State != "closed" {
		t.Errorf("State = %q, want closed", closed.State)
	}
	if closed.Number != 42 {
		t.Errorf("Number = %d, want 42", closed.Number)
	}
}

func TestPRCloseIdempotent(t *testing.T) {
	// Closing an already-closed PR should succeed (idempotent).
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		now := time.Now()
		pr := &gitea.PullRequest{
			Index:   42,
			State:   "closed",
			HTMLURL: "https://codeberg.org/owner/repo/pulls/42",
			Poster:  &gitea.User{UserName: "alice"},
			Created: &now,
			Updated: &now,
		}
		respondJSON(w, http.StatusOK, pr)
	}))
	defer srv.Close()

	f := forgejoadapter.New(srv.URL, "owner", "repo", srv.Client())

	closed, err := f.PRs().Close(context.Background(), forge.PRCloseOptions{Number: 42})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if closed.State != "closed" {
		t.Errorf("State = %q, want closed", closed.State)
	}
}

func TestPRFieldMappingDraft(t *testing.T) {
	// Verify draft → Extras["draft"] mapping.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pr := &gitea.PullRequest{
			Index:   1,
			Title:   "Draft PR",
			State:   "open",
			Draft:   true,
			HTMLURL: "https://codeberg.org/owner/repo/pulls/1",
			Poster:  &gitea.User{UserName: "alice"},
			Base:    &gitea.PRBranchInfo{Ref: "main"},
			Head:    &gitea.PRBranchInfo{Ref: "feat/x"},
			Created: &time.Time{},
			Updated: &time.Time{},
		}
		respondJSON(w, http.StatusOK, pr)
	}))
	defer srv.Close()

	f := forgejoadapter.New(srv.URL, "owner", "repo", srv.Client())

	pr, err := f.PRs().Get(context.Background(), forge.PRGetOptions{Number: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pr.Extras == nil || pr.Extras["draft"] != true {
		t.Errorf("Extras[draft] should be true, got %v", pr.Extras)
	}
}

func TestPRFieldMappingNonDraft(t *testing.T) {
	// Non-draft PR should not have the draft extras key set.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pr := &gitea.PullRequest{
			Index:   1,
			Title:   "Regular PR",
			State:   "open",
			Draft:   false,
			HTMLURL: "https://codeberg.org/owner/repo/pulls/1",
			Poster:  &gitea.User{UserName: "alice"},
			Base:    &gitea.PRBranchInfo{Ref: "main"},
			Head:    &gitea.PRBranchInfo{Ref: "feat/x"},
			Created: &time.Time{},
			Updated: &time.Time{},
		}
		respondJSON(w, http.StatusOK, pr)
	}))
	defer srv.Close()

	f := forgejoadapter.New(srv.URL, "owner", "repo", srv.Client())

	pr, err := f.PRs().Get(context.Background(), forge.PRGetOptions{Number: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pr.Extras != nil && pr.Extras["draft"] != nil {
		t.Errorf("Extras[draft] should not be set for non-draft PR")
	}
}

func TestPRFieldMappingBaseHeadRef(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pr := &gitea.PullRequest{
			Index:   1,
			Title:   "PR",
			State:   "open",
			HTMLURL: "https://codeberg.org/owner/repo/pulls/1",
			Poster:  &gitea.User{UserName: "alice"},
			Base:    &gitea.PRBranchInfo{Ref: "develop"},
			Head:    &gitea.PRBranchInfo{Ref: "feat/new-thing"},
			Created: &time.Time{},
			Updated: &time.Time{},
		}
		respondJSON(w, http.StatusOK, pr)
	}))
	defer srv.Close()

	f := forgejoadapter.New(srv.URL, "owner", "repo", srv.Client())

	pr, err := f.PRs().Get(context.Background(), forge.PRGetOptions{Number: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pr.BaseRef != "develop" {
		t.Errorf("BaseRef = %q, want develop", pr.BaseRef)
	}
	if pr.HeadRef != "feat/new-thing" {
		t.Errorf("HeadRef = %q, want feat/new-thing", pr.HeadRef)
	}
}

// ---- PR error translation ----

func TestPRErrorTranslation404(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"message": "Not Found"})
	}))
	defer srv.Close()

	f := forgejoadapter.New(srv.URL, "owner", "repo", srv.Client())

	_, err := f.PRs().Get(context.Background(), forge.PRGetOptions{Number: 999})
	if err == nil {
		t.Fatal("expected error for 404")
	}
	var se forge.StructuredError
	if !errors.As(err, &se) {
		t.Errorf("error should implement StructuredError, got %T: %v", err, err)
	}
}

func TestPRErrorTranslation401(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"message": "unauthorized"})
	}))
	defer srv.Close()

	f := forgejoadapter.New(srv.URL, "owner", "repo", srv.Client())

	_, _, err := f.PRs().List(context.Background(), forge.PRListOptions{})
	if err == nil {
		t.Fatal("expected error for 401")
	}
	var se forge.StructuredError
	if !errors.As(err, &se) || !strings.Contains(se.Help(), "auth set") {
		t.Errorf("error should have a help hint via StructuredError: %s", err)
	}
}

func TestLabelUpdateWithNewScope(t *testing.T) {
	// When both NewScope and NewName are set, the full label name changes.
	repoLabels := []*gitea.Label{
		{ID: 1, Name: "kind/bug", Color: "ff0000"},
	}
	var patchedName string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			respondJSON(w, http.StatusOK, repoLabels)
			return
		}
		if r.Method == "PATCH" {
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			if n, ok := body["name"].(string); ok {
				patchedName = n
			}
			updated := &gitea.Label{
				ID:    1,
				Name:  patchedName,
				Color: "ff0000",
			}
			respondJSON(w, http.StatusOK, updated)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	f := forgejoadapter.New(srv.URL, "owner", "repo", srv.Client())

	newName := "feature"
	newScope := "type"
	updated, err := f.Labels().Update(context.Background(), forge.LabelUpdateOptions{
		Scope:    "kind",
		Name:     "bug",
		NewName:  &newName,
		NewScope: &newScope,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if patchedName != "type/feature" {
		t.Errorf("patched full name = %q, want type/feature", patchedName)
	}
	if updated.Scope != "type" || updated.Name != "feature" {
		t.Errorf("updated: scope=%q name=%q, want scope=type name=feature", updated.Scope, updated.Name)
	}
}

// ---- Comment mapping ----

func TestCommentListMapping(t *testing.T) {
	now := time.Now()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		comments := []*gitea.Comment{
			{
				ID:      1,
				Body:    "This is a comment.",
				HTMLURL: "https://codeberg.org/owner/repo/issues/1#issuecomment-1",
				Poster:  &gitea.User{UserName: "alice"},
				Created: now,
				Updated: now,
			},
			{
				ID:      2,
				Body:    "Second comment.",
				HTMLURL: "https://codeberg.org/owner/repo/issues/1#issuecomment-2",
				Poster:  &gitea.User{UserName: "bob"},
				Created: now,
				Updated: now,
			},
		}
		respondJSON(w, http.StatusOK, comments)
	}))
	defer srv.Close()

	f := forgejoadapter.New(srv.URL, "owner", "repo", srv.Client())

	comments, err := f.Comments().List(context.Background(), forge.CommentListOptions{IssueNumber: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(comments) != 2 {
		t.Fatalf("expected 2 comments, got %d", len(comments))
	}

	// First comment
	if comments[0].ID != 1 {
		t.Errorf("comment[0].ID = %d, want 1", comments[0].ID)
	}
	if comments[0].Body != "This is a comment." {
		t.Errorf("comment[0].Body = %q", comments[0].Body)
	}
	if comments[0].Author != "alice" {
		t.Errorf("comment[0].Author = %q, want alice", comments[0].Author)
	}
	if comments[0].System {
		t.Errorf("comment[0].System should be false")
	}
	if comments[0].URL != "https://codeberg.org/owner/repo/issues/1#issuecomment-1" {
		t.Errorf("comment[0].URL = %q", comments[0].URL)
	}

	// Second comment
	if comments[1].ID != 2 {
		t.Errorf("comment[1].ID = %d, want 2", comments[1].ID)
	}
	if comments[1].Author != "bob" {
		t.Errorf("comment[1].Author = %q, want bob", comments[1].Author)
	}
}

func TestCommentGetMapping(t *testing.T) {
	now := time.Now()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		comment := &gitea.Comment{
			ID:      42,
			Body:    "Single comment body",
			HTMLURL: "https://codeberg.org/owner/repo/issues/1#issuecomment-42",
			Poster:  &gitea.User{UserName: "alice"},
			Created: now,
			Updated: now,
		}
		respondJSON(w, http.StatusOK, comment)
	}))
	defer srv.Close()

	f := forgejoadapter.New(srv.URL, "owner", "repo", srv.Client())

	c, err := f.Comments().Get(context.Background(), forge.CommentGetOptions{IssueNumber: 1, CommentID: 42})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.ID != 42 {
		t.Errorf("ID = %d, want 42", c.ID)
	}
	if c.Body != "Single comment body" {
		t.Errorf("Body = %q", c.Body)
	}
	if c.Author != "alice" {
		t.Errorf("Author = %q, want alice", c.Author)
	}
	if c.System {
		t.Errorf("System should be false")
	}
	if c.URL != "https://codeberg.org/owner/repo/issues/1#issuecomment-42" {
		t.Errorf("URL = %q", c.URL)
	}
}

func TestCommentCreateMapping(t *testing.T) {
	now := time.Now()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		comment := &gitea.Comment{
			ID:      99,
			Body:    "New comment",
			HTMLURL: "https://codeberg.org/owner/repo/issues/1#issuecomment-99",
			Poster:  &gitea.User{UserName: "alice"},
			Created: now,
			Updated: now,
		}
		respondJSON(w, http.StatusCreated, comment)
	}))
	defer srv.Close()

	f := forgejoadapter.New(srv.URL, "owner", "repo", srv.Client())

	created, err := f.Comments().Create(context.Background(), forge.CommentCreateOptions{
		IssueNumber: 1,
		Body:        "New comment",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if created.ID != 99 {
		t.Errorf("ID = %d, want 99", created.ID)
	}
	if created.Body != "New comment" {
		t.Errorf("Body = %q", created.Body)
	}
	if created.Author != "alice" {
		t.Errorf("Author = %q, want alice", created.Author)
	}
	if created.System {
		t.Errorf("System should be false")
	}
}

func TestCommentUpdateMapping(t *testing.T) {
	var patchedBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "PATCH" {
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			if b, ok := body["body"].(string); ok {
				patchedBody = b
			}
		}
		now := time.Now()
		comment := &gitea.Comment{
			ID:      42,
			Body:    patchedBody,
			HTMLURL: "https://codeberg.org/owner/repo/issues/1#issuecomment-42",
			Poster:  &gitea.User{UserName: "alice"},
			Created: now,
			Updated: now,
		}
		respondJSON(w, http.StatusOK, comment)
	}))
	defer srv.Close()

	f := forgejoadapter.New(srv.URL, "owner", "repo", srv.Client())

	updated, err := f.Comments().Update(context.Background(), forge.CommentUpdateOptions{
		IssueNumber: 1,
		CommentID:   42,
		Body:        "Updated body",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated.Body != "Updated body" {
		t.Errorf("Body = %q, want Updated body", updated.Body)
	}
	if patchedBody != "Updated body" {
		t.Errorf("patched body = %q", patchedBody)
	}
	if updated.ID != 42 {
		t.Errorf("ID = %d, want 42", updated.ID)
	}
}

func TestCommentDeleteMapping(t *testing.T) {
	deleteCalled := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "DELETE" {
			deleteCalled = true
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	f := forgejoadapter.New(srv.URL, "owner", "repo", srv.Client())

	err := f.Comments().Delete(context.Background(), forge.CommentDeleteOptions{
		IssueNumber: 1,
		CommentID:   42,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !deleteCalled {
		t.Error("delete was not called")
	}
}

// ---- Comment error translation ----

func TestCommentListErrorTranslation404(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"message": "Not Found"})
	}))
	defer srv.Close()

	f := forgejoadapter.New(srv.URL, "owner", "repo", srv.Client())

	_, err := f.Comments().List(context.Background(), forge.CommentListOptions{IssueNumber: 1})
	if err == nil {
		t.Fatal("expected error for 404")
	}
	var se forge.StructuredError
	if !errors.As(err, &se) {
		t.Errorf("error should implement StructuredError, got %T: %v", err, err)
	}
	if !strings.Contains(se.Message(), "not found") {
		t.Errorf("message should mention 'not found': %s", se.Message())
	}
}

func TestCommentListErrorTranslation401(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"message": "unauthorized"})
	}))
	defer srv.Close()

	f := forgejoadapter.New(srv.URL, "owner", "repo", srv.Client())

	_, err := f.Comments().List(context.Background(), forge.CommentListOptions{IssueNumber: 1})
	if err == nil {
		t.Fatal("expected error for 401")
	}
	var se forge.StructuredError
	if !errors.As(err, &se) || !strings.Contains(se.Help(), "auth set") {
		t.Errorf("error should have a help hint via StructuredError: %s", err)
	}
}

func TestCommentGetErrorTranslation404(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"message": "Not Found"})
	}))
	defer srv.Close()

	f := forgejoadapter.New(srv.URL, "owner", "repo", srv.Client())

	_, err := f.Comments().Get(context.Background(), forge.CommentGetOptions{IssueNumber: 1, CommentID: 999})
	if err == nil {
		t.Fatal("expected error for 404")
	}
	var se forge.StructuredError
	if !errors.As(err, &se) {
		t.Errorf("error should implement StructuredError, got %T: %v", err, err)
	}
	if !strings.Contains(se.Message(), "not found") {
		t.Errorf("message should mention 'not found': %s", se.Message())
	}
}

func TestCommentCreateErrorTranslation401(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"message": "unauthorized"})
	}))
	defer srv.Close()

	f := forgejoadapter.New(srv.URL, "owner", "repo", srv.Client())

	_, err := f.Comments().Create(context.Background(), forge.CommentCreateOptions{
		IssueNumber: 1,
		Body:        "test",
	})
	if err == nil {
		t.Fatal("expected error for 401")
	}
	var se forge.StructuredError
	if !errors.As(err, &se) {
		t.Errorf("error should implement StructuredError, got %T: %v", err, err)
	}
	if !strings.Contains(se.Message(), "authentication failed") {
		t.Errorf("message should mention 'authentication failed': %s", se.Message())
	}
}

func TestCommentUpdateErrorTranslation404(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"message": "Not Found"})
	}))
	defer srv.Close()

	f := forgejoadapter.New(srv.URL, "owner", "repo", srv.Client())

	_, err := f.Comments().Update(context.Background(), forge.CommentUpdateOptions{
		IssueNumber: 1,
		CommentID:   999,
		Body:        "test",
	})
	if err == nil {
		t.Fatal("expected error for 404")
	}
	var se forge.StructuredError
	if !errors.As(err, &se) {
		t.Errorf("error should implement StructuredError, got %T: %v", err, err)
	}
	if !strings.Contains(se.Message(), "not found") {
		t.Errorf("message should mention 'not found': %s", se.Message())
	}
}

func TestCommentDeleteErrorTranslation404(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"message": "Not Found"})
	}))
	defer srv.Close()

	f := forgejoadapter.New(srv.URL, "owner", "repo", srv.Client())

	err := f.Comments().Delete(context.Background(), forge.CommentDeleteOptions{
		IssueNumber: 1,
		CommentID:   999,
	})
	if err == nil {
		t.Fatal("expected error for 404")
	}
	var se forge.StructuredError
	if !errors.As(err, &se) {
		t.Errorf("error should implement StructuredError, got %T: %v", err, err)
	}
	if !strings.Contains(se.Message(), "not found") {
		t.Errorf("message should mention 'not found': %s", se.Message())
	}
}

func TestCommentSystemAlwaysFalse(t *testing.T) {
	// Forgejo has no system flag on comments; system entries use a separate API.
	// All comments from the issues comments endpoint are user comments.
	now := time.Now()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		comment := &gitea.Comment{
			ID:      1,
			Body:    "A regular user comment",
			HTMLURL: "https://codeberg.org/owner/repo/issues/1#issuecomment-1",
			Poster:  &gitea.User{UserName: "alice"},
			Created: now,
			Updated: now,
		}
		respondJSON(w, http.StatusOK, comment)
	}))
	defer srv.Close()

	f := forgejoadapter.New(srv.URL, "owner", "repo", srv.Client())

	c, err := f.Comments().Get(context.Background(), forge.CommentGetOptions{IssueNumber: 1, CommentID: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.System {
		t.Errorf("System should be false for all Forgejo comments")
	}
}

func TestCommentListPagination(t *testing.T) {
	page := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page++
		comments := []*gitea.Comment{
			{ID: int64(page * 10), Body: "Comment", HTMLURL: "https://codeberg.org/owner/repo/issues/1#issuecomment-1"},
		}
		// Set Link header for pagination
		if page < 3 {
			w.Header().Set("Link", `<https://codeberg.org/api/v1/repos/owner/repo/issues/1/comments?page=2>; rel="next"`)
		}
		respondJSON(w, http.StatusOK, comments)
	}))
	defer srv.Close()

	f := forgejoadapter.New(srv.URL, "owner", "repo", srv.Client())

	comments, err := f.Comments().List(context.Background(), forge.CommentListOptions{IssueNumber: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(comments) < 3 {
		t.Errorf("expected at least 3 comments from pagination, got %d", len(comments))
	}
}

func TestCommentFieldMapping(t *testing.T) {
	// Verify the complete field mapping: body→Body, html_url→URL, user.login→Author, System→false.
	now := time.Now()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		comments := []*gitea.Comment{
			{
				ID:      1,
				Body:    "Comment body text",
				HTMLURL: "https://codeberg.org/owner/repo/issues/1#issuecomment-1",
				Poster:  &gitea.User{UserName: "commenter"},
				Created: now,
				Updated: now,
			},
		}
		respondJSON(w, http.StatusOK, comments)
	}))
	defer srv.Close()

	f := forgejoadapter.New(srv.URL, "owner", "repo", srv.Client())

	comments, err := f.Comments().List(context.Background(), forge.CommentListOptions{IssueNumber: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(comments) != 1 {
		t.Fatalf("expected 1 comment, got %d", len(comments))
	}
	c := comments[0]

	// body → Body
	if c.Body != "Comment body text" {
		t.Errorf("Body = %q, want %q", c.Body, "Comment body text")
	}
	// html_url → URL
	if c.URL != "https://codeberg.org/owner/repo/issues/1#issuecomment-1" {
		t.Errorf("URL = %q", c.URL)
	}
	// user.login → Author
	if c.Author != "commenter" {
		t.Errorf("Author = %q, want %q", c.Author, "commenter")
	}
	// System always false
	if c.System {
		t.Errorf("System should be false")
	}
}

// ---- Relation service: blocking dependencies ----

func TestRelationBlockedBy(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// GET /api/v1/repos/owner/repo/issues/42/dependencies
		if strings.Contains(r.URL.Path, "/dependencies") && r.Method == http.MethodGet {
			issues := []*gitea.Issue{
				{Index: 1, Title: "Blocking issue", State: "open"},
				{Index: 2, Title: "Another blocker", State: "closed"},
			}
			respondJSON(w, http.StatusOK, issues)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	f := forgejoadapter.New(srv.URL, "owner", "repo", srv.Client())

	deps, err := f.Relations().BlockedBy(context.Background(), 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(deps) != 2 {
		t.Fatalf("expected 2 deps, got %d", len(deps))
	}
	if deps[0].Number != 1 || deps[0].Title != "Blocking issue" || deps[0].State != "open" {
		t.Errorf("dep[0] = %+v", deps[0])
	}
	if deps[0].Direction != forge.DirBlockedBy {
		t.Errorf("direction = %q, want %q", deps[0].Direction, forge.DirBlockedBy)
	}
	if deps[1].Number != 2 || deps[1].Title != "Another blocker" || deps[1].State != "closed" {
		t.Errorf("dep[1] = %+v", deps[1])
	}
}

func TestRelationBlocking(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// GET /api/v1/repos/owner/repo/issues/42/blocks
		if strings.Contains(r.URL.Path, "/blocks") && r.Method == http.MethodGet {
			issues := []*gitea.Issue{
				{Index: 3, Title: "Blocked by this", State: "open"},
			}
			respondJSON(w, http.StatusOK, issues)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	f := forgejoadapter.New(srv.URL, "owner", "repo", srv.Client())

	deps, err := f.Relations().Blocking(context.Background(), 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(deps) != 1 {
		t.Fatalf("expected 1 dep, got %d", len(deps))
	}
	if deps[0].Number != 3 || deps[0].Title != "Blocked by this" || deps[0].State != "open" {
		t.Errorf("dep[0] = %+v", deps[0])
	}
	if deps[0].Direction != forge.DirBlocks {
		t.Errorf("direction = %q, want %q", deps[0].Direction, forge.DirBlocks)
	}
}

func TestRelationBlockedByEmpty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/dependencies") && r.Method == http.MethodGet {
			respondJSON(w, http.StatusOK, []*gitea.Issue{})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	f := forgejoadapter.New(srv.URL, "owner", "repo", srv.Client())

	deps, err := f.Relations().BlockedBy(context.Background(), 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(deps) != 0 {
		t.Errorf("expected 0 deps, got %d", len(deps))
	}
}

func TestRelationBlockedByError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/dependencies") && r.Method == http.MethodGet {
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]string{"message": "Not Found"})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	f := forgejoadapter.New(srv.URL, "owner", "repo", srv.Client())

	_, err := f.Relations().BlockedBy(context.Background(), 42)
	if err == nil {
		t.Fatal("expected error for 404")
	}
	var se forge.StructuredError
	if !errors.As(err, &se) {
		t.Errorf("error should implement StructuredError, got %T: %v", err, err)
	}
}

func TestRelationAddBlocks(t *testing.T) {
	var created bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// BlockedBy check for idempotency (GET dependencies of target=2)
		if strings.Contains(r.URL.Path, "/issues/2/dependencies") && r.Method == http.MethodGet {
			respondJSON(w, http.StatusOK, []*gitea.Issue{})
			return
		}
		// POST /issues/2/dependencies to create the block
		if strings.Contains(r.URL.Path, "/issues/2/dependencies") && r.Method == http.MethodPost {
			created = true
			issue := &gitea.Issue{Index: 1, Title: "Blocking", State: "open"}
			respondJSON(w, http.StatusOK, issue)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	f := forgejoadapter.New(srv.URL, "owner", "repo", srv.Client())

	err := f.Relations().AddBlocks(context.Background(), 1, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !created {
		t.Error("dependency was not created")
	}
}

func TestRelationAddBlocksIdempotent(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// BlockedBy check for idempotency (GET dependencies of target=2)
		if strings.Contains(r.URL.Path, "/issues/2/dependencies") && r.Method == http.MethodGet {
			// Return that issue 1 already blocks target 2
			issues := []*gitea.Issue{
				{Index: 1, Title: "Blocking", State: "open"},
			}
			respondJSON(w, http.StatusOK, issues)
			return
		}
		if strings.Contains(r.URL.Path, "/issues/2/dependencies") && r.Method == http.MethodPost {
			callCount++
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	f := forgejoadapter.New(srv.URL, "owner", "repo", srv.Client())

	err := f.Relations().AddBlocks(context.Background(), 1, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if callCount > 0 {
		t.Errorf("expected 0 POST calls for idempotent add, got %d", callCount)
	}
}

func TestRelationRemoveBlocks(t *testing.T) {
	var deleted bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// BlockedBy check for idempotency (GET dependencies of target=2)
		if strings.Contains(r.URL.Path, "/issues/2/dependencies") && r.Method == http.MethodGet {
			issues := []*gitea.Issue{
				{Index: 1, Title: "Blocking", State: "open"},
			}
			respondJSON(w, http.StatusOK, issues)
			return
		}
		// DELETE /issues/2/dependencies to remove the block
		if strings.Contains(r.URL.Path, "/issues/2/dependencies") && r.Method == http.MethodDelete {
			deleted = true
			issue := &gitea.Issue{Index: 1, Title: "Blocking", State: "open"}
			respondJSON(w, http.StatusOK, issue)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	f := forgejoadapter.New(srv.URL, "owner", "repo", srv.Client())

	err := f.Relations().RemoveBlocks(context.Background(), 1, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !deleted {
		t.Error("dependency was not removed")
	}
}

func TestRelationRemoveBlocksIdempotent(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// BlockedBy check for idempotency (GET dependencies of target=2)
		if strings.Contains(r.URL.Path, "/issues/2/dependencies") && r.Method == http.MethodGet {
			respondJSON(w, http.StatusOK, []*gitea.Issue{})
			return
		}
		if strings.Contains(r.URL.Path, "/issues/2/dependencies") && r.Method == http.MethodDelete {
			callCount++
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	f := forgejoadapter.New(srv.URL, "owner", "repo", srv.Client())

	err := f.Relations().RemoveBlocks(context.Background(), 1, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if callCount > 0 {
		t.Errorf("expected 0 DELETE calls for idempotent remove, got %d", callCount)
	}
}

// ---- Relation service: parent/child via title convention ----

func TestRelationChildren(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// GET /api/v1/repos/owner/repo/issues (list all)
		if r.URL.Path == "/api/v1/repos/owner/repo/issues" && r.Method == http.MethodGet {
			issues := []*gitea.Issue{
				{Index: 1, Title: "Parent issue", State: "open"},
				{Index: 2, Title: "[parent:1] Child task 1", State: "open"},
				{Index: 3, Title: "[parent:1] Child task 2", State: "closed"},
				{Index: 4, Title: "[parent:2] Another child", State: "open"},
				{Index: 5, Title: "Regular issue", State: "open"},
			}
			respondJSON(w, http.StatusOK, issues)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	f := forgejoadapter.New(srv.URL, "owner", "repo", srv.Client())

	children, err := f.Relations().Children(context.Background(), 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(children) != 2 {
		t.Fatalf("expected 2 children, got %d", len(children))
	}
	// Children should have [parent:N] stripped from title
	if children[0].Title != "Child task 1" {
		t.Errorf("child[0].Title = %q, want %q", children[0].Title, "Child task 1")
	}
	if children[0].Direction != forge.DirChild {
		t.Errorf("direction = %q, want child", children[0].Direction)
	}
	if children[1].Title != "Child task 2" {
		t.Errorf("child[1].Title = %q, want %q", children[1].Title, "Child task 2")
	}
	if children[1].State != "closed" {
		t.Errorf("child[1].State = %q, want closed", children[1].State)
	}
}

func TestRelationChildrenEmpty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/repos/owner/repo/issues" && r.Method == http.MethodGet {
			issues := []*gitea.Issue{
				{Index: 1, Title: "No children here", State: "open"},
			}
			respondJSON(w, http.StatusOK, issues)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	f := forgejoadapter.New(srv.URL, "owner", "repo", srv.Client())

	children, err := f.Relations().Children(context.Background(), 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(children) != 0 {
		t.Errorf("expected 0 children, got %d", len(children))
	}
}

func TestRelationParent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// GET /issues/2 (child issue with [parent:1] prefix)
		if strings.Contains(r.URL.Path, "/issues/2") && !strings.Contains(r.URL.Path, "/issues/1") {
			issue := &gitea.Issue{
				Index:   2,
				Title:   "[parent:1] Child task",
				State:   "open",
				Poster:  &gitea.User{UserName: "bob"},
				HTMLURL: "https://codeberg.org/owner/repo/issues/2",
				Created: time.Now(),
				Updated: time.Now(),
			}
			respondJSON(w, http.StatusOK, issue)
			return
		}
		// GET /issues/1 (parent issue)
		if strings.Contains(r.URL.Path, "/issues/1") {
			issue := &gitea.Issue{
				Index:   1,
				Title:   "Parent issue",
				State:   "open",
				Poster:  &gitea.User{UserName: "alice"},
				HTMLURL: "https://codeberg.org/owner/repo/issues/1",
				Created: time.Now(),
				Updated: time.Now(),
			}
			respondJSON(w, http.StatusOK, issue)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	f := forgejoadapter.New(srv.URL, "owner", "repo", srv.Client())

	parent, err := f.Relations().Parent(context.Background(), 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if parent == nil {
		t.Fatal("expected parent, got nil")
	}
	if parent.Number != 1 {
		t.Errorf("parent.Number = %d, want 1", parent.Number)
	}
	if parent.Title != "Parent issue" {
		t.Errorf("parent.Title = %q, want %q", parent.Title, "Parent issue")
	}
	if parent.Direction != forge.DirParent {
		t.Errorf("direction = %q, want parent", parent.Direction)
	}
}

func TestRelationParentNone(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		issue := &gitea.Issue{
			Index:   42,
			Title:   "Regular issue with no parent",
			State:   "open",
			Poster:  &gitea.User{UserName: "bob"},
			HTMLURL: "https://codeberg.org/owner/repo/issues/42",
			Created: time.Now(),
			Updated: time.Now(),
		}
		respondJSON(w, http.StatusOK, issue)
	}))
	defer srv.Close()

	f := forgejoadapter.New(srv.URL, "owner", "repo", srv.Client())

	parent, err := f.Relations().Parent(context.Background(), 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if parent != nil {
		t.Errorf("expected nil parent, got %+v", parent)
	}
}

func TestRelationParentError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"message": "Not Found"})
	}))
	defer srv.Close()

	f := forgejoadapter.New(srv.URL, "owner", "repo", srv.Client())

	_, err := f.Relations().Parent(context.Background(), 999)
	if err == nil {
		t.Fatal("expected error for 404")
	}
	var se forge.StructuredError
	if !errors.As(err, &se) {
		t.Errorf("error should implement StructuredError, got %T: %v", err, err)
	}
}

func TestRelationAddParentOf(t *testing.T) {
	var patchedTitle string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// GET /issues/2 (child) — check existing parent
		if strings.Contains(r.URL.Path, "/issues/2") && r.Method == http.MethodGet {
			issue := &gitea.Issue{
				Index:   2,
				Title:   "Child task",
				State:   "open",
				Poster:  &gitea.User{UserName: "bob"},
				HTMLURL: "https://codeberg.org/owner/repo/issues/2",
				Created: time.Now(),
				Updated: time.Now(),
			}
			respondJSON(w, http.StatusOK, issue)
			return
		}
		// PATCH /issues/2 (update title with parent prefix)
		if strings.Contains(r.URL.Path, "/issues/2") && r.Method == "PATCH" {
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			if t, ok := body["title"].(string); ok {
				patchedTitle = t
			}
			issue := &gitea.Issue{
				Index:   2,
				Title:   patchedTitle,
				State:   "open",
				Poster:  &gitea.User{UserName: "bob"},
				HTMLURL: "https://codeberg.org/owner/repo/issues/2",
				Created: time.Now(),
				Updated: time.Now(),
			}
			respondJSON(w, http.StatusOK, issue)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	f := forgejoadapter.New(srv.URL, "owner", "repo", srv.Client())

	err := f.Relations().AddParentOf(context.Background(), 1, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if patchedTitle != "[parent:1] Child task" {
		t.Errorf("patched title = %q, want %q", patchedTitle, "[parent:1] Child task")
	}
}

func TestRelationAddParentOfIdempotent(t *testing.T) {
	patchCallCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// GET /issues/2 (child) — already has parent:1
		if strings.Contains(r.URL.Path, "/issues/2") && r.Method == http.MethodGet {
			issue := &gitea.Issue{
				Index:   2,
				Title:   "[parent:1] Child task",
				State:   "open",
				Poster:  &gitea.User{UserName: "bob"},
				HTMLURL: "https://codeberg.org/owner/repo/issues/2",
				Created: time.Now(),
				Updated: time.Now(),
			}
			respondJSON(w, http.StatusOK, issue)
			return
		}
		// GET /issues/1 (parent issue) — needed for Parent() idempotency check
		if strings.Contains(r.URL.Path, "/issues/1") && r.Method == http.MethodGet {
			issue := &gitea.Issue{
				Index:   1,
				Title:   "Parent issue",
				State:   "open",
				Poster:  &gitea.User{UserName: "alice"},
				HTMLURL: "https://codeberg.org/owner/repo/issues/1",
				Created: time.Now(),
				Updated: time.Now(),
			}
			respondJSON(w, http.StatusOK, issue)
			return
		}
		// PATCH /issues/2 — should not be called
		if strings.Contains(r.URL.Path, "/issues/2") && r.Method == "PATCH" {
			patchCallCount++
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	f := forgejoadapter.New(srv.URL, "owner", "repo", srv.Client())

	err := f.Relations().AddParentOf(context.Background(), 1, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if patchCallCount > 0 {
		t.Errorf("expected 0 PATCH calls for idempotent add, got %d", patchCallCount)
	}
}

func TestRelationAddParentOfChangesParent(t *testing.T) {
	// When the child already has a different parent, change it.
	var patchedTitle string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// GET /issues/2 (child) — currently has parent:1
		if strings.Contains(r.URL.Path, "/issues/2") && r.Method == http.MethodGet {
			issue := &gitea.Issue{
				Index:   2,
				Title:   "[parent:1] Child task",
				State:   "open",
				Poster:  &gitea.User{UserName: "bob"},
				HTMLURL: "https://codeberg.org/owner/repo/issues/2",
				Created: time.Now(),
				Updated: time.Now(),
			}
			respondJSON(w, http.StatusOK, issue)
			return
		}
		// GET /issues/1 (parent issue) — needed for Parent() idempotency check
		if strings.Contains(r.URL.Path, "/issues/1") && r.Method == http.MethodGet {
			issue := &gitea.Issue{
				Index:   1,
				Title:   "Old parent",
				State:   "open",
				Poster:  &gitea.User{UserName: "alice"},
				HTMLURL: "https://codeberg.org/owner/repo/issues/1",
				Created: time.Now(),
				Updated: time.Now(),
			}
			respondJSON(w, http.StatusOK, issue)
			return
		}
		// PATCH /issues/2 — update to new parent:3
		if strings.Contains(r.URL.Path, "/issues/2") && r.Method == "PATCH" {
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			if t, ok := body["title"].(string); ok {
				patchedTitle = t
			}
			issue := &gitea.Issue{
				Index:   2,
				Title:   patchedTitle,
				State:   "open",
				Poster:  &gitea.User{UserName: "bob"},
				HTMLURL: "https://codeberg.org/owner/repo/issues/2",
				Created: time.Now(),
				Updated: time.Now(),
			}
			respondJSON(w, http.StatusOK, issue)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	f := forgejoadapter.New(srv.URL, "owner", "repo", srv.Client())

	err := f.Relations().AddParentOf(context.Background(), 3, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if patchedTitle != "[parent:3] Child task" {
		t.Errorf("patched title = %q, want %q", patchedTitle, "[parent:3] Child task")
	}
}

func TestRelationRemoveParentOf(t *testing.T) {
	var patchedTitle string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// GET /issues/2 (child) — has [parent:1]
		if strings.Contains(r.URL.Path, "/issues/2") && r.Method == http.MethodGet {
			issue := &gitea.Issue{
				Index:   2,
				Title:   "[parent:1] Child task",
				State:   "open",
				Poster:  &gitea.User{UserName: "bob"},
				HTMLURL: "https://codeberg.org/owner/repo/issues/2",
				Created: time.Now(),
				Updated: time.Now(),
			}
			respondJSON(w, http.StatusOK, issue)
			return
		}
		// GET /issues/1 (parent issue) — needed for Parent() idempotency check
		if strings.Contains(r.URL.Path, "/issues/1") && r.Method == http.MethodGet {
			issue := &gitea.Issue{
				Index:   1,
				Title:   "Parent issue",
				State:   "open",
				Poster:  &gitea.User{UserName: "alice"},
				HTMLURL: "https://codeberg.org/owner/repo/issues/1",
				Created: time.Now(),
				Updated: time.Now(),
			}
			respondJSON(w, http.StatusOK, issue)
			return
		}
		// PATCH /issues/2 — strip parent prefix
		if strings.Contains(r.URL.Path, "/issues/2") && r.Method == "PATCH" {
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			if t, ok := body["title"].(string); ok {
				patchedTitle = t
			}
			issue := &gitea.Issue{
				Index:   2,
				Title:   patchedTitle,
				State:   "open",
				Poster:  &gitea.User{UserName: "bob"},
				HTMLURL: "https://codeberg.org/owner/repo/issues/2",
				Created: time.Now(),
				Updated: time.Now(),
			}
			respondJSON(w, http.StatusOK, issue)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	f := forgejoadapter.New(srv.URL, "owner", "repo", srv.Client())

	err := f.Relations().RemoveParentOf(context.Background(), 1, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if patchedTitle != "Child task" {
		t.Errorf("patched title = %q, want %q", patchedTitle, "Child task")
	}
}

func TestRelationRemoveParentOfIdempotent(t *testing.T) {
	patchCallCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// GET /issues/2 (child) — no parent
		if strings.Contains(r.URL.Path, "/issues/2") && r.Method == http.MethodGet {
			issue := &gitea.Issue{
				Index:   2,
				Title:   "Child task",
				State:   "open",
				Poster:  &gitea.User{UserName: "bob"},
				HTMLURL: "https://codeberg.org/owner/repo/issues/2",
				Created: time.Now(),
				Updated: time.Now(),
			}
			respondJSON(w, http.StatusOK, issue)
			return
		}
		// PATCH /issues/2 — should not be called
		if strings.Contains(r.URL.Path, "/issues/2") && r.Method == "PATCH" {
			patchCallCount++
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	f := forgejoadapter.New(srv.URL, "owner", "repo", srv.Client())

	err := f.Relations().RemoveParentOf(context.Background(), 1, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if patchCallCount > 0 {
		t.Errorf("expected 0 PATCH calls for idempotent remove, got %d", patchCallCount)
	}
}

func TestRelationRemoveParentOfDifferentParent(t *testing.T) {
	// When the child has a different parent, RemoveParentOf is no-op (idempotent).
	patchCallCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// GET /issues/2 (child) — has [parent:3]
		if strings.Contains(r.URL.Path, "/issues/2") && r.Method == http.MethodGet {
			issue := &gitea.Issue{
				Index:   2,
				Title:   "[parent:3] Child task",
				State:   "open",
				Poster:  &gitea.User{UserName: "bob"},
				HTMLURL: "https://codeberg.org/owner/repo/issues/2",
				Created: time.Now(),
				Updated: time.Now(),
			}
			respondJSON(w, http.StatusOK, issue)
			return
		}
		// GET /issues/3 (parent issue) — needed for Parent() idempotency check
		if strings.Contains(r.URL.Path, "/issues/3") && r.Method == http.MethodGet {
			issue := &gitea.Issue{
				Index:   3,
				Title:   "Different parent",
				State:   "open",
				Poster:  &gitea.User{UserName: "alice"},
				HTMLURL: "https://codeberg.org/owner/repo/issues/3",
				Created: time.Now(),
				Updated: time.Now(),
			}
			respondJSON(w, http.StatusOK, issue)
			return
		}
		// PATCH /issues/2 — should not be called
		if strings.Contains(r.URL.Path, "/issues/2") && r.Method == "PATCH" {
			patchCallCount++
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	f := forgejoadapter.New(srv.URL, "owner", "repo", srv.Client())

	err := f.Relations().RemoveParentOf(context.Background(), 1, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if patchCallCount > 0 {
		t.Errorf("expected 0 PATCH calls for different parent, got %d", patchCallCount)
	}
}

func TestRelationAddParentOfError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// GET /issues/2 returns 404 and no parent
		if strings.Contains(r.URL.Path, "/issues/2") && r.Method == http.MethodGet {
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]string{"message": "Not Found"})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	f := forgejoadapter.New(srv.URL, "owner", "repo", srv.Client())

	err := f.Relations().AddParentOf(context.Background(), 1, 999)
	if err == nil {
		t.Fatal("expected error for 404")
	}
	var se forge.StructuredError
	if !errors.As(err, &se) {
		t.Errorf("error should implement StructuredError, got %T: %v", err, err)
	}
}
