package core

import (
	"fmt"
	"reflect"
	"strings"
	"sync"

	"codeberg.org/TauCeti/mangle-go/ast"
	"codenerd/internal/logging"
)

// Asserting a predicate with no Decl is silent, and the fact is unreachable.
//
// factToAtom is Decl-blind, so a fact whose predicate was never declared
// converts cleanly, lands in the EDB, and returns nil from Assert — every
// signal a caller has says it worked. But the fixpoint only derives what the
// program declares, so Query never sees it. The fact is written into a space
// nothing reads.
//
// This is not a hypothetical typo. A static scan of non-test Go against the
// 1839 Decls a booted kernel loads found 82 distinct predicates asserted from
// production code with no declaration anywhere — whole state machines
// (campaign_paused, tdd_phase, ouroboros_phase, python_snapshot) whose facts
// have never been visible to a rule. It surfaced through a test that asserted
// a hundred facts concurrently, was told a hundred times that each succeeded,
// read back zero, and reported "Concurrency lost data".
//
// Assert deliberately does NOT start returning an error for these. Eighty-two
// live call sites would begin failing at once, and the honest fix for each is
// a per-predicate decision — declare it and wire a consumer, or drop the
// assert — not a blanket rejection chosen here. What was missing was any
// signal at all, so this provides one: a warning naming the predicate and its
// arity, once per predicate per process, plus a budget test
// (TestUndeclaredAssertBudget) that stops the count from growing.
//
// Arity is part of the identity on purpose. A fact asserted at an arity the
// Decl does not declare is invisible for exactly the same reason as one with
// no Decl at all, and reads as a much more confusing bug.
type undeclaredWarner struct {
	seen sync.Map // ast.PredicateSym -> struct{}
}

// warnIfUndeclaredLocked reports a predicate the loaded program does not
// declare. Call holding k.mu.
//
// Before the first evaluation programInfo is nil and nothing is known yet;
// staying quiet there avoids warning about every boot fact. Those facts are
// re-checked on later asserts of the same predicate only if they recur, which
// is the right trade: this is a diagnostic, not an accounting.
func (k *RealKernel) warnIfUndeclaredLocked(f Fact) {
	if k.sandbox {
		// Sandbox kernels trial-compile candidate rules; undeclared predicates
		// are an expected intermediate state there, not a defect.
		return
	}
	if k.programInfo == nil || k.programInfo.Decls == nil {
		return
	}
	sym := ast.PredicateSym{Symbol: f.Predicate, Arity: len(f.Args)}
	if _, declared := k.programInfo.Decls[sym]; declared {
		return
	}
	if _, already := k.undeclared.seen.LoadOrStore(sym, struct{}{}); already {
		return
	}
	logging.Get(logging.CategoryKernel).Warn(
		"Assert: %s/%d has no Decl — the fact is stored but no rule can read it, and Query will not return it. "+
			"Declare it (and give it a consumer) or drop the assert.",
		f.Predicate, len(f.Args))
}

// validateAgainstDeclLocked rejects a fact whose value contradicts its own
// declaration: a value outside a declared type bound. A predicate with no
// Decl for its symbol AND arity keeps today's behaviour (a warning),
// because the kernel is loaded incrementally and an undeclared fact is not
// necessarily wrong yet. Arity is part of predicate identity in Mangle, so
// state/1 and state/3 are different predicates.
func (k *RealKernel) validateAgainstDeclLocked(fact Fact) error {
	if k.programInfo == nil || k.programInfo.Decls == nil {
		return nil
	}
	sym := ast.PredicateSym{Symbol: fact.Predicate, Arity: len(fact.Args)}
	decl, ok := k.programInfo.Decls[sym]
	if !ok {
		k.warnIfUndeclaredLocked(fact)
		return nil
	}
	bounds := declBounds(decl)
	actual := len(fact.Args)
	for i, bound := range bounds {
		if i >= actual {
			break
		}
		if err := checkBoundAgainstValue(bound, fact.Args[i], fact.Predicate, i); err != nil {
			return err
		}
	}
	return nil
}

// checkBoundAgainstValue enforces one declared type bound.
//
// There is deliberately no /name case: at the Go boundary a /name value is a
// string, atom-shaped ("/coder") or bare ("data_integrity"), so nothing about
// the value distinguishes a violation. Name typing is checked by the Mangle
// analyzer where it can be checked.
func checkBoundAgainstValue(bound string, val any, pred string, idx int) error {
	switch bound {
	case "/number":
		if !isNumberArg(val) {
			return fmt.Errorf("type error asserting %s: arg %d declared %s, got %#v",
				pred, idx, bound, val)
		}
	case "/string":
		if !isStringArg(val) {
			return fmt.Errorf("type error asserting %s: arg %d declared %s, got %#v",
				pred, idx, bound, val)
		}
	default:
		return nil
	}
	return nil
}

func isNumberArg(v any) bool {
	if v == nil {
		return false
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return true
	default:
		return false
	}
}

func isStringArg(v any) bool {
	if v == nil {
		return false
	}
	return reflect.ValueOf(v).Kind() == reflect.String
}

func declBounds(decl any) []string {
	v := reflect.ValueOf(decl)
	for v.Kind() == reflect.Ptr || v.Kind() == reflect.Interface {
		if v.IsNil() {
			return nil
		}
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return extractBoundsFromString(fmt.Sprintf("%#v", decl))
	}
	var out []string
	for i := 0; i < v.NumField(); i++ {
		fv := v.Field(i)
		out = append(out, collectBounds(fv)...)
	}
	if len(out) == 0 {
		out = extractBoundsFromString(fmt.Sprintf("%#v", decl))
	}
	return out
}

func collectBounds(v reflect.Value) []string {
	for v.Kind() == reflect.Ptr || v.Kind() == reflect.Interface {
		if v.IsNil() {
			return nil
		}
		v = v.Elem()
	}
	switch v.Kind() {
	case reflect.String:
		s := strings.TrimSpace(v.String())
		if strings.HasPrefix(s, "/") {
			return []string{s}
		}
		// The AST may store a bound without its leading slash.
		// Map the lowercase type names exactly; capitalized arg names
		// like "Number"/"Text" never match this case-sensitive check.
		switch s {
		case "number":
			return []string{"/number"}
		case "string":
			return []string{"/string"}
		case "name":
			return []string{"/name"}
		}
		return nil
	case reflect.Slice, reflect.Array:
		var out []string
		for i := 0; i < v.Len(); i++ {
			out = append(out, collectBounds(v.Index(i))...)
		}
		return out
	case reflect.Struct:
		var out []string
		t := v.Type()
		for i := 0; i < v.NumField(); i++ {
			name := t.Field(i).Name
			lower := strings.ToLower(name)
			if strings.Contains(lower, "bound") || strings.Contains(lower, "type") {
				out = append(out, collectBounds(v.Field(i))...)
			}
		}
		if len(out) > 0 {
			return out
		}
		// Fallback: scan all string-like fields.
		for i := 0; i < v.NumField(); i++ {
			out = append(out, collectBounds(v.Field(i))...)
		}
		return out
	default:
		return nil
	}
}

func extractBoundsFromString(s string) []string {
	// Scan left-to-right so multi-arg bounds keep positional order.
	// The previous loop grouped by bound kind and discarded the prefix
	// before each hit, scrambling order and dropping bounds.
	// Slash forms ("/number") are matched directly; bare forms are matched
	// quoted ("number") so Go type names like []string in the %#v dump
	// never match. Lowercase matching ignores capitalized arg names
	// ("Number", "Text").
	type pat struct {
		needle string
		bound  string
	}
	pats := []pat{
		{"/number", "/number"},
		{"\"number\"", "/number"},
		{"/string", "/string"},
		{"\"string\"", "/string"},
		{"/name", "/name"},
	}
	var out []string
	for len(s) > 0 && len(out) < 16 {
		best := -1
		bestBound := ""
		bestLen := 0
		for _, p := range pats {
			if idx := strings.Index(s, p.needle); idx >= 0 && (best < 0 || idx < best) {
				best = idx
				bestBound = p.bound
				bestLen = len(p.needle)
			}
		}
		if best < 0 {
			break
		}
		out = append(out, bestBound)
		s = s[best+bestLen:]
	}
	return out
}
