package commands

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tnikic/anvil/internal/auth"
	"github.com/tnikic/anvil/internal/forge"
	"github.com/tnikic/anvil/internal/format"
)

const authLongDesc = `Manage authentication tokens for Git forges.

Tokens are stored in a single JSON file keyed by host. Forge type is inferred
from the host at runtime — no need to specify it when setting a token.

Subcommands:
  status   List all configured hosts with forge type and source path
  set      Store a token for a host
  unset    Remove a token for a host`

func newAuthCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Manage authentication",
		Long:  authLongDesc,
	}
	cmd.AddCommand(
		newAuthStatusCmd(),
		newAuthSetCmd(),
		newAuthUnsetCmd(),
	)
	return cmd
}

func newAuthStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "List configured hosts",
		Long:  "List all configured hosts with their forge type and credentials file path.",
		RunE: func(cmd *cobra.Command, args []string) error {
			store := auth.NewStore(auth.DefaultStorePath())
			entries := store.List()

			sourcePath := auth.CollapseHome(auth.DefaultStorePath())
			rows := make([]format.AuthRow, 0, len(entries))
			for _, e := range entries {
				rows = append(rows, format.AuthRow{
					Forge:  auth.InferForgeType(e.Host),
					Host:   e.Host,
					Source: sourcePath,
				})
			}
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), format.AuthStatus(rows))
			return nil
		},
	}
	setFlagErrorFunc(cmd)
	return cmd
}

func newAuthSetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set <host> <token>",
		Short: "Store a token",
		Long:  "Store an API token for a forge host. The forge type is inferred from the host.",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			host := strings.TrimSpace(args[0])
			token := strings.TrimSpace(args[1])

			if host == "" {
				return forge.NewBaseError("host must not be empty", "")
			}
			if token == "" {
				return forge.NewBaseError("token must not be empty", "")
			}

			store := auth.NewStore(auth.DefaultStorePath())
			if err := store.Set(host, token); err != nil {
				return err
			}

			forgeType := auth.InferForgeType(host)
			sourcePath := auth.CollapseHome(auth.DefaultStorePath())
			_, _ = fmt.Fprintf(cmd.OutOrStdout(),
				"token stored for %s (%s)\ncredentials: %s\n",
				host, forgeType, sourcePath)
			return nil
		},
	}
	setFlagErrorFunc(cmd)
	return cmd
}

func newAuthUnsetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "unset <host>",
		Short: "Remove a token",
		Long:  "Remove the stored token for a host. No-op if no token is stored.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			host := strings.TrimSpace(args[0])

			if host == "" {
				return forge.NewBaseError("host must not be empty", "")
			}

			store := auth.NewStore(auth.DefaultStorePath())
			if err := store.Unset(host); err != nil {
				return err
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "token removed for %s\n", host)
			return nil
		},
	}
	setFlagErrorFunc(cmd)
	return cmd
}
