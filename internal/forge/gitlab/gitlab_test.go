package gitlab_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/tnikic/anvil/internal/forge"
	gitlabadapter "github.com/tnikic/anvil/internal/forge/gitlab"
	gl "gitlab.com/gitlab-org/api/client-go"
)

func respondJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// ---- Interface compliance ----

func TestGitLabForgeSatisfiesInterface(t *testing.T) {
	var _ forge.Forge = (*gitlabadapter.Forge)(nil)
}

// ---- Scope parsing ----

func TestParseLabelScopeScoped(t *testing.T) {
	scope, name := forge.ParseLabelScope("kind::bug", "::")
	if scope != "kind" {
		t.Errorf("scope = %q, want %q", scope, "kind")
	}
	if name != "bug" {
		t.Errorf("name = %q, want %q", name, "bug")
	}
}

func TestParseLabelScopeUnscoped(t *testing.T) {
	scope, name := forge.ParseLabelScope("good-first-issue", "::")
	if scope != "" {
		t.Errorf("scope should be empty, got %q", scope)
	}
	if name != "good-first-issue" {
		t.Errorf("name = %q, want %q", name, "good-first-issue")
	}
}

func TestParseLabelScopeMultipleDoubleColons(t *testing.T) {
	// Only split on first ::
	scope, name := forge.ParseLabelScope("a::b::c", "::")
	if scope != "a" {
		t.Errorf("scope = %q, want %q", scope, "a")
	}
	if name != "b::c" {
		t.Errorf("name = %q, want %q", name, "b::c")
	}
}

func TestLabelFullNameScoped(t *testing.T) {
	full := forge.LabelFullName("kind", "bug", "::")
	if full != "kind::bug" {
		t.Errorf("full = %q, want %q", full, "kind::bug")
	}
}

func TestLabelFullNameUnscoped(t *testing.T) {
	full := forge.LabelFullName("", "good-first-issue", "::")
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
		full := forge.LabelFullName(tt.scope, tt.name, "::")
		scope, name := forge.ParseLabelScope(full, "::")
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
		tm := time.Now()
		issues := []*gl.Issue{
			{
				IID:         1,
				Title:       "Fix login timeout",
				State:       "opened",
				Description: "Users are logged out after 5 minutes of inactivity.",
				Author:      &gl.IssueAuthor{Username: "alice"},
				WebURL:      "https://gitlab.com/owner/repo/-/issues/1",
				CreatedAt:   &tm,
				UpdatedAt:   &tm,
				Labels:      gl.Labels{"kind::bug", "good-first-issue"},
			},
		}
		respondJSON(w, http.StatusOK, issues)
	}))
	defer srv.Close()

	f := gitlabadapter.New(srv.URL, "owner", "repo", srv.Client())

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
		t.Errorf("state = %q, want open", issues[0].State)
	}
	if issues[0].Author != "alice" {
		t.Errorf("author = %q", issues[0].Author)
	}
	if len(issues[0].Labels) != 2 {
		t.Fatalf("expected 2 labels, got %d", len(issues[0].Labels))
	}
	// First label: kind::bug -> scope=kind, name=bug
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
		tm := time.Now()
		issue := &gl.Issue{
			IID:         42,
			Title:       "Test Issue",
			State:       "closed",
			Description: "Full body content",
			Author:      &gl.IssueAuthor{Username: "bob"},
			WebURL:      "https://gitlab.com/owner/repo/-/issues/42",
			CreatedAt:   &tm,
			UpdatedAt:   &tm,
		}
		respondJSON(w, http.StatusOK, issue)
	}))
	defer srv.Close()

	f := gitlabadapter.New(srv.URL, "owner", "repo", srv.Client())

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
		tm := time.Now()
		issue := &gl.Issue{
			IID:       99,
			Title:     "Created Issue",
			State:     "opened",
			Author:    &gl.IssueAuthor{Username: "alice"},
			WebURL:    "https://gitlab.com/owner/repo/-/issues/99",
			CreatedAt: &tm,
			UpdatedAt: &tm,
		}
		respondJSON(w, http.StatusCreated, issue)
	}))
	defer srv.Close()

	f := gitlabadapter.New(srv.URL, "owner", "repo", srv.Client())

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
		issue := &gl.Issue{
			IID:   42,
			State: "closed",
		}
		respondJSON(w, http.StatusOK, issue)
	}))
	defer srv.Close()

	f := gitlabadapter.New(srv.URL, "owner", "repo", srv.Client())

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
		issue := &gl.Issue{
			IID:   42,
			State: "opened",
		}
		respondJSON(w, http.StatusOK, issue)
	}))
	defer srv.Close()

	f := gitlabadapter.New(srv.URL, "owner", "repo", srv.Client())

	reopened, err := f.Issues().Reopen(context.Background(), forge.IssueReopenOptions{Number: 42})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reopened.State != "open" {
		t.Errorf("state = %q, want open", reopened.State)
	}
}

// ---- Self-hosted base URL ----

// ---- Issue assignee resolution ----

func TestIssueCreateWithAssignees(t *testing.T) {
	var capturedAssigneeIDs []int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// User lookup: GET /api/v4/users?username=...
		if strings.Contains(r.URL.Path, "/users") {
			username := r.URL.Query().Get("username")
			users := []*gl.User{
				{ID: 10, Username: username},
			}
			respondJSON(w, http.StatusOK, users)
			return
		}
		// Issue create: POST /api/v4/projects/.../issues
		var reqBody map[string]any
		_ = json.NewDecoder(r.Body).Decode(&reqBody)
		if ids, ok := reqBody["assignee_ids"].([]interface{}); ok {
			for _, id := range ids {
				capturedAssigneeIDs = append(capturedAssigneeIDs, int64(id.(float64)))
			}
		}
		tm := time.Now()
		issue := &gl.Issue{
			IID:       99,
			Title:     "Created Issue",
			State:     "opened",
			Author:    &gl.IssueAuthor{Username: "alice"},
			WebURL:    "https://gitlab.com/owner/repo/-/issues/99",
			CreatedAt: &tm,
			UpdatedAt: &tm,
		}
		respondJSON(w, http.StatusCreated, issue)
	}))
	defer srv.Close()

	f := gitlabadapter.New(srv.URL, "owner", "repo", srv.Client())

	created, err := f.Issues().Create(context.Background(), forge.IssueCreateOptions{
		Title:     forge.String("Created Issue"),
		Assignees: []string{"alice", "bob"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if created.Number != 99 {
		t.Errorf("number = %d, want 99", created.Number)
	}
	if len(capturedAssigneeIDs) != 2 {
		t.Fatalf("expected 2 assignee IDs, got %d: %v", len(capturedAssigneeIDs), capturedAssigneeIDs)
	}
	if capturedAssigneeIDs[0] != 10 {
		t.Errorf("assignee ID 0 = %d, want 10", capturedAssigneeIDs[0])
	}
	if capturedAssigneeIDs[1] != 10 {
		t.Errorf("assignee ID 1 = %d, want 10", capturedAssigneeIDs[1])
	}
}

func TestIssueUpdateWithAssignees(t *testing.T) {
	var capturedAssigneeIDs []int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// User lookup: GET /api/v4/users?username=...
		if strings.Contains(r.URL.Path, "/users") {
			username := r.URL.Query().Get("username")
			users := []*gl.User{
				{ID: 20, Username: username},
			}
			respondJSON(w, http.StatusOK, users)
			return
		}
		// Issue update: PUT /api/v4/projects/.../issues/42
		var reqBody map[string]any
		_ = json.NewDecoder(r.Body).Decode(&reqBody)
		if ids, ok := reqBody["assignee_ids"].([]interface{}); ok {
			for _, id := range ids {
				capturedAssigneeIDs = append(capturedAssigneeIDs, int64(id.(float64)))
			}
		}
		issue := &gl.Issue{
			IID:   42,
			Title: "Updated Issue",
			State: "opened",
		}
		respondJSON(w, http.StatusOK, issue)
	}))
	defer srv.Close()

	f := gitlabadapter.New(srv.URL, "owner", "repo", srv.Client())

	updated, err := f.Issues().Update(context.Background(), forge.IssueUpdateOptions{
		Number:    42,
		Title:     forge.String("Updated Issue"),
		Assignees: []string{"charlie"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated.Number != 42 {
		t.Errorf("number = %d, want 42", updated.Number)
	}
	if len(capturedAssigneeIDs) != 1 {
		t.Fatalf("expected 1 assignee ID, got %d", len(capturedAssigneeIDs))
	}
	if capturedAssigneeIDs[0] != 20 {
		t.Errorf("assignee ID = %d, want 20", capturedAssigneeIDs[0])
	}
}

func TestIssueCreateAssigneeLookupFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// User lookup returns empty — user not found
		if strings.Contains(r.URL.Path, "/users") {
			respondJSON(w, http.StatusOK, []*gl.User{})
			return
		}
		// Should not reach issue create
		t.Error("issue create should not be called after failed user lookup")
	}))
	defer srv.Close()

	f := gitlabadapter.New(srv.URL, "owner", "repo", srv.Client())

	_, err := f.Issues().Create(context.Background(), forge.IssueCreateOptions{
		Title:     forge.String("Test"),
		Assignees: []string{"nonexistent"},
	})
	if err == nil {
		t.Fatal("expected error for unknown user")
	}
	errStr := err.Error()
	if !strings.Contains(errStr, "nonexistent") {
		t.Errorf("error should mention username: %s", errStr)
	}
	var se forge.StructuredError
	if !errors.As(err, &se) {
		t.Errorf("error should be StructuredError, got %T: %v", err, err)
	}
}

func TestIssueUpdateAssigneeLookupFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// User lookup returns 500 server error
		if strings.Contains(r.URL.Path, "/users") {
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]string{"message": "Internal Server Error"})
			return
		}
		t.Error("issue update should not be called after failed user lookup")
	}))
	defer srv.Close()

	f := gitlabadapter.New(srv.URL, "owner", "repo", srv.Client())

	_, err := f.Issues().Update(context.Background(), forge.IssueUpdateOptions{
		Number:    42,
		Assignees: []string{"someuser"},
	})
	if err == nil {
		t.Fatal("expected error for failed user lookup")
	}
	var se forge.StructuredError
	if !errors.As(err, &se) {
		t.Errorf("error should be StructuredError, got %T: %v", err, err)
	}
}

func TestIssueCreateWithoutAssignees(t *testing.T) {
	// When Assignees is empty, no user lookup should happen.
	var usersCalled bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/users") {
			usersCalled = true
		}
		tm := time.Now()
		issue := &gl.Issue{
			IID:       1,
			Title:     "No Assignees",
			State:     "opened",
			CreatedAt: &tm,
			UpdatedAt: &tm,
		}
		respondJSON(w, http.StatusCreated, issue)
	}))
	defer srv.Close()

	f := gitlabadapter.New(srv.URL, "owner", "repo", srv.Client())

	_, err := f.Issues().Create(context.Background(), forge.IssueCreateOptions{
		Title: forge.String("No Assignees"),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if usersCalled {
		t.Error("users API should not be called when Assignees is empty")
	}
}

func TestSelfHostedBaseURL(t *testing.T) {
	// When host is not "gitlab.com", the adapter configures
	// https://<host>/api/v4/ as the base URL.
	// We verify this by checking that requests go to the correct path.
	var requestURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestURL = r.URL.String()
		issues := []*gl.Issue{}
		respondJSON(w, http.StatusOK, issues)
	}))
	defer srv.Close()

	// Pass the full httptest URL as host — the adapter detects the http:// scheme.
	f := gitlabadapter.New(srv.URL, "owner", "repo", srv.Client())

	_, _, err := f.Issues().List(context.Background(), forge.IssueListOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(requestURL, "/api/v4/") {
		t.Errorf("expected /api/v4/ in request URL, got %s", requestURL)
	}
}

// ---- Error translation ----

func TestErrorTranslation401(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"message": "Unauthorized"})
	}))
	defer srv.Close()

	f := gitlabadapter.New(srv.URL, "owner", "repo", srv.Client())

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

	f := gitlabadapter.New(srv.URL, "owner", "repo", srv.Client())

	_, err := f.Issues().Get(context.Background(), forge.IssueGetOptions{Number: 999})
	if err == nil {
		t.Fatal("expected error for 404")
	}
	errStr := err.Error()
	if !strings.Contains(errStr, "not found") && !strings.Contains(errStr, "404") {
		t.Errorf("error should mention not found or 404: %s", errStr)
	}
}

func TestErrorTranslation403(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]string{"message": "Forbidden"})
	}))
	defer srv.Close()

	f := gitlabadapter.New(srv.URL, "owner", "repo", srv.Client())

	_, _, err := f.Issues().List(context.Background(), forge.IssueListOptions{})
	if err == nil {
		t.Fatal("expected error for 403")
	}
	errStr := err.Error()
	if !strings.Contains(errStr, "access denied") {
		t.Errorf("error should mention access denied: %s", errStr)
	}
}

func TestErrorTranslationRateLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Ratelimit-Remaining", "0")
		w.WriteHeader(http.StatusTooManyRequests)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"message": "Too Many Requests",
		})
	}))
	defer srv.Close()

	f := gitlabadapter.New(srv.URL, "owner", "repo", srv.Client())

	_, _, err := f.Issues().List(context.Background(), forge.IssueListOptions{})
	if err == nil {
		t.Fatal("expected error for rate limit")
	}
	errStr := err.Error()
	if !strings.Contains(errStr, "rate limit") {
		t.Errorf("error should mention rate limit: %s", errStr)
	}
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
		issues := []*gl.Issue{
			{IID: int64(page * 10), Title: "Issue", State: "opened"},
		}

		// Set X-Next-Page header for pagination
		if page < 3 {
			w.Header().Set("X-Next-Page", "2")
		}
		respondJSON(w, http.StatusOK, issues)
	}))
	defer srv.Close()

	f := gitlabadapter.New(srv.URL, "owner", "repo", srv.Client())

	issues, _, err := f.Issues().List(context.Background(), forge.IssueListOptions{Limit: 50})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(issues) < 1 {
		t.Errorf("expected at least 1 issue, got %d", len(issues))
	}
}

// ---- Context cancellation ----

func TestContextCancellation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Hang forever
		select {}
	}))
	defer srv.Close()

	f := gitlabadapter.New(srv.URL, "owner", "repo", srv.Client())

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, _, err := f.Issues().List(ctx, forge.IssueListOptions{})
	if err == nil {
		t.Fatal("expected error for canceled context")
	}
}

// ---- Label mapping ----

func TestLabelListMapping(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		labels := []*gl.Label{
			{Name: "kind::bug", Color: "#ff0000", Description: "A bug"},
			{Name: "good-first-issue", Color: "#7057ff"},
		}
		respondJSON(w, http.StatusOK, labels)
	}))
	defer srv.Close()

	f := gitlabadapter.New(srv.URL, "owner", "repo", srv.Client())

	labels, err := f.Labels().List(context.Background(), forge.LabelListOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(labels) != 2 {
		t.Fatalf("expected 2 labels, got %d", len(labels))
	}
	// First label: kind::bug
	if labels[0].Scope != "kind" || labels[0].Name != "bug" {
		t.Errorf("label 0: scope=%q name=%q, want scope=kind name=bug", labels[0].Scope, labels[0].Name)
	}
	if labels[0].Color != "#ff0000" {
		t.Errorf("label 0: color=%q, want #ff0000", labels[0].Color)
	}
	if labels[0].Description != "A bug" {
		t.Errorf("label 0: description=%q, want \"A bug\"", labels[0].Description)
	}
	// Second label: unscoped
	if labels[1].Scope != "" || labels[1].Name != "good-first-issue" {
		t.Errorf("label 1: scope=%q name=%q, want scope= name=good-first-issue", labels[1].Scope, labels[1].Name)
	}
}

func TestLabelCreateScoped(t *testing.T) {
	var createdName string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var reqBody map[string]any
		_ = json.NewDecoder(r.Body).Decode(&reqBody)
		createdName = reqBody["name"].(string)

		label := &gl.Label{
			Name:        createdName,
			Color:       "#ff0000",
			Description: "High priority",
		}
		respondJSON(w, http.StatusCreated, label)
	}))
	defer srv.Close()

	f := gitlabadapter.New(srv.URL, "owner", "repo", srv.Client())

	created, err := f.Labels().Create(context.Background(), forge.LabelCreateOptions{
		Scope:       forge.String("priority"),
		Name:        "high",
		Color:       forge.String("#ff0000"),
		Description: forge.String("High priority"),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if createdName != "priority::high" {
		t.Errorf("sent label name = %q, want %q", createdName, "priority::high")
	}
	if created.Scope != "priority" {
		t.Errorf("scope = %q, want %q", created.Scope, "priority")
	}
	if created.Name != "high" {
		t.Errorf("name = %q, want %q", created.Name, "high")
	}
}

func TestLabelCreateUnscoped(t *testing.T) {
	var createdName string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var reqBody map[string]any
		_ = json.NewDecoder(r.Body).Decode(&reqBody)
		createdName = reqBody["name"].(string)

		label := &gl.Label{
			Name:  createdName,
			Color: "#7057ff",
		}
		respondJSON(w, http.StatusCreated, label)
	}))
	defer srv.Close()

	f := gitlabadapter.New(srv.URL, "owner", "repo", srv.Client())

	created, err := f.Labels().Create(context.Background(), forge.LabelCreateOptions{
		Name:  "enhancement",
		Color: forge.String("#7057ff"),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if createdName != "enhancement" {
		t.Errorf("sent label name = %q, want %q", createdName, "enhancement")
	}
	if created.Scope != "" {
		t.Errorf("scope = %q, want empty", created.Scope)
	}
	if created.Name != "enhancement" {
		t.Errorf("name = %q, want %q", created.Name, "enhancement")
	}
}

func TestLabelUpdate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		label := &gl.Label{
			Name:        "kind::feature",
			Color:       "#00ff00",
			Description: "A feature request",
		}
		respondJSON(w, http.StatusOK, label)
	}))
	defer srv.Close()

	f := gitlabadapter.New(srv.URL, "owner", "repo", srv.Client())

	updated, err := f.Labels().Update(context.Background(), forge.LabelUpdateOptions{
		Scope:       "kind",
		Name:        "bug",
		NewName:     forge.String("feature"),
		Color:       forge.String("#00ff00"),
		Description: forge.String("A feature request"),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated.Scope != "kind" || updated.Name != "feature" {
		t.Errorf("updated label: scope=%q name=%q, want scope=kind name=feature", updated.Scope, updated.Name)
	}
}

func TestLabelUpdateScopeChange(t *testing.T) {
	var reqNewName string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var reqBody map[string]any
		_ = json.NewDecoder(r.Body).Decode(&reqBody)
		if nn, ok := reqBody["new_name"].(string); ok {
			reqNewName = nn
		}

		label := &gl.Label{
			Name: reqNewName,
		}
		respondJSON(w, http.StatusOK, label)
	}))
	defer srv.Close()

	f := gitlabadapter.New(srv.URL, "owner", "repo", srv.Client())

	_, err := f.Labels().Update(context.Background(), forge.LabelUpdateOptions{
		Scope:    "oldscope",
		Name:     "oldname",
		NewScope: forge.String("newscope"),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reqNewName != "newscope::oldname" {
		t.Errorf("new_name = %q, want %q", reqNewName, "newscope::oldname")
	}
}

func TestLabelDelete(t *testing.T) {
	var deleted bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		deleted = true
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	f := gitlabadapter.New(srv.URL, "owner", "repo", srv.Client())

	err := f.Labels().Delete(context.Background(), forge.LabelDeleteOptions{
		Scope: "kind",
		Name:  "bug",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !deleted {
		t.Error("delete was not called")
	}
}

func TestLabelListPagination(t *testing.T) {
	page := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page++
		labels := []*gl.Label{
			{Name: "label", Color: "#000000"},
		}
		if page < 3 {
			w.Header().Set("X-Next-Page", "2")
		}
		respondJSON(w, http.StatusOK, labels)
	}))
	defer srv.Close()

	f := gitlabadapter.New(srv.URL, "owner", "repo", srv.Client())

	labels, err := f.Labels().List(context.Background(), forge.LabelListOptions{Limit: 50})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(labels) < 1 {
		t.Errorf("expected at least 1 label, got %d", len(labels))
	}
}

func TestLabelErrorOnCreate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"message": "Unauthorized"})
	}))
	defer srv.Close()

	f := gitlabadapter.New(srv.URL, "owner", "repo", srv.Client())

	_, err := f.Labels().Create(context.Background(), forge.LabelCreateOptions{Name: "test"})
	if err == nil {
		t.Fatal("expected error for 401")
	}
	if !strings.Contains(err.Error(), "authentication failed") {
		t.Errorf("error should mention authentication: %s", err.Error())
	}
}

func TestLabelErrorOnDelete(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"message": "Not Found"})
	}))
	defer srv.Close()

	f := gitlabadapter.New(srv.URL, "owner", "repo", srv.Client())

	err := f.Labels().Delete(context.Background(), forge.LabelDeleteOptions{Name: "nonexistent"})
	if err == nil {
		t.Fatal("expected error for 404")
	}
	errStr := err.Error()
	if !strings.Contains(errStr, "not found") && !strings.Contains(errStr, "404") {
		t.Errorf("error should mention not found or 404: %s", errStr)
	}
}

// ---- MR mapping ----

func TestMRListMapping(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tm := time.Now()
		mrs := []*gl.BasicMergeRequest{
			{
				IID:          1,
				Title:        "Fix login timeout",
				State:        "opened",
				Description:  "Users are logged out after 5 minutes.",
				SourceBranch: "fix-login",
				TargetBranch: "main",
				Author:       &gl.BasicUser{Username: "alice"},
				WebURL:       "https://gitlab.com/owner/repo/-/merge_requests/1",
				CreatedAt:    &tm,
				UpdatedAt:    &tm,
			},
		}
		respondJSON(w, http.StatusOK, mrs)
	}))
	defer srv.Close()

	f := gitlabadapter.New(srv.URL, "owner", "repo", srv.Client())

	prs, meta, err := f.PRs().List(context.Background(), forge.PRListOptions{State: "open"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(prs) != 1 {
		t.Fatalf("expected 1 PR, got %d", len(prs))
	}
	pr := prs[0]
	if pr.Number != 1 {
		t.Errorf("number = %d, want 1", pr.Number)
	}
	if pr.Title != "Fix login timeout" {
		t.Errorf("title = %q", pr.Title)
	}
	if pr.State != "open" {
		t.Errorf("state = %q, want open", pr.State)
	}
	if pr.HeadRef != "fix-login" {
		t.Errorf("head_ref = %q, want fix-login", pr.HeadRef)
	}
	if pr.BaseRef != "main" {
		t.Errorf("base_ref = %q, want main", pr.BaseRef)
	}
	if pr.Author != "alice" {
		t.Errorf("author = %q, want alice", pr.Author)
	}
	if pr.URL != "https://gitlab.com/owner/repo/-/merge_requests/1" {
		t.Errorf("url = %q", pr.URL)
	}
	if meta.Count != 1 {
		t.Errorf("meta.Count = %d, want 1", meta.Count)
	}
}

func TestMRGetMapping(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tm := time.Now()
		mr := &gl.MergeRequest{
			BasicMergeRequest: gl.BasicMergeRequest{
				IID:          42,
				Title:        "Test MR",
				State:        "opened",
				Description:  "Full body content",
				SourceBranch: "feature",
				TargetBranch: "main",
				Author:       &gl.BasicUser{Username: "bob"},
				WebURL:       "https://gitlab.com/owner/repo/-/merge_requests/42",
				CreatedAt:    &tm,
				UpdatedAt:    &tm,
			},
		}
		respondJSON(w, http.StatusOK, mr)
	}))
	defer srv.Close()

	f := gitlabadapter.New(srv.URL, "owner", "repo", srv.Client())

	pr, err := f.PRs().Get(context.Background(), forge.PRGetOptions{Number: 42})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pr.Number != 42 {
		t.Errorf("number = %d, want 42", pr.Number)
	}
	if pr.Title != "Test MR" {
		t.Errorf("title = %q", pr.Title)
	}
	if pr.State != "open" {
		t.Errorf("state = %q, want open", pr.State)
	}
	if pr.Body != "Full body content" {
		t.Errorf("body = %q", pr.Body)
	}
	if pr.Author != "bob" {
		t.Errorf("author = %q", pr.Author)
	}
}

func TestMRGetAsDraft(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mr := &gl.MergeRequest{
			BasicMergeRequest: gl.BasicMergeRequest{
				IID:   1,
				Title: "Draft: WIP feature",
				State: "opened",
				Draft: true,
			},
		}
		respondJSON(w, http.StatusOK, mr)
	}))
	defer srv.Close()

	f := gitlabadapter.New(srv.URL, "owner", "repo", srv.Client())

	pr, err := f.PRs().Get(context.Background(), forge.PRGetOptions{Number: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pr.Extras == nil {
		t.Fatal("Extras should not be nil for draft MR")
	}
	draft, ok := pr.Extras["draft"].(bool)
	if !ok || !draft {
		t.Errorf("Extras[draft] = %v, want true", pr.Extras["draft"])
	}
}

func TestMRCreateMapping(t *testing.T) {
	var createdTitle, createdSourceBranch, createdTargetBranch string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var reqBody map[string]any
		_ = json.NewDecoder(r.Body).Decode(&reqBody)
		if t, ok := reqBody["title"].(string); ok {
			createdTitle = t
		}
		if sb, ok := reqBody["source_branch"].(string); ok {
			createdSourceBranch = sb
		}
		if tb, ok := reqBody["target_branch"].(string); ok {
			createdTargetBranch = tb
		}

		tm := time.Now()
		mr := &gl.MergeRequest{
			BasicMergeRequest: gl.BasicMergeRequest{
				IID:          99,
				Title:        createdTitle,
				State:        "opened",
				SourceBranch: createdSourceBranch,
				TargetBranch: createdTargetBranch,
				Author:       &gl.BasicUser{Username: "alice"},
				WebURL:       "https://gitlab.com/owner/repo/-/merge_requests/99",
				CreatedAt:    &tm,
				UpdatedAt:    &tm,
			},
		}
		respondJSON(w, http.StatusCreated, mr)
	}))
	defer srv.Close()

	f := gitlabadapter.New(srv.URL, "owner", "repo", srv.Client())

	pr, err := f.PRs().Create(context.Background(), forge.PRCreateOptions{
		Title:   forge.String("Created MR"),
		Body:    forge.String("Body text"),
		HeadRef: forge.String("feature-branch"),
		BaseRef: forge.String("main"),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pr.Number != 99 {
		t.Errorf("number = %d, want 99", pr.Number)
	}
	if pr.Title != "Created MR" {
		t.Errorf("title = %q", pr.Title)
	}
	if createdSourceBranch != "feature-branch" {
		t.Errorf("source_branch = %q", createdSourceBranch)
	}
	if createdTargetBranch != "main" {
		t.Errorf("target_branch = %q", createdTargetBranch)
	}
}

func TestMRCreateDraft(t *testing.T) {
	var createdTitle string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var reqBody map[string]any
		_ = json.NewDecoder(r.Body).Decode(&reqBody)
		if t, ok := reqBody["title"].(string); ok {
			createdTitle = t
		}

		tm := time.Now()
		mr := &gl.MergeRequest{
			BasicMergeRequest: gl.BasicMergeRequest{
				IID:       100,
				Title:     createdTitle,
				State:     "opened",
				Draft:     true,
				Author:    &gl.BasicUser{Username: "alice"},
				WebURL:    "https://gitlab.com/owner/repo/-/merge_requests/100",
				CreatedAt: &tm,
				UpdatedAt: &tm,
			},
		}
		respondJSON(w, http.StatusCreated, mr)
	}))
	defer srv.Close()

	f := gitlabadapter.New(srv.URL, "owner", "repo", srv.Client())

	pr, err := f.PRs().Create(context.Background(), forge.PRCreateOptions{
		Title:   forge.String("My Feature"),
		HeadRef: forge.String("feature"),
		BaseRef: forge.String("main"),
		Draft:   forge.Bool(true),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if createdTitle != "Draft: My Feature" {
		t.Errorf("title = %q, want %q", createdTitle, "Draft: My Feature")
	}
	// The returned MR has Draft=true from the mock, so Extras should be set.
	if pr.Extras == nil || pr.Extras["draft"] != true {
		t.Errorf("Extras[draft] = %v, want true", pr.Extras["draft"])
	}
}

func TestMRUpdateMapping(t *testing.T) {
	var updatedTitle string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var reqBody map[string]any
		_ = json.NewDecoder(r.Body).Decode(&reqBody)
		if t, ok := reqBody["title"].(string); ok {
			updatedTitle = t
		}

		mr := &gl.MergeRequest{
			BasicMergeRequest: gl.BasicMergeRequest{
				IID:   42,
				Title: updatedTitle,
				State: "opened",
			},
		}
		respondJSON(w, http.StatusOK, mr)
	}))
	defer srv.Close()

	f := gitlabadapter.New(srv.URL, "owner", "repo", srv.Client())

	pr, err := f.PRs().Update(context.Background(), forge.PRUpdateOptions{
		Number: 42,
		Title:  forge.String("Updated Title"),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pr.Number != 42 {
		t.Errorf("number = %d", pr.Number)
	}
	if pr.Title != "Updated Title" {
		t.Errorf("title = %q", pr.Title)
	}
	if updatedTitle != "Updated Title" {
		t.Errorf("sent title = %q", updatedTitle)
	}
}

func TestMRMergeMapping(t *testing.T) {
	var merged bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		merged = true
		mr := &gl.MergeRequest{
			BasicMergeRequest: gl.BasicMergeRequest{
				IID:   42,
				Title: "Test MR",
				State: "merged",
			},
		}
		respondJSON(w, http.StatusOK, mr)
	}))
	defer srv.Close()

	f := gitlabadapter.New(srv.URL, "owner", "repo", srv.Client())

	pr, err := f.PRs().Merge(context.Background(), forge.PRMergeOptions{
		Number: 42,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !merged {
		t.Error("merge was not called")
	}
	if pr.State != "merged" {
		t.Errorf("state = %q, want merged", pr.State)
	}
}

func TestMRCloseMapping(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mr := &gl.MergeRequest{
			BasicMergeRequest: gl.BasicMergeRequest{
				IID:   42,
				State: "closed",
			},
		}
		respondJSON(w, http.StatusOK, mr)
	}))
	defer srv.Close()

	f := gitlabadapter.New(srv.URL, "owner", "repo", srv.Client())

	pr, err := f.PRs().Close(context.Background(), forge.PRCloseOptions{Number: 42})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pr.State != "closed" {
		t.Errorf("state = %q, want closed", pr.State)
	}
}

func TestMRGetWithReviewers(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		// First call: Get MR
		if callCount == 1 {
			mr := &gl.MergeRequest{
				BasicMergeRequest: gl.BasicMergeRequest{
					IID:   1,
					Title: "Test MR",
					State: "opened",
				},
			}
			respondJSON(w, http.StatusOK, mr)
			return
		}
		// Second call: Get approvals
		approvals := &gl.MergeRequestApprovals{
			ApprovedBy: []*gl.MergeRequestApproverUser{
				{User: &gl.BasicUser{Username: "reviewer1"}},
				{User: &gl.BasicUser{Username: "reviewer2"}},
			},
		}
		respondJSON(w, http.StatusOK, approvals)
	}))
	defer srv.Close()

	f := gitlabadapter.New(srv.URL, "owner", "repo", srv.Client())

	pr, err := f.PRs().Get(context.Background(), forge.PRGetOptions{Number: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pr.Reviewers) != 2 {
		t.Fatalf("expected 2 reviewers, got %d", len(pr.Reviewers))
	}
	if pr.Reviewers[0].Login != "reviewer1" || pr.Reviewers[0].State != "APPROVED" {
		t.Errorf("reviewer 0: login=%q state=%q", pr.Reviewers[0].Login, pr.Reviewers[0].State)
	}
	if pr.Reviewers[1].Login != "reviewer2" || pr.Reviewers[1].State != "APPROVED" {
		t.Errorf("reviewer 1: login=%q state=%q", pr.Reviewers[1].Login, pr.Reviewers[1].State)
	}
}

func TestMRGetReviewersBestEffort(t *testing.T) {
	// When approvals API fails, Get should still succeed without reviewers.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only the MR endpoint exists; approvals endpoint returns 404.
		mr := &gl.MergeRequest{
			BasicMergeRequest: gl.BasicMergeRequest{
				IID:   1,
				Title: "Test MR",
				State: "opened",
			},
		}
		respondJSON(w, http.StatusOK, mr)
	}))
	defer srv.Close()

	f := gitlabadapter.New(srv.URL, "owner", "repo", srv.Client())

	pr, err := f.PRs().Get(context.Background(), forge.PRGetOptions{Number: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pr.Reviewers != nil {
		t.Errorf("reviewers should be nil when approvals API fails, got %v", pr.Reviewers)
	}
}

func TestMRGetWithCI(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mr := &gl.MergeRequest{
			BasicMergeRequest: gl.BasicMergeRequest{
				IID:   1,
				Title: "Test MR",
				State: "opened",
			},
			HeadPipeline: &gl.Pipeline{
				Status: "success",
			},
		}
		respondJSON(w, http.StatusOK, mr)
	}))
	defer srv.Close()

	f := gitlabadapter.New(srv.URL, "owner", "repo", srv.Client())

	pr, err := f.PRs().Get(context.Background(), forge.PRGetOptions{Number: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pr.Checks == nil {
		t.Fatal("Checks should not be nil when head_pipeline is present")
	}
	if pr.Checks.Passed != 1 || pr.Checks.Total != 1 {
		t.Errorf("Checks = %+v, want Passed=1 Total=1", pr.Checks)
	}
}

func TestMRGetWithFailedCI(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mr := &gl.MergeRequest{
			BasicMergeRequest: gl.BasicMergeRequest{
				IID:   1,
				Title: "Test MR",
				State: "opened",
			},
			HeadPipeline: &gl.Pipeline{
				Status: "failed",
			},
		}
		respondJSON(w, http.StatusOK, mr)
	}))
	defer srv.Close()

	f := gitlabadapter.New(srv.URL, "owner", "repo", srv.Client())

	pr, err := f.PRs().Get(context.Background(), forge.PRGetOptions{Number: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pr.Checks == nil {
		t.Fatal("Checks should not be nil when head_pipeline is present")
	}
	if pr.Checks.Passed != 0 || pr.Checks.Total != 1 {
		t.Errorf("Checks = %+v, want Passed=0 Total=1", pr.Checks)
	}
}

func TestMRCreateDraftAlreadyPrefixed(t *testing.T) {
	var createdTitle string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var reqBody map[string]any
		_ = json.NewDecoder(r.Body).Decode(&reqBody)
		if t, ok := reqBody["title"].(string); ok {
			createdTitle = t
		}

		tm := time.Now()
		mr := &gl.MergeRequest{
			BasicMergeRequest: gl.BasicMergeRequest{
				IID:       101,
				Title:     createdTitle,
				State:     "opened",
				Draft:     true,
				Author:    &gl.BasicUser{Username: "alice"},
				WebURL:    "https://gitlab.com/owner/repo/-/merge_requests/101",
				CreatedAt: &tm,
				UpdatedAt: &tm,
			},
		}
		respondJSON(w, http.StatusCreated, mr)
	}))
	defer srv.Close()

	f := gitlabadapter.New(srv.URL, "owner", "repo", srv.Client())

	_, err := f.PRs().Create(context.Background(), forge.PRCreateOptions{
		Title:   forge.String("Draft: Already Draft"),
		HeadRef: forge.String("feature"),
		BaseRef: forge.String("main"),
		Draft:   forge.Bool(true),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if createdTitle != "Draft: Already Draft" {
		t.Errorf("title = %q, want %q (no double prefix)", createdTitle, "Draft: Already Draft")
	}
}

func TestMRGetWithNonTerminalPipeline(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mr := &gl.MergeRequest{
			BasicMergeRequest: gl.BasicMergeRequest{
				IID:   1,
				Title: "Test MR",
				State: "opened",
			},
			HeadPipeline: &gl.Pipeline{
				Status: "running",
			},
		}
		respondJSON(w, http.StatusOK, mr)
	}))
	defer srv.Close()

	f := gitlabadapter.New(srv.URL, "owner", "repo", srv.Client())

	pr, err := f.PRs().Get(context.Background(), forge.PRGetOptions{Number: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pr.Checks != nil {
		t.Errorf("Checks should be nil for non-terminal pipeline, got %+v", pr.Checks)
	}
}

func TestMRGetWithoutCI(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mr := &gl.MergeRequest{
			BasicMergeRequest: gl.BasicMergeRequest{
				IID:   1,
				Title: "Test MR",
				State: "opened",
			},
		}
		respondJSON(w, http.StatusOK, mr)
	}))
	defer srv.Close()

	f := gitlabadapter.New(srv.URL, "owner", "repo", srv.Client())

	pr, err := f.PRs().Get(context.Background(), forge.PRGetOptions{Number: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pr.Checks != nil {
		t.Errorf("Checks should be nil when no head_pipeline, got %+v", pr.Checks)
	}
}

func TestMRListPagination(t *testing.T) {
	page := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page++
		mrs := []*gl.BasicMergeRequest{
			{IID: int64(page), Title: "MR", State: "opened"},
		}
		// Return page 2 as next on page 1, then stop.
		if page == 1 {
			w.Header().Set("X-Next-Page", "2")
		}
		respondJSON(w, http.StatusOK, mrs)
	}))
	defer srv.Close()

	f := gitlabadapter.New(srv.URL, "owner", "repo", srv.Client())

	prs, _, err := f.PRs().List(context.Background(), forge.PRListOptions{Limit: 50})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(prs) != 2 {
		t.Errorf("expected 2 PRs from 2 pages, got %d", len(prs))
	}
}

func TestMRErrorOnGet(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"message": "Not Found"})
	}))
	defer srv.Close()

	f := gitlabadapter.New(srv.URL, "owner", "repo", srv.Client())

	_, err := f.PRs().Get(context.Background(), forge.PRGetOptions{Number: 999})
	if err == nil {
		t.Fatal("expected error for 404")
	}
	errStr := err.Error()
	if !strings.Contains(errStr, "not found") && !strings.Contains(errStr, "404") {
		t.Errorf("error should mention not found or 404: %s", errStr)
	}
	var se forge.StructuredError
	if !errors.As(err, &se) {
		t.Errorf("error should be StructuredError, got %T: %v", err, err)
	}
}

func TestMRErrorOnList(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"message": "Unauthorized"})
	}))
	defer srv.Close()

	f := gitlabadapter.New(srv.URL, "owner", "repo", srv.Client())

	_, _, err := f.PRs().List(context.Background(), forge.PRListOptions{})
	if err == nil {
		t.Fatal("expected error for 401")
	}
	errStr := err.Error()
	if !strings.Contains(errStr, "authentication failed") {
		t.Errorf("error should mention authentication: %s", errStr)
	}
}
