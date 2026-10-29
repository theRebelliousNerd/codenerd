package system

import (
	"strings"
	"testing"

	"codenerd/internal/core"
)

// The defect this guards (F-JIT-4, observed live 1,209 times in one day):
//
//	[kernel] addFactIfNewLocked: rejecting fact that fails ToAtom: vector_hit -
//	Fact(vector_hit): unsupported arg type func() (float64, error) at index 1
//
// KernelAdapter.AssertBatch parses fact strings into core.Fact args, and for
// float constants it wrote:
//
//	args[i] = t.Float64Value
//
// Float64Value is a METHOD on ast.Constant — func (c Constant) Float64Value()
// (float64, error) — sitting immediately beside NumValue, which IS a field.
// Omitting the parentheses is legal Go: it yields a method value. So every
// float argument became a func() (float64, error) and the kernel rejected the
// whole fact.
//
// vector_hit(atomID, score) is how internal/prompt/selector.go feeds semantic
// ranking into Mangle flesh selection. Every score was dropped, so the
// selector fell back to keyword matching on every turn — the JIT compiler's
// semantic atom selection had never once run on real scores.

func newFloatTestKernel(t *testing.T) *core.RealKernel {
	t.Helper()
	k, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel: %v", err)
	}
	return k
}

// A float constant must convert to a float64, not to a method value. The
// predicate here is deliberately NOT vector_hit: vector_hit is declared
// `bound [/string, /number]`, so a float argument is correctly rejected by the
// declaration regardless of this conversion — see the companion fix in
// internal/prompt/selector.go, which now emits an integer percentage. This test
// isolates the conversion itself.
func TestKernelAdapter_AssertBatch_FloatArgIsNotAMethodValue(t *testing.T) {
	k := newFloatTestKernel(t)
	adapter := &KernelAdapter{kernel: k}

	// float_probe is undeclared, so the kernel may or may not retain it; what
	// matters is that conversion does not produce a func value, which used to
	// fail ToAtom with "unsupported arg type func() (float64, error)".
	if err := adapter.AssertBatch([]any{`float_probe("atom_x", 0.875).`}); err != nil {
		if strings.Contains(err.Error(), "func() (float64, error)") {
			t.Fatalf("float arg became a method value: %v", err)
		}
	}

	facts, err := k.Query("float_probe")
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	for _, f := range facts {
		if len(f.Args) < 2 {
			continue
		}
		switch v := f.Args[1].(type) {
		case float64:
			if v != 0.875 {
				t.Errorf("score = %v, want 0.875", v)
			}
		case func() (float64, error):
			t.Fatal("arg is a method value — Float64Value was referenced without calling it")
		default:
			t.Errorf("score has type %T, want float64", v)
		}
	}
}

// Integers must keep working: NumValue really is a field, so the fix must not
// be applied uniformly to both cases.
func TestKernelAdapter_AssertBatch_IntArgStillWorks(t *testing.T) {
	k := newFloatTestKernel(t)
	adapter := &KernelAdapter{kernel: k}

	if err := adapter.AssertBatch([]any{`vector_hit("atom_y", 3).`}); err != nil {
		t.Fatalf("AssertBatch rejected an integer fact: %v", err)
	}

	facts, err := k.Query("vector_hit")
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(facts) == 0 {
		t.Fatal("integer vector_hit was dropped")
	}
	if _, ok := facts[0].Args[1].(int64); !ok {
		t.Errorf("integer arg has type %T, want int64", facts[0].Args[1])
	}
}

// The real production shape: integer-scaled scores against the declared
// /number bound. Every one must land, because a single rejected arg used to
// take the whole selector's ranking with it.
func TestKernelAdapter_AssertBatch_IntegerScaledScoresAllLand(t *testing.T) {
	k := newFloatTestKernel(t)
	adapter := &KernelAdapter{kernel: k}

	err := adapter.AssertBatch([]any{
		`vector_hit("a", 50).`,
		`vector_hit("b", 100).`,
		`vector_hit("c", 31).`,
	})
	if err != nil {
		t.Fatalf("AssertBatch: %v", err)
	}

	facts, err := k.Query("vector_hit")
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(facts) != 3 {
		t.Fatalf("got %d facts, want 3: %+v", len(facts), facts)
	}
}
