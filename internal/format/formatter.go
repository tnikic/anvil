// Package format provides TOON output formatting for the anvil CLI.
// It transforms domain types into TOON strings at the CLI output boundary,
// keeping TOON conversion separate from the provider layer.
package format

import (
	"fmt"

	"github.com/toon-format/toon-go"
)

// ---- Output types with toon tags ----

// IssueRow is a single row in the issue list tabular output.
// Author, Labels, and Blocked are populated by the caller and suppressed via omitempty
// when --fields does not request them (TOON omits empty optional fields).
type IssueRow struct {
	Number  int    `toon:"number"`
	Title   string `toon:"title"`
	State   string `toon:"state"`
	Author  string `toon:"author,omitempty"`
	Labels  string `toon:"labels,omitempty"`
	Blocked string `toon:"blocked,omitempty"`
}

type issueListOutput struct {
	Issues any    `toon:"issues"`
	Count  string `toon:"count"`
}

// IssueDetail is the full issue view output.
type IssueDetail struct {
	Number    int    `toon:"number"`
	Title     string `toon:"title"`
	State     string `toon:"state"`
	Body      string `toon:"body"`
	BodySize  int    `toon:"body_size"`
	Author    string `toon:"author"`
	CreatedAt string `toon:"created_at"`
	UpdatedAt string `toon:"updated_at"`
	URL       string `toon:"url"`
	Hint      string `toon:"hint,omitempty"`

	// Relationship hints — only set when there are relationships to report.
	BlockedByHint string `toon:"blocked_by,omitempty"`
	BlockingHint  string `toon:"blocking,omitempty"`
	ChildrenHint  string `toon:"children,omitempty"`
	ParentHint    string `toon:"parent,omitempty"`

	// Comment hint — set when there are comments on the issue.
	CommentsHint string `toon:"comments,omitempty"`
}

// LabelRow is a single row in the label list tabular output.
// Exclusive is populated by the caller and suppressed via omitempty
// when --fields does not request it.
type LabelRow struct {
	Name        string `toon:"name"`
	Scope       string `toon:"scope"`
	Color       string `toon:"color"`
	Description string `toon:"description"`
	Exclusive   bool   `toon:"exclusive,omitempty"`
}

type labelListOutput struct {
	Labels any    `toon:"labels"`
	Count  string `toon:"count"`
}

// PRRow is a single row in the PR list tabular output.
// Author and Created are populated by the caller and suppressed via omitempty
// when --fields does not request them.
type PRRow struct {
	Number  int    `toon:"number"`
	Stack   string `toon:"stack"`
	Title   string `toon:"title"`
	State   string `toon:"state"`
	Author  string `toon:"author,omitempty"`
	Created string `toon:"created,omitempty"`
}

type prListOutput struct {
	PRs   any    `toon:"prs"`
	Count string `toon:"count"`
}

// DashboardIssueRow is a row in the dashboard issue output.
// Schema: {number, title, state, author}.
type DashboardIssueRow struct {
	Number int    `toon:"number"`
	Title  string `toon:"title"`
	State  string `toon:"state"`
	Author string `toon:"author"`
}

// DashboardPRRow is a row in the dashboard PR output.
// Schema: {number, title, author}.
type DashboardPRRow struct {
	Number int    `toon:"number"`
	Title  string `toon:"title"`
	Author string `toon:"author"`
}

type dashboardIssueOutput struct {
	Issues any    `toon:"issues"`
	Count  string `toon:"count"`
}

type dashboardPROutput struct {
	PRs   any    `toon:"prs"`
	Count string `toon:"count"`
}

// DepPR is a dependency row in the PR view.
type DepPR struct {
	Number int    `toon:"number"`
	Title  string `toon:"title"`
	State  string `toon:"state"`
}

// ReviewerState is a reviewer row in the PR view.
type ReviewerState struct {
	Login string `toon:"login"`
	State string `toon:"state"`
}

// PRDetail is the full PR view output.
type PRDetail struct {
	Number       int             `toon:"number"`
	Title        string          `toon:"title"`
	State        string          `toon:"state"`
	Body         string          `toon:"body"`
	BodySize     int             `toon:"body_size"`
	BaseRef      string          `toon:"base_ref"`
	HeadRef      string          `toon:"head_ref"`
	Stack        string          `toon:"stack,omitempty"`
	Draft        bool            `toon:"draft,omitempty"`
	DependsOn    []DepPR         `toon:"depends_on"`
	DependedOnBy []DepPR         `toon:"depended_on_by"`
	Reviewers    []ReviewerState `toon:"reviewers,omitempty"`
	ChecksPassed int             `toon:"checks_passed,omitempty"`
	ChecksTotal  int             `toon:"checks_total,omitempty"`
	Author       string          `toon:"author"`
	CreatedAt    string          `toon:"created_at"`
	UpdatedAt    string          `toon:"updated_at"`
	URL          string          `toon:"url"`
	Hint         string          `toon:"hint,omitempty"`
}

// AuthRow is a single row in the auth status output.
type AuthRow struct {
	Forge  string `toon:"forge"`
	Host   string `toon:"host"`
	Source string `toon:"source"`
}

type authStatusOutput struct {
	Hosts []AuthRow `toon:"hosts"`
}

// ---- Formatter functions ----

// IssueList formats a list of issues as TOON tabular output.
// count is the number of items in this response; total is the total matching items.
func IssueList(issues []IssueRow, count, total int) string {
	out := issueListOutput{
		Issues: issues,
		Count:  fmt.Sprintf("%d of %d total", count, total),
	}
	s, err := toon.MarshalString(out)
	if err != nil {
		return fallbackError("issue list", err)
	}
	return s
}

// IssueView formats a single issue as TOON key-value output.
// If full is false, the body is truncated to 500 characters.
func IssueView(issue *IssueDetail, full bool) string {
	v := *issue // copy
	if !full && len(v.Body) > 500 {
		v.Body, _ = TruncateBody(v.Body, 500)
		v.Hint = "Use --full to see the complete body"
	}
	s, err := toon.MarshalString(v)
	if err != nil {
		return fallbackError("issue view", err)
	}
	return s
}

// LabelList formats a list of labels as TOON tabular output.
func LabelList(labels []LabelRow) string {
	out := labelListOutput{
		Labels: labels,
		Count:  fmt.Sprintf("%d labels", len(labels)),
	}
	s, err := toon.MarshalString(out)
	if err != nil {
		return fallbackError("label list", err)
	}
	return s
}

// PRList formats a list of pull requests as TOON tabular output.
func PRList(prs []PRRow, count, total int) string {
	out := prListOutput{
		PRs:   prs,
		Count: fmt.Sprintf("%d of %d total", count, total),
	}
	s, err := toon.MarshalString(out)
	if err != nil {
		return fallbackError("PR list", err)
	}
	return s
}

// DashboardIssueList formats dashboard issues with the schema {number, title, state, author}.
func DashboardIssueList(issues []DashboardIssueRow, count, total int) string {
	out := dashboardIssueOutput{
		Issues: issues,
		Count:  fmt.Sprintf("%d of %d total", count, total),
	}
	s, err := toon.MarshalString(out)
	if err != nil {
		return fallbackError("dashboard issues", err)
	}
	return s
}

// DashboardPRList formats dashboard PRs with the schema {number, title, author}.
func DashboardPRList(prs []DashboardPRRow, count, total int) string {
	out := dashboardPROutput{
		PRs:   prs,
		Count: fmt.Sprintf("%d of %d total", count, total),
	}
	s, err := toon.MarshalString(out)
	if err != nil {
		return fallbackError("dashboard PRs", err)
	}
	return s
}

// PRView formats a single pull request as TOON key-value output.
func PRView(pr *PRDetail, full bool) string {
	v := *pr // copy
	if !full && len(v.Body) > 500 {
		v.Body, _ = TruncateBody(v.Body, 500)
		v.Hint = "Use --full to see the complete body"
	}
	// Ensure DependsOn/DependedOnBy are never nil — marshal as empty arrays
	if v.DependsOn == nil {
		v.DependsOn = []DepPR{}
	}
	if v.DependedOnBy == nil {
		v.DependedOnBy = []DepPR{}
	}
	s, err := toon.MarshalString(v)
	if err != nil {
		return fallbackError("PR view", err)
	}
	return s
}

// AuthStatus formats the auth status as TOON tabular output.
func AuthStatus(hosts []AuthRow) string {
	if len(hosts) == 0 {
		return "No credentials configured.\nUse `anvil auth set <host> <token>` to add a token."
	}
	out := authStatusOutput{
		Hosts: hosts,
	}
	s, err := toon.MarshalString(out)
	if err != nil {
		return fallbackError("auth status", err)
	}
	return s
}

type errorOutput struct {
	Error string `toon:"error"`
	Help  string `toon:"help,omitempty"`
}

// Error formats an error with an optional help hint as TOON output.
func Error(msg, help string) string {
	out := errorOutput{Error: msg, Help: help}
	s, err := toon.MarshalString(out)
	if err != nil {
		return fmt.Sprintf("error: %s", msg)
	}
	return s
}

// AutoCreatedLabel represents a label that was auto-created during issue creation/update.
type AutoCreatedLabel struct {
	Name  string `toon:"name"`
	Color string `toon:"color"`
}

type issueConfirmOutput struct {
	Created           int                `toon:"created"`
	Title             string             `toon:"title"`
	URL               string             `toon:"url"`
	AutoCreatedLabels []AutoCreatedLabel `toon:"auto_created_labels,omitempty"`
}

// IssueCreateConfirm formats an issue creation confirmation.
func IssueCreateConfirm(number int, title, url string, autoCreatedLabels []AutoCreatedLabel) string {
	out := issueConfirmOutput{
		Created:           number,
		Title:             title,
		URL:               url,
		AutoCreatedLabels: autoCreatedLabels,
	}
	s, err := toon.MarshalString(out)
	if err != nil {
		return fmt.Sprintf("created: %d\ntitle: %s", number, title)
	}
	return s
}

type issueUpdateOutput struct {
	Updated           int                `toon:"updated"`
	Title             string             `toon:"title"`
	URL               string             `toon:"url"`
	Labels            []string           `toon:"labels,omitempty"`
	AutoCreatedLabels []AutoCreatedLabel `toon:"auto_created_labels,omitempty"`
}

// IssueUpdateConfirm formats an issue update confirmation with resulting labels.
func IssueUpdateConfirm(number int, title, url string, labels []string, autoCreatedLabels []AutoCreatedLabel) string {
	out := issueUpdateOutput{
		Updated:           number,
		Title:             title,
		URL:               url,
		AutoCreatedLabels: autoCreatedLabels,
	}
	if len(labels) > 0 {
		out.Labels = labels
	}
	s, err := toon.MarshalString(out)
	if err != nil {
		return fmt.Sprintf("updated: %d\ntitle: %s", number, title)
	}
	return s
}

type issueClosedOutput struct {
	Closed int `toon:"closed"`
}

// IssueCloseConfirm formats an issue close confirmation.
func IssueCloseConfirm(number int) string {
	out := issueClosedOutput{Closed: number}
	s, err := toon.MarshalString(out)
	if err != nil {
		return fmt.Sprintf("closed: %d", number)
	}
	return s
}

type prConfirmOutput struct {
	Created int    `toon:"created"`
	Title   string `toon:"title"`
	URL     string `toon:"url"`
}

// PRCreateConfirm formats a PR creation confirmation.
func PRCreateConfirm(number int, title, url string) string {
	out := prConfirmOutput{Created: number, Title: title, URL: url}
	s, err := toon.MarshalString(out)
	if err != nil {
		return fmt.Sprintf("created: %d\ntitle: %s", number, title)
	}
	return s
}

type prMergedOutput struct {
	Merged int `toon:"merged"`
}

// PRMergeConfirm formats a PR merge confirmation.
func PRMergeConfirm(number int) string {
	out := prMergedOutput{Merged: number}
	s, err := toon.MarshalString(out)
	if err != nil {
		return fmt.Sprintf("merged: %d", number)
	}
	return s
}

type diagnosticOutput struct {
	Diagnostic string `toon:"diagnostic"`
}

// Diagnostic formats a diagnostic message (e.g., broken stack warning).
func Diagnostic(msg string) string {
	out := diagnosticOutput{Diagnostic: msg}
	s, err := toon.MarshalString(out)
	if err != nil {
		return fmt.Sprintf("diagnostic: %s", msg)
	}
	return s
}

type labelConfirmOutput struct {
	Created string `toon:"created"`
	Scope   string `toon:"scope,omitempty"`
}

// LabelCreateConfirm formats a label creation confirmation.
func LabelCreateConfirm(name, scope string) string {
	out := labelConfirmOutput{Created: name, Scope: scope}
	s, err := toon.MarshalString(out)
	if err != nil {
		return fmt.Sprintf("created: %s", name)
	}
	return s
}

type labelUpdatedOutput struct {
	Updated string `toon:"updated"`
	Scope   string `toon:"scope,omitempty"`
}

// LabelUpdateConfirm formats a label update confirmation.
func LabelUpdateConfirm(name, scope string) string {
	out := labelUpdatedOutput{Updated: name, Scope: scope}
	s, err := toon.MarshalString(out)
	if err != nil {
		return fmt.Sprintf("updated: %s", name)
	}
	return s
}

type labelDeletedOutput struct {
	Deleted string `toon:"deleted"`
	Scope   string `toon:"scope,omitempty"`
}

// LabelDeleteConfirm formats a label deletion confirmation.
func LabelDeleteConfirm(name, scope string) string {
	out := labelDeletedOutput{Deleted: name, Scope: scope}
	s, err := toon.MarshalString(out)
	if err != nil {
		return fmt.Sprintf("deleted: %s", name)
	}
	return s
}

// IssueDependencyRow is a single row in a dependency list tabular output.
type IssueDependencyRow struct {
	Number    int    `toon:"number"`
	Title     string `toon:"title"`
	State     string `toon:"state"`
	Direction string `toon:"direction"`
}

type relationListOutput struct {
	Issues any    `toon:"issues"`
	Count  string `toon:"count"`
}

// RelationList formats a list of issue dependencies as TOON tabular output.
func RelationList(deps []IssueDependencyRow) string {
	out := relationListOutput{
		Issues: deps,
		Count:  fmt.Sprintf("%d issue(s)", len(deps)),
	}
	s, err := toon.MarshalString(out)
	if err != nil {
		return fallbackError("relation list", err)
	}
	return s
}

type parentOutput struct {
	Issue *IssueDependencyRow `toon:"issue,omitempty"`
}

// ParentIssue formats a single parent issue as TOON key-value output.
// If dep is nil, outputs "none".
func ParentIssue(dep *IssueDependencyRow) string {
	if dep == nil {
		return "none\n"
	}
	out := parentOutput{Issue: dep}
	s, err := toon.MarshalString(out)
	if err != nil {
		return fallbackError("parent issue", err)
	}
	return s
}

// ---- Comment output types ----

// CommentRow is a single row in the comment list tabular output.
type CommentRow struct {
	ID     int    `toon:"id"`
	Author string `toon:"author"`
	Body   string `toon:"body"`
	System bool   `toon:"system,omitempty"`
}

type commentListOutput struct {
	Comments any    `toon:"comments"`
	Count    string `toon:"count"`
}

// CommentDetail is the full comment view output.
type CommentDetail struct {
	ID        int    `toon:"id"`
	Body      string `toon:"body"`
	BodySize  int    `toon:"body_size"`
	Author    string `toon:"author"`
	System    bool   `toon:"system,omitempty"`
	CreatedAt string `toon:"created_at"`
	UpdatedAt string `toon:"updated_at"`
	URL       string `toon:"url"`
	Hint      string `toon:"hint,omitempty"`
}

type commentConfirmOutput struct {
	Created int    `toon:"created,omitempty"`
	Updated int    `toon:"updated,omitempty"`
	Deleted int    `toon:"deleted,omitempty"`
	Issue   int    `toon:"issue"`
	URL     string `toon:"url,omitempty"`
}

// CommentList formats a list of comments as TOON tabular output.
func CommentList(comments []CommentRow, totalIncluded, totalAvailable int) string {
	// Truncate body to 80 chars for tabular display.
	for i := range comments {
		if len(comments[i].Body) > 80 {
			comments[i].Body = comments[i].Body[:77] + "..."
		}
	}
	out := commentListOutput{
		Comments: comments,
		Count:    fmt.Sprintf("%d of %d comments", totalIncluded, totalAvailable),
	}
	s, err := toon.MarshalString(out)
	if err != nil {
		return fallbackError("comment list", err)
	}
	return s
}

// CommentView formats a single comment as TOON key-value output.
func CommentView(comment *CommentDetail, full bool) string {
	v := *comment
	if !full && len(v.Body) > 500 {
		v.Body, _ = TruncateBody(v.Body, 500)
		v.Hint = "Use --full to see the complete body"
	}
	s, err := toon.MarshalString(v)
	if err != nil {
		return fallbackError("comment view", err)
	}
	return s
}

// CommentCreateConfirm formats a comment creation confirmation.
func CommentCreateConfirm(issueNumber int, commentID int, url string) string {
	out := commentConfirmOutput{
		Created: commentID,
		Issue:   issueNumber,
		URL:     url,
	}
	s, err := toon.MarshalString(out)
	if err != nil {
		return fmt.Sprintf("created: %d\nissue: %d", commentID, issueNumber)
	}
	return s
}

// CommentUpdateConfirm formats a comment update confirmation.
func CommentUpdateConfirm(issueNumber int, commentID int, url string) string {
	out := commentConfirmOutput{
		Updated: commentID,
		Issue:   issueNumber,
		URL:     url,
	}
	s, err := toon.MarshalString(out)
	if err != nil {
		return fmt.Sprintf("updated: %d\nissue: %d", commentID, issueNumber)
	}
	return s
}

// ---- Relation confirmation output types ----

type relationConfirmOutput struct {
	Added   *relationDetail `toon:"added,omitempty"`
	Removed *relationDetail `toon:"removed,omitempty"`
}

type relationDetail struct {
	Source int    `toon:"source"`
	Target int    `toon:"target"`
	Type   string `toon:"type"`
}

// RelationAddConfirm formats a relationship addition confirmation.
func RelationAddConfirm(source, target int, relType string) string {
	out := relationConfirmOutput{
		Added: &relationDetail{
			Source: source,
			Target: target,
			Type:   relType,
		},
	}
	s, err := toon.MarshalString(out)
	if err != nil {
		return fmt.Sprintf("added: #%d → #%d (%s)", source, target, relType)
	}
	return s
}

// RelationRemoveConfirm formats a relationship removal confirmation.
func RelationRemoveConfirm(source, target int, relType string) string {
	out := relationConfirmOutput{
		Removed: &relationDetail{
			Source: source,
			Target: target,
			Type:   relType,
		},
	}
	s, err := toon.MarshalString(out)
	if err != nil {
		return fmt.Sprintf("removed: #%d → #%d (%s)", source, target, relType)
	}
	return s
}

// CommentDeleteConfirm formats a comment deletion confirmation.
func CommentDeleteConfirm(issueNumber int, commentID int) string {
	out := commentConfirmOutput{
		Deleted: commentID,
		Issue:   issueNumber,
	}
	s, err := toon.MarshalString(out)
	if err != nil {
		return fmt.Sprintf("deleted: comment %d\nissue: %d", commentID, issueNumber)
	}
	return s
}

// TruncateBody truncates body to maxLen characters, appending "..." if truncated.
// Returns the truncated string and the original total size.
// A maxLen of 0 or less means no truncation.
func TruncateBody(body string, maxLen int) (string, int) {
	total := len(body)
	if maxLen <= 0 || total <= maxLen {
		return body, total
	}
	// Reserve 3 chars for "..."
	if maxLen <= 3 {
		return body[:maxLen], total
	}
	return body[:maxLen-3] + "...", total
}

// fallbackError returns a plain-text error when TOON marshaling fails.
func fallbackError(context string, err error) string {
	return fmt.Sprintf("error: failed to format %s: %v", context, err)
}
