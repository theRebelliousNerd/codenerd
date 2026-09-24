package prompt

import (
	"context"
	"fmt"
	"slices"
	"sort"
	"strings"
	"testing"

	"codenerd/internal/core"
)

// realKernelQuerier is the production selection path for tests in this
// package: a real Mangle kernel (the shipped jit_compiler.mg and policy corpus)
// behind the KernelQuerier interface, with a per-compile clone as the
// production KernelAdapter gives (internal/system/factory_adapters.go). It is
// here, not borrowed, because internal/system imports this package.
type realKernelQuerier struct{ k *core.RealKernel }

func (q *realKernelQuerier) Query(predicate string) ([]Fact, error) {
	facts, err := q.k.Query(predicate)
	if err != nil {
		return nil, err
	}
	out := make([]Fact, len(facts))
	for i, f := range facts {
		out[i] = Fact{Predicate: f.Predicate, Args: f.Args}
	}
	return out, nil
}

func (q *realKernelQuerier) AssertBatch(facts []any) error {
	batch := make([]core.Fact, 0, len(facts))
	for _, raw := range facts {
		switch v := raw.(type) {
		case core.Fact:
			batch = append(batch, v)
		case string:
			f, err := core.ParseFactString(strings.TrimSuffix(strings.TrimSpace(v), "."))
			if err != nil {
				return err
			}
			batch = append(batch, f)
		default:
			return fmt.Errorf("unsupported fact type %T", raw)
		}
	}
	return q.k.AssertBatch(batch)
}

func (q *realKernelQuerier) Retract(predicate string) error { return q.k.Retract(predicate) }

type realKernelScope struct{ *realKernelQuerier }

func (s *realKernelScope) Close() error { return nil }

func (q *realKernelQuerier) NewCompilationScope() (KernelCompilationScope, error) {
	return &realKernelScope{&realKernelQuerier{k: q.k.Clone()}}, nil
}

// newEmbeddedCorpusCompiler compiles from the SHIPPED corpus with the real
// kernel doing selection -- the question "what does this compile serve" is
// about the corpus the product ships, not a four-atom fixture. The kernel is
// returned so a test can ask it the needs the session executor asks.
func newEmbeddedCorpusCompiler(t *testing.T) (*JITPromptCompiler, *core.RealKernel) {
	t.Helper()
	k, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel: %v", err)
	}
	embedded, err := LoadEmbeddedCorpus()
	if err != nil {
		t.Fatalf("LoadEmbeddedCorpus: %v", err)
	}
	compiler, err := NewJITPromptCompiler(
		WithKernel(&realKernelQuerier{k: k}),
		WithEmbeddedCorpus(embedded),
	)
	if err != nil {
		t.Fatalf("NewJITPromptCompiler: %v", err)
	}
	t.Cleanup(func() { _ = compiler.Close() })
	return compiler, k
}

// kernelTargetNeeds asks the kernel what the session executor asks at every
// compile boundary (session/work_steps.go targetNeeds): target_need with the
// compile's language bound, /undetected when there is none.
func kernelTargetNeeds(t *testing.T, k *core.RealKernel, language string) []string {
	t.Helper()
	if language == "" {
		language = "/undetected"
	}
	facts, err := k.Query("target_need(" + language + ", Need)")
	if err != nil {
		t.Fatalf("target_need(%s): %v", language, err)
	}
	var needs []string
	for _, f := range facts {
		if len(f.Args) == 2 {
			needs = append(needs, strings.TrimPrefix(fmt.Sprint(f.Args[1]), "/"))
		}
	}
	return needs
}

// coderTurnContext is the compilation context the session executor builds for
// a coder turn (buildCompilationContext): persona, verb, target and its
// language, the persona's tool envelope, the serving vendor, a production
// budget. needs is what the kernel derived for it (policy/jit_needs.mg).
func coderTurnContext(t *testing.T, verb, target, language string, needs []string) *CompilationContext {
	t.Helper()
	tools, err := NewDefaultConfigFactory().ResolveAllowedTools(context.Background(), verb)
	if err != nil {
		t.Fatalf("ResolveAllowedTools(%s): %v", verb, err)
	}
	cc := NewCompilationContextWithBudget(1048576)
	cc.ShardType = "/coder"
	cc.ShardID = "coder"
	cc.IntentVerb = verb
	cc.IntentTarget = target
	cc.Language = language
	cc.Provider = "/meta"
	cc.Model = "/muse_spark"
	cc.AvailableTools = tools
	cc.DerivedNeeds = needs
	return cc
}

func compiledAtoms(t *testing.T, c *JITPromptCompiler, cc *CompilationContext) (map[string]*PromptAtom, *CompilationResult) {
	t.Helper()
	result, err := c.Compile(context.Background(), cc)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	ids := make(map[string]*PromptAtom, len(result.IncludedAtoms))
	for _, a := range result.IncludedAtoms {
		ids[a.ID] = a
	}
	return ids, result
}

// gatedOn lists the shipped atoms whose world_states gate names this need.
func gatedOn(t *testing.T, need string, mandatoryOnly bool) []string {
	t.Helper()
	embedded, err := LoadEmbeddedCorpus()
	if err != nil {
		t.Fatalf("LoadEmbeddedCorpus: %v", err)
	}
	var ids []string
	for _, a := range embedded.All() {
		if a == nil || (mandatoryOnly && !a.IsMandatory) {
			continue
		}
		for _, s := range a.WorldStates {
			if strings.TrimPrefix(s, "/") == need {
				ids = append(ids, a.ID)
				break
			}
		}
	}
	sort.Strings(ids)
	return ids
}

func unionSorted(a, b []string) []string {
	out := slices.Clone(a)
	for _, s := range b {
		if !slices.Contains(out, s) {
			out = append(out, s)
		}
	}
	sort.Strings(out)
	return out
}

// The Go failure modes and the Go-only hallucination guards are served when the
// kernel derives /authoring_go -- a compile aimed at Go -- and on no other
// compile: not on a Markdown target, not on a compile whose language nothing
// could tell (which, with the language key alone, admitted every
// language-tagged mandatory atom), and not on a /go compile that does not carry
// the need (the gate is the need, not the language).
func TestSkeletonNeeds_GoGuidanceFollowsAGoTarget(t *testing.T) {
	c, k := newEmbeddedCorpusCompiler(t)
	// The eleven measured on a Markdown turn (2026-09-22), plus any atom gated
	// on the need since.
	goAtoms := unionSorted([]string{
		"language/go/ai_failures/overview",
		"language/go/ai_failures/loop_capture",
		"language/go/ai_failures/nil_interface",
		"language/go/ai_failures/defer_gotchas",
		"language/go/ai_failures/slice_append",
		"language/go/ai_failures/map_safety",
		"language/go/ai_failures/error_handling",
		"language/go/ai_failures/goroutine_leaks",
		"hallucination/coder/error_suppression",
		"hallucination/coder/scope_leak",
		"hallucination/coder/deprecated_pattern",
	}, gatedOn(t, "authoring_go", true))

	goNeeds := kernelTargetNeeds(t, k, "/go")
	if !slices.Contains(goNeeds, "authoring_go") {
		t.Fatalf("the kernel derives %v for a Go target, want authoring_go", goNeeds)
	}
	served, _ := compiledAtoms(t, c, coderTurnContext(t, "/fix", "internal/session/executor.go", "/go", goNeeds))
	for _, id := range goAtoms {
		if served[id] == nil {
			t.Errorf("a Go /fix compile does not serve %s", id)
		}
	}

	for _, tc := range []struct {
		name     string
		target   string
		language string
		needs    []string
	}{
		{"markdown target", "Docs/architecture/features/03-GAP-ANALYSIS.md", "/markdown", kernelTargetNeeds(t, k, "/markdown")},
		{"undetected language", "Makefile", "", kernelTargetNeeds(t, k, "")},
		{"go language without the need", "internal/session/executor.go", "/go", nil},
	} {
		served, _ := compiledAtoms(t, c, coderTurnContext(t, "/create", tc.target, tc.language, tc.needs))
		for _, id := range goAtoms {
			if served[id] != nil {
				t.Errorf("%s (needs %v): served %s", tc.name, tc.needs, id)
			}
		}
	}
}

// The code-authoring guidance -- compile check, self-correction, the coder's
// 7-phase protocol, the coverage mandate, import/API fabrication, scratch
// programs -- is served on a compile that writes code and one whose language
// is undetected, and not on a compile writing a Markdown document. The
// guidance that holds for any file the coder writes stays on every coder
// compile: that is the audit's other half, pinned here.
func TestSkeletonNeeds_CodeGuidanceFollowsACodeTarget(t *testing.T) {
	c, k := newEmbeddedCorpusCompiler(t)
	// Served on a coder /create compile; ai_era_fundamentals and the debugging
	// atoms also need a /fix-class verb, so they are checked on /fix below.
	createServed := []string{
		"methodology/tdd/coverage_mandate",
		"shards/coder/compile_check",
		"shards/coder/self_correction_protocol",
		"shards/coder/cognitive_protocol",
		"hallucination/coder/import_fabrication",
		"hallucination/coder/api_fabrication",
		"hallucination/coder/scratch_artifacts",
	}
	fixServed := []string{
		"methodology/tdd/ai_era_fundamentals",
		"methodology/debugging/ai_code_debugging",
		"methodology/debugging/concurrent_debugging",
	}
	codeAtoms := unionSorted(append(slices.Clone(createServed), fixServed...), gatedOn(t, "authoring_code", true))
	// Kept on every coder compile, prose included (the audit's reasons are in
	// the lane log and the atoms): editing and file-creation discipline hold
	// for a document as much as for code.
	keptEverywhere := []string{
		"methodology/editing_discipline",
		"hallucination/coder/duplicate_file_creation",
		"hallucination/coder/scope_creep",
		"identity/coder/investigate_first",
	}

	for _, tc := range []struct {
		name     string
		target   string
		language string
	}{
		{"go target", "internal/session/work_steps.go", "/go"},
		{"mangle target", "internal/core/defaults/policy/jit_needs.mg", "/mangle"},
		{"undetected language", "Makefile", ""},
	} {
		needs := kernelTargetNeeds(t, k, tc.language)
		if !slices.Contains(needs, "authoring_code") {
			t.Fatalf("%s: the kernel derives %v, want authoring_code", tc.name, needs)
		}
		created, _ := compiledAtoms(t, c, coderTurnContext(t, "/create", tc.target, tc.language, needs))
		for _, id := range append(slices.Clone(createServed), keptEverywhere...) {
			if created[id] == nil {
				t.Errorf("%s /create: %s not served", tc.name, id)
			}
		}
	}
	goFix, _ := compiledAtoms(t, c, coderTurnContext(t, "/fix", "internal/session/work_steps.go", "/go", kernelTargetNeeds(t, k, "/go")))
	for _, id := range fixServed {
		if goFix[id] == nil {
			t.Errorf("go /fix: %s not served", id)
		}
	}

	mdNeeds := kernelTargetNeeds(t, k, "/markdown")
	if slices.Contains(mdNeeds, "authoring_code") {
		t.Fatalf("the kernel derives authoring_code for a Markdown target: %v", mdNeeds)
	}
	for _, verb := range []string{"/create", "/fix"} {
		served, _ := compiledAtoms(t, c, coderTurnContext(t, verb, "Docs/architecture/features/03-GAP-ANALYSIS.md", "/markdown", mdNeeds))
		for _, id := range codeAtoms {
			if served[id] != nil {
				t.Errorf("markdown %s: served %s, which serves a code target", verb, id)
			}
		}
		for _, id := range keptEverywhere {
			if served[id] == nil {
				t.Errorf("markdown %s: %s not served; it holds for any file the coder writes", verb, id)
			}
		}
	}
}

// TestSkeletonNeeds_Inventory is the MEASUREMENT: what a coder compile serves
// for a Markdown target and a Go target, with the needs the kernel derives for
// each. It logs the atoms with their mandatory flag and size, and fails only if
// a compile is empty. Run with -v to read it.
func TestSkeletonNeeds_Inventory(t *testing.T) {
	c, k := newEmbeddedCorpusCompiler(t)
	for _, tc := range []struct {
		name, verb, target, language string
	}{
		{"markdown create", "/create", "Docs/architecture/features/03-GAP-ANALYSIS.md", "/markdown"},
		{"go fix", "/fix", "internal/session/executor.go", "/go"},
		{"undetected create", "/create", "Makefile", ""},
	} {
		ids, result := compiledAtoms(t, c, coderTurnContext(t, tc.verb, tc.target, tc.language, kernelTargetNeeds(t, k, tc.language)))
		if len(ids) == 0 {
			t.Fatalf("%s: empty compile", tc.name)
		}
		keys := make([]string, 0, len(ids))
		for id := range ids {
			keys = append(keys, id)
		}
		sort.Strings(keys)
		t.Logf("%s: %d atoms, %d tokens, prompt %d chars", tc.name, len(ids), result.TotalTokens, len(result.Prompt))
		for _, id := range keys {
			t.Logf("  %-60s mand=%-5v tok=%5d", id, ids[id].IsMandatory, ids[id].TokenCount)
		}
	}
}
