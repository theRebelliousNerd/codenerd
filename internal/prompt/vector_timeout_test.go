package prompt

import (
	"testing"
	"time"
)

// The configured vector-search bound reaches the selector through WithConfig
// and SetConfig; a config without one keeps the bound rather than resetting it
// to a second default.
func TestVectorSearchTimeoutReachesTheSelector(t *testing.T) {
	timeout := func(c *JITPromptCompiler) time.Duration { return c.selector.vectorConfigSnapshot().timeout }

	compiler, err := NewJITPromptCompiler()
	if err != nil {
		t.Fatal(err)
	}
	defer compiler.Close()
	if got := timeout(compiler); got != defaultVectorSearchTimeout || got <= 0 {
		t.Fatalf("default bound = %v", got)
	}

	cfg := DefaultCompilerConfig()
	cfg.VectorSearchTimeout = 3 * time.Second
	if err := WithConfig(cfg)(compiler); err != nil {
		t.Fatal(err)
	}
	if got := timeout(compiler); got != 3*time.Second {
		t.Fatalf("WithConfig bound = %v, want 3s", got)
	}

	compiler.SetConfig(CompilerConfig{DefaultTokenBudget: 1000})
	if got := timeout(compiler); got != 3*time.Second {
		t.Fatalf("a config without a timeout reset the bound to %v", got)
	}
}
