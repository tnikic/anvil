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
