package commands

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tnikic/anvil/internal/commands/blocking"
	"github.com/tnikic/anvil/internal/forge"
	"github.com/tnikic/anvil/internal/format"
)

// resolveAssignees replaces any "@me" entries in assignees with the
// authenticated user's login via forge.CurrentUser. Non-@me values
// pass through unchanged. Errors from CurrentUser are returned as-is;
// forge adapters already produce structured errors for auth/network failures.
func resolveAssignees(ctx context.Context, f forge.Forge, assignees []string) ([]string, error) {
	hasMe := false
	for _, a := range assignees {
		if a == "@me" {
			hasMe = true
			break
		}
	}
	if !hasMe {
		return assignees, nil
	}

	login, err := f.CurrentUser(ctx)
	if err != nil {
		return nil, err
	}

	resolved := make([]string, len(assignees))
	for i, a := range assignees {
		if a == "@me" {
			resolved[i] = login
		} else {
			resolved[i] = a
		}
	}
	return resolved, nil
}

// autoCreateLabels resolves label references by checking which ones don't exist
// and creating them with a placeholder color. It returns auto-created label
// entries for downstream TOON confirmation output.
func autoCreateLabels(ctx context.Context, labelSvc forge.LabelService, labelRefs []string) ([]format.AutoCreatedLabel, error) {
	existing, err := labelSvc.List(ctx, forge.LabelListOptions{})
	if err != nil {
		return nil, err
	}

	// Build a lookup set for existing labels by scope-qualified name.
	existingSet := make(map[string]bool, len(existing))
	for _, l := range existing {
		existingSet[forge.BuildLabelName(l.Scope, l.Name)] = true
	}

	var autoCreated []format.AutoCreatedLabel
	for _, ref := range labelRefs {
		if existingSet[ref] {
			continue
		}
		scope, name := forge.SplitLabel(ref)

		opts := forge.LabelCreateOptions{
			Name:  name,
			Color: forge.String("333333"),
		}
		if scope != "" {
			opts.Scope = forge.String(scope)
		}

		if _, err := labelSvc.Create(ctx, opts); err != nil {
			return autoCreated, err
		}

		autoCreated = append(autoCreated, format.AutoCreatedLabel{
			Name:  ref,
			Color: "333333",
		})
	}
	return autoCreated, nil
}

func newIssueCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "issue",
		Short: "Manage issues",
		Long: `Manage issues: list, view, create, update, close, reopen, and query relationships.

Subcommands:
  list        List issues in the repository
  view        View a single issue by number
  create      Create a new issue
  update      Update an existing issue
  close       Close an issue (idempotent)
  reopen      Reopen a closed issue (idempotent)
  blocked-by  List issues that block this issue
  blocking    List issues blocked by this issue
  children    List sub-issues of this issue
  parent      Show the parent issue (or "none")
  comment     Manage issue comments (list, view, create, update, delete)
  relation    Manage issue relationships (add, remove)`,
	}
	cmd.AddCommand(
		newIssueListCmd(),
		newIssueViewCmd(),
		newIssueCreateCmd(),
		newIssueUpdateCmd(),
		newIssueCloseCmd(),
		newIssueReopenCmd(),
		newIssueBlockedByCmd(),
		newIssueBlockingCmd(),
		newIssueChildrenCmd(),
		newIssueParentCmd(),
		newIssueCommentCmd(),
		newIssueRelationCmd(),
	)
	return cmd
}

func newIssueListCmd() *cobra.Command {
	var (
		state     string
		labels    []string
		assignees []string
		fields    []string
		limit     int
		sort      string
		direction string
		unblocked bool
		blocked   bool
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List issues",
		Long: `List issues in the current repository.

Output is TOON tabular with number, title, state by default.
Use --fields to add author, labels, and blocked columns.
The count aggregate shows N of M total.`,
		RunE: wrapForge(func(cmd *cobra.Command, args []string, f forge.Forge) error {
			bf := &blocking.Filter{
				Unblocked:   unblocked,
				Blocked:     blocked,
				ShowBlocked: hasField(fields, "blocked"),
			}

			if err := bf.Validate(); err != nil {
				return err
			}

			opts := forge.IssueListOptions{
				State:     state,
				Labels:    labels,
				Sort:      sort,
				Direction: direction,
				Limit:     limit,
			}
			issues, meta, err := f.Issues().List(cmd.Context(), opts)
			if err != nil {
				return err
			}

			// Pre-compute blocker counts when any blocking feature is active.
			var blockedCounts map[int]int
			if bf.NeedsBlocking() {
				blockedCounts, err = bf.ComputeCounts(cmd.Context(), f.Relations(), issues)
				if err != nil {
					return err
				}
			}

			unfilteredCount := len(issues)

			rows := make([]format.IssueRow, 0, len(issues))
			for _, i := range issues {
				// Apply blocking filters.
				if bf.NeedsBlocking() {
					openCount := blockedCounts[i.Number]
					if bf.ShouldSkip(openCount) {
						continue
					}
				}

				row := format.IssueRow{
					Number: i.Number,
					Title:  i.Title,
					State:  i.State,
				}
				if hasField(fields, "author") {
					row.Author = i.Author
				}
				if hasField(fields, "labels") {
					labelNames := make([]string, len(i.Labels))
					for j, l := range i.Labels {
						labelNames[j] = l.Name
					}
					row.Labels = strings.Join(labelNames, ", ")
				}
				if bf.ShowBlocked {
					row.Blocked = bf.BlockedValue(blockedCounts[i.Number])
				}
				rows = append(rows, row)
			}

			count := len(rows)
			total := bf.AdjustTotal(meta, unfilteredCount, count)

			_, _ = fmt.Fprintln(cmd.OutOrStdout(), format.IssueList(rows, count, total))
			return nil
		}),
	}

	cmd.Flags().StringVar(&state, "state", "open", "Filter by state: open, closed, all")
	cmd.Flags().StringSliceVar(&labels, "label", nil, "Filter by label (repeatable)")
	cmd.Flags().StringSliceVar(&assignees, "assignee", nil, "Filter by assignee (repeatable)")
	cmd.Flags().StringSliceVar(&fields, "fields", nil, "Extra fields: author, labels, blocked")
	cmd.Flags().IntVar(&limit, "limit", 0, "Max results (default 30)")
	cmd.Flags().StringVar(&sort, "sort", "", "Sort by: created, updated, comments")
	cmd.Flags().StringVar(&direction, "direction", "", "Sort direction: asc, desc")
	cmd.Flags().BoolVar(&unblocked, "unblocked", false, "Show only issues with no open blockers")
	cmd.Flags().BoolVar(&blocked, "blocked", false, "Show only issues with open blockers")

	setFlagErrorFunc(cmd)
	return cmd
}

func newIssueViewCmd() *cobra.Command {
	var full bool

	cmd := &cobra.Command{
		Use:   "view <number>",
		Short: "View an issue",
		Long: `View a single issue by number.

Output is TOON key-value with full metadata. The body is truncated to 500
characters with total body_size shown. Use --full to see the complete body.`,
		Args: cobra.ExactArgs(1),
		RunE: wrapForge(func(cmd *cobra.Command, args []string, f forge.Forge) error {
			number, err := strconv.Atoi(args[0])
			if err != nil {
				return forge.NewBaseError(
					fmt.Sprintf("invalid issue number: %s", args[0]),
					"Usage: anvil issue view <number>",
				)
			}

			issue, err := f.Issues().Get(cmd.Context(), forge.IssueGetOptions{Number: number})
			if err != nil {
				return err
			}

			hints := forge.EnrichIssueHints(cmd.Context(), f, number)

			detail := &format.IssueDetail{
				Number:        issue.Number,
				Title:         issue.Title,
				State:         issue.State,
				Body:          issue.Body,
				BodySize:      len(issue.Body),
				Author:        issue.Author,
				CreatedAt:     issue.CreatedAt.Format("2006-01-02T15:04:05Z"),
				UpdatedAt:     issue.UpdatedAt.Format("2006-01-02T15:04:05Z"),
				URL:           issue.URL,
				CommentsHint:  hints.CommentsHint,
				BlockedByHint: hints.BlockedByHint,
				BlockingHint:  hints.BlockingHint,
				ChildrenHint:  hints.ChildrenHint,
				ParentHint:    hints.ParentHint,
			}

			_, _ = fmt.Fprintln(cmd.OutOrStdout(), format.IssueView(detail, full))
			return nil
		}),
	}

	cmd.Flags().BoolVar(&full, "full", false, "Show full body without truncation")

	setFlagErrorFunc(cmd)
	return cmd
}

func newIssueCreateCmd() *cobra.Command {
	var (
		title     string
		body      string
		labels    []string
		assignees []string
	)

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create an issue",
		Long: `Create a new issue in the repository.

At minimum, --title is required. On success, prints a TOON confirmation with
the new issue's number, title, and URL.`,
		RunE: wrapForge(func(cmd *cobra.Command, args []string, f forge.Forge) error {
			if strings.TrimSpace(title) == "" {
				return forge.NewBaseError(
					"missing required flag: --title",
					"Usage: anvil issue create --title \"...\" [--body \"...\"] [--label ...] [--assignee ...]",
				)
			}

			resolved, err := resolveAssignees(cmd.Context(), f, assignees)
			if err != nil {
				return err
			}

			// Auto-create any labels that don't exist yet.
			autoCreated, err := autoCreateLabels(cmd.Context(), f.Labels(), labels)
			if err != nil {
				return err
			}

			opts := forge.IssueCreateOptions{
				Title:  &title,
				Labels: labels,
			}
			if body != "" {
				opts.Body = &body
			}
			if len(resolved) > 0 {
				opts.Assignees = resolved
			}

			issue, err := f.Issues().Create(cmd.Context(), opts)
			if err != nil {
				return err
			}

			_, _ = fmt.Fprintln(cmd.OutOrStdout(), format.IssueCreateConfirm(issue.Number, issue.Title, issue.URL, autoCreated))
			return nil
		}),
	}

	cmd.Flags().StringVar(&title, "title", "", "Issue title (required)")
	cmd.Flags().StringVar(&body, "body", "", "Issue body (markdown)")
	cmd.Flags().StringSliceVar(&labels, "label", nil, "Label name (repeatable)")
	cmd.Flags().StringSliceVar(&assignees, "assignee", nil, "Assignee username (repeatable)")

	setFlagErrorFunc(cmd)
	return cmd
}

func newIssueUpdateCmd() *cobra.Command {
	var (
		title        string
		body         string
		labels       []string
		addLabels    []string
		removeLabels []string
		assignees    []string
		state        string
	)

	cmd := &cobra.Command{
		Use:   "update <number>",
		Short: "Update an issue",
		Long: `Update an existing issue by number.

Any combination of --title, --body, --label, --add-label, --remove-label,
--assignee, and --state may be provided. --label replaces all labels;
--add-label and --remove-label modify labels incrementally without affecting
the issue's other labels. --label is mutually exclusive with --add-label
and --remove-label.

On success, prints a TOON confirmation with the updated issue's number,
title, URL, and resulting labels.`,
		Args: cobra.ExactArgs(1),
		RunE: wrapForge(func(cmd *cobra.Command, args []string, f forge.Forge) error {
			number, err := strconv.Atoi(args[0])
			if err != nil {
				return forge.NewBaseError(
					fmt.Sprintf("invalid issue number: %s", args[0]),
					"Usage: anvil issue update <number>",
				)
			}

			// Mutually exclusive: --label vs --add-label/--remove-label.
			if len(labels) > 0 && (len(addLabels) > 0 || len(removeLabels) > 0) {
				return forge.NewBaseError(
					"--label is mutually exclusive with --add-label and --remove-label",
					"Use either --label to replace all labels, or --add-label/--remove-label to modify incrementally",
				)
			}

			resolved, err := resolveAssignees(cmd.Context(), f, assignees)
			if err != nil {
				return err
			}

			// Auto-create any labels that don't exist yet (for --label and --add-label).
			var autoCreated []format.AutoCreatedLabel
			labelsToCheck := append([]string{}, labels...)
			labelsToCheck = append(labelsToCheck, addLabels...)
			if len(labelsToCheck) > 0 {
				autoCreated, err = autoCreateLabels(cmd.Context(), f.Labels(), labelsToCheck)
				if err != nil {
					return err
				}
			}

			opts := forge.IssueUpdateOptions{Number: number}
			if title != "" {
				opts.Title = &title
			}
			if body != "" {
				opts.Body = &body
			}
			if len(labels) > 0 {
				opts.Labels = labels
			}
			if len(addLabels) > 0 {
				opts.AddLabels = addLabels
			}
			if len(removeLabels) > 0 {
				opts.RemoveLabels = removeLabels
			}
			if len(resolved) > 0 {
				opts.Assignees = resolved
			}
			if state != "" {
				opts.State = &state
			}

			issue, err := f.Issues().Update(cmd.Context(), opts)
			if err != nil {
				return err
			}

			// Build label names for confirmation output.
			labelNames := make([]string, len(issue.Labels))
			for i, l := range issue.Labels {
				if l.Scope != "" {
					labelNames[i] = l.Scope + ":" + l.Name
				} else {
					labelNames[i] = l.Name
				}
			}

			_, _ = fmt.Fprintln(cmd.OutOrStdout(), format.IssueUpdateConfirm(issue.Number, issue.Title, issue.URL, labelNames, autoCreated))
			return nil
		}),
	}

	cmd.Flags().StringVar(&title, "title", "", "New title")
	cmd.Flags().StringVar(&body, "body", "", "New body (markdown)")
	cmd.Flags().StringSliceVar(&labels, "label", nil, "Set labels (repeatable, replaces existing)")
	cmd.Flags().StringSliceVar(&addLabels, "add-label", nil, "Add a label without affecting existing labels (repeatable)")
	cmd.Flags().StringSliceVar(&removeLabels, "remove-label", nil, "Remove a label without affecting existing labels (repeatable)")
	cmd.Flags().StringSliceVar(&assignees, "assignee", nil, "Set assignees (repeatable, replaces existing)")
	cmd.Flags().StringVar(&state, "state", "", "Set state: open or closed")

	setFlagErrorFunc(cmd)
	return cmd
}

func newIssueCloseCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "close <number>",
		Short: "Close an issue",
		Long: `Close an issue by number. Idempotent: closing an already-closed issue
prints a no-op message and exits with code 0.`,
		Args: cobra.ExactArgs(1),
		RunE: wrapForge(func(cmd *cobra.Command, args []string, f forge.Forge) error {
			number, err := strconv.Atoi(args[0])
			if err != nil {
				return forge.NewBaseError(
					fmt.Sprintf("invalid issue number: %s", args[0]),
					"Usage: anvil issue close <number>",
				)
			}

			// Check current state first for idempotent no-op detection.
			issue, err := f.Issues().Get(cmd.Context(), forge.IssueGetOptions{Number: number})
			if err != nil {
				return err
			}
			if issue.State == forge.StateClosed {
				return nil
			}

			_, err = f.Issues().Close(cmd.Context(), forge.IssueCloseOptions{Number: number})
			if err != nil {
				return err
			}

			_, _ = fmt.Fprintln(cmd.OutOrStdout(), format.IssueCloseConfirm(number))
			return nil
		}),
	}

	setFlagErrorFunc(cmd)
	return cmd
}

func newIssueReopenCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "reopen <number>",
		Short: "Reopen an issue",
		Long: `Reopen a closed issue by number. Idempotent: reopening an already-open
issue prints a no-op message and exits with code 0.`,
		Args: cobra.ExactArgs(1),
		RunE: wrapForge(func(cmd *cobra.Command, args []string, f forge.Forge) error {
			number, err := strconv.Atoi(args[0])
			if err != nil {
				return forge.NewBaseError(
					fmt.Sprintf("invalid issue number: %s", args[0]),
					"Usage: anvil issue reopen <number>",
				)
			}

			// Check current state first for idempotent no-op detection.
			issue, err := f.Issues().Get(cmd.Context(), forge.IssueGetOptions{Number: number})
			if err != nil {
				return err
			}
			if issue.State == forge.StateOpen {
				return nil
			}

			_, err = f.Issues().Reopen(cmd.Context(), forge.IssueReopenOptions{Number: number})
			if err != nil {
				return err
			}

			_, _ = fmt.Fprintln(cmd.OutOrStdout(), format.IssueCloseConfirm(number))
			return nil
		}),
	}

	setFlagErrorFunc(cmd)
	return cmd
}

func newIssueRelationCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "relation",
		Short: "Manage issue relationships",
		Long: `Manage issue relationships: add or remove blocks and parent/child relationships.

Relationships are edges between issues, not properties of one issue —
use this subcommand instead of 'issue update' for relationship mutations.

Subcommands:
  add     Add a relationship
  remove  Remove a relationship`,
	}
	cmd.AddCommand(
		newIssueRelationAddCmd(),
		newIssueRelationRemoveCmd(),
	)
	return cmd
}

func newIssueRelationAddCmd() *cobra.Command {
	var (
		blocks   int
		parentOf int
	)

	cmd := &cobra.Command{
		Use:   "add <number>",
		Short: "Add a relationship",
		Long: `Add a relationship to an issue.

Exactly one of --blocks or --parent-of must be provided.

  anvil issue relation add 42 --blocks 100
    Makes issue 42 block issue 100.

  anvil issue relation add 42 --parent-of 100
    Makes issue 42 the parent of issue 100.`,
		Args: cobra.ExactArgs(1),
		RunE: wrapForge(func(cmd *cobra.Command, args []string, f forge.Forge) error {
			number, err := strconv.Atoi(args[0])
			if err != nil {
				return forge.NewBaseError(
					fmt.Sprintf("invalid issue number: %s", args[0]),
					"Usage: anvil issue relation add <number> --blocks <other> | --parent-of <other>",
				)
			}

			if blocks == 0 && parentOf == 0 {
				return forge.NewBaseError(
					"exactly one of --blocks or --parent-of is required",
					"Usage: anvil issue relation add <number> --blocks <other> | --parent-of <other>",
				)
			}
			if blocks != 0 && parentOf != 0 {
				return forge.NewBaseError(
					"--blocks and --parent-of are mutually exclusive",
					"Usage: anvil issue relation add <number> --blocks <other> | --parent-of <other>",
				)
			}

			rel := f.Relations()
			if blocks != 0 {
				if err := rel.AddBlocks(cmd.Context(), number, blocks); err != nil {
					return err
				}
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), format.RelationAddConfirm(number, blocks, "blocks"))
			} else {
				if err := rel.AddParentOf(cmd.Context(), number, parentOf); err != nil {
					return err
				}
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), format.RelationAddConfirm(number, parentOf, "parent_of"))
			}
			return nil
		}),
	}

	cmd.Flags().IntVar(&blocks, "blocks", 0, "Mark this issue as blocking another issue")
	cmd.Flags().IntVar(&parentOf, "parent-of", 0, "Set this issue as the parent of another issue")

	setFlagErrorFunc(cmd)
	return cmd
}

func newIssueRelationRemoveCmd() *cobra.Command {
	var (
		blocks   int
		parentOf int
	)

	cmd := &cobra.Command{
		Use:   "remove <number>",
		Short: "Remove a relationship",
		Long: `Remove a relationship from an issue.

Exactly one of --blocks or --parent-of must be provided.

  anvil issue relation remove 42 --blocks 100
    Removes the "blocks" relationship: issue 42 no longer blocks issue 100.

  anvil issue relation remove 42 --parent-of 100
    Removes the parent/child relationship: issue 42 is no longer parent of 100.`,
		Args: cobra.ExactArgs(1),
		RunE: wrapForge(func(cmd *cobra.Command, args []string, f forge.Forge) error {
			number, err := strconv.Atoi(args[0])
			if err != nil {
				return forge.NewBaseError(
					fmt.Sprintf("invalid issue number: %s", args[0]),
					"Usage: anvil issue relation remove <number> --blocks <other> | --parent-of <other>",
				)
			}

			if blocks == 0 && parentOf == 0 {
				return forge.NewBaseError(
					"exactly one of --blocks or --parent-of is required",
					"Usage: anvil issue relation remove <number> --blocks <other> | --parent-of <other>",
				)
			}
			if blocks != 0 && parentOf != 0 {
				return forge.NewBaseError(
					"--blocks and --parent-of are mutually exclusive",
					"Usage: anvil issue relation remove <number> --blocks <other> | --parent-of <other>",
				)
			}

			rel := f.Relations()
			if blocks != 0 {
				if err := rel.RemoveBlocks(cmd.Context(), number, blocks); err != nil {
					return err
				}
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), format.RelationRemoveConfirm(number, blocks, "blocks"))
			} else {
				if err := rel.RemoveParentOf(cmd.Context(), number, parentOf); err != nil {
					return err
				}
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), format.RelationRemoveConfirm(number, parentOf, "parent_of"))
			}
			return nil
		}),
	}

	cmd.Flags().IntVar(&blocks, "blocks", 0, "Remove blocking relationship to another issue")
	cmd.Flags().IntVar(&parentOf, "parent-of", 0, "Remove parent relationship to another issue")

	setFlagErrorFunc(cmd)
	return cmd
}

func newIssueCommentCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "comment",
		Short: "Manage issue comments",
		Long: `Manage issue comments: list, view, create, update, delete.

Subcommands:
  list    List comments on an issue
  view    View a single comment
  create  Create a comment on an issue
  update  Update a comment
  delete  Delete a comment`,
	}
	cmd.AddCommand(
		newIssueCommentListCmd(),
		newIssueCommentViewCmd(),
		newIssueCommentCreateCmd(),
		newIssueCommentUpdateCmd(),
		newIssueCommentDeleteCmd(),
	)
	return cmd
}

func newIssueCommentListCmd() *cobra.Command {
	var includeSystem bool

	cmd := &cobra.Command{
		Use:   "list <issue-number>",
		Short: "List comments on an issue",
		Long: `List comments on an issue in TOON tabular format.

System-generated notes (GitLab status changes, label changes, etc.) are
filtered out by default. Use --include-system to include them.`,
		Args: cobra.ExactArgs(1),
		RunE: wrapForge(func(cmd *cobra.Command, args []string, f forge.Forge) error {
			number, err := strconv.Atoi(args[0])
			if err != nil {
				return forge.NewBaseError(
					fmt.Sprintf("invalid issue number: %s", args[0]),
					"Usage: anvil issue comment list <number>",
				)
			}

			allComments, err := f.Comments().List(cmd.Context(), forge.CommentListOptions{
				IssueNumber:   number,
				IncludeSystem: includeSystem,
			})
			if err != nil {
				return err
			}

			rows := make([]format.CommentRow, 0, len(allComments))
			for _, c := range allComments {
				rows = append(rows, format.CommentRow{
					ID:     c.ID,
					Author: c.Author,
					Body:   c.Body,
					System: c.System,
				})
			}

			// Count all comments (including system notes) for the aggregate.
			totalAvailable := len(allComments)
			if !includeSystem {
				all, err := f.Comments().List(cmd.Context(), forge.CommentListOptions{
					IssueNumber:   number,
					IncludeSystem: true,
				})
				if err == nil {
					totalAvailable = len(all)
				}
			}

			_, _ = fmt.Fprintln(cmd.OutOrStdout(), format.CommentList(rows, len(rows), totalAvailable))
			return nil
		}),
	}

	cmd.Flags().BoolVar(&includeSystem, "include-system", false, "Include system-generated notes")

	setFlagErrorFunc(cmd)
	return cmd
}

func newIssueCommentViewCmd() *cobra.Command {
	var full bool

	cmd := &cobra.Command{
		Use:   "view <issue-number> <comment-id>",
		Short: "View a comment",
		Long: `View a single comment by issue number and comment ID.

Output is TOON key-value. The body is truncated to 500 characters with total
body_size shown. Use --full to see the complete body.`,
		Args: cobra.ExactArgs(2),
		RunE: wrapForge(func(cmd *cobra.Command, args []string, f forge.Forge) error {
			issueNumber, err := strconv.Atoi(args[0])
			if err != nil {
				return forge.NewBaseError(
					fmt.Sprintf("invalid issue number: %s", args[0]),
					"Usage: anvil issue comment view <issue> <comment-id>",
				)
			}
			commentID, err := strconv.Atoi(args[1])
			if err != nil {
				return forge.NewBaseError(
					fmt.Sprintf("invalid comment ID: %s", args[1]),
					"Usage: anvil issue comment view <issue> <comment-id>",
				)
			}

			comment, err := f.Comments().Get(cmd.Context(), forge.CommentGetOptions{
				IssueNumber: issueNumber,
				CommentID:   commentID,
			})
			if err != nil {
				return err
			}

			detail := &format.CommentDetail{
				ID:        comment.ID,
				Body:      comment.Body,
				BodySize:  len(comment.Body),
				Author:    comment.Author,
				System:    comment.System,
				CreatedAt: comment.CreatedAt.Format("2006-01-02T15:04:05Z"),
				UpdatedAt: comment.UpdatedAt.Format("2006-01-02T15:04:05Z"),
				URL:       comment.URL,
			}

			_, _ = fmt.Fprintln(cmd.OutOrStdout(), format.CommentView(detail, full))
			return nil
		}),
	}

	cmd.Flags().BoolVar(&full, "full", false, "Show full body without truncation")

	setFlagErrorFunc(cmd)
	return cmd
}

func newIssueCommentCreateCmd() *cobra.Command {
	var body string

	cmd := &cobra.Command{
		Use:   "create <issue-number>",
		Short: "Create a comment on an issue",
		Long: `Create a new comment on an issue.

--body is required. On success, prints a TOON confirmation with the new
comment's ID, issue number, and URL.`,
		Args: cobra.ExactArgs(1),
		RunE: wrapForge(func(cmd *cobra.Command, args []string, f forge.Forge) error {
			number, err := strconv.Atoi(args[0])
			if err != nil {
				return forge.NewBaseError(
					fmt.Sprintf("invalid issue number: %s", args[0]),
					"Usage: anvil issue comment create <number> --body \"...\"",
				)
			}

			if strings.TrimSpace(body) == "" {
				return forge.NewBaseError(
					"missing required flag: --body",
					"Usage: anvil issue comment create <number> --body \"...\"",
				)
			}

			comment, err := f.Comments().Create(cmd.Context(), forge.CommentCreateOptions{
				IssueNumber: number,
				Body:        body,
			})
			if err != nil {
				return err
			}

			_, _ = fmt.Fprintln(cmd.OutOrStdout(), format.CommentCreateConfirm(number, comment.ID, comment.URL))
			return nil
		}),
	}

	cmd.Flags().StringVar(&body, "body", "", "Comment body (markdown, required)")

	setFlagErrorFunc(cmd)
	return cmd
}

func newIssueCommentUpdateCmd() *cobra.Command {
	var body string

	cmd := &cobra.Command{
		Use:   "update <issue-number> <comment-id>",
		Short: "Update a comment",
		Long: `Update an existing comment's body.

--body is required. On success, prints a TOON confirmation with the updated
comment's ID, issue number, and URL.`,
		Args: cobra.ExactArgs(2),
		RunE: wrapForge(func(cmd *cobra.Command, args []string, f forge.Forge) error {
			issueNumber, err := strconv.Atoi(args[0])
			if err != nil {
				return forge.NewBaseError(
					fmt.Sprintf("invalid issue number: %s", args[0]),
					"Usage: anvil issue comment update <issue> <comment-id> --body \"...\"",
				)
			}
			commentID, err := strconv.Atoi(args[1])
			if err != nil {
				return forge.NewBaseError(
					fmt.Sprintf("invalid comment ID: %s", args[1]),
					"Usage: anvil issue comment update <issue> <comment-id> --body \"...\"",
				)
			}

			if strings.TrimSpace(body) == "" {
				return forge.NewBaseError(
					"missing required flag: --body",
					"Usage: anvil issue comment update <issue> <comment-id> --body \"...\"",
				)
			}

			comment, err := f.Comments().Update(cmd.Context(), forge.CommentUpdateOptions{
				IssueNumber: issueNumber,
				CommentID:   commentID,
				Body:        body,
			})
			if err != nil {
				return err
			}

			_, _ = fmt.Fprintln(cmd.OutOrStdout(), format.CommentUpdateConfirm(issueNumber, comment.ID, comment.URL))
			return nil
		}),
	}

	cmd.Flags().StringVar(&body, "body", "", "New comment body (markdown, required)")

	setFlagErrorFunc(cmd)
	return cmd
}

func newIssueCommentDeleteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <issue-number> <comment-id>",
		Short: "Delete a comment",
		Long: `Delete a comment by issue number and comment ID.

On success, prints a TOON confirmation.`,
		Args: cobra.ExactArgs(2),
		RunE: wrapForge(func(cmd *cobra.Command, args []string, f forge.Forge) error {
			issueNumber, err := strconv.Atoi(args[0])
			if err != nil {
				return forge.NewBaseError(
					fmt.Sprintf("invalid issue number: %s", args[0]),
					"Usage: anvil issue comment delete <issue> <comment-id>",
				)
			}
			commentID, err := strconv.Atoi(args[1])
			if err != nil {
				return forge.NewBaseError(
					fmt.Sprintf("invalid comment ID: %s", args[1]),
					"Usage: anvil issue comment delete <issue> <comment-id>",
				)
			}

			if err := f.Comments().Delete(cmd.Context(), forge.CommentDeleteOptions{
				IssueNumber: issueNumber,
				CommentID:   commentID,
			}); err != nil {
				return err
			}

			_, _ = fmt.Fprintln(cmd.OutOrStdout(), format.CommentDeleteConfirm(issueNumber, commentID))
			return nil
		}),
	}

	setFlagErrorFunc(cmd)
	return cmd
}

func newIssueBlockedByCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "blocked-by <number>",
		Short: "List issues that block this issue",
		Long: `List open issues that the given issue is blocked by.

Output is TOON tabular with number, title, and state.`,
		Args: cobra.ExactArgs(1),
		RunE: wrapForge(func(cmd *cobra.Command, args []string, f forge.Forge) error {
			number, err := strconv.Atoi(args[0])
			if err != nil {
				return forge.NewBaseError(
					fmt.Sprintf("invalid issue number: %s", args[0]),
					"Usage: anvil issue blocked-by <number>",
				)
			}

			deps, err := f.Relations().BlockedBy(cmd.Context(), number)
			if err != nil {
				return err
			}

			rows := make([]format.IssueDependencyRow, 0, len(deps))
			for _, d := range deps {
				if d.State != forge.StateOpen {
					continue
				}
				rows = append(rows, format.IssueDependencyRow{
					Number:    d.Number,
					Title:     d.Title,
					State:     d.State,
					Direction: string(d.Direction),
				})
			}

			_, _ = fmt.Fprintln(cmd.OutOrStdout(), format.RelationList(rows))
			return nil
		}),
	}

	setFlagErrorFunc(cmd)
	return cmd
}

func newIssueBlockingCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "blocking <number>",
		Short: "List issues blocked by this issue",
		Long: `List open issues that are blocked by the given issue.

Output is TOON tabular with number, title, and state.`,
		Args: cobra.ExactArgs(1),
		RunE: wrapForge(func(cmd *cobra.Command, args []string, f forge.Forge) error {
			number, err := strconv.Atoi(args[0])
			if err != nil {
				return forge.NewBaseError(
					fmt.Sprintf("invalid issue number: %s", args[0]),
					"Usage: anvil issue blocking <number>",
				)
			}

			deps, err := f.Relations().Blocking(cmd.Context(), number)
			if err != nil {
				return err
			}

			rows := make([]format.IssueDependencyRow, 0, len(deps))
			for _, d := range deps {
				if d.State != forge.StateOpen {
					continue
				}
				rows = append(rows, format.IssueDependencyRow{
					Number:    d.Number,
					Title:     d.Title,
					State:     d.State,
					Direction: string(d.Direction),
				})
			}

			_, _ = fmt.Fprintln(cmd.OutOrStdout(), format.RelationList(rows))
			return nil
		}),
	}

	setFlagErrorFunc(cmd)
	return cmd
}

func newIssueChildrenCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "children <number>",
		Short: "List sub-issues of this issue",
		Long: `List sub-issues (children) of the given issue.

Output is TOON tabular with number, title, and state.`,
		Args: cobra.ExactArgs(1),
		RunE: wrapForge(func(cmd *cobra.Command, args []string, f forge.Forge) error {
			number, err := strconv.Atoi(args[0])
			if err != nil {
				return forge.NewBaseError(
					fmt.Sprintf("invalid issue number: %s", args[0]),
					"Usage: anvil issue children <number>",
				)
			}

			deps, err := f.Relations().Children(cmd.Context(), number)
			if err != nil {
				return err
			}

			rows := make([]format.IssueDependencyRow, 0, len(deps))
			for _, d := range deps {
				if d.State != forge.StateOpen {
					continue
				}
				rows = append(rows, format.IssueDependencyRow{
					Number:    d.Number,
					Title:     d.Title,
					State:     d.State,
					Direction: string(d.Direction),
				})
			}

			_, _ = fmt.Fprintln(cmd.OutOrStdout(), format.RelationList(rows))
			return nil
		}),
	}

	setFlagErrorFunc(cmd)
	return cmd
}

func newIssueParentCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "parent <number>",
		Short: "Show the parent issue",
		Long: `Show the parent issue of the given issue, or "none" if it has no parent.

Output is TOON key-value for the parent issue.`,
		Args: cobra.ExactArgs(1),
		RunE: wrapForge(func(cmd *cobra.Command, args []string, f forge.Forge) error {
			number, err := strconv.Atoi(args[0])
			if err != nil {
				return forge.NewBaseError(
					fmt.Sprintf("invalid issue number: %s", args[0]),
					"Usage: anvil issue parent <number>",
				)
			}

			dep, err := f.Relations().Parent(cmd.Context(), number)
			if err != nil {
				return err
			}

			var row *format.IssueDependencyRow
			if dep != nil {
				row = &format.IssueDependencyRow{
					Number:    dep.Number,
					Title:     dep.Title,
					State:     dep.State,
					Direction: string(dep.Direction),
				}
			}

			_, _ = fmt.Fprintln(cmd.OutOrStdout(), format.ParentIssue(row))
			return nil
		}),
	}

	setFlagErrorFunc(cmd)
	return cmd
}
