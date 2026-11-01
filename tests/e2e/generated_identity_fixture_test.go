//go:build integration

package e2e_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"testing"
	"time"

	"codenerd/internal/autopoiesis"
	"codenerd/internal/system"
	"codenerd/internal/types"
)

const productionPipelineCompanion = productionPipelineChild + "_companion"

// Compile a separate owned artifact and persist its actual immutable identity
// before boot. Neither a registry alias nor a synthetic receipt creates overlap.
func compilePipelineCompanion(t *testing.T, root, name string) types.GeneratedToolIdentity {
	t.Helper()
	if name == productionPipelineChild {
		t.Fatal("companion must have an independent operation identity")
	}
	if err := os.WriteFile(filepath.Join(root, ".nerd", "tools", name+".go"), []byte(productionPipelineSource), 0600); err != nil {
		t.Fatal(err)
	}
	cfg := autopoiesis.DefaultOuroborosConfig(root)
	cfg.TargetOS, cfg.TargetArch = runtime.GOOS, runtime.GOARCH
	cfg.CompileTimeout = 90 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	compiled, err := autopoiesis.NewToolCompiler(cfg).Compile(ctx, &autopoiesis.GeneratedTool{
		Name: name, Description: "Independent literal effect and protocol fault fixture", Code: productionPipelineSource,
	})
	if err != nil || compiled == nil || !compiled.Success {
		t.Fatalf("compile independent generated fixture: %+v, %v", compiled, err)
	}
	identity := types.GeneratedToolIdentity{Name: name, BinaryPath: compiled.OutputPath,
		BinaryHash: compiled.Hash, Protocol: types.GeneratedStdinV1, Workspace: root}
	encoded, err := json.Marshal(identity)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(identity.BinaryPath+".identity.json", encoded, 0600); err != nil {
		t.Fatal(err)
	}
	return identity
}

func verifyPipelineCompanion(t *testing.T, cortex *system.Cortex, expected types.GeneratedToolIdentity) {
	t.Helper()
	actual, err := cortex.VirtualStore.GetToolRegistry().GeneratedToolIdentity(expected.Name)
	if err != nil || actual != expected {
		t.Fatalf("companion boot identity differs: %+v, %v", actual, err)
	}
	backend, err := cortex.Orchestrator.GeneratedToolIdentity(expected.Name)
	if err != nil || backend.BinaryPath != expected.BinaryPath || backend.BinaryHash != expected.BinaryHash || backend.Protocol != expected.Protocol {
		t.Fatalf("companion backend identity differs: %+v, %v", backend, err)
	}
}

func (f *pipelineFixture) pipelineToolNames() []string {
	names := make([]string, 0, len(f.identities))
	for name := range f.identities {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func (f *pipelineFixture) pipelineIdentity(name string) types.GeneratedToolIdentity {
	f.t.Helper()
	identity, ok := f.identities[name]
	if !ok {
		f.t.Fatalf("fixture has no restored generated identity for %q", name)
	}
	return identity
}

func (f *pipelineFixture) planForTool(name, id, content, mode, gate string) pipelinePlan {
	f.t.Helper()
	f.pipelineIdentity(name)
	plan := f.plan(id, content, mode, gate)
	plan.Call.Name = name
	f.model.mu.Lock()
	f.model.plans[id] = plan
	f.model.mu.Unlock()
	return plan
}

// Totals are a fixture observation over separately reopened product records.
// Never publish this aggregate or use it as execution/acceptance evidence.
func (f *pipelineFixture) pipelineLearningTotals(store *autopoiesis.LearningStore) *autopoiesis.ToolLearning {
	var total *autopoiesis.ToolLearning
	for _, name := range f.pipelineToolNames() {
		if actual := store.GetLearning(name); actual != nil {
			if total == nil {
				total = &autopoiesis.ToolLearning{}
			}
			total.TotalExecutions += actual.TotalExecutions
		}
	}
	return total
}

func TestGeneratedPipelineDistinctToolOverlap(t *testing.T) {
	f := newPipelineFixtureWithCompanion(t, false, productionPipelineCompanion)
	if f.identity.Name == f.pipelineIdentity(productionPipelineCompanion).Name || f.identity.BinaryPath == f.pipelineIdentity(productionPipelineCompanion).BinaryPath {
		t.Fatal("concurrent fixture reused one generated operation/artifact identity")
	}
	gate := newPipelineGate(t)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	plans := []pipelinePlan{
		f.plan("independent-a", "independent A\n", "", gate.server.URL),
		f.planForTool(productionPipelineCompanion, "independent-b", "independent B\n", "", gate.server.URL),
	}
	var completions []<-chan pipelineOutcome
	for _, plan := range plans {
		f.approve(plan)
		executor := f.cortex.SessionExecutor.CloneForTask()
		executor.SetSessionID("distinct-identity-" + plan.Call.ID)
		f.configure(executor)
		completions = append(completions, f.start(ctx, executor, plan))
	}
	admitted := map[string]bool{}
	for range plans {
		admitted[awaitPipeline(t, gate.admitted, "independent tool admission before release")] = true
	}
	if len(admitted) != 2 || !admitted[plans[0].Call.ID] || !admitted[plans[1].Call.ID] {
		t.Fatalf("independent identities did not overlap before release: %v", admitted)
	}
	// Both real child processes are blocked on the unopened gate now. No file
	// effect or learning acknowledgment may be fabricated before they finish.
	for _, plan := range plans {
		f.assertNoEffect(plan)
		if _, ok := f.learning().ExecutionAcknowledgment(f.request(plan).Key()); ok {
			t.Fatal("blocked process already had durable completed feedback")
		}
	}
	gate.open()
	for index, plan := range plans {
		requirePipelineTransportOutcomeWithWriteRefusal(t, f, plan, awaitPipeline(t, completions[index], "independent tool completion"))
		f.assertReleased(plan)
		awaitPipeline(t, gate.drained, "independent child drain")
	}
	f.assertTotals(2, 2, 2)
	for _, name := range f.pipelineToolNames() {
		learning := f.learning().GetLearning(name)
		if learning == nil || learning.TotalExecutions != 1 || learning.SuccessRate != 1 {
			t.Fatalf("independent durable tool count differs for %s: %+v", name, learning)
		}
	}
	f.assertBaseline()
}

func TestGeneratedPipelineCompanionPermission(t *testing.T) {
	for _, mode := range []string{"missing_approval", "other_tool_action"} {
		t.Run(mode, func(t *testing.T) {
			f := newPipelineFixtureWithCompanion(t, false, productionPipelineCompanion)
			plan := f.planForTool(productionPipelineCompanion, "companion-refused", "must not land\n", "", "")
			if mode == "other_tool_action" {
				wrong := plan
				wrong.Call.Name = productionPipelineChild
				// Same call/authorization ID, target and payload, wrong action and
				// immutable tool identity. Approval must not cross this boundary.
				f.approve(wrong)
			}
			assertPipelineFailed(t, f.run(plan))
			f.model.mu.Lock()
			issued := f.model.issued[plan.Call.ID]
			f.model.mu.Unlock()
			if issued != 1 {
				t.Fatalf("companion refusal did not reach the actual offered call: %d", issued)
			}
			f.assertNoEffect(plan)
			f.assertReleased(plan)
			f.assertTotals(0, 0, 0)
			f.assertBaseline()
		})
	}
}
