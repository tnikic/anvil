package commands

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tnikic/anvil/internal/forge"
	"github.com/tnikic/anvil/internal/format"
)

func newLabelCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "label",
		Short: "Manage labels",
		Long: `Manage labels: list, create, update, delete.

Subcommands:
  list      List labels in the repository
  create    Create a new label
  update    Update an existing label
  delete    Delete a label`,
	}
	cmd.AddCommand(
		newLabelListCmd(),
		newLabelCreateCmd(),
		newLabelUpdateCmd(),
		newLabelDeleteCmd(),
	)
	return cmd
}

func newLabelListCmd() *cobra.Command {
	var (
		fields []string
		limit  int
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List labels",
		Long: `List labels in the current repository.

Output is TOON tabular with name, scope, color, description by default.
Use --fields to add the exclusive column.
The count aggregate shows N labels.`,
		RunE: wrapForge(func(cmd *cobra.Command, args []string, f forge.Forge) error {
			labels, err := f.Labels().List(cmd.Context(), forge.LabelListOptions{
				Limit: limit,
			})
			if err != nil {
				return err
			}

			rows := make([]format.LabelRow, 0, len(labels))
			for _, l := range labels {
				rows = append(rows, format.LabelRow{
					Name:        l.Name,
					Scope:       l.Scope,
					Color:       l.Color,
					Description: l.Description,
					Exclusive:   l.Exclusive,
				})
			}

			_, _ = fmt.Fprintln(cmd.OutOrStdout(), format.LabelList(rows))
			return nil
		}),
	}

	cmd.Flags().StringSliceVar(&fields, "fields", nil, "Extra fields: exclusive")
	cmd.Flags().IntVar(&limit, "limit", 0, "Max results (default 30)")

	setFlagErrorFunc(cmd)
	return cmd
}

func newLabelCreateCmd() *cobra.Command {
	var (
		scope       string
		name        string
		color       string
		description string
		exclusive   bool
	)

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a label",
		Long: `Create a new label in the repository.

At minimum, --name and --color are required. --scope optionally scopes the
label (e.g., --scope kind --name bug creates a GitHub "kind:bug" label).
On success, prints a TOON confirmation with the label name and scope.`,
		RunE: wrapForge(func(cmd *cobra.Command, args []string, f forge.Forge) error {
			if strings.TrimSpace(name) == "" {
				return forge.NewBaseError(
					"missing required flag: --name",
					"Usage: anvil label create --name <name> --color <hex> [--scope <scope>] [--description <desc>] [--exclusive]",
				)
			}

			// Idempotent create: if the label already exists, update it with
			// only the provided fields (partial merge). If nothing changes,
			// it's a safe no-op.
			labelSvc := f.Labels()
			existing, err := labelSvc.List(cmd.Context(), forge.LabelListOptions{})
			if err != nil {
				return err
			}

			for _, l := range existing {
				if l.Scope == scope && l.Name == name {
					// Label exists — partial merge via update.
					updateOpts := forge.LabelUpdateOptions{
						Scope: scope,
						Name:  name,
					}
					hasChanges := false
					if color != "" {
						updateOpts.Color = forge.String(strings.TrimPrefix(color, "#"))
						hasChanges = true
					}
					if description != "" {
						updateOpts.Description = forge.String(description)
						hasChanges = true
					}
					if cmd.Flags().Changed("exclusive") {
						updateOpts.Exclusive = forge.Bool(exclusive)
						hasChanges = true
					}

					if !hasChanges {
						// No-op: nothing to update.
						return nil
					}

					updated, err := labelSvc.Update(cmd.Context(), updateOpts)
					if err != nil {
						return err
					}
					_, _ = fmt.Fprintln(cmd.OutOrStdout(), format.LabelUpdateConfirm(updated.Name, updated.Scope))
					return nil
				}
			}

			// Label does not exist — create it.
			opts := forge.LabelCreateOptions{
				Name: name,
			}
			if scope != "" {
				opts.Scope = forge.String(scope)
			}
			if color != "" {
				opts.Color = forge.String(strings.TrimPrefix(color, "#"))
			}
			if description != "" {
				opts.Description = forge.String(description)
			}
			if exclusive {
				opts.Exclusive = forge.Bool(true)
			}

			l, err := labelSvc.Create(cmd.Context(), opts)
			if err != nil {
				return err
			}

			_, _ = fmt.Fprintln(cmd.OutOrStdout(), format.LabelCreateConfirm(l.Name, l.Scope))
			return nil
		}),
	}

	cmd.Flags().StringVar(&scope, "scope", "", "Label scope (e.g., kind, priority)")
	cmd.Flags().StringVar(&name, "name", "", "Label name (required)")
	cmd.Flags().StringVar(&color, "color", "", "Hex color (e.g., \"#d73a4a\" or \"d73a4a\")")
	cmd.Flags().StringVar(&description, "description", "", "Label description")
	cmd.Flags().BoolVar(&exclusive, "exclusive", false, "Only one label allowed per scope")

	setFlagErrorFunc(cmd)
	return cmd
}

func newLabelUpdateCmd() *cobra.Command {
	var (
		scope       string
		color       string
		description string
		exclusive   bool
		newName     string
		newScope    string
	)

	cmd := &cobra.Command{
		Use:   "update <name>",
		Short: "Update a label",
		Long: `Update an existing label by name and scope.

The label is identified by its current name (positional argument) and --scope.
If the label is unscoped, omit --scope. Use --new-name and --new-scope to
rename or rescope the label. On success, prints a TOON confirmation.`,
		Args: cobra.ExactArgs(1),
		RunE: wrapForge(func(cmd *cobra.Command, args []string, f forge.Forge) error {
			name := strings.TrimSpace(args[0])
			if name == "" {
				return forge.NewBaseError(
					"label name must not be empty",
					"Usage: anvil label update <name> [--scope <scope>]",
				)
			}

			opts := forge.LabelUpdateOptions{
				Scope: scope,
				Name:  name,
			}
			if color != "" {
				opts.Color = forge.String(strings.TrimPrefix(color, "#"))
			}
			if description != "" {
				opts.Description = forge.String(description)
			}
			if cmd.Flags().Changed("exclusive") {
				opts.Exclusive = forge.Bool(exclusive)
			}
			if newName != "" {
				opts.NewName = forge.String(newName)
			}
			if newScope != "" {
				opts.NewScope = forge.String(newScope)
			}

			l, err := f.Labels().Update(cmd.Context(), opts)
			if err != nil {
				return err
			}

			_, _ = fmt.Fprintln(cmd.OutOrStdout(), format.LabelUpdateConfirm(l.Name, l.Scope))
			return nil
		}),
	}

	cmd.Flags().StringVar(&scope, "scope", "", "Current label scope (omit for unscoped labels)")
	cmd.Flags().StringVar(&color, "color", "", "New hex color")
	cmd.Flags().StringVar(&description, "description", "", "New description")
	cmd.Flags().BoolVar(&exclusive, "exclusive", false, "Set exclusive flag")
	cmd.Flags().StringVar(&newName, "new-name", "", "Rename the label")
	cmd.Flags().StringVar(&newScope, "new-scope", "", "Change the label scope")

	setFlagErrorFunc(cmd)
	return cmd
}

func newLabelDeleteCmd() *cobra.Command {
	var scope string

	cmd := &cobra.Command{
		Use:   "delete <name>",
		Short: "Delete a label",
		Long: `Delete a label by name and scope.

The label is identified by its name (positional argument) and --scope.
If the label is unscoped, omit --scope.
On success, prints a TOON confirmation.`,
		Args: cobra.ExactArgs(1),
		RunE: wrapForge(func(cmd *cobra.Command, args []string, f forge.Forge) error {
			name := strings.TrimSpace(args[0])
			if name == "" {
				return forge.NewBaseError(
					"label name must not be empty",
					"Usage: anvil label delete <name> [--scope <scope>]",
				)
			}

			opts := forge.LabelDeleteOptions{
				Scope: scope,
				Name:  name,
			}

			if err := f.Labels().Delete(cmd.Context(), opts); err != nil {
				return err
			}

			_, _ = fmt.Fprintln(cmd.OutOrStdout(), format.LabelDeleteConfirm(name, scope))
			return nil
		}),
	}

	cmd.Flags().StringVar(&scope, "scope", "", "Label scope (omit for unscoped labels)")

	setFlagErrorFunc(cmd)
	return cmd
}
