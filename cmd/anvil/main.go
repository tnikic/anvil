package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/tnikic/anvil/internal/commands"
)

// version is set at build time via -ldflags="-X main.version=...".
// goreleaser injects the tag name (e.g., v0.1.0). The default "dev"
// appears in development builds.
var version = "dev"

func main() {
	defer func() {
		if r := recover(); r != nil {
			_, _ = fmt.Fprintf(os.Stdout, "error: internal error: %v\n", r)
			os.Exit(1)
		}
	}()

	cmd := commands.NewRoot()
	cmd.Version = version
	if err := cmd.Execute(); err != nil {
		// main.go is the sole owner of error output formatting.
		// All subcommands return errors unprinted; main.go formats
		// them to stdout per AXI §6.
		commands.PrintFormatted(cmd.OutOrStdout(), err)
		if isUsageError(err) {
			os.Exit(2)
		}
		os.Exit(1)
	}
}

func isUsageError(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "unknown flag") ||
		strings.Contains(msg, "unknown command") ||
		strings.Contains(msg, "required flag")
}
