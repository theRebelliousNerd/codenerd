package core

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/constant"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	mast "codeberg.org/TauCeti/mangle-go/ast"
)

// =============================================================================
// /name SLOT PRODUCER SWEEP
// =============================================================================
//
// Admission tolerates a bare Go string in a slot the corpus declares /name and
// reports it once per process ("arg N is declared /name but its producer
// passed a bare Go string"). The tolerated value lands as a string constant no
// /name rule can match, so the report is a defect report, and a dogfood session
// only finds the producers it happens to exercise. This sweep finds them all
// statically, with full type information: every types.Fact / prompt.Fact
// composite literal in the module whose Predicate is a constant, against the
// production corpus Decls. An argument is a defect when no bound alternative
// accepts a string at its position, some alternative declares /name there, and
// its Go type is string (or a named string type other than MangleAtom).
//
// internal/types/fact_conventions_guard_test.go is the syntactic sweep (it sees
// literals only); this one sees variables and fields, which is where every
// producer the 2026-10-28 dogfood log named was hiding (tool.Capabilities,
// fileInfo.Language, the shardType parameter of ResultToFacts).
//
// A "/" + x concatenation is a name by construction and is not reported.
//
// The sweep is a ratchet. nameSlotStringBaseline lists the producers that
// pass a string-typed value today, counted per file, predicate and argument so
// edits elsewhere in a file do not move it. Some of them hold a "/name" at
// runtime and land as names; the rest are the tolerance's live defect reports.
// A new site, or one more site under an existing key, fails. A key whose count
// falls must be lowered here, so a fixed producer cannot quietly regress.
//
// It shells out to `go list -export`, so it is skipped under -short.

type sweepPackage struct {
	ImportPath string
	Dir        string
	Export     string
	GoFiles    []string
	CgoFiles   []string
	Module     *struct{ Path string }
	Standard   bool
}

func TestFactNameSlotProducersPassNames(t *testing.T) {
	if testing.Short() {
		t.Skip("type-checks the module; skipped under -short")
	}
	root := sweepRepoRoot(t)

	k, err := NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel: %v", err)
	}
	if err := k.Evaluate(); err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	nameOnly := nameOnlySlots(k)

	pkgs := listModulePackages(t, root)
	exports := map[string]string{}
	for _, p := range pkgs {
		if p.Export != "" {
			exports[p.ImportPath] = p.Export
		}
	}
	fset := token.NewFileSet()
	imp := importer.ForCompiler(fset, "gc", func(path string) (io.ReadCloser, error) {
		f, ok := exports[path]
		if !ok {
			return nil, fmt.Errorf("no export data for %s", path)
		}
		return os.Open(f)
	})

	counts := map[string]int{}
	sites := map[string][]string{}
	checked := 0
	for _, p := range pkgs {
		if p.Standard || p.Module == nil || p.Module.Path != "codenerd" {
			continue
		}
		var files []*ast.File
		for _, name := range append(append([]string{}, p.GoFiles...), p.CgoFiles...) {
			f, err := parser.ParseFile(fset, filepath.Join(p.Dir, name), nil, 0)
			if err != nil {
				t.Fatalf("parse %s: %v", name, err)
			}
			files = append(files, f)
		}
		info := &types.Info{
			Types: map[ast.Expr]types.TypeAndValue{},
		}
		conf := types.Config{Importer: imp, FakeImportC: true, Error: func(error) {}}
		_, _ = conf.Check(p.ImportPath, fset, files, info)

		for _, f := range files {
			ast.Inspect(f, func(n ast.Node) bool {
				lit, ok := n.(*ast.CompositeLit)
				if !ok || !isKernelFactType(info.Types[lit].Type) {
					return true
				}
				pred, args := sweepFactParts(lit, info)
				if pred == "" || args == nil {
					return true
				}
				slots, declared := nameOnly[sweepKey(pred, len(args.Elts))]
				if !declared {
					return true
				}
				checked++
				for j, arg := range args.Elts {
					if !slots[j] || !isGoStringArg(arg, info) {
						continue
					}
					pos := fset.Position(arg.Pos())
					rel, _ := filepath.Rel(root, pos.Filename)
					key := fmt.Sprintf("%s|%s/%d#%d", filepath.ToSlash(rel), pred, len(args.Elts), j)
					counts[key]++
					sites[key] = append(sites[key], fmt.Sprintf("%s:%d", filepath.ToSlash(rel), pos.Line))
				}
				return true
			})
		}
	}
	if checked == 0 {
		t.Fatal("the sweep checked no fact literal; it is inert")
	}
	var findings []string
	for key, n := range counts {
		if allowed := nameSlotStringBaseline[key]; n > allowed {
			findings = append(findings, fmt.Sprintf("%s: %d site(s), baseline %d (%s): a /name argument gets a Go string; pass types.MangleAtom or types.Atom",
				key, n, allowed, strings.Join(sites[key], ", ")))
		}
	}
	for key, allowed := range nameSlotStringBaseline {
		if n := counts[key]; n < allowed {
			findings = append(findings, fmt.Sprintf("%s: baseline %d but %d site(s) remain; lower nameSlotStringBaseline", key, allowed, n))
		}
	}
	sort.Strings(findings)
	for _, f := range findings {
		t.Error(f)
	}
	t.Logf("checked %d declared fact literals", checked)
}

// nameSlotStringBaseline: "file|pred/arity#arg" -> string-typed sites as of
// 2026-10-28. Only ever lowered.
var nameSlotStringBaseline = map[string]int{
	"cmd/nerd/chat/process_continuation.go|shard_result/5#1":                 1,
	"cmd/nerd/chat/reflection.go|trace_recall_result/4#2":                    1,
	"internal/campaign/assault_tasks.go|phase_estimate/3#2":                  1,
	"internal/campaign/decomposer_documents.go|doc_layer/3#1":                1,
	"internal/campaign/decomposer_documents.go|doc_metadata/5#2":             1,
	"internal/campaign/decomposer_documents.go|doc_tag/2#1":                  1,
	"internal/campaign/decomposer_documents.go|goal_topic/2#1":               1,
	"internal/campaign/orchestrator_lifecycle.go|campaign_task/5#4":          1,
	"internal/campaign/orchestrator_task_handlers.go|goal_requires/2#1":      1,
	"internal/campaign/orchestrator_task_handlers.go|missing_tool_for/2#0":   1,
	"internal/campaign/orchestrator_task_handlers.go|missing_tool_for/2#1":   1,
	"internal/campaign/orchestrator_tasks.go|campaign_task/5#3":              1,
	"internal/campaign/orchestrator_tasks.go|campaign_task/5#4":              1,
	"internal/campaign/orchestrator_utils.go|campaign/5#1":                   1,
	"internal/campaign/orchestrator_utils.go|campaign/5#4":                   1,
	"internal/campaign/orchestrator_utils.go|phase_checkpoint/5#1":           1,
	"internal/campaign/recovery_admission.go|campaign_recovery_class/2#1":    1,
	"internal/campaign/recurse_policy.go|recurse_attempt/5#3":                1,
	"internal/campaign/recurse_policy.go|recurse_finding/6#3":                1,
	"internal/campaign/recurse_policy.go|recurse_ratchet_changed/2#1":        1,
	"internal/campaign/recurse_policy.go|recurse_ratchet_gate/6#2":           1,
	"internal/campaign/recurse_policy.go|recurse_ratchet_gate/6#3":           1,
	"internal/campaign/recurse_policy.go|recurse_ratchet_target/3#2":         1,
	"internal/campaign/risk_gate_contract.go|campaign_risk_concern/3#1":      1,
	"internal/campaign/risk_gate_contract.go|campaign_risk_concern/3#2":      1,
	"internal/campaign/risk_gate_contract.go|campaign_risk_gate_outcome/3#1": 1,
	"internal/campaign/risk_gate_contract.go|campaign_risk_gate_outcome/3#2": 1,
	"internal/campaign/risk_gate_contract.go|campaign_risk_override/2#1":     1,
	"internal/campaign/risk_gate_contract.go|campaign_risk_posture/4#3":      1,
	"internal/campaign/types.go|campaign/5#1":                                1,
	"internal/campaign/types.go|campaign/5#4":                                1,
	"internal/campaign/types.go|campaign_phase/6#4":                          1,
	"internal/campaign/types.go|campaign_task/5#3":                           1,
	"internal/campaign/types.go|campaign_task/5#4":                           1,
	"internal/campaign/types.go|phase_category/2#1":                          1,
	"internal/campaign/types.go|phase_dependency/3#2":                        1,
	"internal/campaign/types.go|phase_estimate/3#2":                          1,
	"internal/campaign/types.go|phase_objective/4#1":                         1,
	"internal/campaign/types.go|phase_objective/4#3":                         1,
	"internal/campaign/types.go|source_document/4#2":                         1,
	"internal/campaign/types.go|task_artifact/4#1":                           1,
	"internal/campaign/types.go|task_attempt/4#2":                            1,
	"internal/campaign/types.go|task_attempt_signal/3#2":                     1,
	"internal/campaign/types.go|task_error/3#1":                              1,
	"internal/campaign/types.go|task_priority/2#1":                           1,
	"internal/context/serializer.go|user_intent/5#1":                         1,
	"internal/context/serializer.go|user_intent/5#2":                         1,
	"internal/core/dream_plan_manager.go|dream_plan/4#2":                     1,
	"internal/core/dream_router.go|dream_risk_pattern/3#0":                   1,
	"internal/core/hybrid_loader.go|intent_definition/3#1":                   1,
	"internal/core/shadow_mode.go|shadow_state/3#2":                          1,
	"internal/core/shards/manager_tools.go|user_intent/5#0":                  1,
	"internal/core/virtual_store_actions.go|test_state/1#0":                  1,
	"internal/core/virtual_store_predicates.go|knowledge_link/3#1":           1,
	"internal/core/virtual_store_python.go|python_pytest_result/4#1":         1,
	"internal/core/virtual_store_routing.go|execution_result/6#1":            1,
	"internal/core/virtual_store_tool_facts.go|tool_execution/3#1":           1,
	"internal/core/virtual_store_workflows.go|world_model_updating/2#0":      1,
	"internal/core/virtual_store_workflows.go|world_model_updating/2#1":      1,
	"internal/shards/system/constitution.go|appeal_available/4#1":            1,
	"internal/shards/system/constitution.go|appeal_available/4#3":            1,
	"internal/shards/system/constitution.go|appeal_denied/4#1":               1,
	"internal/shards/system/constitution.go|appeal_denied/4#2":               1,
	"internal/shards/system/constitution.go|appeal_granted/4#1":              1,
	"internal/shards/system/constitution.go|appeal_pending/4#1":              1,
	"internal/shards/system/constitution.go|permitted_action/5#1":            1,
	"internal/shards/system/constitution.go|security_violation/3#0":          1,
	"internal/shards/system/constitution.go|temporary_override/2#0":          1,
	"internal/shards/system/executive.go|executive_trace/4#0":                1,
	"internal/shards/system/executive.go|pending_action/5#1":                 1,
	"internal/shards/system/executive_intent.go|no_action_reason/2#0":        1,
	"internal/shards/system/perception.go|processed_intent/1#0":              1,
	"internal/shards/system/perception.go|user_intent/5#0":                   1,
	"internal/shards/system/planner.go|agenda_item/5#3":                      2,
	"internal/shards/system/planner.go|campaign_task/5#3":                    1,
	"internal/shards/system/planner.go|plan_task/4#2":                        1,
	"internal/shards/system/router.go|no_action_reason/2#0":                  1,
	"internal/shards/system/router.go|system_event_handled/3#0":              1,
	"internal/system/factory.go|user_agent_declared_tool/2#0":                1,
	"internal/world/code_elements.go|api_client_function/3#2":                1,
	"internal/world/code_elements.go|api_handler_function/3#2":               1,
}

func sweepKey(pred string, arity int) string { return fmt.Sprintf("%s/%d", pred, arity) }

// nameOnlySlots maps pred/arity to the argument positions where some bound
// alternative declares /name and none accepts a string.
func nameOnlySlots(k *RealKernel) map[string]map[int]bool {
	str := mast.String("s")
	out := map[string]map[int]bool{}
	info := k.GetProgramInfo()
	k.mu.Lock()
	defer k.mu.Unlock()
	for sym := range info.Decls {
		pa, err := k.admissionForLocked(sym)
		if err != nil || pa == nil {
			continue
		}
		for j := 0; j < sym.Arity; j++ {
			if pa.boundIsAt(j, mast.NameBound) && !pa.acceptsAt(j, str) {
				if out[sweepKey(sym.Symbol, sym.Arity)] == nil {
					out[sweepKey(sym.Symbol, sym.Arity)] = map[int]bool{}
				}
				out[sweepKey(sym.Symbol, sym.Arity)][j] = true
			}
		}
	}
	return out
}

func isKernelFactType(t types.Type) bool {
	named, ok := types.Unalias(t).(*types.Named)
	if !ok || named.Obj().Pkg() == nil || named.Obj().Name() != "Fact" {
		return false
	}
	switch named.Obj().Pkg().Path() {
	case "codenerd/internal/types", "codenerd/internal/prompt":
		return true
	}
	return false
}

func sweepFactParts(lit *ast.CompositeLit, info *types.Info) (string, *ast.CompositeLit) {
	var pred string
	var args *ast.CompositeLit
	for _, elt := range lit.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		key, ok := kv.Key.(*ast.Ident)
		if !ok {
			continue
		}
		switch key.Name {
		case "Predicate":
			if tv := info.Types[kv.Value]; tv.Value != nil && tv.Value.Kind() == constant.String {
				pred = constant.StringVal(tv.Value)
			}
		case "Args":
			args, _ = kv.Value.(*ast.CompositeLit)
		}
	}
	return pred, args
}

// isGoStringArg reports whether arg reaches ToAtom as a Go string a /name
// slot can only tolerate. A constant that is already a name ("/coder") is
// encoded as one and is not reported.
func isGoStringArg(arg ast.Expr, info *types.Info) bool {
	tv, ok := info.Types[arg]
	if !ok || tv.Type == nil {
		return false
	}
	if named, ok := types.Unalias(tv.Type).(*types.Named); ok && named.Obj().Name() == "MangleAtom" {
		return false
	}
	basic, ok := tv.Type.Underlying().(*types.Basic)
	if !ok || basic.Info()&types.IsString == 0 {
		return false
	}
	if tv.Value != nil && strings.HasPrefix(constant.StringVal(tv.Value), "/") {
		return false
	}
	// "/" + verb is a name by construction: ToAtom encodes it as one, and the
	// tolerance this sweep guards never sees it.
	if bin, ok := ast.Unparen(arg).(*ast.BinaryExpr); ok && bin.Op == token.ADD {
		left := bin.X
		for {
			inner, ok := ast.Unparen(left).(*ast.BinaryExpr)
			if !ok || inner.Op != token.ADD {
				break
			}
			left = inner.X
		}
		if lv := info.Types[left].Value; lv != nil && lv.Kind() == constant.String && strings.HasPrefix(constant.StringVal(lv), "/") {
			return false
		}
	}
	return true
}

func listModulePackages(t *testing.T, root string) []sweepPackage {
	t.Helper()
	cmd := exec.Command("go", "list", "-e", "-deps", "-export", "-json=ImportPath,Dir,Export,GoFiles,CgoFiles,Module,Standard", "./...")
	cmd.Dir = root
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("go list -export: %v\n%s", err, stderr.String())
	}
	var pkgs []sweepPackage
	dec := json.NewDecoder(bytes.NewReader(out))
	for dec.More() {
		var p sweepPackage
		if err := dec.Decode(&p); err != nil {
			t.Fatalf("decode go list output: %v", err)
		}
		pkgs = append(pkgs, p)
	}
	return pkgs
}

func sweepRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found")
		}
		dir = parent
	}
}
