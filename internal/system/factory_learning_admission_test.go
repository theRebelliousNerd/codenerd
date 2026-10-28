package system

import (
	"context"
	"testing"

	"codenerd/internal/config"
	"codenerd/internal/prompt"
)

// Boot's learning loop installs the generated-atom admission check on the
// prompt evolver it builds, bounded by the configured jit.atom_max_chars.
func TestInitLearningLoop_InstallsAtomAdmission(t *testing.T) {
	compiler, err := prompt.NewJITPromptCompiler()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = compiler.Close() })
	cfg := config.DefaultUserConfig()
	cfg.JIT = &config.JITConfig{AtomMaxChars: 1200}
	bctx := &bootContext{ctx: context.Background(), workspace: t.TempDir(), appCfg: cfg, jitCompiler: compiler, llmClient: &fakeMeteredLLMClient{}}
	if err := initLearningLoop(bctx); err != nil {
		t.Fatalf("initLearningLoop: %v", err)
	}
	if bctx.promptEvolver == nil {
		t.Fatal("no prompt evolver built")
	}
	t.Cleanup(func() { _ = bctx.promptEvolver.Close() })
	maxChars, judged := bctx.promptEvolver.AtomAdmissionBound()
	if maxChars != 1200 {
		t.Fatalf("admission bound %d, want the configured 1200", maxChars)
	}
	if judged {
		t.Fatal("no typesafe section is configured, yet the check claims a Jev reader")
	}
}
