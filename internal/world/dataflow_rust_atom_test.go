package world

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The old cut kept 30 characters and wrote "...", so these two calls became
// one atom. The shared prefix is the fixture: if it stops being 30 characters
// the collision this test pins is no longer the one the cut produced.
const (
	rustLongCallA = "a_very_long_function_name_aaaaaaaa(1)"
	rustLongCallB = "a_very_long_function_name_aaaaaaab(1)"
)

func TestRustExprAtom_LongCallsStayDistinct(t *testing.T) {
	if rustLongCallA[:30] != rustLongCallB[:30] {
		t.Fatal("fixture no longer shares the 30-character prefix the old cut kept")
	}
	atomA := string(rustExprAtom(rustLongCallA))
	atomB := string(rustExprAtom(rustLongCallB))
	if atomA == atomB {
		t.Fatalf("long calls collapsed to one atom: %s", atomA)
	}
	if strings.Contains(atomA, "...") || strings.Contains(atomB, "...") {
		t.Fatalf("atom was shortened: %s %s", atomA, atomB)
	}
	if !strings.Contains(atomA, "aaaaaaaa%281%29") || !strings.Contains(atomB, "aaaaaaab%281%29") {
		t.Fatalf("tails were not kept: %s %s", atomA, atomB)
	}
	for _, atom := range []string{atomA, atomB} {
		f := Fact{Predicate: "error_checked_return", Args: []any{MangleAtom(atom), "f.rs", int64(1)}}
		if _, err := f.ToAtom(); err != nil {
			t.Fatalf("ToAtom(%s): %v", atom, err)
		}
	}
}

func TestRustExprAtom_PercentIsInjective(t *testing.T) {
	encoded := string(rustExprAtom("a%b"))
	literal := string(rustExprAtom("a%25b"))
	if encoded != "/a%25b" {
		t.Fatalf("a%%b encoded to %s", encoded)
	}
	if literal != "/a%2525b" {
		t.Fatalf("a%%25b encoded to %s", literal)
	}
	if encoded == literal {
		t.Fatal("percent encoding collided")
	}
}

func TestRustTryOperator_KeepsWholeCallAndIdentifierSpelling(t *testing.T) {
	src := `fn check(input: &str) -> Result<(), Error> {
    let data = parse_data(input)?;
    let direct = data?;
    let a = a_very_long_function_name_aaaaaaaa(1)?;
    let b = a_very_long_function_name_aaaaaaab(1)?;
    let m = obj.a_very_long_method_name_cccccccc(2)?;
    Ok(())
}
`
	dir := t.TempDir()
	path := filepath.Join(dir, "check.rs")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	ex := NewMultiLangDataFlowExtractor()
	defer ex.Close()
	facts, err := ex.ExtractDataFlow(path)
	if err != nil {
		t.Fatal(err)
	}
	var atoms []string
	for _, f := range facts {
		if f.Predicate != "error_checked_return" || len(f.Args) == 0 {
			continue
		}
		atom, ok := f.Args[0].(MangleAtom)
		if !ok {
			t.Fatalf("error_checked_return arg is %T (%v); a bare identifier must stay a name constant", f.Args[0], f.Args[0])
		}
		atoms = append(atoms, string(atom))
		if _, err := f.ToAtom(); err != nil {
			t.Fatalf("ToAtom(%s): %v", atom, err)
		}
	}
	joined := strings.Join(atoms, "\n")
	for _, want := range []string{
		"/parse_data%28input%29",
		"/data",
		"/a_very_long_function_name_aaaaaaaa%281%29",
		"/a_very_long_function_name_aaaaaaab%281%29",
		"/obj.a_very_long_method_name_cccccccc%282%29",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("missing %s in:\n%s", want, joined)
		}
	}
	if strings.Contains(joined, "...") {
		t.Errorf("a call atom was shortened:\n%s", joined)
	}
}
