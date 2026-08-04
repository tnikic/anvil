package forgejo

import (
	"fmt"
	"regexp"

	gitea "code.gitea.io/sdk/gitea"
	"github.com/tnikic/anvil/internal/forge"
)

// parentTitleRE matches the [parent:N] prefix that indicates a sub-issue.
var parentTitleRE = regexp.MustCompile(`^\[parent:(\d+)\]\s*`)

// Parse extracts the parent issue number from a title that uses the
// [parent:N] convention. Returns (parent, cleanTitle).
// If no parent prefix is found, parent is nil and cleanTitle equals title.
func Parse(title string) (parent *int, cleanTitle string) {
	matches := parentTitleRE.FindStringSubmatch(title)
	if matches == nil {
		return nil, title
	}
	var parentNum int
	if _, err := fmt.Sscanf(matches[1], "%d", &parentNum); err != nil {
		return nil, title
	}
	cleanTitle = title[len(matches[0]):]
	return &parentNum, cleanTitle
}

// Inject prepends [parent:N] to the title. If parent is nil, returns the
// title unchanged. If the title already has a parent prefix, it is
// replaced (idempotent).
func Inject(title string, parent *int) string {
	if parent == nil {
		return title
	}
	// Strip any existing prefix first, then inject.
	_, clean := Parse(title)
	return fmt.Sprintf("[parent:%d] %s", *parent, clean)
}

// Strip removes any [parent:N] prefix from the title.
// Returns the title without the prefix. If no prefix is present, returns
// the title unchanged.
func Strip(title string) string {
	_, clean := Parse(title)
	return clean
}

// FindChildren scans a list of issues and returns IssueDependency entries
// for those that have the given issue number as their parent (via the
// [parent:N] title prefix).
func FindChildren(issues []*gitea.Issue, parentNumber int) []forge.IssueDependency {
	var deps []forge.IssueDependency
	for _, i := range issues {
		if i == nil {
			continue
		}
		parent, cleanTitle := Parse(i.Title)
		if parent != nil && *parent == parentNumber {
			deps = append(deps, forge.IssueDependency{
				Number:    int(i.Index),
				Title:     cleanTitle,
				State:     string(i.State),
				Direction: forge.DirChild,
			})
		}
	}
	return deps
}
