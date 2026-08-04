package commands

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"
	"github.com/tnikic/anvil/internal/forge"
	"github.com/tnikic/anvil/internal/format"
	"github.com/tnikic/anvil/internal/stack"
)

func newPRCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pr",
		Short: "Manage pull requests",
		Long: `Manage pull requests: list, view, create, merge.

Subcommands:
  list      List pull requests in the repository
  view      View a single pull request by number
  create    Create a new pull request
  merge     Merge a pull request`,
	}
	cmd.AddCommand(
		newPRListCmd(),
		newPRViewCmd(),
		newPRCreateCmd(),
		newPRMergeCmd(),
	)
	return cmd
}

func newPRListCmd() *cobra.Command {
	var (
		state     string
		fields    []string
		limit     int
		sort      string
		direction string
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List pull requests",
		Long: `List pull requests in the current repository.

Output is TOON tabular with number, stack, title, state by default.
PRs are grouped and ordered by stack membership.
Use --fields to add author and created columns.
The count aggregate shows N of M total.`,
		RunE: wrapForge(func(cmd *cobra.Command, args []string, f forge.Forge) error {
			opts := forge.PRListOptions{
				State:     state,
				Sort:      sort,
				Direction: direction,
				Limit:     limit,
			}
			prs, meta, err := f.PRs().List(cmd.Context(), opts)
			if err != nil {
				return err
			}

			// Populate stack info and sort
			stack.Populate(prs)
			stack.Sort(prs)

			rows := make([]format.PRRow, 0, len(prs))
			for _, p := range prs {
				row := format.PRRow{
					Number: p.Number,
					Stack:  p.Stack,
					Title:  p.Title,
					State:  p.State,
				}
				if hasField(fields, "author") {
					row.Author = p.Author
				}
				if hasField(fields, "created") {
					row.Created = p.CreatedAt.Format("2006-01-02T15:04:05Z")
				}
				rows = append(rows, row)
			}

			count := len(rows)
			total := count
			if meta != nil {
				total = meta.Total
			}

			_, _ = fmt.Fprintln(cmd.OutOrStdout(), format.PRList(rows, count, total))

			// Diagnose broken stacks (only when listing open PRs by default)
			if state == "open" || state == "" {
				allPRs, _, err := f.PRs().List(cmd.Context(), forge.PRListOptions{State: forge.StateAll})
				if err == nil {
					diags := stack.DiagnoseBroken(prs, allPRs)
					for _, d := range diags {
						_, _ = fmt.Fprintln(cmd.OutOrStdout(), format.Diagnostic(d))
					}
				}
			}

			return nil
		}),
	}

	cmd.Flags().StringVar(&state, "state", "open", "Filter by state: open, closed, merged, all")
	cmd.Flags().StringSliceVar(&fields, "fields", nil, "Extra fields: author, created")
	cmd.Flags().IntVar(&limit, "limit", 0, "Max results (default 30)")
	cmd.Flags().StringVar(&sort, "sort", "", "Sort by: created, updated")
	cmd.Flags().StringVar(&direction, "direction", "", "Sort direction: asc, desc")

	setFlagErrorFunc(cmd)
	return cmd
}

func newPRViewCmd() *cobra.Command {
	var full bool

	cmd := &cobra.Command{
		Use:   "view <number>",
		Short: "View a pull request",
		Long: `View a single pull request by number.

Output is TOON key-value with full metadata including dependency chains.
depends_on shows the chain below this PR (derived from base.ref walking);
depended_on_by shows the chain above. The body is truncated to 500 characters
with total body_size shown. Use --full to see the complete body.`,
		Args: cobra.ExactArgs(1),
		RunE: wrapForge(func(cmd *cobra.Command, args []string, f forge.Forge) error {
			number, err := strconv.Atoi(args[0])
			if err != nil {
				return forge.NewBaseError(
					fmt.Sprintf("invalid PR number: %s", args[0]),
					"Usage: anvil pr view <number>",
				)
			}

			pr, err := f.PRs().Get(cmd.Context(), forge.PRGetOptions{Number: number})
			if err != nil {
				return err
			}

			// List all PRs to reconstruct dependency chains
			allPRs, _, listErr := f.PRs().List(cmd.Context(), forge.PRListOptions{State: forge.StateAll})
			if listErr != nil {
				// If we can't list all PRs, still show what we have but without deps
				allPRs = nil
			}

			// Populate stack and dependencies
			if allPRs != nil {
				stack.Populate(allPRs)
				stack.ComputeDepends(allPRs)

				// Find our PR in the list to pick up the computed deps
				for _, p := range allPRs {
					if p.Number == pr.Number {
						pr.Stack = p.Stack
						pr.DependsOn = p.DependsOn
						pr.DependedOnBy = p.DependedOnBy
						break
					}
				}

				// Build DepPR slices for depends_on and depended_on_by
				dependsOn := buildDepPRs(pr.DependsOn, allPRs)
				dependedOnBy := buildDepPRs(pr.DependedOnBy, allPRs)

				detail := buildPRDetail(pr, dependsOn, dependedOnBy)
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), format.PRView(detail, full))
			} else {
				// No dependency info available, show basic view
				detail := buildPRDetail(pr, nil, nil)
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), format.PRView(detail, full))
			}

			return nil
		}),
	}

	cmd.Flags().BoolVar(&full, "full", false, "Show full body without truncation")

	setFlagErrorFunc(cmd)
	return cmd
}

// buildDepPRs builds DepPR slices from PR numbers and the allPRs list.
func buildDepPRs(numbers []int, allPRs []forge.PR) []format.DepPR {
	byNumber := make(map[int]forge.PR, len(allPRs))
	for _, p := range allPRs {
		byNumber[p.Number] = p
	}

	result := make([]format.DepPR, 0, len(numbers))
	for _, n := range numbers {
		if p, ok := byNumber[n]; ok {
			result = append(result, format.DepPR{
				Number: p.Number,
				Title:  stack.CleanTitle(p.Title),
				State:  p.State,
			})
		} else {
			result = append(result, format.DepPR{
				Number: n,
				Title:  "(unknown)",
				State:  "unknown",
			})
		}
	}
	return result
}

// buildPRDetail converts a forge.PR to a format.PRDetail with all fields populated.
func buildPRDetail(pr *forge.PR, dependsOn, dependedOnBy []format.DepPR) *format.PRDetail {
	detail := &format.PRDetail{
		Number:       pr.Number,
		Title:        pr.Title,
		State:        pr.State,
		Body:         pr.Body,
		BodySize:     len(pr.Body),
		BaseRef:      pr.BaseRef,
		HeadRef:      pr.HeadRef,
		Stack:        pr.Stack,
		DependsOn:    dependsOn,
		DependedOnBy: dependedOnBy,
		Author:       pr.Author,
		CreatedAt:    pr.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:    pr.UpdatedAt.Format("2006-01-02T15:04:05Z"),
		URL:          pr.URL,
	}

	if draft, ok := pr.Extras["draft"]; ok {
		if b, ok := draft.(bool); ok && b {
			detail.Draft = true
		}
	}

	for _, r := range pr.Reviewers {
		detail.Reviewers = append(detail.Reviewers, format.ReviewerState{
			Login: r.Login,
			State: r.State,
		})
	}

	if pr.Checks != nil {
		detail.ChecksPassed = pr.Checks.Passed
		detail.ChecksTotal = pr.Checks.Total
	}

	return detail
}

func newPRCreateCmd() *cobra.Command {
	var (
		title string
		body  string
		head  string
		base  string
		stk   string
		draft bool
	)

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a pull request",
		Long: `Create a new pull request with optional stack tracking.

Use --stack to explicitly name the stack. When omitted, the stack name is
auto-derived from the head branch: if the branch contains a '/', the part
after the last '/' becomes the stack name (e.g., "feat/auth" → "auth").
Branch names without '/' create unstacked PRs.

When a stack is detected or specified, the PR title is prefixed with
[stackname:N/M] and all other open PRs in that stack are renumbered.`,
		RunE: wrapForge(func(cmd *cobra.Command, args []string, f forge.Forge) error {
			if title == "" {
				return forge.NewBaseError(
					"missing required flag: --title",
					"Usage: anvil pr create --title \"My PR\" --head my-branch",
				)
			}

			// Determine stack name
			stackName := ""
			if cmd.Flags().Changed("stack") {
				stackName = stk // may be empty string (explicitly no stack)
			} else if head != "" {
				stackName = stack.DeriveName(head)
			}

			finalTitle := title
			if stackName != "" {
				// List all open PRs to find existing stack members
				allPRs, _, listErr := f.PRs().List(cmd.Context(), forge.PRListOptions{State: forge.StateOpen})
				if listErr != nil {
					return forge.NewBaseError(
						fmt.Sprintf("listing open PRs: %v", listErr),
						"Cannot determine stack ordering; try again",
					)
				}
				stack.Populate(allPRs)

				existing := stack.CollectOpen(stackName, allPRs)
				total := len(existing) + 1
				pos := total // new PR goes at the end
				finalTitle = stack.FormatPrefix(stack.Prefix{Name: stackName, Pos: pos, Total: total}) + " " + title

				opts := forge.PRCreateOptions{
					Title: &finalTitle,
					Body:  &body,
					Draft: &draft,
					Stack: &stackName,
				}
				if head != "" {
					opts.HeadRef = &head
				}
				if base != "" {
					opts.BaseRef = &base
				}

				created, err := f.PRs().Create(cmd.Context(), opts)
				if err != nil {
					return err
				}

				// Renumber all open stack PRs (including the new one)
				updatedExisting := stack.CollectOpen(stackName, append(allPRs, *created))
				if len(updatedExisting) > 1 {
					tracker := stack.NewTracker(f.PRs())
					if err := tracker.Renumber(cmd.Context(), updatedExisting, stackName); err != nil {
						PrintFormatted(cmd.OutOrStdout(), forge.NewBaseError(
							fmt.Sprintf("renumbering stack PRs: %v", err),
							"The PR was created but stack renumbering failed. Use pr list to see current state.",
						))
					}
				}

				_, _ = fmt.Fprintln(cmd.OutOrStdout(), format.PRCreateConfirm(created.Number, created.Title, created.URL))
			} else {
				// No stack — plain PR
				opts := forge.PRCreateOptions{
					Title: &finalTitle,
					Body:  &body,
					Draft: &draft,
				}
				if head != "" {
					opts.HeadRef = &head
				}
				if base != "" {
					opts.BaseRef = &base
				}

				created, err := f.PRs().Create(cmd.Context(), opts)
				if err != nil {
					return err
				}

				_, _ = fmt.Fprintln(cmd.OutOrStdout(), format.PRCreateConfirm(created.Number, created.Title, created.URL))
			}

			return nil
		}),
	}

	cmd.Flags().StringVar(&title, "title", "", "PR title (required)")
	cmd.Flags().StringVar(&body, "body", "", "PR body/description")
	cmd.Flags().StringVar(&head, "head", "", "Source branch (default: current branch)")
	cmd.Flags().StringVar(&base, "base", "", "Target branch (default: repository default)")
	cmd.Flags().StringVar(&stk, "stack", "", "Stack name; auto-derived from branch if omitted")
	cmd.Flags().BoolVar(&draft, "draft", false, "Create as draft PR")

	setFlagErrorFunc(cmd)
	return cmd
}

func newPRMergeCmd() *cobra.Command {
	var (
		method string
		mTitle string
		mBody  string
	)

	cmd := &cobra.Command{
		Use:   "merge <number>",
		Short: "Merge a pull request",
		Long: `Merge a pull request by number.

If the PR is part of a stack, the remaining open stack PRs are renumbered
after the merge. Merged PRs retain their stack title prefix permanently.`,
		Args: cobra.ExactArgs(1),
		RunE: wrapForge(func(cmd *cobra.Command, args []string, f forge.Forge) error {
			number, err := strconv.Atoi(args[0])
			if err != nil {
				return forge.NewBaseError(
					fmt.Sprintf("invalid PR number: %s", args[0]),
					"Usage: anvil pr merge <number>",
				)
			}

			// Get the PR first to check for stack membership
			pr, err := f.PRs().Get(cmd.Context(), forge.PRGetOptions{Number: number})
			if err != nil {
				return err
			}

			sp, hasStack := stack.ParsePrefix(pr.Title)

			// Merge the PR (title prefix is preserved by the forge)
			mergeOpts := forge.PRMergeOptions{Number: number}
			if method != "" {
				mergeOpts.Method = &method
			}
			if mTitle != "" {
				mergeOpts.Title = &mTitle
			}
			if mBody != "" {
				mergeOpts.Body = &mBody
			}

			merged, err := f.PRs().Merge(cmd.Context(), mergeOpts)
			if err != nil {
				return err
			}

			_, _ = fmt.Fprintln(cmd.OutOrStdout(), format.PRMergeConfirm(merged.Number))

			// If the merged PR was part of a stack, renumber remaining open PRs
			if hasStack {
				allPRs, _, listErr := f.PRs().List(cmd.Context(), forge.PRListOptions{State: forge.StateOpen})
				if listErr != nil {
					PrintFormatted(cmd.OutOrStdout(), forge.NewBaseError(
						fmt.Sprintf("stack renumbering failed: %v", listErr),
						"The PR was merged but stack renumbering could not complete. Use pr list to see current state.",
					))
					return nil
				}

				remaining := stack.CollectOpen(sp.Name, allPRs)
				if len(remaining) > 0 {
					tracker := stack.NewTracker(f.PRs())
					if err := tracker.Renumber(cmd.Context(), remaining, sp.Name); err != nil {
						PrintFormatted(cmd.OutOrStdout(), forge.NewBaseError(
							fmt.Sprintf("stack renumbering failed: %v", err),
							"The PR was merged but stack renumbering could not complete. Use pr list to see current state.",
						))
					}
				}
			}

			return nil
		}),
	}

	cmd.Flags().StringVar(&method, "method", "", "Merge method: merge, squash, rebase")
	cmd.Flags().StringVar(&mTitle, "merge-title", "", "Merge commit title")
	cmd.Flags().StringVar(&mBody, "merge-body", "", "Merge commit body")

	setFlagErrorFunc(cmd)
	return cmd
}
