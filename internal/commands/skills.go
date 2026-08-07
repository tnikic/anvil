package commands

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/tnikic/anvil/internal/forge"
	"github.com/tnikic/anvil/internal/skillgen"
	"github.com/tnikic/anvil/internal/skills"
)

func newSkillsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "skills",
		Short: "Manage the anvil agent skill",
		Long: `Manage the anvil agent skill file.

Agent harnesses discover skills in ~/.agents/skills/. The skills
subcommand extracts the embedded SKILL.md from the binary and installs
it to ~/.agents/skills/anvil/, keeping the skill version-locked
to the binary.`,
	}
	cmd.AddCommand(
		newSkillsInstallCmd(),
		newSkillsListCmd(),
		newSkillsUpdateCmd(),
		newSkillsUninstallCmd(),
		newSkillsStatusCmd(),
	)
	setFlagErrorFunc(cmd)
	return cmd
}

// skillsDir is the standard agent skill discovery directory.
func skillsDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", forge.NewBaseError("cannot determine home directory", "Set $HOME")
	}
	return filepath.Join(home, ".agents", "skills", "anvil"), nil
}

// embeddedSkills returns the list of skill file paths embedded in the binary.
func embeddedSkills() ([]string, error) {
	var paths []string
	err := fs.WalkDir(skills.SkillsFS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			// Strip "anvil/" prefix to get the relative path.
			rel := strings.TrimPrefix(path, "anvil/")
			paths = append(paths, rel)
		}
		return nil
	})
	return paths, err
}

func newSkillsInstallCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install the skill to ~/.agents/skills/anvil/",
		Long:  "Extract the embedded SKILL.md from the binary and install it to ~/.agents/skills/anvil/.",
		RunE: func(cmd *cobra.Command, args []string) error {
			err := installOrUpdateSkills(cmd.OutOrStdout(), "installed")
			if err != nil {
				return err
			}
			printStaleNote(cmd.OutOrStdout())
			return nil
		},
	}
	setFlagErrorFunc(cmd)
	return cmd
}

func newSkillsListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List files embedded in the binary",
		Long:  "List the skill files embedded in this binary.",
		RunE: func(cmd *cobra.Command, args []string) error {
			paths, err := embeddedSkills()
			if err != nil {
				e := forge.NewBaseError(
					fmt.Sprintf("cannot list embedded skills: %v", err),
					"",
				)
				return e
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "skills[%d]{path}:\n", len(paths))
			for _, p := range paths {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  %s\n", p)
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "count: %d total\n", len(paths))
			return nil
		},
	}
	setFlagErrorFunc(cmd)
	return cmd
}

func newSkillsUpdateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Re-install the skill (overwrite)",
		Long:  "Regenerate and overwrite the skill files in ~/.agents/skills/anvil/.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return regenerateInstalledSkill(cmd.OutOrStdout())
		},
	}
	setFlagErrorFunc(cmd)
	return cmd
}

func newSkillsUninstallCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "uninstall",
		Short: "Remove the skill from ~/.agents/skills/anvil/",
		Long:  "Remove the installed skill directory from ~/.agents/skills/anvil/.",
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, err := skillsDir()
			if err != nil {
				return err
			}

			if _, statErr := os.Stat(dir); os.IsNotExist(statErr) {
				return nil
			}

			if err := os.RemoveAll(dir); err != nil {
				e := forge.NewBaseError(
					fmt.Sprintf("cannot remove skills directory: %v", err),
					"",
				)
				return e
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "removed: %s\n", collapseHome(dir))
			return nil
		},
	}
	setFlagErrorFunc(cmd)
	return cmd
}

func newSkillsStatusCmd() *cobra.Command {
	var check bool

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show install status",
		Long:  "Show whether and where the skill is installed. Use --check to verify the embedded skill matches what this binary would generate.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if check {
				return runSkillsCheck(cmd.OutOrStdout())
			}

			dir, err := skillsDir()
			if err != nil {
				return err
			}

			skillPath := filepath.Join(dir, "SKILL.md")
			_, statErr := os.Stat(skillPath)
			installed := statErr == nil
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "name: anvil\n")
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "path: %s\n", collapseHome(skillPath))
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "installed: %v\n", installed)

			// Also print staleness when installed.
			if installed {
				printStaleNote(cmd.OutOrStdout())
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&check, "check", false, "Verify embedded skill matches generated output")
	setFlagErrorFunc(cmd)
	return cmd
}

// installOrUpdateSkills creates the skills directory, copies embedded files,
// and prints the result with the given verb ("installed" or "updated").
func installOrUpdateSkills(w io.Writer, verb string) error {
	dir, err := skillsDir()
	if err != nil {
		PrintFormatted(w, err)
		return err
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		e := forge.NewBaseError(
			fmt.Sprintf("cannot create skills directory: %v", err),
			"",
		)
		PrintFormatted(w, e)
		return e
	}

	count, err := copySkills(dir)
	if err != nil {
		PrintFormatted(w, err)
		return err
	}

	_, _ = fmt.Fprintf(w, "%s: %s\nfiles: %d\n",
		verb,
		collapseHome(filepath.Join(dir, "SKILL.md")),
		count,
	)
	return nil
}

// copySkills copies all embedded skill files from SkillsFS to destDir.
// Returns the number of files copied.
func copySkills(destDir string) (int, error) {
	var count int
	err := fs.WalkDir(skills.SkillsFS, "anvil", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return forge.NewBaseError(
				fmt.Sprintf("cannot walk embedded skills: %v", err),
				"",
			)
		}
		if d.IsDir() {
			return nil
		}

		// path is "anvil/SKILL.md"; strip the "anvil/" prefix.
		rel := strings.TrimPrefix(path, "anvil/")
		dest := filepath.Join(destDir, rel)

		// Ensure parent directory exists.
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return forge.NewBaseError(
				fmt.Sprintf("cannot create directory %s: %v", filepath.Dir(dest), err),
				"",
			)
		}

		data, err := fs.ReadFile(skills.SkillsFS, path)
		if err != nil {
			return forge.NewBaseError(
				fmt.Sprintf("cannot read embedded file %s: %v", path, err),
				"",
			)
		}

		if err := os.WriteFile(dest, data, 0o644); err != nil {
			return forge.NewBaseError(
				fmt.Sprintf("cannot write %s: %v", dest, err),
				"",
			)
		}
		count++
		return nil
	})
	if err != nil {
		return count, err
	}
	return count, nil
}

// printStaleNote checks whether the embedded SKILL.md is stale and prints
// a one-line note to w.
func printStaleNote(w io.Writer) {
	embedded, err := skillgen.ReadEmbedded(skills.SkillsFS)
	if err != nil {
		return
	}
	ok, _, err := skillgen.Check(embedded)
	if err != nil || ok {
		_, _ = fmt.Fprintf(w, "skill: current\n")
	} else {
		_, _ = fmt.Fprintf(w, "skill: stale; run `anvil skills update` to refresh\n")
	}
}

// runSkillsCheck implements `anvil skills status --check`.
// Regenerates in-memory and diffs against the embedded file.
// Exits 0 when matching, exits 1 on drift.
func runSkillsCheck(w io.Writer) error {
	embedded, err := skillgen.ReadEmbedded(skills.SkillsFS)
	if err != nil {
		return err
	}

	ok, diff, err := skillgen.Check(embedded)
	if err != nil {
		return err
	}

	if ok {
		_, _ = fmt.Fprintf(w, "skill:\n  status: current\n")
		return nil
	}

	_, _ = fmt.Fprintf(w, "skill:\n  status: stale\n")
	if diff != "" {
		for _, line := range strings.Split(strings.TrimSpace(diff), "\n") {
			_, _ = fmt.Fprintf(w, "  %s\n", line)
		}
	}
	return forge.NewBaseError(
		"embedded SKILL.md is stale; regenerate with `go generate ./internal/skills/...`",
		"Run `anvil skills update` to refresh the installed skill",
	)
}

// regenerateInstalledSkill regenerates SKILL.md from the content package
// and writes it to the installed skill directory.
func regenerateInstalledSkill(w io.Writer) error {
	skill, err := skillgen.Render()
	if err != nil {
		return err
	}

	dir, err := skillsDir()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		e := forge.NewBaseError(
			fmt.Sprintf("cannot create skills directory: %v", err),
			"",
		)
		PrintFormatted(w, e)
		return e
	}

	skillPath := filepath.Join(dir, "SKILL.md")
	if err := os.WriteFile(skillPath, []byte(skill), 0o644); err != nil {
		e := forge.NewBaseError(
			fmt.Sprintf("cannot write %s: %v", skillPath, err),
			"",
		)
		PrintFormatted(w, e)
		return e
	}

	_, _ = fmt.Fprintf(w, "updated: %s\nfiles: 1\n", collapseHome(skillPath))
	printStaleNote(w)
	return nil
}
