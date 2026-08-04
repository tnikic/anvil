package stack

import (
	"context"
	"fmt"
	"sort"

	"github.com/tnikic/anvil/internal/forge"
)

// PRUpdater is the narrow interface the stack module needs from a forge.
// forge.PRService satisfies this automatically.
type PRUpdater interface {
	Update(ctx context.Context, opts forge.PRUpdateOptions) (*forge.PR, error)
}

// Tracker orchestrates stack operations that require forge API access.
type Tracker struct {
	prs PRUpdater
}

// NewTracker creates a Tracker with the given PR updater.
func NewTracker(prs PRUpdater) *Tracker {
	return &Tracker{prs: prs}
}

// Renumber updates the title prefix of every open PR in the slice to reflect
// the current position (1-indexed) and total count. PRs whose title already
// matches the computed prefix are skipped.
func (t *Tracker) Renumber(ctx context.Context, prs []forge.PR, stackName string) error {
	total := len(prs)
	for i, pr := range prs {
		pos := i + 1
		clean := CleanTitle(pr.Title)
		newTitle := FormatPrefix(Prefix{Name: stackName, Pos: pos, Total: total}) + " " + clean
		if newTitle != pr.Title {
			if _, err := t.prs.Update(ctx, forge.PRUpdateOptions{
				Number: pr.Number,
				Title:  forge.String(newTitle),
			}); err != nil {
				return fmt.Errorf("updating PR #%d title: %w", pr.Number, err)
			}
		}
	}
	return nil
}

// DiagnoseBroken checks for broken stacks — PRs closed (not merged) between
// the first and last open PR in a stack. Returns diagnostic messages for each
// gap found.
//
// The caller must fetch all PRs (open and closed) and pass them via allPRs.
// openPRs is the subset the caller already has; DiagnoseBroken uses it to
// find the active range within each stack.
func DiagnoseBroken(openPRs, allPRs []forge.PR) []string {
	Populate(allPRs)

	// Group all PRs by stack.
	stacks := make(map[string][]forge.PR)
	for _, pr := range allPRs {
		if pr.Stack == "" {
			continue
		}
		stacks[pr.Stack] = append(stacks[pr.Stack], pr)
	}

	var diags []string
	for name, prs := range stacks {
		sortByPos(prs)

		// Find first and last open PR in the stack.
		firstOpen := -1
		lastOpen := -1
		for i, pr := range prs {
			if pr.State == forge.StateOpen {
				if firstOpen == -1 {
					firstOpen = i
				}
				lastOpen = i
			}
		}
		if firstOpen == -1 {
			continue
		}

		// Check for closed (not merged) PRs between firstOpen and lastOpen.
		for i := firstOpen; i <= lastOpen; i++ {
			if prs[i].State == forge.StateClosed {
				diags = append(diags, fmt.Sprintf(
					"stack %q is broken: PR #%d (%s) is closed without merging. Consider: anvil pr rebase --onto <target> --skip %d",
					name, prs[i].Number, CleanTitle(prs[i].Title), prs[i].Number,
				))
			}
		}
	}
	return diags
}

// sortByPos sorts a slice of PRs by their stack prefix position in place.
func sortByPos(prs []forge.PR) {
	sort.Slice(prs, func(i, j int) bool {
		pi, _ := ParsePrefix(prs[i].Title)
		pj, _ := ParsePrefix(prs[j].Title)
		return pi.Pos < pj.Pos
	})
}
