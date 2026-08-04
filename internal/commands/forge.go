package commands

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tnikic/anvil/internal/auth"
	"github.com/tnikic/anvil/internal/forge"
	"github.com/tnikic/anvil/internal/forge/forgejo"
	"github.com/tnikic/anvil/internal/forge/github"
	"github.com/tnikic/anvil/internal/forge/gitlab"
)

// ForgeFn creates a forge adapter given host, owner, repo, and token.
// Set by main.go for production use; overridden in tests.
var ForgeFn func(host, owner, repo, token string) forge.Forge

func init() {
	ForgeFn = defaultForgeFn
}

// adapterConstructors maps forge type identifiers (from auth.InferForgeType)
// to adapter constructors. When a second adapter (GitLab, Forgejo) is added,
// register its constructor here and the dispatch becomes real.
var adapterConstructors = map[string]func(host, owner, repo string, httpClient *http.Client) forge.Forge{
	"github": func(host, owner, repo string, httpClient *http.Client) forge.Forge {
		return github.New(host, owner, repo, httpClient)
	},
	"gitlab": func(host, owner, repo string, httpClient *http.Client) forge.Forge {
		return gitlab.New(host, owner, repo, httpClient)
	},
	"forgejo": func(host, owner, repo string, httpClient *http.Client) forge.Forge {
		return forgejo.New(host, owner, repo, httpClient)
	},
}

func defaultForgeFn(host, owner, repo, token string) forge.Forge {
	httpClient := &http.Client{
		Transport: &tokenTransport{
			token: token,
			base:  http.DefaultTransport,
		},
	}

	forgeType := auth.InferForgeType(host)
	if ctor, ok := adapterConstructors[forgeType]; ok {
		return ctor(host, owner, repo, httpClient)
	}

	// Fall back to GitHub for unknown types — the only adapter today.
	// When more adapters are added, this should return an error instead.
	return github.New(host, owner, repo, httpClient)
}

type tokenTransport struct {
	token string
	base  http.RoundTripper
}

func (t *tokenTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("Authorization", "Bearer "+t.token)
	return t.base.RoundTrip(req)
}

// Token returns the bearer token carried by this transport.
func (t *tokenTransport) Token() string { return t.token }

// ForgeHandlerFunc is the signature for subcommand handlers that need a resolved
// forge. The forge is pre-resolved by wrapForge before the handler runs.
type ForgeHandlerFunc func(cmd *cobra.Command, args []string, f forge.Forge) error

// wrapForge returns a cobra RunE function that resolves the forge via resolveForge
// and passes it to handler. Errors are returned unprinted — main.go is the sole
// owner of error output formatting.
func wrapForge(handler ForgeHandlerFunc) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		f, _, _, err := resolveForge(cmd)
		if err != nil {
			return err
		}
		return handler(cmd, args, f)
	}
}

// resolveForge extracts forge/repo from flags or auto-detection, resolves the
// auth token, and returns a forge.Forge interface, the host, and the full repo
// (owner/name). On error, returns a structured Error that can be printed to stdout.
func resolveForge(cmd *cobra.Command) (forge.Forge, string, string, error) {
	forgeFlag, _ := cmd.Flags().GetString("forge")
	repoFlag, _ := cmd.Flags().GetString("repo")

	host, repo, err := forge.Detect(forgeFlag, repoFlag)
	if err != nil {
		return nil, "", "", forge.NewBaseError(
			err.Error(),
			"anvil --forge <host> --repo <owner/name>",
		)
	}

	tok, err := auth.ResolveToken(host)
	if err != nil {
		return nil, "", "", forge.NewBaseError(
			err.Error(),
			authErrHelp(host),
		)
	}

	parts := strings.SplitN(repo, "/", 2)
	if len(parts) != 2 {
		return nil, "", "", forge.NewBaseError(
			fmt.Sprintf("invalid repository: %s", repo),
			"Expected format: owner/name",
		)
	}

	f := ForgeFn(host, parts[0], parts[1], tok)
	return f, host, repo, nil
}

func authErrHelp(host string) string {
	return fmt.Sprintf("Run \"anvil auth set %s <token>\"", host)
}
