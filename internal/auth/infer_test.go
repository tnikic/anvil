package auth_test

import (
	"testing"

	"github.com/tnikic/anvil/internal/auth"
)

func TestInferForgeType(t *testing.T) {
	tests := []struct {
		host string
		want string
	}{
		{"github.com", "github"},
		{"GitHub.com", "github"}, // case insensitive
		{"gitlab.com", "gitlab"},
		{"GITLAB.COM", "gitlab"},
		{"codeberg.org", "forgejo"},
		// Self-hosted heuristics
		{"git.mycorp.com", "unknown"},
		{"github.mycorp.com", "github"},
		{"gitlab.mycorp.com", "gitlab"},
		{"gitea.mycorp.com", "forgejo"},
		{"forgejo.mycorp.com", "forgejo"},
		{"unknown-forge.example.com", "unknown"},
	}
	for _, tt := range tests {
		got := auth.InferForgeType(tt.host)
		if got != tt.want {
			t.Errorf("InferForgeType(%q) = %q, want %q", tt.host, got, tt.want)
		}
	}
}
