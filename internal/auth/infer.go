package auth

import "strings"

// InferForgeType returns a forge type identifier (e.g., "github", "gitlab",
// "forgejo") inferred from the host string.
func InferForgeType(host string) string {
	h := strings.ToLower(host)

	// Well-known hosts.
	switch h {
	case "github.com":
		return "github"
	case "gitlab.com":
		return "gitlab"
	case "codeberg.org":
		return "forgejo"
	}

	// Heuristics for self-hosted instances.
	if strings.Contains(h, "github") {
		return "github"
	}
	if strings.Contains(h, "gitlab") {
		return "gitlab"
	}
	if strings.Contains(h, "gitea") || strings.Contains(h, "forgejo") {
		return "forgejo"
	}

	return "unknown"
}
