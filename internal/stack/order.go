package stack

import (
	"fmt"
	"sort"
	"strings"

	"github.com/tnikic/anvil/internal/forge"
)

// Populate scans PR titles and sets each PR's Stack field from its prefix.
// PRs without a prefix keep their Stack field unchanged.
func Populate(prs []forge.PR) {
	for i := range prs {
		if p, ok := ParsePrefix(prs[i].Title); ok {
			prs[i].Stack = p.Name
		}
	}
}

// SortKey returns a deterministic sort key for a PR that groups by stack:
// unstacked PRs sort first by number; stacked PRs sort by (stack name, position, number).
func SortKey(pr forge.PR) string {
	if pr.Stack == "" {
		return fmt.Sprintf("!%010d", pr.Number)
	}
	p, ok := ParsePrefix(pr.Title)
	if !ok {
		return fmt.Sprintf("%s!%010d", pr.Stack, pr.Number)
	}
	return fmt.Sprintf("%s!%010d!%010d", pr.Stack, p.Pos, pr.Number)
}

// Sort sorts PRs in place: unstacked first by number, then by stack name, then position.
func Sort(prs []forge.PR) {
	sort.Slice(prs, func(i, j int) bool {
		return SortKey(prs[i]) < SortKey(prs[j])
	})
}

// ComputeDepends walks base.ref/head.ref relationships across the slice
// and populates DependsOn and DependedOnBy on each PR. PRs must already have
// Stack populated.
func ComputeDepends(prs []forge.PR) {
	byHead := make(map[string]*forge.PR, len(prs))
	for i := range prs {
		if prs[i].HeadRef != "" {
			byHead[prs[i].HeadRef] = &prs[i]
		}
	}

	for i := range prs {
		pr := &prs[i]
		// DependsOn: find PR whose head.ref == this PR's base.ref
		if below, ok := byHead[pr.BaseRef]; ok && below.Number != pr.Number {
			pr.DependsOn = append(pr.DependsOn, below.Number)
		}
		// DependedOnBy: find PRs whose base.ref == this PR's head.ref
		for j := range prs {
			if prs[j].BaseRef == pr.HeadRef && prs[j].Number != pr.Number {
				pr.DependedOnBy = append(pr.DependedOnBy, prs[j].Number)
			}
		}
	}
}

// DeriveName returns a stack name from a branch name. If the branch contains
// a '/', the segment after the last '/' becomes the name. Branches without '/'
// return the empty string.
//
//	"feat/auth"   → "auth"
//	"fix-branch"  → ""
func DeriveName(branch string) string {
	if idx := strings.LastIndex(branch, "/"); idx >= 0 && idx < len(branch)-1 {
		return branch[idx+1:]
	}
	return ""
}

// CollectOpen returns all open PRs belonging to the given stack, sorted by
// their position prefix. PRs without a stack prefix are skipped.
func CollectOpen(stackName string, prs []forge.PR) []forge.PR {
	var result []forge.PR
	for _, pr := range prs {
		if pr.State != forge.StateOpen {
			continue
		}
		p, ok := ParsePrefix(pr.Title)
		if !ok || p.Name != stackName {
			continue
		}
		result = append(result, pr)
	}
	sort.Slice(result, func(i, j int) bool {
		pi, _ := ParsePrefix(result[i].Title)
		pj, _ := ParsePrefix(result[j].Title)
		return pi.Pos < pj.Pos
	})
	return result
}
