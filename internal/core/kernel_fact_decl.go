package core

import (
	"fmt"
	"math"
	"slices"
	"strings"
	"sync"
	"time"

	"codenerd/internal/logging"

	"codeberg.org/TauCeti/mangle-go/analysis"
	"codeberg.org/TauCeti/mangle-go/ast"
	"codeberg.org/TauCeti/mangle-go/symbols"
)

// =============================================================================
// TYPED ADMISSION: DECLARATIONS AT THE GO BOUNDARY
// =============================================================================
//
// A Decl's bounds are a disjunction of whole tuples: `bound [/name, /name]
// bound [/name, /string]` admits (name, name) or (name, string), and a value is
// valid when SOME alternative accepts EVERY argument. The previous boundary
// flattened every alternative into one list of strings by reflection and then
// checked /number and /string positionally, so a second alternative shifted
// the positions and nested or crossed tuples were never checked at all.
//
// Admission now runs on the converted atom against the analyzed (desugared)
// Decls, with the same primitive the upstream runtime checker uses
// (symbols.TypeHandle.HasType, which is what builtin.TypeChecker.CheckOneBoundDecl
// evaluates). Handles are compiled once per predicate per program instead of
// once per argument per fact; TestTypedAdmissionAgreesWithUpstreamTypeChecker
// pins the equivalence over the production corpus.
//
// Before the check, a small Decl-directed adapter repairs the cases where
// types.Fact.ToAtom (Decl-blind) chose an encoding the Go value did not mean:
//
//   - an integral float64 in a slot only /number accepts becomes ast.Number,
//     a fractional, non-finite or out-of-range one is rejected (the pinned
//     fork compares int64 only; one Float64 in a compared slot aborts the
//     whole fixpoint);
//   - a time.Duration or time.Time in a slot only /number accepts becomes its
//     nanosecond count, which is how every timestamp predicate is declared;
//   - a plain Go string that ToAtom read as a name ("/tmp", "/coder") becomes
//     a string when the slot does not accept the name but does accept the
//     string. The Go type said string; only the shape heuristic said name.
//
// The adapter never prefixes a bare string into a name: that would be an
// implicit reinterpretation, and a name the producer did not write.
//
// One documented tolerance remains. Go has no name type, so production code
// passes bare strings ("coder", "data_integrity") into /name slots, and ToAtom
// encodes them as strings. Rejecting them would refuse a large number of live
// producers at once, and their facts are readable by Go consumers that query
// them back. Such a value is admitted against a bare /name bound only, and the
// predicate/argument is reported once per process (lenientNameWarning) so each
// producer can be fixed at its root. Every other bound -- including crossed
// alternatives and MangleAtom values in /string slots -- is enforced.

// predAdmission is the compiled type check of one declared predicate.
type predAdmission struct {
	sym ast.PredicateSym
	// handles[alt][arg] is the type of argument arg in bound alternative alt.
	handles [][]symbols.TypeHandle
	// bounds mirrors handles with the raw bound expressions, for diagnostics
	// and for recognising the simple /number and /name bounds.
	bounds [][]ast.BaseTerm
}

// admissionSchema caches predAdmissions for one analyzed program. It is
// per-kernel and filled lazily, so a clone that admits three predicates
// compiles three, and a new program (a different *ProgramInfo) starts over.
type admissionSchema struct {
	mu    sync.Mutex
	info  *analysis.ProgramInfo
	preds map[ast.PredicateSym]*predAdmission
}

// lenientNameWarnings reports each predicate/argument that relied on the
// bare-string /name tolerance, once per process.
var lenientNameWarnings sync.Map // string -> struct{}

// admissionForLocked returns the compiled admission of pred, or nil when the
// program does not declare it (the undeclared path warns instead). Call
// holding k.mu.
func (k *RealKernel) admissionForLocked(pred ast.PredicateSym) (*predAdmission, error) {
	if k.programInfo == nil || k.programInfo.Decls == nil {
		return nil, nil
	}
	decl, ok := k.programInfo.Decls[pred]
	if !ok || decl == nil {
		return nil, nil
	}
	if k.admission == nil || k.admission.info != k.programInfo {
		k.admission = &admissionSchema{info: k.programInfo, preds: make(map[ast.PredicateSym]*predAdmission)}
	}
	s := k.admission
	s.mu.Lock()
	defer s.mu.Unlock()
	if pa, ok := s.preds[pred]; ok {
		return pa, nil
	}
	pa, err := compileAdmission(pred, decl)
	if err != nil {
		return nil, err
	}
	s.preds[pred] = pa
	return pa, nil
}

// compileAdmission builds the handles of one desugared Decl. A desugared Decl
// carries one bound per argument in every alternative (an unbounded Decl
// desugars to a single all-/any tuple), so a tuple whose length is not the
// arity is a malformed Decl, reported rather than indexed past.
func compileAdmission(pred ast.PredicateSym, decl *ast.Decl) (*predAdmission, error) {
	pa := &predAdmission{sym: pred}
	for alt, bd := range decl.Bounds {
		if len(bd.Bounds) != pred.Arity {
			return nil, fmt.Errorf("Decl %s/%d bound alternative %d has %d bounds", pred.Symbol, pred.Arity, alt, len(bd.Bounds))
		}
		handles := make([]symbols.TypeHandle, len(bd.Bounds))
		for j, b := range bd.Bounds {
			h, err := symbols.NewBoundHandle(b)
			if err != nil {
				return nil, fmt.Errorf("Decl %s/%d bound alternative %d arg %d: %w", pred.Symbol, pred.Arity, alt, j, err)
			}
			handles[j] = h
		}
		pa.handles = append(pa.handles, handles)
		pa.bounds = append(pa.bounds, bd.Bounds)
	}
	return pa, nil
}

// acceptsAt reports whether some alternative admits c at argument j. Used by
// the adapter to choose an encoding; the tuple check below is the verdict.
func (pa *predAdmission) acceptsAt(j int, c ast.Constant) bool {
	for _, alt := range pa.handles {
		if alt[j].HasType(c) {
			return true
		}
	}
	return false
}

// boundIsAt reports whether some alternative's bound at j is exactly want.
func (pa *predAdmission) boundIsAt(j int, want ast.Constant) bool {
	for _, alt := range pa.bounds {
		if c, ok := alt[j].(ast.Constant); ok && c.Equals(want) {
			return true
		}
	}
	return false
}

// admitAtomLocked adapts atom's encodings to the Decl of its predicate and
// then checks it against the Decl's bound alternatives. raw holds the Go values
// the atom was converted from; the adapter needs them to tell a Go string
// from a name. Undeclared predicates pass unchanged. Call holding k.mu.
func (k *RealKernel) admitAtomLocked(atom ast.Atom, raw []any) (ast.Atom, error) {
	pa, err := k.admissionForLocked(atom.Predicate)
	if err != nil || pa == nil {
		return atom, err
	}
	if len(atom.Args) != pa.sym.Arity {
		return ast.Atom{}, fmt.Errorf("%s has %d arguments, its Decl declares %d", atom.Predicate.Symbol, len(atom.Args), pa.sym.Arity)
	}
	if len(pa.handles) == 0 {
		return atom, nil // a nullary or bound-free Decl constrains nothing
	}
	adapted, err := pa.adapt(atom, raw)
	if err != nil {
		return ast.Atom{}, err
	}
	if err := pa.check(adapted, raw); err != nil {
		return ast.Atom{}, err
	}
	return adapted, nil
}

// adapt applies the Decl-directed encoding repairs described at the top of
// this file. It only ever replaces an argument some alternative rejects.
func (pa *predAdmission) adapt(atom ast.Atom, raw []any) (ast.Atom, error) {
	var out []ast.BaseTerm
	set := func(j int, t ast.BaseTerm) {
		if out == nil {
			out = slices.Clone(atom.Args)
		}
		out[j] = t
	}
	for j, arg := range atom.Args {
		c, ok := arg.(ast.Constant)
		if !ok || pa.acceptsAt(j, c) {
			continue
		}
		var rv any
		if j < len(raw) {
			rv = raw[j]
		}
		switch c.Type {
		case ast.Float64Type:
			if !pa.boundIsAt(j, ast.NumberBound) {
				continue
			}
			n, err := integralNumber(atom.Predicate.Symbol, j, c)
			if err != nil {
				return ast.Atom{}, err
			}
			set(j, n)
		case ast.DurationType, ast.TimeType:
			var nanos int64
			switch v := rv.(type) {
			case time.Duration:
				nanos = int64(v)
			case time.Time:
				nanos = v.UnixNano()
			default:
				continue
			}
			if n := ast.Number(nanos); pa.acceptsAt(j, n) {
				set(j, n)
			}
		case ast.NameType:
			s, plain := rv.(string)
			if !plain {
				continue // MangleAtom and friends meant a name; the check decides
			}
			if str := ast.String(s); pa.acceptsAt(j, str) {
				set(j, str)
			}
		}
	}
	if out == nil {
		return atom, nil
	}
	return ast.Atom{Predicate: atom.Predicate, Args: out}, nil
}

// integralNumber narrows a float constant bound for a /number slot, refusing
// what int64 cannot hold exactly. Integral but unrepresentable: float64→int64
// conversion of an out-of-range value is implementation-defined (amd64 yields
// MinInt64), so a huge "integer" would silently become a wildly wrong one. The
// valid range is [-2^63, 2^63).
func integralNumber(pred string, j int, c ast.Constant) (ast.Constant, error) {
	f, err := c.Float64Value()
	if err != nil {
		return ast.Constant{}, fmt.Errorf("%s arg %d: unreadable float: %w", pred, j, err)
	}
	if math.IsNaN(f) || math.IsInf(f, 0) || f != math.Trunc(f) {
		return ast.Constant{}, fmt.Errorf(
			"%s arg %d is declared %s but got the non-integral float %v; "+
				"scale ratios to integer percent at the assert site (see types.PercentScale)",
			pred, j, numberBoundSymbol, f)
	}
	if f >= 1<<63 || f < -(1<<63) {
		return ast.Constant{}, fmt.Errorf(
			"%s arg %d is declared %s but got %v, outside the int64 range",
			pred, j, numberBoundSymbol, f)
	}
	return ast.Number(int64(f)), nil
}

// check is the verdict: some alternative must accept every argument. It
// returns the first failing argument of each alternative, so a one-tuple Decl
// reads "arg 1 declared /number, got ...".
func (pa *predAdmission) check(atom ast.Atom, raw []any) error {
	var reasons []string
	for alt, handles := range pa.handles {
		failed := -1
		lenient := -1
		for j, h := range handles {
			c, ok := atom.Args[j].(ast.Constant)
			if !ok {
				failed = j
				break
			}
			if h.HasType(c) {
				continue
			}
			if bareStringInNameSlot(pa.bounds[alt][j], c, raw, j) {
				lenient = j
				continue
			}
			failed = j
			break
		}
		if failed < 0 {
			if lenient >= 0 {
				noteLenientName(pa.sym, lenient)
			}
			return nil
		}
		reasons = append(reasons, fmt.Sprintf("arg %d declared %v, got %v", failed, pa.bounds[alt][failed], atom.Args[failed]))
	}
	if len(reasons) == 1 {
		return fmt.Errorf("type error asserting %s: %s", atom.Predicate.Symbol, reasons[0])
	}
	for i := range reasons {
		reasons[i] = fmt.Sprintf("alternative %d: %s", i, reasons[i])
	}
	return fmt.Errorf("type error asserting %s: no declared bound alternative accepts it (%s)",
		atom.Predicate.Symbol, strings.Join(reasons, "; "))
}

// bareStringInNameSlot is the one tolerance: a plain Go string ToAtom encoded
// as a string, against a bound that is exactly /name.
func bareStringInNameSlot(bound ast.BaseTerm, c ast.Constant, raw []any, j int) bool {
	b, ok := bound.(ast.Constant)
	if !ok || !b.Equals(ast.NameBound) || c.Type != ast.StringType || j >= len(raw) {
		return false
	}
	_, plain := raw[j].(string)
	return plain
}

func noteLenientName(sym ast.PredicateSym, j int) {
	key := fmt.Sprintf("%s/%d#%d", sym.Symbol, sym.Arity, j)
	if _, seen := lenientNameWarnings.LoadOrStore(key, struct{}{}); seen {
		return
	}
	logging.Get(logging.CategoryKernel).Warn(
		"admission: %s/%d arg %d is declared /name but its producer passed a bare Go string; it is stored as a string no /name rule can match — pass types.MangleAtom(\"/...\") at the assert site",
		sym.Symbol, sym.Arity, j)
}

// numberBoundSymbol is the Decl bound that means "int64" in the corpus.
const numberBoundSymbol = "/number"

// factToAtomLocked converts a fact and admits it against its Decl. Every path
// that fills k.cachedAtoms or the store must go through here, otherwise an
// unadmitted atom reaches the fixpoint. Call holding k.mu.
func (k *RealKernel) factToAtomLocked(f Fact) (ast.Atom, error) {
	atom, err := f.ToAtom()
	if err != nil {
		return ast.Atom{}, err
	}
	// Before the first evaluate() there is no programInfo, so no bounds are
	// known yet. Mark the cache so evaluate reconverts once the Decls exist
	// rather than leaving those boot facts unadmitted forever.
	if k.programInfo == nil {
		k.atomCacheStale = true
		return atom, nil
	}
	return k.admitAtomLocked(atom, f.Args)
}
