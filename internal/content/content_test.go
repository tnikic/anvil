package content

import (
	"strings"
	"testing"
)

func TestDescriptionNotEmpty(t *testing.T) {
	if Description == "" {
		t.Error("Description should not be empty")
	}
}

func TestCommandsNotEmpty(t *testing.T) {
	if len(Commands) == 0 {
		t.Error("Commands should not be empty")
	}

	seen := make(map[string]bool)
	for _, c := range Commands {
		if c.Name == "" {
			t.Errorf("CommandDef.Name should not be empty")
		}
		if c.Short == "" {
			t.Errorf("CommandDef.Short should not be empty for %q", c.Name)
		}
		if seen[c.Name] {
			t.Errorf("duplicate command name: %q", c.Name)
		}
		seen[c.Name] = true
	}
}

func TestGlobalTipsNotEmpty(t *testing.T) {
	if len(GlobalTips) == 0 {
		t.Error("GlobalTips should not be empty")
	}

	for i, tip := range GlobalTips {
		if strings.TrimSpace(tip) == "" {
			t.Errorf("GlobalTips[%d] should not be empty", i)
		}
	}
}

func TestDashboardTips(t *testing.T) {
	tests := []struct {
		name       string
		issueTotal int
		prTotal    int
		wantIssue  string
		wantPR     string
	}{
		{
			name:       "both exceed limit",
			issueTotal: 47,
			prTotal:    10,
			wantIssue:  "Run `anvil issue list` for all 47 open issues",
			wantPR:     "Run `anvil pr list` for all 10 open PRs",
		},
		{
			name:       "both under limit",
			issueTotal: 2,
			prTotal:    1,
			wantIssue:  "Run `anvil issue list` to see open issues",
			wantPR:     "Run `anvil pr list` to see open PRs",
		},
		{
			name:       "zero items",
			issueTotal: 0,
			prTotal:    0,
			wantIssue:  "Run `anvil issue list` to see open issues",
			wantPR:     "Run `anvil pr list` to see open PRs",
		},
		{
			name:       "issue exceeds, pr under",
			issueTotal: 10,
			prTotal:    2,
			wantIssue:  "Run `anvil issue list` for all 10 open issues",
			wantPR:     "Run `anvil pr list` to see open PRs",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tips := DashboardTips(tt.issueTotal, tt.prTotal)
			if len(tips) != 2 {
				t.Fatalf("expected 2 tips, got %d", len(tips))
			}
			if tips[0] != tt.wantIssue {
				t.Errorf("issue tip = %q, want %q", tips[0], tt.wantIssue)
			}
			if tips[1] != tt.wantPR {
				t.Errorf("pr tip = %q, want %q", tips[1], tt.wantPR)
			}
		})
	}
}

func TestCommandNamesAreValid(t *testing.T) {
	validNames := map[string]bool{
		"issue":  true,
		"pr":     true,
		"label":  true,
		"auth":   true,
		"skills": true,
	}

	for _, c := range Commands {
		if !validNames[c.Name] {
			t.Errorf("unexpected command name: %q", c.Name)
		}
	}
}
