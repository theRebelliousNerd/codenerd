package campaign

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
)

// The event type set must stay closed.
//
// OrchestratorEvent.Type is switched on by name in three consumers. A type
// written at an emit site and nowhere else produces an event every consumer
// drops through its default branch — the campaign still runs, the operator just
// never sees that step happen. This reads the emit sites and fails if any of
// them names a type the constant list does not.
//
// This scan is now the SECOND line of defence, not the first. It inspects string
// literals, and while emitEvent took a plain string that was a real hole: an
// untyped string constant compiles fine, so naming the literal
//
//	const zzPhantom = "zz_phantom_event"
//	o.emitEvent(zzPhantom, ...)
//
// slipped straight past it and produced exactly the defect this test exists to
// catch. OrchestratorEventType is now a defined type, so the compiler rejects
// that, and no regexp or AST walk has to be clever enough to see through a
// rename.
//
// What this scan still owns is the reverse direction, which the type system
// cannot express: a literal at an emit site must correspond to a declared
// constant, so a new event type has to be added to the list — and therefore
// considered by the UIs — before it can ship.
func TestOrchestratorEventTypes_AreClosedSet(t *testing.T) {
	pkgDir := filepath.Join(repoRoot(t), "internal", "campaign")

	known := make(map[string]bool, len(orchestratorEventTypes))
	for _, v := range orchestratorEventTypes {
		known[string(v)] = true
	}

	entries, err := os.ReadDir(pkgDir)
	if err != nil {
		t.Fatalf("read package dir: %v", err)
	}

	// Package-level string constants and vars, so an identifier at an emit site
	// can be resolved to the value it actually carries.
	//
	// Without this the scan sees only literals, and naming the literal walks
	// straight past it. A defined type closes half the hole — a `string`
	// variable is now a compile error — but Go converts an UNTYPED string
	// constant to a named string type implicitly, so
	//
	//	const zzPhantom = "zz_phantom_event"
	//	o.emitEvent(zzPhantom, ...)
	//
	// still compiles. That is the exact shape an adversarial review used to
	// slip a phantom event past this guard, and it is a plausible accident too:
	// hoisting a literal into a constant is a refactor nobody would think twice
	// about.
	constStrings := map[string]string{}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		fset := token.NewFileSet()
		file, perr := parser.ParseFile(fset, filepath.Join(pkgDir, name), nil, 0)
		if perr != nil {
			t.Fatalf("parse %s: %v", name, perr)
		}
		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || (gen.Tok != token.CONST && gen.Tok != token.VAR) {
				continue
			}
			for _, spec := range gen.Specs {
				vs, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for i, id := range vs.Names {
					if i >= len(vs.Values) {
						continue
					}
					lit, ok := vs.Values[i].(*ast.BasicLit)
					if !ok || lit.Kind != token.STRING {
						continue
					}
					if v, uerr := strconv.Unquote(lit.Value); uerr == nil {
						constStrings[id.Name] = v
					}
				}
			}
		}
	}

	var offenders []string
	emitSites := 0

	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		fset := token.NewFileSet()
		file, perr := parser.ParseFile(fset, filepath.Join(pkgDir, name), nil, 0)
		if perr != nil {
			t.Fatalf("parse %s: %v", name, perr)
		}

		// Walk function by function, so an exemption belongs to the function
		// that declares it.
		//
		// The forwarding exemption used to be built per FILE, keyed by
		// parameter name, from every FuncDecl in it. Since emitRiskAudit and
		// emitEvent both name their parameter eventType, the identifier
		// "eventType" was exempt file-wide — so any local variable called
		// eventType, holding anything at all, sailed through. An adversarial
		// pass smuggled a computed fmt.Sprintf value past the scan that way,
		// and the control (the same code with the local renamed) was correctly
		// reported, which is what made it unmistakable.
		inspectCall := func(call *ast.CallExpr, forwarded map[string]bool) {
			if len(call.Args) == 0 {
				return
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || (sel.Sel.Name != "emitEvent" && sel.Sel.Name != "emitRiskAudit") {
				return
			}
			emitSites++

			describe := func(expr ast.Expr) string {
				var b strings.Builder
				if err := printer.Fprint(&b, token.NewFileSet(), expr); err != nil {
					return fmt.Sprintf("%T", expr)
				}
				return b.String()
			}

			switch arg := call.Args[0].(type) {
			case *ast.BasicLit:
				if arg.Kind != token.STRING {
					offenders = append(offenders, name+": non-string literal "+arg.Value)
					return
				}
				value, uerr := strconv.Unquote(arg.Value)
				if uerr != nil {
					offenders = append(offenders, name+": unparseable literal "+arg.Value)
					return
				}
				if !known[value] {
					offenders = append(offenders, name+": literal "+strconv.Quote(value))
				}
			case *ast.Ident:
				if forwarded[arg.Name] {
					// This function's own typed parameter, passed straight
					// through; the type system already vouches for it.
					return
				}
				value, resolved := constStrings[arg.Name]
				if !resolved {
					offenders = append(offenders,
						name+": "+arg.Name+" (an event type this scan cannot resolve to a "+
							"declared constant; pass one of the Event* constants directly)")
					return
				}
				if !known[value] {
					offenders = append(offenders, name+": "+arg.Name+" = "+strconv.Quote(value))
				}
			default:
				// DEFAULT DENY. Everything that is not a literal or a resolvable
				// identifier is an offender, including the three forms that used
				// to fall through this switch untouched and pass:
				//
				//	OrchestratorEventType("zz")   a conversion  (*ast.CallExpr)
				//	zzPhantomVars[0]              an index      (*ast.IndexExpr)
				//	zzHolder.T                    a selector    (*ast.SelectorExpr)
				//
				// The last is also the shape a constant imported from another
				// package takes, so "the compiler will have caught it" was never
				// true for any of them. If the scan cannot see what an event
				// type is, neither can a reviewer reading the call site.
				offenders = append(offenders,
					name+": "+describe(arg)+" (an event type this scan cannot evaluate; "+
						"pass one of the Event* constants directly)")
			}
		}

		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}
			forwarded := map[string]bool{}
			if fn.Type.Params != nil {
				for _, field := range fn.Type.Params.List {
					id, ok := field.Type.(*ast.Ident)
					if !ok || id.Name != "OrchestratorEventType" {
						continue
					}
					for _, pn := range field.Names {
						forwarded[pn.Name] = true
					}
				}
			}
			// Two passes over the function: calls, then method VALUES.
			//
			// Taking the method as a value —
			//
			//	zzEmit := o.emitEvent
			//	zzEmit("zz_phantom_event", ...)
			//
			// makes the call's Fun an *ast.Ident, so the call pass never sees
			// it. Worse, the site is not counted either, which quietly erodes
			// the emitSites canary that exists to notice exactly this drift.
			// The untyped string converts implicitly at the call, so the
			// compiler does not help. Rather than chase the alias, refuse the
			// aliasing: there is no reason to take these methods as values, and
			// a rule that says so is far more robust than one that tries to
			// follow them.
			calledFuns := map[ast.Node]bool{}
			ast.Inspect(fn, func(n ast.Node) bool {
				if call, ok := n.(*ast.CallExpr); ok {
					calledFuns[call.Fun] = true
					inspectCall(call, forwarded)
				}
				return true
			})
			ast.Inspect(fn, func(n ast.Node) bool {
				sel, ok := n.(*ast.SelectorExpr)
				if !ok || calledFuns[ast.Node(sel)] {
					return true
				}
				if sel.Sel.Name != "emitEvent" && sel.Sel.Name != "emitRiskAudit" {
					return true
				}
				offenders = append(offenders,
					name+": "+sel.Sel.Name+" taken as a method value in "+fn.Name.Name+
						" (this hides the emit site from the scan; call it directly)")
				return true
			})
		}
	}

	if emitSites < 20 {
		t.Fatalf("found only %d emit sites; the AST scan is broken, not the event set", emitSites)
	}
	if len(offenders) > 0 {
		sort.Strings(offenders)
		t.Errorf("event types emitted as bare literals outside the closed set:\n  %s\n"+
			"Add the constant to orchestrator_events.go and use it, so every UI that "+
			"switches on event type is forced to consider the new event.",
			strings.Join(offenders, "\n  "))
	}
}

func TestOrchestratorEventTypes_ListIsSortedAndUnique(t *testing.T) {
	// IsKnownOrchestratorEventType binary-searches the slice.
	for i := 1; i < len(orchestratorEventTypes); i++ {
		if orchestratorEventTypes[i-1] >= orchestratorEventTypes[i] {
			t.Fatalf("orchestratorEventTypes is not strictly sorted at %d: %q then %q; "+
				"IsKnownOrchestratorEventType would miss entries",
				i, orchestratorEventTypes[i-1], orchestratorEventTypes[i])
		}
	}
	for _, v := range orchestratorEventTypes {
		if !IsKnownOrchestratorEventType(v) {
			t.Errorf("IsKnownOrchestratorEventType(%q) = false for a listed type", v)
		}
	}
	if IsKnownOrchestratorEventType("definitely_not_an_event") {
		t.Error("IsKnownOrchestratorEventType accepted an unlisted type")
	}
}

func TestOrchestratorEventTypes_ReturnsCopy(t *testing.T) {
	first := OrchestratorEventTypes()
	if len(first) == 0 {
		t.Fatal("no event types returned")
	}
	first[0] = "mutated"
	if OrchestratorEventTypes()[0] == "mutated" {
		t.Fatal("OrchestratorEventTypes exposed its backing array; a caller could corrupt the closed set")
	}
}

// Metrics hooks must observe real work without the engine depending on any
// particular backend.
func TestMetricsSink_ShouldObserveTaskAndCheckpointDurations(t *testing.T) {
	orch, _ := newCheckpointRegressionOrchestrator(t, "PASS: verified")
	metrics := NewInMemoryMetrics()
	orch.SetMetricsSink(metrics)

	orch.markPhaseStart(orch.campaign.Phases[0].ID)
	orch.observeTaskDuration(orch.campaign.Phases[0].ID, &orch.campaign.Phases[0].Tasks[0], "completed", 250*time.Millisecond)

	if err := orch.runPhase(t.Context(), &orch.campaign.Phases[0]); err != nil {
		t.Fatalf("runPhase: %v", err)
	}

	snapshot := metrics.Snapshot()
	tasks, _ := snapshot["tasks"].(map[string]any)
	if len(tasks) == 0 {
		t.Fatal("no task durations observed")
	}
	entry, ok := tasks[string(TaskTypeFileCreate)+"|completed"].(map[string]any)
	if !ok {
		t.Fatalf("expected an entry for file_create|completed, got %v", tasks)
	}
	if entry["count"].(int) != 1 || entry["max_ms"].(int64) != 250 {
		t.Errorf("task observation lost detail: %v", entry)
	}

	checkpoints, _ := snapshot["checkpoints"].(map[string]any)
	if len(checkpoints) == 0 {
		t.Fatal("no checkpoint outcomes observed; a passing verification recorded nothing")
	}
	if _, ok := checkpoints[string(VerifyShardValidate)]; !ok {
		t.Errorf("expected an entry for %s, got %v", VerifyShardValidate, checkpoints)
	}

	phases, _ := snapshot["phases_ms"].(map[string]int64)
	if _, ok := phases[orch.campaign.Phases[0].ID]; !ok {
		t.Errorf("phase duration was not observed on completion: %v", phases)
	}
}

// A nil sink must cost nothing and never panic — that is the whole reason the
// hooks are optional rather than a hard dependency.
func TestMetricsSink_WhenUnset_ShouldBeInert(t *testing.T) {
	orch, _ := newCheckpointRegressionOrchestrator(t, "PASS: verified")

	orch.markPhaseStart("/phase_ckpt_0")
	orch.observePhaseDuration("/phase_ckpt_0")
	orch.observeCheckpoint("/phase_ckpt_0", string(VerifyNone), true, time.Second)
	orch.observeTaskDuration("/phase_ckpt_0", &orch.campaign.Phases[0].Tasks[0], "completed", time.Second)
	orch.observeRiskPreflight("/campaign_ckpt", &RiskGateEvaluation{Allowed: true})

	if _, loaded := orch.phaseStarts.Load("/phase_ckpt_0"); loaded {
		t.Error("phase start times were recorded with no sink attached")
	}
}
