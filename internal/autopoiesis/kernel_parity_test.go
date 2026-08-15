package autopoiesis

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"codenerd/internal/types"
)

// Post-boot parity between the runtime tool registry and the kernel's
// tool_registered facts (TODO P1). Two failure modes hide here, both silent:
//
//   - a tool restored from .nerd/tools/.compiled that never reaches the kernel
//     is executable but invisible to every logic-driven routing decision, so
//     the kernel keeps deriving missing_tool_for for a capability it already
//     has, and Ouroboros regenerates a tool that exists;
//   - a tool_registered fact with no binary behind it makes the kernel plan
//     around a capability that fails at call time.
//
// The check runs automatically at the end of syncExistingToolsToKernel; these
// tests pin its verdicts.

func runtimeToolFixture(name string) *RuntimeTool {
	return &RuntimeTool{
		Name:         name,
		Description:  "fixture " + name,
		BinaryPath:   filepath.Join(os.TempDir(), name),
		Hash:         "hash_" + name,
		RegisteredAt: time.Now(),
	}
}

func kernelWithRegisteredTools(names ...string) *MockKernelInterface {
	kernel := &MockKernelInterface{}
	kernel.QueryPredicateFunc = func(predicate string) ([]types.KernelFact, error) {
		if predicate != "tool_registered" {
			return nil, nil
		}
		facts := make([]types.KernelFact, 0, len(names))
		for _, n := range names {
			facts = append(facts, types.KernelFact{Predicate: "tool_registered", Args: []any{n, int64(1)}})
		}
		return facts, nil
	}
	return kernel
}

func TestVerifyKernelToolParity_WhenRegistryAndKernelAgree_ShouldReportParity(t *testing.T) {
	orch, _, _ := createTestOrchestrator(t)
	mock := replaceOuroborosWithMock(orch)
	mock.ListRuntimeToolsFunc = func() []*RuntimeTool {
		return []*RuntimeTool{runtimeToolFixture("alpha"), runtimeToolFixture("beta")}
	}
	orch.SetKernel(kernelWithRegisteredTools("alpha", "beta"))

	report, err := orch.VerifyKernelToolParity()
	if err != nil {
		t.Fatalf("parity check failed to run: %v", err)
	}
	if !report.InParity() {
		t.Fatalf("expected parity, got %s", report.Describe())
	}
	if report.RegistryCount != 2 || report.KernelCount != 2 {
		t.Errorf("counts = registry %d / kernel %d, want 2/2", report.RegistryCount, report.KernelCount)
	}
}

func TestVerifyKernelToolParity_WhenToolMissingFromKernel_ShouldReportIt(t *testing.T) {
	orch, _, _ := createTestOrchestrator(t)
	mock := replaceOuroborosWithMock(orch)
	mock.ListRuntimeToolsFunc = func() []*RuntimeTool {
		return []*RuntimeTool{runtimeToolFixture("alpha"), runtimeToolFixture("orphan")}
	}
	orch.SetKernel(kernelWithRegisteredTools("alpha"))

	report, err := orch.VerifyKernelToolParity()
	if err != nil {
		t.Fatalf("parity check failed to run: %v", err)
	}
	if report.InParity() {
		t.Fatal("expected a parity break: a registered tool has no tool_registered fact")
	}
	if len(report.MissingInKernel) != 1 || report.MissingInKernel[0] != "orphan" {
		t.Errorf("MissingInKernel = %v, want [orphan]", report.MissingInKernel)
	}
	if len(report.UnknownInKernel) != 0 {
		t.Errorf("UnknownInKernel = %v, want empty", report.UnknownInKernel)
	}
	if !strings.Contains(report.Describe(), "orphan") {
		t.Errorf("Describe() should name the offending tool, got %q", report.Describe())
	}
}

func TestVerifyKernelToolParity_WhenKernelKnowsUnbuiltTool_ShouldReportIt(t *testing.T) {
	orch, _, _ := createTestOrchestrator(t)
	mock := replaceOuroborosWithMock(orch)
	mock.ListRuntimeToolsFunc = func() []*RuntimeTool {
		return []*RuntimeTool{runtimeToolFixture("alpha")}
	}
	// The kernel reports a name constant, the form Mangle round-trips
	// identifier-like strings into. The comparison has to see through it or
	// every single tool would look like a mismatch.
	orch.SetKernel(kernelWithRegisteredTools("/alpha", "ghost"))

	report, err := orch.VerifyKernelToolParity()
	if err != nil {
		t.Fatalf("parity check failed to run: %v", err)
	}
	if len(report.MissingInKernel) != 0 {
		t.Errorf("MissingInKernel = %v: the /-prefixed name constant should match the registry entry", report.MissingInKernel)
	}
	if len(report.UnknownInKernel) != 1 || report.UnknownInKernel[0] != "ghost" {
		t.Errorf("UnknownInKernel = %v, want [ghost]", report.UnknownInKernel)
	}
}

func TestVerifyKernelToolParity_WhenNoKernelAttached_ShouldReturnError(t *testing.T) {
	mockLLM := &MockLLMClient{}
	orch := NewOrchestrator(mockLLM, Config{
		ToolsDir:      t.TempDir(),
		AgentsDir:     t.TempDir(),
		WorkspaceRoot: t.TempDir(),
	})

	if _, err := orch.VerifyKernelToolParity(); err == nil {
		t.Fatal("expected an error when no kernel is attached; a silent 'in parity' would be a false all-clear")
	}
}

// SetKernel runs the sync and then the parity check. With a mock kernel that
// records asserted facts but answers QueryPredicate from the same set, boot
// must come out in parity — this is the post-boot invariant the TODO asks for.
func TestSetKernel_WhenRestoredToolsAreSynced_ShouldEndInParity(t *testing.T) {
	mockLLM := &MockLLMClient{}
	orch := NewOrchestrator(mockLLM, Config{
		ToolsDir:      t.TempDir(),
		AgentsDir:     t.TempDir(),
		WorkspaceRoot: t.TempDir(),
	})

	mock := replaceOuroborosWithMock(orch)
	restored := []*RuntimeTool{runtimeToolFixture("restored_one"), runtimeToolFixture("restored_two")}
	mock.ListRuntimeToolsFunc = func() []*RuntimeTool { return restored }

	kernel := &MockKernelInterface{}
	kernel.QueryPredicateFunc = func(predicate string) ([]types.KernelFact, error) {
		var out []types.KernelFact
		for _, f := range kernel.AssertedFacts {
			if f.Predicate == predicate {
				out = append(out, f)
			}
		}
		return out, nil
	}

	orch.SetKernel(kernel)

	report, err := orch.VerifyKernelToolParity()
	if err != nil {
		t.Fatalf("parity check failed to run: %v", err)
	}
	if !report.InParity() {
		t.Fatalf("boot sync left the registry and the kernel out of parity: %s", report.Describe())
	}
	if report.RegistryCount != len(restored) {
		t.Errorf("registry count = %d, want %d", report.RegistryCount, len(restored))
	}
}
