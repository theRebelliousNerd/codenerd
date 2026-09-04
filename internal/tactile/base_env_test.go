package tactile

import (
	"slices"
	"strings"
	"testing"
)

func TestBuildEnvironment_AppendsBaseEnvironment(t *testing.T) {
	t.Parallel()
	config := DefaultExecutorConfig()
	config.AllowedEnvironment = []string{"PATH"}
	config.BaseEnvironment = []string{"CGO_CFLAGS=-Ix", "GOFLAGS=-tags=sqlite_vec"}
	executor := NewDirectExecutorWithConfig(config)

	env := executor.buildEnvironment([]string{"GOFLAGS=-tags=other"})

	if !slices.Contains(env, "CGO_CFLAGS=-Ix") {
		t.Errorf("expected CGO_CFLAGS=-Ix in environment, got %v", env)
	}
	last := ""
	for _, e := range env {
		if v, ok := strings.CutPrefix(e, "GOFLAGS="); ok {
			last = v
		}
	}
	if last != "-tags=other" {
		t.Errorf("expected last GOFLAGS entry to be %q (command env wins), got %q (env=%v)", "-tags=other", last, env)
	}
}
