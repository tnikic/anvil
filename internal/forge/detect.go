package forge

import (
	"fmt"
	"os/exec"
	"strings"
)

// Detect determines the forge host and repository owner/name.
// If forgeFlag or repoFlag are provided, they override auto-detection.
func Detect(forgeFlag, repoFlag string) (forge, repo string, err error) {
	if forgeFlag != "" && repoFlag != "" {
		return forgeFlag, repoFlag, nil
	}

	remote, err := RemoteFn()
	if err != nil {
		return "", "", fmt.Errorf("not in a git repository; cannot auto-detect forge and repo")
	}

	forge, repo, err = ParseRemote(remote)
	if err != nil {
		return "", "", fmt.Errorf("cannot parse git remote: %w", err)
	}

	if forgeFlag != "" {
		forge = forgeFlag
	}
	if repoFlag != "" {
		repo = repoFlag
	}

	return forge, repo, nil
}

// RemoteFn resolves the URL of the "origin" git remote.
// Defaults to shelling out to git; overridden in tests.
var RemoteFn func() (string, error)

func init() {
	RemoteFn = defaultRemoteFn
}

func defaultRemoteFn() (string, error) {
	cmd := exec.Command("git", "remote", "get-url", "origin")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// ParseRemote extracts forge host and owner/repo from a git remote URL.
// Handles HTTPS (https://github.com/owner/repo.git) and SSH (git@github.com:owner/repo.git).
func ParseRemote(remote string) (forge, repo string, err error) {
	remote = strings.TrimSuffix(remote, ".git")

	if strings.HasPrefix(remote, "https://") {
		rest := strings.TrimPrefix(remote, "https://")
		parts := strings.SplitN(rest, "/", 2)
		if len(parts) != 2 {
			return "", "", fmt.Errorf("invalid HTTPS remote: %s", remote)
		}
		return parts[0], strings.TrimSuffix(parts[1], "/"), nil
	}

	if strings.HasPrefix(remote, "git@") {
		// git@github.com:owner/repo
		rest := strings.TrimPrefix(remote, "git@")
		parts := strings.SplitN(rest, ":", 2)
		if len(parts) != 2 {
			return "", "", fmt.Errorf("invalid SSH remote: %s", remote)
		}
		return parts[0], strings.TrimSuffix(parts[1], "/"), nil
	}

	return "", "", fmt.Errorf("unsupported remote format: %s", remote)
}
