package types

import (
	"testing"

	"github.com/google/mangle/ast"
)

func TestIsValidMangleNameConstant(t *testing.T) {
	if !isValidMangleNameConstant("/valid") {
		t.Fatalf("expected /valid to be a valid name constant")
	}
	if isValidMangleNameConstant("valid") {
		t.Fatalf("expected valid without leading slash to be invalid")
	}
	if isValidMangleNameConstant("/") {
		t.Fatalf("expected / to be invalid")
	}
	if isValidMangleNameConstant("/bad//name") {
		t.Fatalf("expected /bad//name to be invalid")
	}
	if isValidMangleNameConstant(`/bad"quote`) {
		t.Fatalf("expected name with quotes to be invalid")
	}
}

func TestFactString(t *testing.T) {
	fact := Fact{
		Predicate: "test",
		Args: []interface{}{
			MangleAtom("/name"),
			"/valid",
			"/bad//name",
			"plain",
			1,
			int64(2),
			float64(0.5),
			float64(42.7),
			true,
			false,
		},
	}

	got := fact.String()
	want := `test(/name, /valid, "/bad//name", "plain", 1, 2, 0.500000, 42.700000, /true, /false).`
	if got != want {
		t.Fatalf("unexpected fact string:\nwant: %s\ngot:  %s", want, got)
	}
}

func TestFactToAtomConversion(t *testing.T) {
	fact := Fact{
		Predicate: "test",
		Args: []interface{}{
			MangleAtom("/name"),
			MangleAtom("not-atom"),
			"/valid",
			"/bad//name",
			"plain",
			int(3),
			int64(4),
			float64(0.42),
			float64(2.7),
			true,
			false,
		},
	}

	atom, err := fact.ToAtom()
	if err != nil {
		t.Fatalf("unexpected ToAtom error: %v", err)
	}
	if atom.Predicate.Symbol != "test" {
		t.Fatalf("unexpected predicate symbol: %s", atom.Predicate.Symbol)
	}
	if len(atom.Args) != len(fact.Args) {
		t.Fatalf("unexpected arg count: %d", len(atom.Args))
	}

	assertNameConstant(t, atom.Args[0], "/name")
	assertStringConstant(t, atom.Args[1], "not-atom")
	assertNameConstant(t, atom.Args[2], "/valid")
	assertStringConstant(t, atom.Args[3], "/bad//name")
	assertStringConstant(t, atom.Args[4], "plain")
	assertNumberConstant(t, atom.Args[5], 3)
	assertNumberConstant(t, atom.Args[6], 4)
	assertNumberConstant(t, atom.Args[7], 42)  // 0.42 * 100
	assertNumberConstant(t, atom.Args[8], 270) // 2.7 * 100
	assertNameConstant(t, atom.Args[9], "/true")
	assertNameConstant(t, atom.Args[10], "/false")
}

func TestFactToAtomInvalidMangleAtom(t *testing.T) {
	fact := Fact{
		Predicate: "test",
		Args: []interface{}{
			MangleAtom("/bad//name"),
		},
	}
	if _, err := fact.ToAtom(); err == nil {
		t.Fatalf("expected error for invalid mangle atom")
	}
}

func TestKernelFactToFact(t *testing.T) {
	kf := KernelFact{
		Predicate: "pred",
		Args:      []interface{}{"arg"},
	}
	fact := kf.ToFact()
	if fact.Predicate != "pred" || len(fact.Args) != 1 || fact.Args[0] != "arg" {
		t.Fatalf("unexpected ToFact conversion")
	}
}

func assertNameConstant(t *testing.T, term ast.BaseTerm, want string) {
	t.Helper()
	c, ok := term.(ast.Constant)
	if !ok {
		t.Fatalf("expected constant term")
	}
	if c.Type != ast.NameType {
		t.Fatalf("expected NameType, got %v", c.Type)
	}
	if c.Symbol != want {
		t.Fatalf("expected symbol %q, got %q", want, c.Symbol)
	}
}

func assertStringConstant(t *testing.T, term ast.BaseTerm, want string) {
	t.Helper()
	c, ok := term.(ast.Constant)
	if !ok {
		t.Fatalf("expected constant term")
	}
	if c.Type != ast.StringType {
		t.Fatalf("expected StringType, got %v", c.Type)
	}
	if c.Symbol != want {
		t.Fatalf("expected symbol %q, got %q", want, c.Symbol)
	}
}

func assertNumberConstant(t *testing.T, term ast.BaseTerm, want int64) {
	t.Helper()
	c, ok := term.(ast.Constant)
	if !ok {
		t.Fatalf("expected constant term")
	}
	if c.Type != ast.NumberType {
		t.Fatalf("expected NumberType, got %v", c.Type)
	}
	if c.NumValue != want {
		t.Fatalf("expected number %d, got %d", want, c.NumValue)
	}
}
