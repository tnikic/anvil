package blocking_test

import (
	"context"
	"testing"

	"github.com/tnikic/anvil/internal/commands/blocking"
	"github.com/tnikic/anvil/internal/forge"
	"github.com/tnikic/anvil/internal/forge/forgetest"
)

func TestNeedsBlocking(t *testing.T) {
	tests := []struct {
		name string
		f    blocking.Filter
		want bool
	}{
		{"none", blocking.Filter{}, false},
		{"unblocked", blocking.Filter{Unblocked: true}, true},
		{"blocked", blocking.Filter{Blocked: true}, true},
		{"showBlocked", blocking.Filter{ShowBlocked: true}, true},
		{"all three", blocking.Filter{Unblocked: true, Blocked: true, ShowBlocked: true}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.f.NeedsBlocking(); got != tt.want {
				t.Errorf("NeedsBlocking() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidate_MutualExclusivity(t *testing.T) {
	f := blocking.Filter{Unblocked: true, Blocked: true}
	err := f.Validate()
	if err == nil {
		t.Fatal("expected error when both --unblocked and --blocked are set")
	}
	msg := err.Error()
	if !contains(msg, "mutually exclusive") {
		t.Errorf("error should mention 'mutually exclusive', got: %s", msg)
	}
}

func TestValidate_Pass(t *testing.T) {
	f := blocking.Filter{Unblocked: true}
	if err := f.Validate(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestComputeCounts(t *testing.T) {
	rel := &forgetest.FakeRelationService{}
	rel.BlockedByItems = []forge.IssueDependency{
		{Number: 1, Title: "Open blocker", State: forge.StateOpen, Direction: forge.DirBlockedBy},
		{Number: 2, Title: "Closed blocker", State: forge.StateClosed, Direction: forge.DirBlockedBy},
		{Number: 3, Title: "Another open", State: forge.StateOpen, Direction: forge.DirBlockedBy},
	}

	f := blocking.Filter{ShowBlocked: true}
	issues := []forge.Issue{
		{Number: 42},
		{Number: 99},
	}

	counts, err := f.ComputeCounts(context.Background(), rel, issues)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Both issues get the same 3 BlockedByItems, but only 2 are open.
	if counts[42] != 2 {
		t.Errorf("counts[42] = %d, want 2 (two open blockers)", counts[42])
	}
	if counts[99] != 2 {
		t.Errorf("counts[99] = %d, want 2", counts[99])
	}
	if rel.LastBlockedByNumber != 99 {
		t.Errorf("LastBlockedByNumber = %d, want 99 (last issue queried)", rel.LastBlockedByNumber)
	}
}

func TestComputeCounts_Error(t *testing.T) {
	rel := &forgetest.FakeRelationService{}
	rel.BlockedByFn = func(ctx context.Context, number int) ([]forge.IssueDependency, error) {
		return nil, forge.NewBaseError("failed to fetch blockers", "Retry later")
	}

	f := blocking.Filter{Unblocked: true}
	issues := []forge.Issue{{Number: 1}}

	_, err := f.ComputeCounts(context.Background(), rel, issues)
	if err == nil {
		t.Fatal("expected error when BlockedBy fails")
	}
	if !contains(err.Error(), "failed to fetch blockers") {
		t.Errorf("error should propagate BlockedBy error, got: %s", err.Error())
	}
}

func TestShouldSkip_Unblocked(t *testing.T) {
	f := blocking.Filter{Unblocked: true}

	if f.ShouldSkip(0) {
		t.Error("ShouldSkip(0) = true, want false (no open blockers → keep)")
	}
	if !f.ShouldSkip(1) {
		t.Error("ShouldSkip(1) = false, want true (has open blocker → skip)")
	}
	if !f.ShouldSkip(5) {
		t.Error("ShouldSkip(5) = false, want true (has open blockers → skip)")
	}
}

func TestShouldSkip_Blocked(t *testing.T) {
	f := blocking.Filter{Blocked: true}

	if !f.ShouldSkip(0) {
		t.Error("ShouldSkip(0) = false, want true (no open blockers → skip)")
	}
	if f.ShouldSkip(1) {
		t.Error("ShouldSkip(1) = true, want false (has open blocker → keep)")
	}
	if f.ShouldSkip(3) {
		t.Error("ShouldSkip(3) = true, want false (has open blockers → keep)")
	}
}

func TestShouldSkip_None(t *testing.T) {
	f := blocking.Filter{ShowBlocked: true} // no filter flags

	if f.ShouldSkip(0) {
		t.Error("ShouldSkip(0) = true, want false (no filter → never skip)")
	}
	if f.ShouldSkip(5) {
		t.Error("ShouldSkip(5) = true, want false (no filter → never skip)")
	}
}

func TestBlockedValue(t *testing.T) {
	f := blocking.Filter{}

	tests := []struct {
		openCount int
		want      string
	}{
		{0, "none"},
		{1, "1"},
		{2, "2"},
		{10, "10"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := f.BlockedValue(tt.openCount); got != tt.want {
				t.Errorf("BlockedValue(%d) = %q, want %q", tt.openCount, got, tt.want)
			}
		})
	}
}

func TestAdjustTotal(t *testing.T) {
	tests := []struct {
		name            string
		f               blocking.Filter
		meta            *forge.ListMeta
		unfilteredCount int
		count           int
		want            int
	}{
		{
			name:            "no meta, no filter",
			f:               blocking.Filter{},
			meta:            nil,
			unfilteredCount: 10,
			count:           10,
			want:            10,
		},
		{
			name:            "with meta, no filter",
			f:               blocking.Filter{},
			meta:            &forge.ListMeta{Total: 100},
			unfilteredCount: 10,
			count:           10,
			want:            100, // server total preserved
		},
		{
			name:            "filtered, meta total capped",
			f:               blocking.Filter{Unblocked: true},
			meta:            &forge.ListMeta{Total: 100},
			unfilteredCount: 10,
			count:           7,
			want:            10, // capped at unfilteredCount
		},
		{
			name:            "filtered, meta total within range",
			f:               blocking.Filter{Unblocked: true},
			meta:            &forge.ListMeta{Total: 5},
			unfilteredCount: 10,
			count:           7,
			want:            5, // meta total is smaller, not capped
		},
		{
			name:            "filtered, no meta",
			f:               blocking.Filter{Blocked: true},
			meta:            nil,
			unfilteredCount: 5,
			count:           2,
			want:            2, // fallback to count
		},
		{
			name:            "showBlocked only, meta total not capped",
			f:               blocking.Filter{ShowBlocked: true},
			meta:            &forge.ListMeta{Total: 100},
			unfilteredCount: 10,
			count:           10,
			want:            10, // ShowBlocked caps (conservative), but count == unfiltered
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.f.AdjustTotal(tt.meta, tt.unfilteredCount, tt.count); got != tt.want {
				t.Errorf("AdjustTotal() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestFilter_Integration(t *testing.T) {
	// Simulate the full pipeline: compute → filter → populate.
	rel := &forgetest.FakeRelationService{}
	rel.BlockedByFn = func(ctx context.Context, number int) ([]forge.IssueDependency, error) {
		switch number {
		case 1:
			return nil, nil // no blockers
		case 2:
			return []forge.IssueDependency{
				{Number: 10, State: forge.StateOpen, Direction: forge.DirBlockedBy},
			}, nil // 1 open blocker
		case 3:
			return []forge.IssueDependency{
				{Number: 11, State: forge.StateClosed, Direction: forge.DirBlockedBy},
			}, nil // only closed blockers → 0 open
		default:
			return nil, nil
		}
	}

	f := blocking.Filter{Unblocked: true}
	issues := []forge.Issue{
		{Number: 1},
		{Number: 2},
		{Number: 3},
	}

	counts, err := f.ComputeCounts(context.Background(), rel, issues)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	kept := 0
	for _, i := range issues {
		if f.ShouldSkip(counts[i.Number]) {
			continue
		}
		kept++
	}

	if kept != 2 {
		t.Errorf("kept = %d, want 2 (issues 1 and 3 are unblocked)", kept)
	}
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
