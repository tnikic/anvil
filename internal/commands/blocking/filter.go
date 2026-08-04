// Package blocking provides client-side filtering of issues by blocking status
// for the issue list command. It supports three blocking-related features:
//
//	--unblocked        Show only issues with no open blockers
//	--blocked          Show only issues with open blockers
//	--fields blocked   Display the count of open blockers per issue
//
// The Filter type encapsulates validation, data pre-computation, row filtering,
// and total-count adjustment so that the command handler reads as a simple pipeline.
package blocking

import (
	"context"
	"strconv"

	"github.com/tnikic/anvil/internal/forge"
)

// Filter holds the configuration for client-side blocking filtering.
type Filter struct {
	Unblocked   bool // --unblocked: keep only issues with no open blockers
	Blocked     bool // --blocked: keep only issues with at least one open blocker
	ShowBlocked bool // --fields blocked: populate the blocked column
}

// NeedsBlocking returns true when at least one blocking-related feature is active.
func (f *Filter) NeedsBlocking() bool {
	return f.Unblocked || f.Blocked || f.ShowBlocked
}

// Validate checks configuration rules for blocking flags.
// It returns an error if --unblocked and --blocked are both set.
func (f *Filter) Validate() error {
	if f.Unblocked && f.Blocked {
		return forge.NewBaseError(
			"--unblocked and --blocked are mutually exclusive",
			"Use either --unblocked or --blocked, not both",
		)
	}
	return nil
}

// ComputeCounts calls BlockedBy for each issue and returns a map from issue
// number to the count of open (non-closed) blockers.
// Closed blockers are not counted. The first BlockedBy error halts computation
// and is returned.
func (f *Filter) ComputeCounts(ctx context.Context, rel forge.RelationService, issues []forge.Issue) (map[int]int, error) {
	counts := make(map[int]int, len(issues))
	for _, i := range issues {
		deps, err := rel.BlockedBy(ctx, i.Number)
		if err != nil {
			return nil, err
		}
		openCount := 0
		for _, d := range deps {
			if d.State == forge.StateOpen {
				openCount++
			}
		}
		counts[i.Number] = openCount
	}
	return counts, nil
}

// ShouldSkip reports whether an issue with openCount open blockers should be
// excluded from results given the configured filter flags.
func (f *Filter) ShouldSkip(openCount int) bool {
	if f.Unblocked && openCount > 0 {
		return true
	}
	if f.Blocked && openCount == 0 {
		return true
	}
	return false
}

// BlockedValue returns the string value for the blocked column.
// Returns "none" when there are no open blockers, or the decimal count otherwise.
func (f *Filter) BlockedValue(openCount int) string {
	if openCount == 0 {
		return "none"
	}
	return strconv.Itoa(openCount)
}

// AdjustTotal computes the correct total count for the filtered result set.
// When client-side filtering is active and the server-reported total exceeds
// the pre-filter issue count, the total is capped at the pre-filter count
// so the "N of M total" aggregate remains honest.
//
// meta may be nil; unfilteredCount is len(issues) before filtering;
// count is len(rows) after filtering.
func (f *Filter) AdjustTotal(meta *forge.ListMeta, unfilteredCount, count int) int {
	total := count
	if meta != nil {
		total = meta.Total
	}
	if f.NeedsBlocking() && total > unfilteredCount {
		total = unfilteredCount
	}
	return total
}
