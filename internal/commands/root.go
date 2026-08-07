package commands

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tnikic/anvil/internal/content"
	"github.com/tnikic/anvil/internal/forge"
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
	)

	return cmd
}

func runHome(cmd *cobra.Command, forgeFlag, repoFlag string) error {
	w := cmd.OutOrStdout()

	binPath := binPath()
	_, _ = fmt.Fprintf(w, "bin: %s\n", binPath)
	_, _ = fmt.Fprintf(w, "description: %s\n", content.Description)

	// Determine forge and repo
	f, r, err := forge.Detect(forgeFlag, repoFlag)
	if err != nil {
		return forge.NewBaseError(err.Error(), "anvil --forge <host> --repo <owner/name>")
	}

	_, _ = fmt.Fprintf(w, "\nforge: %s\n", f)
	_, _ = fmt.Fprintf(w, "repo: %s\n", r)

	_, _ = fmt.Fprintf(w, "\nhelp[%d]:\n", len(content.GlobalTips))
	for _, tip := range content.GlobalTips {
		_, _ = fmt.Fprintf(w, "  %s\n", tip)
	}

	return nil
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
