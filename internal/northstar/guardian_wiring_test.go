package northstar

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// =============================================================================
// THRESHOLD ORDERING
// =============================================================================

func TestNormalizeGuardianConfig_WhenThresholdsOutOfOrder_ShouldRepairOrdering(t *testing.T) {
	got := NormalizeGuardianConfig(GuardianConfig{
		WarningThreshold: 0.3,
		FailureThreshold: 0.7,
		BlockThreshold:   0.5,
	})
	if !(got.BlockThreshold <= got.FailureThreshold && got.FailureThreshold <= got.WarningThreshold) {
		t.Fatalf("thresholds still unordered: block=%.2f failure=%.2f warning=%.2f",
			got.BlockThreshold, got.FailureThreshold, got.WarningThreshold)
	}
}

func TestNormalizeGuardianConfig_WhenThresholdOutOfRange_ShouldFallBackToDefault(t *testing.T) {
	got := NormalizeGuardianConfig(GuardianConfig{
		WarningThreshold: 4.2,
		FailureThreshold: -1,
		BlockThreshold:   0.3,
	})
	if got.WarningThreshold != 0.7 || got.FailureThreshold != 0.5 || got.BlockThreshold != 0.3 {
		t.Fatalf("out-of-range thresholds not repaired: %+v", got)
	}
}

func TestNewGuardian_WhenThresholdsInverted_ShouldNotClassifyEverythingAsPassed(t *testing.T) {
	store, _ := newBridgeStore(t)
	g := NewGuardian(store, GuardianConfig{
		WarningThreshold: 0.3, // inverted on purpose
		FailureThreshold: 0.5,
		BlockThreshold:   0.7,
	})
	if got := g.classifyScore(0.4); got == AlignmentPassed {
		t.Fatal("score 0.4 classified as passed under an inverted threshold set; the guardian would wave through work it was configured to fail")
	}
	if got := g.classifyScore(0.1); got != AlignmentBlocked {
		t.Errorf("classifyScore(0.1) = %q, want blocked", got)
	}
}

func TestNormalizeGuardianConfig_WhenIntervalNonPositive_ShouldUseDefault(t *testing.T) {
	if got := NormalizeGuardianConfig(GuardianConfig{PeriodicCheckInterval: 0}); got.PeriodicCheckInterval != 5 {
		t.Errorf("PeriodicCheckInterval = %d, want 5 (a zero interval makes every task due for a check)", got.PeriodicCheckInterval)
	}
}

// =============================================================================
// SINGLETON GUARDIAN
// =============================================================================

func TestAcquireGuardian_WhenCalledTwiceForSameDir_ShouldReturnSameInstance(t *testing.T) {
	t.Cleanup(ResetGuardianRegistry)
	nerdDir := t.TempDir()

	first, err := AcquireGuardian(nerdDir, DefaultGuardianConfig())
	if err != nil {
		t.Fatalf("first acquire: %v", err)
	}
	second, err := AcquireGuardian(nerdDir, DefaultGuardianConfig())
	if err != nil {
		t.Fatalf("second acquire: %v", err)
	}
	if first != second {
		t.Fatal("two Guardians for one .nerd dir: each caches its own state and opens its own SQLite handle")
	}
	if got := GuardianRefCount(nerdDir); got != 2 {
		t.Errorf("ref count = %d, want 2", got)
	}
}

func TestAcquireGuardian_WhenDifferentDirs_ShouldReturnDistinctInstances(t *testing.T) {
	t.Cleanup(ResetGuardianRegistry)
	a, err := AcquireGuardian(t.TempDir(), DefaultGuardianConfig())
	if err != nil {
		t.Fatalf("acquire a: %v", err)
	}
	b, err := AcquireGuardian(t.TempDir(), DefaultGuardianConfig())
	if err != nil {
		t.Fatalf("acquire b: %v", err)
	}
	if a == b {
		t.Fatal("two different workspaces shared one Guardian")
	}
}

func TestReleaseGuardian_WhenLastReferenceReleased_ShouldCloseStore(t *testing.T) {
	t.Cleanup(ResetGuardianRegistry)
	nerdDir := t.TempDir()

	g, err := AcquireGuardian(nerdDir, DefaultGuardianConfig())
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	if _, err := AcquireGuardian(nerdDir, DefaultGuardianConfig()); err != nil {
		t.Fatalf("second acquire: %v", err)
	}

	if err := ReleaseGuardian(g); err != nil {
		t.Fatalf("first release: %v", err)
	}
	if got := GuardianRefCount(nerdDir); got != 1 {
		t.Fatalf("ref count after one release = %d, want 1", got)
	}
	if _, err := g.store.GetState(); err != nil {
		t.Fatalf("store closed while a reference is still outstanding: %v", err)
	}

	if err := ReleaseGuardian(g); err != nil {
		t.Fatalf("final release: %v", err)
	}
	if got := GuardianRefCount(nerdDir); got != 0 {
		t.Errorf("ref count after final release = %d, want 0", got)
	}
	if _, err := g.store.GetState(); err == nil {
		t.Error("store still usable after the last reference was released; the handle leaked")
	}
}

func TestReleaseGuardian_WhenGuardianNotFromRegistry_ShouldBeNoop(t *testing.T) {
	store, _ := newBridgeStore(t)
	if err := ReleaseGuardian(NewGuardian(store, DefaultGuardianConfig())); err != nil {
		t.Fatalf("releasing a directly constructed guardian returned %v, want nil", err)
	}
	if err := ReleaseGuardian(nil); err != nil {
		t.Fatalf("releasing nil returned %v, want nil", err)
	}
}

// =============================================================================
// ALIGNMENT MODEL SELECTION
// =============================================================================

type recordingModelClient struct {
	model  string
	system string
	plain  int
}

func (c *recordingModelClient) CompleteWithSystem(ctx context.Context, system, user string) (string, error) {
	c.plain++
	c.system = system
	return "SCORE: 0.9\nRESULT: passed\nEXPLANATION: ok\nSUGGESTIONS: none", nil
}

func (c *recordingModelClient) CompleteWithSystemModel(ctx context.Context, model, system, user string) (string, error) {
	c.model = model
	c.system = system
	return "SCORE: 0.9\nRESULT: passed\nEXPLANATION: ok\nSUGGESTIONS: none", nil
}

type plainClient struct{ calls int }

func (c *plainClient) CompleteWithSystem(ctx context.Context, system, user string) (string, error) {
	c.calls++
	return "SCORE: 0.9\nRESULT: passed\nEXPLANATION: ok\nSUGGESTIONS: none", nil
}

func guardianWithVision(t *testing.T, config GuardianConfig) *Guardian {
	t.Helper()
	store, _ := newBridgeStore(t)
	if err := store.SaveVision(sampleWizardVision()); err != nil {
		t.Fatalf("SaveVision: %v", err)
	}
	g := NewGuardian(store, config)
	if err := g.Initialize(); err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	return g
}

func TestCheckAlignment_WhenAlignmentModelSet_ShouldRouteThroughModelSelectingClient(t *testing.T) {
	config := DefaultGuardianConfig()
	config.AlignmentModel = "claude-opus-4"
	g := guardianWithVision(t, config)

	client := &recordingModelClient{}
	g.SetLLMClient(client)

	if _, err := g.CheckAlignment(context.Background(), TriggerManual, "subject", ""); err != nil {
		t.Fatalf("CheckAlignment: %v", err)
	}
	if client.model != "claude-opus-4" {
		t.Errorf("model = %q, want the configured AlignmentModel", client.model)
	}
	if client.plain != 0 {
		t.Error("fell back to the default-model path even though the client can select a model")
	}
}

func TestCheckAlignment_WhenAlignmentModelUnset_ShouldUseDefaultCompletion(t *testing.T) {
	g := guardianWithVision(t, DefaultGuardianConfig())
	client := &recordingModelClient{}
	g.SetLLMClient(client)

	if _, err := g.CheckAlignment(context.Background(), TriggerManual, "subject", ""); err != nil {
		t.Fatalf("CheckAlignment: %v", err)
	}
	if client.model != "" {
		t.Errorf("model = %q, want no model override when AlignmentModel is empty", client.model)
	}
	if client.plain != 1 {
		t.Errorf("plain completions = %d, want 1", client.plain)
	}
}

func TestCheckAlignment_WhenClientCannotSelectModel_ShouldStillComplete(t *testing.T) {
	config := DefaultGuardianConfig()
	config.AlignmentModel = "some-model"
	g := guardianWithVision(t, config)

	client := &plainClient{}
	g.SetLLMClient(client)

	check, err := g.CheckAlignment(context.Background(), TriggerManual, "subject", "")
	if err != nil {
		t.Fatalf("CheckAlignment: %v", err)
	}
	if client.calls != 1 {
		t.Errorf("calls = %d, want the check to proceed on the client's default model", client.calls)
	}
	if check.Result != AlignmentPassed {
		t.Errorf("result = %q, want passed", check.Result)
	}
}

// =============================================================================
// PROMPT ATOMIZATION
// =============================================================================

func TestAlignmentAtom_ShouldResolveEveryGuardianAtom(t *testing.T) {
	for _, id := range AlignmentAtomIDs() {
		if strings.TrimSpace(AlignmentAtom(id)) == "" {
			t.Errorf("atom %q resolved to empty content", id)
		}
	}
}

func TestBuildAlignmentSystemPrompt_ShouldComposeFromAtoms(t *testing.T) {
	g := guardianWithVision(t, DefaultGuardianConfig())
	prompt := g.buildAlignmentSystemPrompt(g.GetVision(), "internal/northstar/guardian.go")

	for _, id := range []string{atomGuardianRole, atomGuardianTask, atomGuardianOutputContract} {
		if !strings.Contains(prompt, AlignmentAtom(id)) {
			t.Errorf("system prompt does not contain the content of atom %q", id)
		}
	}
	if !strings.Contains(prompt, "Make logic the executive") {
		t.Error("system prompt lost the project mission")
	}
	// The response contract is what parseAlignmentResponse depends on.
	if !strings.Contains(prompt, "SCORE:") || !strings.Contains(prompt, "RESULT:") {
		t.Error("system prompt lost the machine-readable response contract")
	}
}

func TestBuildAlignmentUserPrompt_ShouldEndWithAtomInstruction(t *testing.T) {
	g := guardianWithVision(t, DefaultGuardianConfig())
	prompt := g.buildAlignmentUserPrompt("subject", "context")
	if !strings.Contains(prompt, AlignmentAtom(atomGuardianUserInstruction)) {
		t.Error("user prompt does not use the corpus instruction atom")
	}
}

// =============================================================================
// BOOT WIRE PARITY
// =============================================================================

// The two chat boot paths must wire the Guardian into the kernel identically.
// session_boot.go was missing SetParentKernel entirely, so on the primary boot
// path northstar_defined() was never asserted and every
// injectable_context(/northstar_*) rule produced nothing. A unit test cannot
// boot the TUI, so this asserts the wiring at the source level -- which is
// exactly the invariant that broke.
func TestChatBootPaths_ShouldWireGuardianKernelIdentically(t *testing.T) {
	repoRoot := findRepoRoot(t)
	paths := []string{
		filepath.Join(repoRoot, "cmd", "nerd", "chat", "session_boot.go"),
		filepath.Join(repoRoot, "cmd", "nerd", "chat", "session_shared_boot.go"),
	}

	// The guardian wiring block: from AcquireGuardian/NewGuardian to Initialize.
	block := regexp.MustCompile(`(?s)(AcquireGuardian|northstar\.NewGuardian)\(.*?guardian\.Initialize\(\)`)

	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		match := block.FindString(string(data))
		if match == "" {
			t.Fatalf("%s: could not locate the northstar guardian wiring block", filepath.Base(path))
		}
		if !strings.Contains(match, "guardian.SetLLMClient(") {
			t.Errorf("%s: guardian wiring never sets an LLM client", filepath.Base(path))
		}
		if !strings.Contains(match, "guardian.SetParentKernel(") {
			t.Errorf("%s: guardian wiring never calls SetParentKernel, so this boot path projects no northstar_* facts into the kernel", filepath.Base(path))
		}
		if !strings.Contains(match, "AcquireGuardian(") {
			t.Errorf("%s: boot builds its own Guardian instead of taking the shared one, reintroducing dual DB handles", filepath.Base(path))
		}
	}
}

func findRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for i := 0; i < 6; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		dir = filepath.Dir(dir)
	}
	t.Fatal("could not locate repo root from the test working directory")
	return ""
}
