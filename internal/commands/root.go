package commands

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tnikic/anvil/internal/auth"
	"github.com/tnikic/anvil/internal/content"
	"github.com/tnikic/anvil/internal/forge"
	"github.com/tnikic/anvil/internal/format"
)

// PrintFormatted prints an error to w. If the error satisfies
// forge.StructuredError it formats "error: msg" optionally followed
// by "help: hint"; otherwise it prints the error string.
func PrintFormatted(w io.Writer, err error) {
	var se forge.StructuredError
	if errors.As(err, &se) {
		if se.Help() != "" {
			_, _ = fmt.Fprintf(w, "error: %s\nhelp: %s\n", se.Message(), se.Help())
		} else {
			_, _ = fmt.Fprintf(w, "error: %s\n", se.Message())
		}
		return
	}
	_, _ = fmt.Fprintln(w, err)
}

func NewRoot() *cobra.Command {
	var (
		forgeFlag string
		repoFlag  string
	)

	cmd := &cobra.Command{
		Use:           "anvil",
		Short:         "AXI-compliant Git forge CLI for AI agents",
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runHome(cmd, forgeFlag, repoFlag)
		},
	}

	cmd.PersistentFlags().StringVar(&forgeFlag, "forge", "", "Forge host (e.g., github.com, gitlab.com)")
	cmd.PersistentFlags().StringVar(&repoFlag, "repo", "", "Repository (owner/name)")

	setFlagErrorFunc(cmd)

	cmd.AddCommand(
		newIssueCmd(),
		newLabelCmd(),
		newPRCmd(),
		newAuthCmd(),
		newSkillsCmd(),
		newSetupCmd(),
	)

	return cmd
}

func runHome(cmd *cobra.Command, forgeFlag, repoFlag string) error {
	w := cmd.OutOrStdout()

	// Header (always shown)
	binPath := binPath()
	_, _ = fmt.Fprintf(w, "bin: %s\n", binPath)
	_, _ = fmt.Fprintf(w, "description: %s\n", content.Description)

	// Detect forge and repo
	host, repo, detectErr := forge.Detect(forgeFlag, repoFlag)
	if detectErr != nil {
		printFallback(w)
		return nil
	}

	_, _ = fmt.Fprintf(w, "\nforge: %s\n", host)
	_, _ = fmt.Fprintf(w, "repo: %s\n", repo)

	// Resolve forge client (includes auth)
	f, resolveErr := resolveForgeForDashboard(cmd, host, repo)
	if resolveErr != nil {
		printFallback(w)
		return nil
	}

	// Query live data from forge
	ctx := cmd.Context()
	issues, issueMeta, _ := f.Issues().List(ctx, forge.IssueListOptions{
		State:     forge.StateOpen,
		Sort:      "updated",
		Direction: "desc",
	})
	prs, prMeta, _ := f.PRs().List(ctx, forge.PRListOptions{
		State:     forge.StateOpen,
		Sort:      "updated",
		Direction: "desc",
	})

	// Print live data sections
	printDashboardIssues(w, issues, issueMeta)
	printDashboardPRs(w, prs, prMeta)

	// Print contextual help hints
	issueTotal := 0
	prTotal := 0
	if issueMeta != nil {
		issueTotal = issueMeta.Total
	}
	if prMeta != nil {
		prTotal = prMeta.Total
	}
	tips := content.DashboardTips(issueTotal, prTotal)
	_, _ = fmt.Fprintf(w, "\nhelp[%d]:\n", len(tips))
	for _, tip := range tips {
		_, _ = fmt.Fprintf(w, "  %s\n", tip)
	}

	return nil
}

// resolveForgeForDashboard creates a forge client from already-detected host and repo.
// Returns an error if auth resolution or forge creation fails.
func resolveForgeForDashboard(cmd *cobra.Command, host, repo string) (forge.Forge, error) {
	tok, err := resolveTokenForHost(host)
	if err != nil {
		return nil, err
	}

	parts := strings.SplitN(repo, "/", 2)
	if len(parts) != 2 {
		return nil, forge.NewBaseError(
			fmt.Sprintf("invalid repository: %s", repo),
			"Expected format: owner/name",
		)
	}

	return ForgeFn(host, parts[0], parts[1], tok), nil
}

// resolveTokenForHost resolves the auth token for a host.
func resolveTokenForHost(host string) (string, error) {
	return auth.ResolveToken(host)
}

// printFallback prints fallback help hints when forge/auth is unavailable.
func printFallback(w io.Writer) {
	_, _ = fmt.Fprintf(w, "\nhelp[%d]:\n", len(content.GlobalTips))
	for _, tip := range content.GlobalTips {
		_, _ = fmt.Fprintf(w, "  %s\n", tip)
	}
}

// printDashboardIssues prints the issues section of the dashboard.
func printDashboardIssues(w io.Writer, issues []forge.Issue, meta *forge.ListMeta) {
	if len(issues) == 0 {
		_, _ = fmt.Fprint(w, "\nissues: 0 open\n")
		return
	}

	display := issues
	if len(display) > 3 {
		display = display[:3]
	}

	rows := make([]format.DashboardIssueRow, len(display))
	for i, iss := range display {
		rows[i] = format.DashboardIssueRow{
			Number: iss.Number,
			Title:  iss.Title,
			State:  iss.State,
			Author: iss.Author,
		}
	}

	count := len(display)
	total := count
	if meta != nil {
		total = meta.Total
	}

	_, _ = fmt.Fprint(w, "\n"+format.DashboardIssueList(rows, count, total))
}

// printDashboardPRs prints the PRs section of the dashboard.
func printDashboardPRs(w io.Writer, prs []forge.PR, meta *forge.ListMeta) {
	if len(prs) == 0 {
		_, _ = fmt.Fprint(w, "\nprs: 0 open\n")
		return
	}

	display := prs
	if len(display) > 3 {
		display = display[:3]
	}

	rows := make([]format.DashboardPRRow, len(display))
	for i, p := range display {
		rows[i] = format.DashboardPRRow{
			Number: p.Number,
			Title:  p.Title,
			Author: p.Author,
		}
	}

	count := len(display)
	total := count
	if meta != nil {
		total = meta.Total
	}

	_, _ = fmt.Fprint(w, "\n"+format.DashboardPRList(rows, count, total))
}

// collapseHome replaces the home directory prefix in path with "~".
// Returns path unchanged if home cannot be determined or path is not under home.
func collapseHome(path string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	if strings.HasPrefix(path, home+string(os.PathSeparator)) {
		return "~" + path[len(home):]
	}
	return path
}

func binPath() string {
	exe, err := os.Executable()
	if err != nil {
		return "anvil"
	}
	return collapseHome(filepath.Clean(exe))
}
