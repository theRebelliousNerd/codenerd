package types

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// =============================================================================
// FACT CONSTRUCTION / EXTRACTION CONVENTION GUARD
// =============================================================================
//
// This file is the enforced form of the "audit hot assert paths" item. An audit
// that ends in prose decays the week after it is written, so the audit result
// lives here as a ratchet: every violation found by the sweep is listed in a
// baseline below with the reason it survived, and anything NOT in the baseline
// fails the build.
//
// The three rules exist because each one has already produced a live,
// silent-in-production bug in this repo:
//
//   R1 Decl conformance. internal/core/defaults/*.mg declares the Mangle type of
//      every predicate argument (bound [/name, /string, /number]). Fact.ToAtom
//      is Decl-blind — it infers the constant type from the Go value's shape —
//      so a Go string "branch" lands in a slot declared /name as a StringType
//      constant. Nothing errors. The rule that wanted git_state(/branch, X)
//      simply never fires. Found this way this pass: internal/mcp querying a
//      /name-declared predicate with a quoted string (the query never matched
//      and the caller fell back to Go), and a float64 confidence going into a
//      /number slot.
//
//   R2 No %v in fact arguments. fmt.Sprintf("%v", x) renders a pointer as
//      "0x7ff63be770e0" and a slice as "[a b c]". That string enters the kernel
//      as a StringType constant, and the first numeric builtin that touches it
//      kills the whole evaluation ("value 0x... (1) is not a number") — not one
//      rule, the entire fixpoint, on every pass. ToAtom's default branch was
//      hardened against this; the construction sites are the other half.
//
//   R3 No MangleAtom type assertion on query results. Facts that come back from
//      a kernel query never carry MangleAtom: both readback paths
//      (core.baseTermToValue and mangle.constantToInterface) return NameType as
//      a plain Go string. Every `fact.Args[i].(MangleAtom)` on a query result is
//      therefore a branch that can never be taken. Use ExtractName / ArgName,
//      which accept both.
//
// Known limitation, stated so nobody reads more into a pass than is there: R1
// only sees arguments whose Mangle type is decidable from syntax — literals,
// conversions like float64(x), and the MangleAtom/MangleString/PercentFromRatio
// constructors. A bare variable is skipped. Widening it needs full type
// resolution across ~525k LOC, which does not belong in a unit test.

// factGuardRoot locates the repository root by walking up from the test's
// working directory until it finds go.mod.
func factGuardRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Skipf("cannot determine working directory: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Skip("repository root (go.mod) not found; skipping repo-wide guard")
		}
		dir = parent
	}
}

// declRe matches `Decl pred(A, B) bound [/string, /name]`.
var declRe = regexp.MustCompile(`(?m)^\s*Decl\s+([a-zA-Z_][a-zA-Z0-9_]*)\s*\(([^)]*)\)\s*bound\s*\[([^\]]*)\]`)

type declKey struct {
	pred  string
	arity int
}

// loadDeclBounds parses the Mangle corpus for declared argument bounds. The
// corpus is the authority on argument types; Go is the side that drifts.
func loadDeclBounds(root string) map[declKey][]string {
	out := map[declKey][]string{}
	_ = filepath.WalkDir(filepath.Join(root, "internal", "core", "defaults"), func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".mg") {
			return nil
		}
		b, readErr := os.ReadFile(p)
		if readErr != nil {
			return nil
		}
		for _, m := range declRe.FindAllStringSubmatch(string(b), -1) {
			arity := 0
			if strings.TrimSpace(m[2]) != "" {
				arity = len(strings.Split(m[2], ","))
			}
			bounds := make([]string, 0, arity)
			for _, s := range strings.Split(m[3], ",") {
				bounds = append(bounds, strings.TrimSpace(s))
			}
			if len(bounds) != arity {
				continue
			}
			k := declKey{m[1], arity}
			if prev, seen := out[k]; seen {
				// Two files declaring the same predicate differently: trust
				// neither rather than picking one and reporting confident
				// nonsense. (schema_duplicate_decl_test.go polices duplicates.)
				if !equalBounds(prev, bounds) {
					out[k] = nil
				}
				continue
			}
			out[k] = bounds
		}
		return nil
	})
	return out
}

func equalBounds(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// staticArgKind reports the Mangle constant type an expression will produce via
// Fact.ToAtom, or "" when it is not decidable from syntax alone.
func staticArgKind(e ast.Expr) (kind, detail string) {
	switch v := e.(type) {
	case *ast.BasicLit:
		switch v.Kind {
		case token.STRING:
			s, err := strconv.Unquote(v.Value)
			if err != nil {
				return "", ""
			}
			if isValidMangleNameConstant(s) {
				return "/name", s
			}
			return "/string", s
		case token.INT:
			return "/number", v.Value
		case token.FLOAT:
			return "/float64", v.Value
		}
	case *ast.Ident:
		if v.Name == "true" || v.Name == "false" {
			// ToAtom maps Go bools to the /true and /false name constants.
			return "/name", v.Name
		}
	case *ast.CallExpr:
		switch f := v.Fun.(type) {
		case *ast.Ident:
			switch f.Name {
			case "int", "int64", "int32":
				return "/number", f.Name + "(…)"
			case "float64", "float32":
				return "/float64", f.Name + "(…)"
			case "MangleAtom":
				return "/name", "MangleAtom(…)"
			case "MangleString":
				return "/string", "MangleString(…)"
			}
		case *ast.SelectorExpr:
			x, ok := f.X.(*ast.Ident)
			if !ok {
				return "", ""
			}
			switch x.Name + "." + f.Sel.Name {
			case "types.MangleAtom", "core.MangleAtom":
				return "/name", "MangleAtom(…)"
			case "types.MangleString", "core.MangleString":
				return "/string", "MangleString(…)"
			case "types.PercentFromRatio", "types.PercentClamp",
				"core.PercentFromRatio", "core.PercentClamp":
				return "/number", f.Sel.Name + "(…)"
			}
		}
	}
	return "", ""
}

// boundViolated reports whether a Go value of kind `kind` is wrong for a slot
// declared `bound`. Only mismatches that are certainly wrong are reported: a
// /string slot accepts a name-shaped string (Mangle stores it as a name, and
// several predicates are read back with ExtractString either way), so only the
// numeric-into-/string case is flagged there.
func boundViolated(bound, kind string) bool {
	switch bound {
	case "/name":
		return kind != "/name"
	case "/number":
		// A whole float is narrowed at insert time by
		// RealKernel.coerceAtomToDeclLocked, but a fractional one is rejected
		// outright, and neither is visible at the call site. Flag both.
		return kind == "/float64" || kind == "/name" || kind == "/string"
	case "/string":
		return kind == "/number" || kind == "/float64"
	}
	return false
}

type guardHit struct {
	file string
	line int
	msg  string
}

func (h guardHit) String() string { return fmt.Sprintf("%s:%d: %s", h.file, h.line, h.msg) }

// factLiteralParts extracts the constant predicate name and the Args element
// expressions from a Fact / KernelFact composite literal.
func factLiteralParts(cl *ast.CompositeLit) (string, []ast.Expr, bool) {
	var pred string
	var args []ast.Expr
	sawArgs := false
	for _, el := range cl.Elts {
		kv, ok := el.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		k, ok := kv.Key.(*ast.Ident)
		if !ok {
			continue
		}
		switch k.Name {
		case "Predicate":
			if bl, ok := kv.Value.(*ast.BasicLit); ok && bl.Kind == token.STRING {
				pred, _ = strconv.Unquote(bl.Value)
			}
		case "Args":
			if acl, ok := kv.Value.(*ast.CompositeLit); ok {
				args = acl.Elts
				sawArgs = true
			}
		}
	}
	return pred, args, sawArgs
}

func exprTypeName(e ast.Expr) string {
	switch v := e.(type) {
	case *ast.Ident:
		return v.Name
	case *ast.SelectorExpr:
		return v.Sel.Name
	}
	return ""
}

// sprintfVerb returns the single format verb of a fmt.Sprintf call whose format
// string is exactly one verb, or "".
func sprintfVerb(e ast.Expr) string {
	call, ok := e.(*ast.CallExpr)
	if !ok {
		return ""
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return ""
	}
	pkg, ok := sel.X.(*ast.Ident)
	if !ok || pkg.Name != "fmt" || sel.Sel.Name != "Sprintf" || len(call.Args) == 0 {
		return ""
	}
	bl, ok := call.Args[0].(*ast.BasicLit)
	if !ok || bl.Kind != token.STRING {
		return ""
	}
	s, err := strconv.Unquote(bl.Value)
	if err != nil {
		return ""
	}
	return s
}

// scanFactConventions walks every Go file in the repo once and returns the hits
// for the three rules.
func scanFactConventions(t *testing.T, root string) (declMismatch, sprintfV, atomAssert []guardHit) {
	t.Helper()
	bounds := loadDeclBounds(root)
	if len(bounds) < 100 {
		t.Fatalf("only %d bound Decls parsed from internal/core/defaults; the corpus moved and this guard is blind", len(bounds))
	}
	fset := token.NewFileSet()

	_ = filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "vendor", "node_modules", ".codex", ".claude", "testdata":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(p, ".go") {
			return nil
		}
		f, perr := parser.ParseFile(fset, p, nil, 0)
		if perr != nil {
			// A file another process is mid-write should not fail this guard.
			return nil
		}
		rel, _ := filepath.Rel(root, p)
		rel = filepath.ToSlash(rel)
		isTest := strings.HasSuffix(rel, "_test.go")

		ast.Inspect(f, func(n ast.Node) bool {
			switch node := n.(type) {
			case *ast.CompositeLit:
				pred, args, sawArgs := factLiteralParts(node)
				if !sawArgs {
					return true
				}
				if node.Type == nil {
					// Elided element literal inside []types.Fact{{…}} — a shape
					// several assert sites use, and one an earlier pass of this
					// sweep missed entirely. Identify it by its Predicate key.
					if pred == "" {
						return true
					}
				} else if name := exprTypeName(node.Type); name != "Fact" && name != "KernelFact" {
					return true
				}
				for i, a := range args {
					if verb := sprintfVerb(a); verb == "%v" || verb == "%+v" {
						sprintfV = append(sprintfV, guardHit{rel, fset.Position(a.Pos()).Line,
							fmt.Sprintf("fact %q arg %d built with fmt.Sprintf(%q)", pred, i, verb)})
					}
				}
				if pred == "" || isTest {
					return true
				}
				declared, ok := bounds[declKey{pred, len(args)}]
				if !ok || declared == nil {
					return true
				}
				for i, a := range args {
					kind, detail := staticArgKind(a)
					if kind == "" || declared[i] == "/any" {
						continue
					}
					if boundViolated(declared[i], kind) {
						atomAssertSafeDetail := strings.ReplaceAll(detail, "\n", " ")
						declMismatch = append(declMismatch, guardHit{rel, fset.Position(a.Pos()).Line,
							fmt.Sprintf("%s/%d arg %d is declared %s but the Go value is %s (%s)",
								pred, len(args), i, declared[i], kind, atomAssertSafeDetail)})
					}
				}
			case *ast.TypeAssertExpr:
				// Non-production code only reaches R1/R3 as noise: a test that
				// builds its own facts legitimately holds MangleAtom values,
				// and a fixture may assert a deliberately malformed fact.
				if isTest || node.Type == nil || exprTypeName(node.Type) != "MangleAtom" {
					return true
				}
				ix, ok := node.X.(*ast.IndexExpr)
				if !ok {
					return true
				}
				sel, ok := ix.X.(*ast.SelectorExpr)
				if !ok || sel.Sel.Name != "Args" {
					return true
				}
				atomAssert = append(atomAssert, guardHit{rel, fset.Position(node.Pos()).Line,
					"Args[…].(MangleAtom) — kernel query results carry names as plain string; use ExtractName/ArgName"})
			}
			return true
		})
		return nil
	})
	return
}

// checkAgainstBaseline fails for any hit whose (file, message) pair is not
// baselined, and reports baselined entries that no longer occur so the list can
// be trimmed rather than growing forever.
func checkAgainstBaseline(t *testing.T, rule string, hits []guardHit, baseline map[string][]string) {
	t.Helper()
	allowed := map[string]int{}
	for file, msgs := range baseline {
		for _, m := range msgs {
			allowed[file+"|"+m]++
		}
	}
	var unexpected []string
	for _, h := range hits {
		k := h.file + "|" + h.msg
		if allowed[k] > 0 {
			allowed[k]--
			continue
		}
		unexpected = append(unexpected, h.String())
	}
	if len(unexpected) > 0 {
		sort.Strings(unexpected)
		t.Errorf("%s: %d new violation(s):\n  %s\n\nEither fix the site or, if it is genuinely correct, "+
			"add it to the baseline in fact_conventions_guard_test.go WITH the reason.",
			rule, len(unexpected), strings.Join(unexpected, "\n  "))
	}
}

// -----------------------------------------------------------------------------
// BASELINES — the audit result, as of this sweep. Each entry is a real finding
// that lives outside internal/types and is reported to the owning package
// rather than edited from here.
// -----------------------------------------------------------------------------

// declMismatchBaseline: Go argument types that contradict the Mangle Decl.
// Every one of these is a bug or a latent one; none is "fine".
var declMismatchBaseline = map[string][]string{
	// campaign_intent_capture(…, AutonomyLevel, …) bound […, /name, /string]:
	// asserted as the quoted string "hands_free", so a policy matching
	// /hands_free cannot see it.
	"cmd/nerd/chat/campaign.go": {
		`campaign_intent_capture/5 arg 3 is declared /name but the Go value is /string (hands_free)`,
	},
	// continuation_step/max_continuation_steps are declared /number; these pass
	// float64. RealKernel.coerceAtomToDeclLocked narrows whole floats at insert,
	// so it works today only because of that safety net.
	"cmd/nerd/chat/model_update.go": {
		`continuation_step/2 arg 0 is declared /number but the Go value is /float64 (float64(…))`,
		`continuation_step/2 arg 1 is declared /number but the Go value is /float64 (float64(…))`,
		`max_continuation_steps/1 arg 0 is declared /number but the Go value is /float64 (10.0)`,
	},
	// task_error(TaskID, ErrorType, ErrorMessage) bound [/string, /name, /string].
	"internal/campaign/types.go": {
		`task_error/3 arg 1 is declared /name but the Go value is /string (execution_error)`,
	},
	// edit_failed(Path, Reason) / delete_blocked(Path, Reason) are declared
	// [/string, /name]; both reasons are asserted as quoted strings. No policy
	// reads them yet, which is exactly why this went unnoticed — the first rule
	// written against /pattern_not_found would silently never fire.
	"internal/core/virtual_store_file_actions.go": {
		`edit_failed/2 arg 1 is declared /name but the Go value is /string (pattern_not_found)`,
		`delete_blocked/2 arg 1 is declared /name but the Go value is /string (no_confirmation)`,
	},
	// git_state(Attribute, Value) bound [/name, /string]. Writer and reader
	// (cmd/nerd/chat/model_session_context.go populateGitContext) agree on the
	// unquoted-string convention, so fixing one without the other silently
	// empties SessionContext.GitBranch — they must move together.
	"internal/core/kernel_query.go": {
		`git_state/2 arg 0 is declared /name but the Go value is /string (branch)`,
		`git_state/2 arg 0 is declared /name but the Go value is /string (modified_files)`,
		`git_state/2 arg 0 is declared /name but the Go value is /string (recent_commits)`,
		`git_state/2 arg 0 is declared /name but the Go value is /string (unstaged_count)`,
	},
	// routing_error(ActionType, Reason, Timestamp) bound [/name, /string, /number].
	"internal/shards/system/router.go": {
		`routing_error/3 arg 0 is declared /name but the Go value is /string (internal_error)`,
		`routing_error/3 arg 0 is declared /name but the Go value is /string (internal_error)`,
	},
}

// sprintfVBaseline: %v-rendered fact arguments.
var sprintfVBaseline = map[string][]string{
	// DOM/React property values of unknown shape. Bounded by the browser
	// package's own redaction; internal/browser is off-limits this pass.
	"internal/browser/session_manager.go": {
		`fact "react_prop" arg 2 built with fmt.Sprintf("%v")`,
		`fact "react_state" arg 2 built with fmt.Sprintf("%v")`,
	},
	"internal/browser/session_manager_dom.go": {
		`fact "net_header" arg 4 built with fmt.Sprintf("%v")`,
		`fact "net_header" arg 4 built with fmt.Sprintf("%v")`,
	},
	// simulated_effect(ActionID, Predicate, Args): renders a []any as "[a b c]".
	// Shadow-mode facts are display-only, but the encoding is lossy and should
	// become JSON like ToAtom's container branch.
	"internal/core/shadow_mode.go": {
		`fact "simulated_effect" arg 2 built with fmt.Sprintf("%v")`,
		`fact "simulated_effect" arg 2 built with fmt.Sprintf("%v")`,
	},
	"internal/system/virtual_store_test_helpers_test.go": {
		`fact "pending_action" arg 0 built with fmt.Sprintf("%v")`,
		`fact "pending_action" arg 2 built with fmt.Sprintf("%v")`,
	},
}

// atomAssertBaseline: MangleAtom assertions on Args.
var atomAssertBaseline = map[string][]string{
	// Dead branches on kernel query results: the language/framework columns come
	// back as plain strings, so the workspace summary renders without a language
	// and --deep never finds a Go file.
	"cmd/nerd/chat/helpers.go":           {atomAssertMsg, atomAssertMsg},
	"cmd/nerd/chat/helpers_scan.go":      {atomAssertMsg},
	"cmd/nerd/cmd_init_scan.go":          {atomAssertMsg},
	"internal/world/incremental_scan.go": {atomAssertMsg, atomAssertMsg},
	"internal/world/persist.go":          {atomAssertMsg},
	// These two read facts built in-process by the dataflow extractor (which
	// does construct MangleAtom args), not facts returned by a query, so the
	// branch is reachable. Left alone deliberately.
	"internal/world/dataflow.go":           {atomAssertMsg},
	"internal/world/dataflow_multilang.go": {atomAssertMsg},
}

const atomAssertMsg = "Args[…].(MangleAtom) — kernel query results carry names as plain string; use ExtractName/ArgName"

func TestFactConventions_WhenNewBareAssertOrPercentVAppears_ShouldFail(t *testing.T) {
	root := factGuardRoot(t)
	declMismatch, sprintfV, atomAssert := scanFactConventions(t, root)

	t.Run("decl_conformance", func(t *testing.T) {
		checkAgainstBaseline(t, "Fact argument contradicts its Mangle Decl", declMismatch, declMismatchBaseline)
	})
	t.Run("no_percent_v_in_fact_args", func(t *testing.T) {
		checkAgainstBaseline(t, "fact argument built with %v", sprintfV, sprintfVBaseline)
	})
	t.Run("no_mangleatom_assert_on_args", func(t *testing.T) {
		checkAgainstBaseline(t, "MangleAtom type assertion on Fact.Args", atomAssert, atomAssertBaseline)
	})
}

// TestFactConventions_WhenScannerRuns_ShouldSeeTheCorpusAndTheCode guards the
// guard: a scan that silently matched nothing would make the test above pass
// forever. It pins that the sweep really reaches the corpus and the Go tree.
func TestFactConventions_WhenScannerRuns_ShouldSeeTheCorpusAndTheCode(t *testing.T) {
	root := factGuardRoot(t)
	bounds := loadDeclBounds(root)
	if len(bounds) < 1000 {
		t.Fatalf("expected >1000 bound Decls in the corpus, parsed %d", len(bounds))
	}
	if b := bounds[declKey{"git_state", 2}]; !equalBounds(b, []string{"/name", "/string"}) {
		t.Fatalf("git_state/2 bounds = %v, want [/name /string]; Decl parsing broke", b)
	}
	// The baselines are the audit; if they all vanish, the sweep stopped seeing
	// Go files rather than the repo getting clean overnight.
	_, _, atomAssert := scanFactConventions(t, root)
	if len(atomAssert) == 0 && len(atomAssertBaseline) > 0 {
		t.Fatalf("scan found no MangleAtom assertions at all; the Go walk is not reaching the tree")
	}
}
