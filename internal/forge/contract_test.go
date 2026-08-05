package forge_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	gitea "code.gitea.io/sdk/gitea"
	gh "github.com/google/go-github/v90/github"
	"github.com/tnikic/anvil/internal/forge"
	forgejoadapter "github.com/tnikic/anvil/internal/forge/forgejo"
	"github.com/tnikic/anvil/internal/forge/forgetest"
	githubadapter "github.com/tnikic/anvil/internal/forge/github"
	gitlabadapter "github.com/tnikic/anvil/internal/forge/gitlab"
	gl "gitlab.com/gitlab-org/api/client-go"
)

// respondJSON writes a JSON response to the test HTTP server.
func respondJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// ---- contract test harness ----

// newGitHubForge creates a github.Forge backed by an httptest server with the
// given handler. The server is auto-closed via t.Cleanup.
func newGitHubForge(t *testing.T, handler http.HandlerFunc) forge.Forge {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return githubadapter.New(srv.URL, "owner", "repo", srv.Client())
}

// newGitLabForge creates a gitlab.Forge backed by an httptest server with the
// given handler. The server is auto-closed via t.Cleanup.
func newGitLabForge(t *testing.T, handler http.HandlerFunc) forge.Forge {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return gitlabadapter.New(srv.URL, "owner", "repo", srv.Client())
}

// newForgejoForge creates a forgejo.Forge backed by an httptest server with the
// given handler. The server is auto-closed via t.Cleanup.
func newForgejoForge(t *testing.T, handler http.HandlerFunc) forge.Forge {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return forgejoadapter.New(srv.URL, "owner", "repo", srv.Client())
}

// forEachAdapter runs sub-tests for each forge adapter that supports the
// given capability. Current adapters: FakeForge, GitHub, GitLab.
//
//nolint:unused // Used in future contract test refactoring (see #36, #37).
func forEachAdapter(t *testing.T, name string, fn func(t *testing.T, f forge.Forge)) {
	t.Helper()
	t.Run(name+"_FakeForge", func(t *testing.T) {
		fn(t, forgetest.NewFakeForge())
	})
	t.Run(name+"_GitHub", func(t *testing.T) {
		// Each sub-test creates its own server; see specific tests.
		fn(t, nil) // placeholder — sub-tests create their own
	})
	t.Run(name+"_GitLab", func(t *testing.T) {
		fn(t, nil) // placeholder — sub-tests create their own
	})
}

// ---- label normalization contract ----

func TestContract_LabelNormalization(t *testing.T) {
	// FakeForge: labels are stored directly with scope+name split.
	// GitHub: labels arrive as "kind:bug" and must be split.
	// Both must produce: Label{Scope: "kind", Name: "bug"}.

	ctx := context.Background()

	t.Run("scoped label", func(t *testing.T) {
		// FakeForge
		fake := forgetest.NewFakeForge()
		created, err := fake.Labels().Create(ctx, forge.LabelCreateOptions{
			Scope: forge.String("kind"),
			Name:  "bug",
			Color: forge.String("ff0000"),
		})
		if err != nil {
			t.Fatalf("FakeForge create: %v", err)
		}
		if created.Scope != "kind" || created.Name != "bug" {
			t.Errorf("FakeForge: scope=%q name=%q, want scope=kind name=bug",
				created.Scope, created.Name)
		}

		// GitHub (httptest)
		ghForge := newGitHubForge(t, func(w http.ResponseWriter, r *http.Request) {
			label := &gh.Label{
				Name:        "kind:bug",
				Color:       "ff0000",
				Description: gh.Ptr("A bug"),
			}
			respondJSON(w, http.StatusCreated, label)
		})
		ghCreated, err := ghForge.Labels().Create(ctx, forge.LabelCreateOptions{
			Scope: forge.String("kind"),
			Name:  "bug",
			Color: forge.String("ff0000"),
		})
		if err != nil {
			t.Fatalf("GitHub create: %v", err)
		}
		if ghCreated.Scope != "kind" || ghCreated.Name != "bug" {
			t.Errorf("GitHub: scope=%q name=%q, want scope=kind name=bug",
				ghCreated.Scope, ghCreated.Name)
		}

		// GitLab (httptest)
		glForge := newGitLabForge(t, func(w http.ResponseWriter, r *http.Request) {
			label := &gl.Label{
				Name:        "kind::bug",
				Color:       "#ff0000",
				Description: "A bug",
			}
			respondJSON(w, http.StatusCreated, label)
		})
		glCreated, err := glForge.Labels().Create(ctx, forge.LabelCreateOptions{
			Scope: forge.String("kind"),
			Name:  "bug",
			Color: forge.String("#ff0000"),
		})
		if err != nil {
			t.Fatalf("GitLab create: %v", err)
		}
		if glCreated.Scope != "kind" || glCreated.Name != "bug" {
			t.Errorf("GitLab: scope=%q name=%q, want scope=kind name=bug",
				glCreated.Scope, glCreated.Name)
		}

		// Forgejo (httptest)
		fjForge := newForgejoForge(t, func(w http.ResponseWriter, r *http.Request) {
			label := &gitea.Label{
				Name:  "kind/bug",
				Color: "ff0000",
			}
			respondJSON(w, http.StatusCreated, label)
		})
		fjCreated, err := fjForge.Labels().Create(ctx, forge.LabelCreateOptions{
			Scope: forge.String("kind"),
			Name:  "bug",
			Color: forge.String("ff0000"),
		})
		if err != nil {
			t.Fatalf("Forgejo create: %v", err)
		}
		if fjCreated.Scope != "kind" || fjCreated.Name != "bug" {
			t.Errorf("Forgejo: scope=%q name=%q, want scope=kind name=bug",
				fjCreated.Scope, fjCreated.Name)
		}
	})

	t.Run("unscoped label", func(t *testing.T) {
		// FakeForge
		fake := forgetest.NewFakeForge()
		created, err := fake.Labels().Create(ctx, forge.LabelCreateOptions{
			Name:  "good-first-issue",
			Color: forge.String("7057ff"),
		})
		if err != nil {
			t.Fatalf("FakeForge create: %v", err)
		}
		if created.Scope != "" || created.Name != "good-first-issue" {
			t.Errorf("FakeForge: scope=%q name=%q, want scope=<empty> name=good-first-issue",
				created.Scope, created.Name)
		}

		// GitHub (httptest)
		ghForge := newGitHubForge(t, func(w http.ResponseWriter, r *http.Request) {
			label := &gh.Label{
				Name:  "good-first-issue",
				Color: "7057ff",
			}
			respondJSON(w, http.StatusCreated, label)
		})
		ghCreated, err := ghForge.Labels().Create(ctx, forge.LabelCreateOptions{
			Name:  "good-first-issue",
			Color: forge.String("7057ff"),
		})
		if err != nil {
			t.Fatalf("GitHub create: %v", err)
		}
		if ghCreated.Scope != "" || ghCreated.Name != "good-first-issue" {
			t.Errorf("GitHub: scope=%q name=%q, want scope=<empty> name=good-first-issue",
				ghCreated.Scope, ghCreated.Name)
		}

		// GitLab (httptest)
		glForge := newGitLabForge(t, func(w http.ResponseWriter, r *http.Request) {
			label := &gl.Label{
				Name:  "good-first-issue",
				Color: "#7057ff",
			}
			respondJSON(w, http.StatusCreated, label)
		})
		glCreated, err := glForge.Labels().Create(ctx, forge.LabelCreateOptions{
			Name:  "good-first-issue",
			Color: forge.String("#7057ff"),
		})
		if err != nil {
			t.Fatalf("GitLab create: %v", err)
		}
		if glCreated.Scope != "" || glCreated.Name != "good-first-issue" {
			t.Errorf("GitLab: scope=%q name=%q, want scope=<empty> name=good-first-issue",
				glCreated.Scope, glCreated.Name)
		}

		// Forgejo (httptest)
		fjForge := newForgejoForge(t, func(w http.ResponseWriter, r *http.Request) {
			label := &gitea.Label{
				Name:  "good-first-issue",
				Color: "7057ff",
			}
			respondJSON(w, http.StatusCreated, label)
		})
		fjCreated, err := fjForge.Labels().Create(ctx, forge.LabelCreateOptions{
			Name:  "good-first-issue",
			Color: forge.String("7057ff"),
		})
		if err != nil {
			t.Fatalf("Forgejo create: %v", err)
		}
		if fjCreated.Scope != "" || fjCreated.Name != "good-first-issue" {
			t.Errorf("Forgejo: scope=%q name=%q, want scope=<empty> name=good-first-issue",
				fjCreated.Scope, fjCreated.Name)
		}
	})
}

// ---- issue shape contract ----

func TestContract_IssueFields(t *testing.T) {
	// Both adapters must return Issue structs with the same normalized fields
	// populated for equivalent operations.

	ctx := context.Background()

	t.Run("list returns normalized issues", func(t *testing.T) {
		// FakeForge: pre-populate with an issue
		fake := forgetest.NewFakeForge()
		fake.IssueSvc.ListFn = func(ctx context.Context, opts forge.IssueListOptions) ([]forge.Issue, *forge.ListMeta, error) {
			return []forge.Issue{
				{Number: 1, Title: "Test Issue", State: "open", Author: "alice", URL: "https://github.com/owner/repo/issues/1"},
			}, &forge.ListMeta{Total: 1, Count: 1}, nil
		}

		issues, meta, err := fake.Issues().List(ctx, forge.IssueListOptions{State: "open"})
		if err != nil {
			t.Fatalf("FakeForge list: %v", err)
		}
		if len(issues) != 1 || issues[0].Number != 1 || issues[0].Title != "Test Issue" {
			t.Errorf("FakeForge: unexpected result: %+v", issues)
		}
		if meta.Total != 1 {
			t.Errorf("FakeForge: meta.Total = %d, want 1", meta.Total)
		}
		// GitHub (httptest)
		ghForge := newGitHubForge(t, func(w http.ResponseWriter, r *http.Request) {
			issues := []*gh.Issue{{
				Number:  gh.Ptr(1),
				Title:   gh.Ptr("Test Issue"),
				State:   gh.Ptr("open"),
				User:    &gh.User{Login: gh.Ptr("alice")},
				HTMLURL: gh.Ptr("https://github.com/owner/repo/issues/1"),
				Labels:  []*gh.Label{},
			}}
			respondJSON(w, http.StatusOK, issues)
		})
		ghIssues, ghMeta, err := ghForge.Issues().List(ctx, forge.IssueListOptions{State: "open"})
		if err != nil {
			t.Fatalf("GitHub list: %v", err)
		}
		if len(ghIssues) != 1 || ghIssues[0].Number != 1 || ghIssues[0].Title != "Test Issue" {
			t.Errorf("GitHub: unexpected result: %+v", ghIssues)
		}
		if ghMeta.Count != 1 {
			t.Errorf("GitHub: meta.Count = %d, want 1", ghMeta.Count)
		}
	})

	t.Run("get returns all normalized fields", func(t *testing.T) {
		// FakeForge
		fake := forgetest.NewFakeForge()
		fake.IssueSvc.GetFn = func(ctx context.Context, opts forge.IssueGetOptions) (*forge.Issue, error) {
			return &forge.Issue{
				Number: 42, Title: "Found", State: "open", Body: "Body text",
				Author: "bob", URL: "https://github.com/owner/repo/issues/42",
			}, nil
		}

		issue, err := fake.Issues().Get(ctx, forge.IssueGetOptions{Number: 42})
		if err != nil {
			t.Fatalf("FakeForge get: %v", err)
		}
		if issue.Number != 42 || issue.Title != "Found" || issue.Body != "Body text" || issue.Author != "bob" {
			t.Errorf("FakeForge: unexpected fields: %+v", issue)
		}

		// GitHub (httptest)
		ghForge := newGitHubForge(t, func(w http.ResponseWriter, r *http.Request) {
			issue := &gh.Issue{
				Number:  gh.Ptr(42),
				Title:   gh.Ptr("Found"),
				State:   gh.Ptr("open"),
				Body:    gh.Ptr("Body text"),
				User:    &gh.User{Login: gh.Ptr("bob")},
				HTMLURL: gh.Ptr("https://github.com/owner/repo/issues/42"),
			}
			respondJSON(w, http.StatusOK, issue)
		})
		ghIssue, err := ghForge.Issues().Get(ctx, forge.IssueGetOptions{Number: 42})
		if err != nil {
			t.Fatalf("GitHub get: %v", err)
		}
		if ghIssue.Number != 42 || ghIssue.Title != "Found" || ghIssue.Body != "Body text" || ghIssue.Author != "bob" {
			t.Errorf("GitHub: unexpected fields: %+v", ghIssue)
		}

		// GitLab (httptest)
		glForge := newGitLabForge(t, func(w http.ResponseWriter, r *http.Request) {
			tm := time.Now()
			issue := &gl.Issue{
				IID:         42,
				Title:       "Found",
				State:       "opened",
				Description: "Body text",
				Author:      &gl.IssueAuthor{Username: "bob"},
				WebURL:      "https://gitlab.com/owner/repo/-/issues/42",
				CreatedAt:   &tm,
				UpdatedAt:   &tm,
			}
			respondJSON(w, http.StatusOK, issue)
		})
		glIssue, err := glForge.Issues().Get(ctx, forge.IssueGetOptions{Number: 42})
		if err != nil {
			t.Fatalf("GitLab get: %v", err)
		}
		if glIssue.Number != 42 || glIssue.Title != "Found" || glIssue.Body != "Body text" || glIssue.Author != "bob" {
			t.Errorf("GitLab: unexpected fields: %+v", glIssue)
		}

		// Forgejo (httptest)
		fjForge := newForgejoForge(t, func(w http.ResponseWriter, r *http.Request) {
			issue := &gitea.Issue{
				Index:   42,
				Title:   "Found",
				State:   "open",
				Body:    "Body text",
				Poster:  &gitea.User{UserName: "bob"},
				HTMLURL: "https://codeberg.org/owner/repo/issues/42",
			}
			respondJSON(w, http.StatusOK, issue)
		})
		fjIssue, err := fjForge.Issues().Get(ctx, forge.IssueGetOptions{Number: 42})
		if err != nil {
			t.Fatalf("Forgejo get: %v", err)
		}
		if fjIssue.Number != 42 || fjIssue.Title != "Found" || fjIssue.Body != "Body text" || fjIssue.Author != "bob" {
			t.Errorf("Forgejo: unexpected fields: %+v", fjIssue)
		}
		if fjIssue.URL != "https://codeberg.org/owner/repo/issues/42" {
			t.Errorf("Forgejo: URL=%q, want https://codeberg.org/owner/repo/issues/42", fjIssue.URL)
		}
	})
}

// ---- error contract ----

func TestContract_StructuredError(t *testing.T) {
	// Both adapters must return errors that satisfy forge.StructuredError
	// for known failure modes (auth, not found, rate limit).

	ctx := context.Background()

	t.Run("not found is StructuredError", func(t *testing.T) {
		// FakeForge: configure GetFn to return a BaseError
		fake := forgetest.NewFakeForge()
		fake.IssueSvc.GetFn = func(ctx context.Context, opts forge.IssueGetOptions) (*forge.Issue, error) {
			return nil, forge.NewBaseError("not found", "Run \"anvil issue list\"")
		}
		_, err := fake.Issues().Get(ctx, forge.IssueGetOptions{Number: 999})
		assertStructuredError(t, err, "not found")

		// GitHub (httptest): returns 404
		ghForge := newGitHubForge(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]string{"message": "Not Found"})
		})
		_, err = ghForge.Issues().Get(ctx, forge.IssueGetOptions{Number: 999})
		if err == nil {
			t.Fatal("GitHub: expected error for 404")
		}
		var se forge.StructuredError
		if !asStructuredError(err, &se) {
			t.Errorf("GitHub 404 error should satisfy StructuredError, got %T: %v", err, err)
		}

		// GitLab (httptest): returns 404
		glForge := newGitLabForge(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]string{"message": "Not Found"})
		})
		_, err = glForge.Issues().Get(ctx, forge.IssueGetOptions{Number: 999})
		if err == nil {
			t.Fatal("GitLab: expected error for 404")
		}
		if !asStructuredError(err, &se) {
			t.Errorf("GitLab 404 error should satisfy StructuredError, got %T: %v", err, err)
		}

		// Forgejo (httptest): returns 404
		fjForge := newForgejoForge(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]string{"message": "Not Found"})
		})
		_, err = fjForge.Issues().Get(ctx, forge.IssueGetOptions{Number: 999})
		if err == nil {
			t.Fatal("Forgejo: expected error for 404")
		}
		if !asStructuredError(err, &se) {
			t.Errorf("Forgejo 404 error should satisfy StructuredError, got %T: %v", err, err)
		}
	})

	t.Run("auth failure is StructuredError", func(t *testing.T) {
		// FakeForge: BaseError simulates auth failure
		fake := forgetest.NewFakeForge()
		fake.IssueSvc.ListFn = func(ctx context.Context, opts forge.IssueListOptions) ([]forge.Issue, *forge.ListMeta, error) {
			return nil, nil, forge.NewBaseError("authentication failed", "Run \"anvil auth set\"")
		}
		_, _, err := fake.Issues().List(ctx, forge.IssueListOptions{})
		assertStructuredError(t, err, "authentication failed")

		// GitHub (httptest): returns 401
		ghForge := newGitHubForge(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]string{"message": "Bad credentials"})
		})
		_, _, err = ghForge.Issues().List(ctx, forge.IssueListOptions{})
		if err == nil {
			t.Fatal("GitHub: expected error for 401")
		}
		var se forge.StructuredError
		if !asStructuredError(err, &se) {
			t.Errorf("GitHub 401 error should satisfy StructuredError, got %T: %v", err, err)
		}

		// GitLab (httptest): returns 401
		glForge := newGitLabForge(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]string{"message": "Unauthorized"})
		})
		_, _, err = glForge.Issues().List(ctx, forge.IssueListOptions{})
		if err == nil {
			t.Fatal("GitLab: expected error for 401")
		}
		if !asStructuredError(err, &se) {
			t.Errorf("GitLab 401 error should satisfy StructuredError, got %T: %v", err, err)
		}

		// Forgejo (httptest): returns 401
		fjForge := newForgejoForge(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]string{"message": "unauthorized"})
		})
		_, _, err = fjForge.Issues().List(ctx, forge.IssueListOptions{})
		if err == nil {
			t.Fatal("Forgejo: expected error for 401")
		}
		if !asStructuredError(err, &se) {
			t.Errorf("Forgejo 401 error should satisfy StructuredError, got %T: %v", err, err)
		}
	})
}

// ---- label CRUD consistency ----

func TestContract_LabelCRUD(t *testing.T) {
	// Labels created via the adapter should be listable and deletable
	// with consistent scope+name identity.

	ctx := context.Background()

	t.Run("create then list returns the label", func(t *testing.T) {
		// FakeForge: state-based
		fake := forgetest.NewFakeForge()
		_, err := fake.Labels().Create(ctx, forge.LabelCreateOptions{
			Scope: forge.String("priority"),
			Name:  "high",
			Color: forge.String("ff0000"),
		})
		if err != nil {
			t.Fatalf("FakeForge create: %v", err)
		}
		labels, err := fake.Labels().List(ctx, forge.LabelListOptions{})
		if err != nil {
			t.Fatalf("FakeForge list: %v", err)
		}
		if len(labels) != 1 || labels[0].Scope != "priority" || labels[0].Name != "high" {
			t.Errorf("FakeForge: list returned %+v", labels)
		}

		// GitHub (httptest): create returns the label, list returns it
		var createdLabel string
		ghForge := newGitHubForge(t, func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPost {
				label := &gh.Label{
					Name:        "priority:high",
					Color:       "ff0000",
					Description: gh.Ptr(""),
				}
				createdLabel = "priority:high"
				respondJSON(w, http.StatusCreated, label)
				return
			}
			// GET — list
			labels := []*gh.Label{{
				Name:  createdLabel,
				Color: "ff0000",
			}}
			respondJSON(w, http.StatusOK, labels)
		})
		_, err = ghForge.Labels().Create(ctx, forge.LabelCreateOptions{
			Scope: forge.String("priority"),
			Name:  "high",
			Color: forge.String("ff0000"),
		})
		if err != nil {
			t.Fatalf("GitHub create: %v", err)
		}
		ghLabels, err := ghForge.Labels().List(ctx, forge.LabelListOptions{})
		if err != nil {
			t.Fatalf("GitHub list: %v", err)
		}
		if len(ghLabels) != 1 || ghLabels[0].Scope != "priority" || ghLabels[0].Name != "high" {
			t.Errorf("GitHub: list returned %+v", ghLabels)
		}

		// GitLab (httptest): create returns the label, list returns it
		var glCreatedLabel string
		glForge := newGitLabForge(t, func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPost {
				label := &gl.Label{
					Name:        "priority::high",
					Color:       "#ff0000",
					Description: "",
				}
				glCreatedLabel = "priority::high"
				respondJSON(w, http.StatusCreated, label)
				return
			}
			// GET — list
			labels := []*gl.Label{{
				Name:  glCreatedLabel,
				Color: "#ff0000",
			}}
			respondJSON(w, http.StatusOK, labels)
		})
		_, err = glForge.Labels().Create(ctx, forge.LabelCreateOptions{
			Scope: forge.String("priority"),
			Name:  "high",
			Color: forge.String("#ff0000"),
		})
		if err != nil {
			t.Fatalf("GitLab create: %v", err)
		}
		glLabels, err := glForge.Labels().List(ctx, forge.LabelListOptions{})
		if err != nil {
			t.Fatalf("GitLab list: %v", err)
		}
		if len(glLabels) != 1 || glLabels[0].Scope != "priority" || glLabels[0].Name != "high" {
			t.Errorf("GitLab: list returned %+v", glLabels)
		}

		// Forgejo (httptest): create returns the label, list returns it
		var fjCreatedLabel string
		fjForge := newForgejoForge(t, func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPost {
				label := &gitea.Label{
					Name:  "priority/high",
					Color: "ff0000",
				}
				fjCreatedLabel = "priority/high"
				respondJSON(w, http.StatusCreated, label)
				return
			}
			// GET — list
			labels := []*gitea.Label{{
				Name:  fjCreatedLabel,
				Color: "ff0000",
			}}
			respondJSON(w, http.StatusOK, labels)
		})
		_, err = fjForge.Labels().Create(ctx, forge.LabelCreateOptions{
			Scope: forge.String("priority"),
			Name:  "high",
			Color: forge.String("ff0000"),
		})
		if err != nil {
			t.Fatalf("Forgejo create: %v", err)
		}
		fjLabels, err := fjForge.Labels().List(ctx, forge.LabelListOptions{})
		if err != nil {
			t.Fatalf("Forgejo list: %v", err)
		}
		if len(fjLabels) != 1 || fjLabels[0].Scope != "priority" || fjLabels[0].Name != "high" {
			t.Errorf("Forgejo: list returned %+v", fjLabels)
		}
	})

	t.Run("delete unscoped label", func(t *testing.T) {
		// FakeForge
		fake := forgetest.NewFakeForge()
		_, err := fake.Labels().Create(ctx, forge.LabelCreateOptions{
			Name:  "enhancement",
			Color: forge.String("0052cc"),
		})
		if err != nil {
			t.Fatalf("FakeForge create: %v", err)
		}
		err = fake.Labels().Delete(ctx, forge.LabelDeleteOptions{Name: "enhancement"})
		if err != nil {
			t.Fatalf("FakeForge delete: %v", err)
		}
		labels, err := fake.Labels().List(ctx, forge.LabelListOptions{})
		if err != nil {
			t.Fatalf("FakeForge list after delete: %v", err)
		}
		if len(labels) != 0 {
			t.Errorf("FakeForge: expected 0 labels after delete, got %d", len(labels))
		}

		// GitHub (httptest)
		ghForge := newGitHubForge(t, func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPost {
				label := &gh.Label{Name: "enhancement", Color: "0052cc"}
				respondJSON(w, http.StatusCreated, label)
				return
			}
			if r.Method == http.MethodDelete || r.Method == "DELETE" {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			// GET — empty list after delete
			respondJSON(w, http.StatusOK, []*gh.Label{})
		})
		_, err = ghForge.Labels().Create(ctx, forge.LabelCreateOptions{
			Name:  "enhancement",
			Color: forge.String("0052cc"),
		})
		if err != nil {
			t.Fatalf("GitHub create: %v", err)
		}
		err = ghForge.Labels().Delete(ctx, forge.LabelDeleteOptions{Name: "enhancement"})
		if err != nil {
			t.Fatalf("GitHub delete: %v", err)
		}
		ghLabels, err := ghForge.Labels().List(ctx, forge.LabelListOptions{})
		if err != nil {
			t.Fatalf("GitHub list after delete: %v", err)
		}
		if len(ghLabels) != 0 {
			t.Errorf("GitHub: expected 0 labels after delete, got %d", len(ghLabels))
		}

		// GitLab (httptest)
		glForge := newGitLabForge(t, func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPost {
				label := &gl.Label{Name: "enhancement", Color: "#0052cc"}
				respondJSON(w, http.StatusCreated, label)
				return
			}
			if r.Method == http.MethodDelete || r.Method == "DELETE" {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			// GET — empty list after delete
			respondJSON(w, http.StatusOK, []*gl.Label{})
		})
		_, err = glForge.Labels().Create(ctx, forge.LabelCreateOptions{
			Name:  "enhancement",
			Color: forge.String("#0052cc"),
		})
		if err != nil {
			t.Fatalf("GitLab create: %v", err)
		}
		err = glForge.Labels().Delete(ctx, forge.LabelDeleteOptions{Name: "enhancement"})
		if err != nil {
			t.Fatalf("GitLab delete: %v", err)
		}
		glLabels, err := glForge.Labels().List(ctx, forge.LabelListOptions{})
		if err != nil {
			t.Fatalf("GitLab list after delete: %v", err)
		}
		if len(glLabels) != 0 {
			t.Errorf("GitLab: expected 0 labels after delete, got %d", len(glLabels))
		}

		// Forgejo (httptest): delete requires label ID resolution via list
		var fjStoredLabels []*gitea.Label
		fjForge := newForgejoForge(t, func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPost {
				newLabel := &gitea.Label{
					ID:    int64(len(fjStoredLabels) + 1),
					Name:  "enhancement",
					Color: "0052cc",
				}
				fjStoredLabels = append(fjStoredLabels, newLabel)
				respondJSON(w, http.StatusCreated, newLabel)
				return
			}
			if r.Method == "DELETE" {
				fjStoredLabels = nil
				w.WriteHeader(http.StatusNoContent)
				return
			}
			// GET — list (used by resolveLabelID and final list)
			respondJSON(w, http.StatusOK, fjStoredLabels)
		})
		_, err = fjForge.Labels().Create(ctx, forge.LabelCreateOptions{
			Name:  "enhancement",
			Color: forge.String("0052cc"),
		})
		if err != nil {
			t.Fatalf("Forgejo create: %v", err)
		}
		err = fjForge.Labels().Delete(ctx, forge.LabelDeleteOptions{Name: "enhancement"})
		if err != nil {
			t.Fatalf("Forgejo delete: %v", err)
		}
		fjLabels, err := fjForge.Labels().List(ctx, forge.LabelListOptions{})
		if err != nil {
			t.Fatalf("Forgejo list after delete: %v", err)
		}
		if len(fjLabels) != 0 {
			t.Errorf("Forgejo: expected 0 labels after delete, got %d", len(fjLabels))
		}
	})
}

// ---- PR field mapping (Forgejo) ----

func TestContract_Forgejo_PRFields(t *testing.T) {
	// The Forgejo adapter must map PullRequest fields correctly:
	// draft → Extras["draft"], base.ref → BaseRef, head.ref → HeadRef,
	// body → Body, html_url → URL, user.login → Author.

	ctx := context.Background()

	t.Run("get returns normalized PR fields", func(t *testing.T) {
		now := time.Now()
		fjForge := newForgejoForge(t, func(w http.ResponseWriter, r *http.Request) {
			pr := &gitea.PullRequest{
				Index:   42,
				Title:   "Add OAuth support",
				State:   "open",
				Draft:   true,
				Body:    "Implements OAuth 2.0 flow.",
				HTMLURL: "https://codeberg.org/owner/repo/pulls/42",
				Poster:  &gitea.User{UserName: "pr-author"},
				Base:    &gitea.PRBranchInfo{Ref: "main"},
				Head:    &gitea.PRBranchInfo{Ref: "feat/oauth"},
				Created: &now,
				Updated: &now,
			}
			respondJSON(w, http.StatusOK, pr)
		})

		pr, err := fjForge.PRs().Get(ctx, forge.PRGetOptions{Number: 42})
		if err != nil {
			t.Fatalf("Forgejo PR get: %v", err)
		}
		if pr.Number != 42 {
			t.Errorf("Number = %d, want 42", pr.Number)
		}
		if pr.Body != "Implements OAuth 2.0 flow." {
			t.Errorf("Body = %q", pr.Body)
		}
		if pr.URL != "https://codeberg.org/owner/repo/pulls/42" {
			t.Errorf("URL = %q", pr.URL)
		}
		if pr.Author != "pr-author" {
			t.Errorf("Author = %q, want pr-author", pr.Author)
		}
		if pr.BaseRef != "main" {
			t.Errorf("BaseRef = %q, want main", pr.BaseRef)
		}
		if pr.HeadRef != "feat/oauth" {
			t.Errorf("HeadRef = %q, want feat/oauth", pr.HeadRef)
		}
		if pr.Extras == nil || pr.Extras["draft"] != true {
			t.Errorf("Extras[draft] should be true for draft PR, got %v", pr.Extras)
		}
	})

	t.Run("non-draft PR has no draft extras", func(t *testing.T) {
		now := time.Now()
		fjForge := newForgejoForge(t, func(w http.ResponseWriter, r *http.Request) {
			pr := &gitea.PullRequest{
				Index:   1,
				Title:   "Regular PR",
				State:   "open",
				Draft:   false,
				HTMLURL: "https://codeberg.org/owner/repo/pulls/1",
				Poster:  &gitea.User{UserName: "alice"},
				Base:    &gitea.PRBranchInfo{Ref: "main"},
				Head:    &gitea.PRBranchInfo{Ref: "feat/x"},
				Created: &now,
				Updated: &now,
			}
			respondJSON(w, http.StatusOK, pr)
		})

		pr, err := fjForge.PRs().Get(ctx, forge.PRGetOptions{Number: 1})
		if err != nil {
			t.Fatalf("Forgejo PR get: %v", err)
		}
		if pr.Extras != nil && pr.Extras["draft"] != nil {
			t.Errorf("Extras[draft] should not be set for non-draft PR")
		}
	})
}

// ---- sub-issue title convention (Forgejo) ----

func TestContract_Forgejo_SubIssueTitle(t *testing.T) {
	// Forgejo stores parent-child relationships via [parent:N] title prefix.
	// The adapter must parse the prefix on read and inject it on write.

	ctx := context.Background()

	t.Run("read parses parent from title prefix", func(t *testing.T) {
		fjForge := newForgejoForge(t, func(w http.ResponseWriter, r *http.Request) {
			issue := &gitea.Issue{
				Index:   2,
				Title:   "[parent:42] Sub-task for login",
				State:   "open",
				Body:    "Sub-task body",
				Poster:  &gitea.User{UserName: "alice"},
				HTMLURL: "https://codeberg.org/owner/repo/issues/2",
			}
			respondJSON(w, http.StatusOK, issue)
		})

		issue, err := fjForge.Issues().Get(ctx, forge.IssueGetOptions{Number: 2})
		if err != nil {
			t.Fatalf("Forgejo get: %v", err)
		}
		if issue.Parent == nil {
			t.Fatal("expected Parent to be set")
		}
		if *issue.Parent != 42 {
			t.Errorf("Parent = %d, want 42", *issue.Parent)
		}
		if issue.Title != "Sub-task for login" {
			t.Errorf("Title = %q, want %q", issue.Title, "Sub-task for login")
		}
	})

	t.Run("read returns nil Parent for regular issue", func(t *testing.T) {
		fjForge := newForgejoForge(t, func(w http.ResponseWriter, r *http.Request) {
			issue := &gitea.Issue{
				Index:   1,
				Title:   "Regular issue",
				State:   "open",
				Poster:  &gitea.User{UserName: "bob"},
				HTMLURL: "https://codeberg.org/owner/repo/issues/1",
			}
			respondJSON(w, http.StatusOK, issue)
		})

		issue, err := fjForge.Issues().Get(ctx, forge.IssueGetOptions{Number: 1})
		if err != nil {
			t.Fatalf("Forgejo get: %v", err)
		}
		if issue.Parent != nil {
			t.Errorf("Parent should be nil for regular issue, got %d", *issue.Parent)
		}
	})

	t.Run("add parent injects title prefix", func(t *testing.T) {
		// AddParentOf sends a PATCH with [parent:N] prefix in the title.
		var patchedTitle string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// GET /issues/2 (child) — returned by Parent() check and GetIssue
			if r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/issues/2") && !strings.Contains(r.URL.Path, "/issues/1") {
				issue := &gitea.Issue{
					Index:   2,
					Title:   "Child task",
					State:   "open",
					Poster:  &gitea.User{UserName: "bob"},
					HTMLURL: "https://codeberg.org/owner/repo/issues/2",
				}
				respondJSON(w, http.StatusOK, issue)
				return
			}
			// PATCH /issues/2 — update title with parent prefix
			if r.Method == "PATCH" && strings.Contains(r.URL.Path, "/issues/2") {
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
				}
				respondJSON(w, http.StatusOK, issue)
				return
			}
			w.WriteHeader(http.StatusNotFound)
		}))
		defer srv.Close()

		f := forgejoadapter.New(srv.URL, "owner", "repo", srv.Client())

		err := f.Relations().AddParentOf(context.Background(), 42, 2)
		if err != nil {
			t.Fatalf("AddParentOf: %v", err)
		}
		if patchedTitle != "[parent:42] Child task" {
			t.Errorf("patched title = %q, want %q", patchedTitle, "[parent:42] Child task")
		}
	})

	t.Run("update preserves parent prefix", func(t *testing.T) {
		// When updating a sub-issue's title, the parent prefix should be preserved.
		var patchedTitle string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/issues/2") {
				issue := &gitea.Issue{
					Index:   2,
					Title:   "[parent:1] Old Title",
					State:   "open",
					Poster:  &gitea.User{UserName: "bob"},
					HTMLURL: "https://codeberg.org/owner/repo/issues/2",
				}
				respondJSON(w, http.StatusOK, issue)
				return
			}
			if r.Method == "PATCH" && strings.Contains(r.URL.Path, "/issues/2") {
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
				}
				respondJSON(w, http.StatusOK, issue)
				return
			}
			w.WriteHeader(http.StatusNotFound)
		}))
		defer srv.Close()

		f := forgejoadapter.New(srv.URL, "owner", "repo", srv.Client())

		newTitle := "New Title"
		updated, err := f.Issues().Update(context.Background(), forge.IssueUpdateOptions{
			Number: 2,
			Title:  &newTitle,
		})
		if err != nil {
			t.Fatalf("Update: %v", err)
		}
		if patchedTitle != "[parent:1] New Title" {
			t.Errorf("patched title = %q, want %q", patchedTitle, "[parent:1] New Title")
		}
		// The returned issue should have the parent parsed out of the title.
		if updated.Parent == nil || *updated.Parent != 1 {
			t.Errorf("Parent should be 1, got %v", updated.Parent)
		}
		if updated.Title != "New Title" {
			t.Errorf("Title = %q, want %q", updated.Title, "New Title")
		}
	})
}

// ---- comment mapping (Forgejo) ----

func TestContract_Forgejo_CommentMapping(t *testing.T) {
	// The Forgejo adapter must map comment fields correctly:
	// body → Body, html_url → URL, user.login → Author, System always false.

	ctx := context.Background()

	t.Run("list returns normalized comments", func(t *testing.T) {
		now := time.Now()
		fjForge := newForgejoForge(t, func(w http.ResponseWriter, r *http.Request) {
			comments := []*gitea.Comment{
				{
					ID:      1,
					Body:    "This is a comment.",
					HTMLURL: "https://codeberg.org/owner/repo/issues/1#issuecomment-1",
					Poster:  &gitea.User{UserName: "commenter"},
					Created: now,
					Updated: now,
				},
			}
			respondJSON(w, http.StatusOK, comments)
		})

		comments, err := fjForge.Comments().List(ctx, forge.CommentListOptions{IssueNumber: 1})
		if err != nil {
			t.Fatalf("Forgejo comment list: %v", err)
		}
		if len(comments) != 1 {
			t.Fatalf("expected 1 comment, got %d", len(comments))
		}
		c := comments[0]
		if c.Body != "This is a comment." {
			t.Errorf("Body = %q", c.Body)
		}
		if c.URL != "https://codeberg.org/owner/repo/issues/1#issuecomment-1" {
			t.Errorf("URL = %q", c.URL)
		}
		if c.Author != "commenter" {
			t.Errorf("Author = %q, want commenter", c.Author)
		}
		if c.System {
			t.Errorf("System should always be false for Forgejo comments")
		}
	})

	t.Run("get returns normalized comment", func(t *testing.T) {
		now := time.Now()
		fjForge := newForgejoForge(t, func(w http.ResponseWriter, r *http.Request) {
			comment := &gitea.Comment{
				ID:      42,
				Body:    "Single comment body",
				HTMLURL: "https://codeberg.org/owner/repo/issues/1#issuecomment-42",
				Poster:  &gitea.User{UserName: "alice"},
				Created: now,
				Updated: now,
			}
			respondJSON(w, http.StatusOK, comment)
		})

		c, err := fjForge.Comments().Get(ctx, forge.CommentGetOptions{IssueNumber: 1, CommentID: 42})
		if err != nil {
			t.Fatalf("Forgejo comment get: %v", err)
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
			t.Errorf("System should always be false for Forgejo comments")
		}
		if c.URL != "https://codeberg.org/owner/repo/issues/1#issuecomment-42" {
			t.Errorf("URL = %q", c.URL)
		}
	})
}

// ---- helpers ----

func assertStructuredError(t *testing.T, err error, wantMsg string) {
	t.Helper()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var se forge.StructuredError
	if !asStructuredError(err, &se) {
		t.Fatalf("error should satisfy forge.StructuredError, got %T: %v", err, err)
	}
	if se.Message() != wantMsg {
		t.Errorf("Message() = %q, want %q", se.Message(), wantMsg)
	}
}

func asStructuredError(err error, target *forge.StructuredError) bool {
	// Check if the error itself implements StructuredError.
	if se, ok := err.(forge.StructuredError); ok {
		*target = se
		return true
	}
	// Check if it wraps a StructuredError via Unwrap().
	type unwrapper interface {
		Unwrap() error
	}
	for {
		u, ok := err.(unwrapper)
		if !ok {
			return false
		}
		err = u.Unwrap()
		if err == nil {
			return false
		}
		if se, ok := err.(forge.StructuredError); ok {
			*target = se
			return true
		}
	}
}
