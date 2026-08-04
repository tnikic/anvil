package commands

import (
	"fmt"
	"slices"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tnikic/anvil/internal/forge"
)

// hasField returns true if name is in fields.
func hasField(fields []string, name string) bool {
	return slices.Contains(fields, name)
}

// setFlagErrorFunc configures a cobra.Command to produce structured,
// AXI-compliant errors for unknown flags. The error includes the flag name,
// the subcommand path, and the valid flags inlined.
//
// --help always passes through: pflag intercepts it before FlagErrorFunc fires.
func setFlagErrorFunc(cmd *cobra.Command) {
	cmd.SetFlagErrorFunc(func(cmd *cobra.Command, err error) error {
		path := cmd.CommandPath()

		// Extract the flag identifier from pflag's error message.
		// pflag errors: "unknown flag: --stat" or "unknown shorthand flag: 'x' in -x"
		flagPart := strings.TrimPrefix(err.Error(), "unknown flag: ")
		flagPart = strings.TrimPrefix(flagPart, "unknown shorthand flag: ")

		// Collect valid flags (both local and inherited) for the subcommand.
		flagUsages := strings.TrimRight(cmd.Flags().FlagUsages(), "\n")

		e := forge.NewBaseError(
			fmt.Sprintf("unknown flag %s for %q", flagPart, path),
			fmt.Sprintf("Valid flags:\n%s", flagUsages),
		)
		return e
	})
}
