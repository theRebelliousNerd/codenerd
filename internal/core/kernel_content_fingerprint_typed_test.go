package core

import (
	"encoding/json"
	"testing"
	"time"
)

// An /any slot admits int64(1) as a Number and float64(1) as a Float64, and
// Mangle tells them apart: the literal premise fp_value(1) matches only the
// Number. The EDB content sum used to hash each fact's deduplication key,
// which spells both "1", so the two kernels had one fingerprint while
// answering differently (audit mangle-content-fingerprint-typed-identity).
const typedFingerprintPolicy = `
Decl fp_value(V) bound [/any].
Decl fp_integer_present() bound [].
fp_integer_present() :- fp_value(1).
`

func typedFingerprintKernel(t *testing.T) *RealKernel {
	t.Helper()
	k := newRealKernelBase()
	k.AppendPolicy(typedFingerprintPolicy)
	if err := k.Evaluate(); err != nil {
		t.Fatal(err)
	}
	return k
}

func integerPresent(t *testing.T, k *RealKernel) bool {
	t.Helper()
	got, err := k.Query("fp_integer_present")
	if err != nil {
		t.Fatal(err)
	}
	return len(got) > 0
}

func TestContentFingerprint_IntegerAndIntegralFloatAreDifferentContent(t *testing.T) {
	a, b := typedFingerprintKernel(t), typedFingerprintKernel(t)
	if err := a.Assert(Fact{Predicate: "fp_value", Args: []any{int64(1)}}); err != nil {
		t.Fatal(err)
	}
	if err := b.Assert(Fact{Predicate: "fp_value", Args: []any{float64(1)}}); err != nil {
		t.Fatal(err)
	}
	// The pair is semantically different: only the Number satisfies the rule.
	if !integerPresent(t, a) || integerPresent(t, b) {
		t.Fatalf("precondition: fp_integer_present derived int=%v float=%v, want true/false",
			integerPresent(t, a), integerPresent(t, b))
	}
	if mustFingerprint(t, a) == mustFingerprint(t, b) {
		t.Fatal("kernels holding int64(1) and float64(1) under an /any slot share a content fingerprint but answer fp_integer_present differently")
	}
}

// The same transition inside one kernel: retract the integer, admit the
// float. The fingerprint must leave the integer state, and putting the
// integer back must return to it exactly.
func TestContentFingerprint_RetractAndReplaceWithIntegralFloat(t *testing.T) {
	k := typedFingerprintKernel(t)
	intFact := Fact{Predicate: "fp_value", Args: []any{int64(1)}}
	floatFact := Fact{Predicate: "fp_value", Args: []any{float64(1)}}
	if err := k.Assert(intFact); err != nil {
		t.Fatal(err)
	}
	intState := mustFingerprint(t, k)
	if err := k.RetractExactFact(intFact); err != nil {
		t.Fatal(err)
	}
	if err := k.Assert(floatFact); err != nil {
		t.Fatal(err)
	}
	if integerPresent(t, k) {
		t.Fatal("precondition: the float state still derives fp_integer_present")
	}
	if mustFingerprint(t, k) == intState {
		t.Fatal("replacing int64(1) with float64(1) left the content fingerprint unchanged")
	}
	if err := k.RetractExactFact(floatFact); err != nil {
		t.Fatal(err)
	}
	if err := k.Assert(intFact); err != nil {
		t.Fatal(err)
	}
	if got := mustFingerprint(t, k); got != intState {
		t.Fatalf("restoring int64(1) gave fingerprint %s, want the integer state's %s", got, intState)
	}
}

// Every host value pair below shares a deduplication key but is a different
// value to admission, so no pair may share a fingerprint contribution.
func TestEDBDigest_DistinguishesValuesTheDedupKeyErases(t *testing.T) {
	pairs := []struct {
		name string
		a, b any
	}{
		{"int64 vs float64", int64(1), float64(1)},
		{"int vs uint", 1, uint(1)},
		{"string vs atom", "/a", MangleAtom("/a")},
		{"bool vs name string", true, "/true"},
		{"duration vs string", time.Second, "1s"},
		{"json numbers", json.Number("1"), json.Number("1.0")},
		{"list of bool vs list of name", []any{true}, []any{"/true"}},
		{"list of int vs list of float", []any{int64(1)}, []any{float64(1)}},
		{"map values", map[string]any{"k": int64(2)}, map[string]any{"k": float64(2)}},
	}
	for _, p := range pairs {
		fa := Fact{Predicate: "fp_value", Args: []any{p.a}}
		fb := Fact{Predicate: "fp_value", Args: []any{p.b}}
		if canonValue(p.a) != canonValue(p.b) {
			t.Errorf("%s: the dedup keys already differ (%q, %q); the pair no longer pins the defect", p.name, canonValue(p.a), canonValue(p.b))
			continue
		}
		if edbDigest(fa) == edbDigest(fb) {
			t.Errorf("%s: %T and %T share a content digest", p.name, p.a, p.b)
		}
	}
	// One value always has one digest, whatever the map iteration order.
	m := map[any]any{1: "x", "1": "y", int64(1): "z"}
	f := Fact{Predicate: "fp_value", Args: []any{m}}
	want := edbDigest(f)
	for i := 0; i < 50; i++ {
		if edbDigest(f) != want {
			t.Fatal("one held fact gave two digests: insert and remove would not cancel")
		}
	}
}
