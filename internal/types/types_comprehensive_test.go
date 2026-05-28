package types

import (
	"context"
	"testing"
	"time"

	"github.com/google/mangle/ast"
)

// =============================================================================
// ToAtom() Comprehensive Tests
// =============================================================================

func TestToAtom_WhenStringArg_ShouldProduceStringConstant(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		input string
	}{
		{"plain string", "hello world"},
		{"empty string", ""},
		{"string with special chars", `line1\nline2`},
		{"string with quotes", `she said "hi"`},
		{"file path starting with /", "/mnt/c/path/to/file.go"},
		{"string with unicode", "日本語テスト"},
		{"string with slashes but file ext", "/foo/bar.go"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			fact := Fact{Predicate: "test_str", Args: []any{tt.input}}
			atom, err := fact.ToAtom()
			if err != nil {
				t.Fatalf("ToAtom() error: %v", err)
			}
			if len(atom.Args) != 1 {
				t.Fatalf("expected 1 arg, got %d", len(atom.Args))
			}
			assertStringConstant(t, atom.Args[0], tt.input)
		})
	}
}

func TestToAtom_WhenNameConstant_ShouldProduceNameType(t *testing.T) {
	t.Parallel()
	validNames := []string{"/true", "/false", "/coder", "/active", "/read_file"}
	for _, name := range validNames {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			fact := Fact{Predicate: "test_name", Args: []any{name}}
			atom, err := fact.ToAtom()
			if err != nil {
				t.Fatalf("ToAtom() error: %v", err)
			}
			assertNameConstant(t, atom.Args[0], name)
		})
	}
}

func TestToAtom_WhenMangleAtomValid_ShouldProduceNameType(t *testing.T) {
	t.Parallel()
	fact := Fact{Predicate: "test_matom", Args: []any{MangleAtom("/valid")}}
	atom, err := fact.ToAtom()
	if err != nil {
		t.Fatalf("ToAtom() error: %v", err)
	}
	assertNameConstant(t, atom.Args[0], "/valid")
}

func TestToAtom_WhenMangleAtomWithoutSlash_ShouldFallbackToString(t *testing.T) {
	t.Parallel()
	fact := Fact{Predicate: "test_matom_fallback", Args: []any{MangleAtom("no-slash")}}
	atom, err := fact.ToAtom()
	if err != nil {
		t.Fatalf("ToAtom() error: %v", err)
	}
	assertStringConstant(t, atom.Args[0], "no-slash")
}

func TestToAtom_WhenMangleAtomInvalid_ShouldReturnError(t *testing.T) {
	t.Parallel()
	fact := Fact{Predicate: "test_matom_err", Args: []any{MangleAtom("/bad//name")}}
	_, err := fact.ToAtom()
	if err == nil {
		t.Fatal("expected error for invalid MangleAtom, got nil")
	}
}

func TestToAtom_WhenIntArg_ShouldProduceNumberType(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		arg  any
		want int64
	}{
		{"int zero", int(0), 0},
		{"int positive", int(42), 42},
		{"int negative", int(-7), -7},
		{"int64 zero", int64(0), 0},
		{"int64 large", int64(1 << 40), 1 << 40},
		{"int64 negative", int64(-999), -999},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			fact := Fact{Predicate: "test_int", Args: []any{tt.arg}}
			atom, err := fact.ToAtom()
			if err != nil {
				t.Fatalf("ToAtom() error: %v", err)
			}
			assertNumberConstant(t, atom.Args[0], tt.want)
		})
	}
}

func TestToAtom_WhenFloat64Arg_ShouldProduceFloat64Type(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		arg  float64
		want float64
	}{
		{"zero", 0.0, 0.0},
		{"positive", 3.14, 3.14},
		{"negative", -2.5, -2.5},
		{"very small", 0.000001, 0.000001},
		{"large", 1e10, 1e10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			fact := Fact{Predicate: "test_f64", Args: []any{tt.arg}}
			atom, err := fact.ToAtom()
			if err != nil {
				t.Fatalf("ToAtom() error: %v", err)
			}
			assertFloat64Constant(t, atom.Args[0], tt.want)
		})
	}
}

func TestToAtom_WhenFloat32Arg_ShouldProduceFloat64Type(t *testing.T) {
	t.Parallel()
	fact := Fact{Predicate: "test_f32", Args: []any{float32(2.5)}}
	atom, err := fact.ToAtom()
	if err != nil {
		t.Fatalf("ToAtom() error: %v", err)
	}
	assertFloat64Constant(t, atom.Args[0], 2.5)
}

func TestToAtom_WhenBoolArg_ShouldProduceTrueOrFalseConstant(t *testing.T) {
	t.Parallel()
	t.Run("true", func(t *testing.T) {
		t.Parallel()
		fact := Fact{Predicate: "test_bool", Args: []any{true}}
		atom, err := fact.ToAtom()
		if err != nil {
			t.Fatalf("ToAtom() error: %v", err)
		}
		assertNameConstant(t, atom.Args[0], "/true")
	})
	t.Run("false", func(t *testing.T) {
		t.Parallel()
		fact := Fact{Predicate: "test_bool", Args: []any{false}}
		atom, err := fact.ToAtom()
		if err != nil {
			t.Fatalf("ToAtom() error: %v", err)
		}
		assertNameConstant(t, atom.Args[0], "/false")
	})
}

func TestToAtom_WhenTimeArg_ShouldProduceTimeType(t *testing.T) {
	t.Parallel()
	now := time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)
	fact := Fact{Predicate: "test_time", Args: []any{now}}
	atom, err := fact.ToAtom()
	if err != nil {
		t.Fatalf("ToAtom() error: %v", err)
	}
	c, ok := atom.Args[0].(ast.Constant)
	if !ok {
		t.Fatalf("expected constant term, got %T", atom.Args[0])
	}
	if c.Type != ast.TimeType {
		t.Errorf("expected TimeType, got %v", c.Type)
	}
}

func TestToAtom_WhenDurationArg_ShouldProduceDurationType(t *testing.T) {
	t.Parallel()
	dur := 5 * time.Second
	fact := Fact{Predicate: "test_dur", Args: []any{dur}}
	atom, err := fact.ToAtom()
	if err != nil {
		t.Fatalf("ToAtom() error: %v", err)
	}
	c, ok := atom.Args[0].(ast.Constant)
	if !ok {
		t.Fatalf("expected constant term, got %T", atom.Args[0])
	}
	if c.Type != ast.DurationType {
		t.Errorf("expected DurationType, got %v", c.Type)
	}
}

func TestToAtom_WhenNilArg_ShouldProduceStringFallback(t *testing.T) {
	t.Parallel()
	fact := Fact{Predicate: "test_nil", Args: []any{nil}}
	atom, err := fact.ToAtom()
	if err != nil {
		t.Fatalf("ToAtom() error: %v", err)
	}
	assertStringConstant(t, atom.Args[0], "<nil>")
}

func TestToAtom_WhenNoArgs_ShouldSucceed(t *testing.T) {
	t.Parallel()
	fact := Fact{Predicate: "test_empty", Args: nil}
	atom, err := fact.ToAtom()
	if err != nil {
		t.Fatalf("ToAtom() error: %v", err)
	}
	if len(atom.Args) != 0 {
		t.Fatalf("expected 0 args, got %d", len(atom.Args))
	}
	if atom.Predicate.Symbol != "test_empty" {
		t.Errorf("expected predicate 'test_empty', got %q", atom.Predicate.Symbol)
	}
}

func TestToAtom_WhenUnknownType_ShouldFallbackToStringRepr(t *testing.T) {
	t.Parallel()
	type custom struct{ x int }
	fact := Fact{Predicate: "test_custom", Args: []any{custom{x: 42}}}
	atom, err := fact.ToAtom()
	if err != nil {
		t.Fatalf("ToAtom() error: %v", err)
	}
	c, ok := atom.Args[0].(ast.Constant)
	if !ok {
		t.Fatalf("expected constant, got %T", atom.Args[0])
	}
	if c.Type != ast.StringType {
		t.Errorf("expected StringType for unknown type, got %v", c.Type)
	}
}

func TestToAtom_WhenMixedArgs_ShouldHandleAllTypes(t *testing.T) {
	t.Parallel()
	fact := Fact{
		Predicate: "mixed",
		Args: []any{
			MangleAtom("/active"),
			"plain",
			int(1),
			int64(2),
			float32(3.0),
			float64(4.0),
			true,
			false,
			time.Unix(0, 0),
			time.Duration(0),
		},
	}
	atom, err := fact.ToAtom()
	if err != nil {
		t.Fatalf("ToAtom() error: %v", err)
	}
	if len(atom.Args) != 10 {
		t.Fatalf("expected 10 args, got %d", len(atom.Args))
	}
}

// =============================================================================
// Fact.String() Comprehensive Tests
// =============================================================================

func TestFactString_WhenNoArgs_ShouldProduceEmptyParens(t *testing.T) {
	t.Parallel()
	fact := Fact{Predicate: "empty", Args: nil}
	want := "empty()."
	got := fact.String()
	if got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

func TestFactString_WhenSingleStringArg_ShouldQuoteIt(t *testing.T) {
	t.Parallel()
	fact := Fact{Predicate: "test", Args: []any{"hello"}}
	want := `test("hello").`
	got := fact.String()
	if got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

func TestFactString_WhenBoolArgs_ShouldUseMangleConvention(t *testing.T) {
	t.Parallel()
	fact := Fact{Predicate: "test", Args: []any{true, false}}
	want := "test(/true, /false)."
	got := fact.String()
	if got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

// =============================================================================
// isValidMangleNameConstant Comprehensive Tests
// =============================================================================

func TestIsValidMangleNameConstant_WhenValid_ShouldReturnTrue(t *testing.T) {
	t.Parallel()
	valid := []string{"/valid", "/coder", "/true", "/false", "/read_file", "/a/b"}
	for _, v := range valid {
		t.Run(v, func(t *testing.T) {
			t.Parallel()
			if !isValidMangleNameConstant(v) {
				t.Errorf("expected %q to be valid", v)
			}
		})
	}
}

func TestIsValidMangleNameConstant_WhenInvalid_ShouldReturnFalse(t *testing.T) {
	t.Parallel()
	invalid := []struct {
		name  string
		input string
	}{
		{"no slash prefix", "valid"},
		{"just slash", "/"},
		{"double slash", "/bad//name"},
		{"contains space", "/bad name"},
		{"contains tab", "/bad\tname"},
		{"contains newline", "/bad\nname"},
		{"file path .go", "/foo/bar.go"},
		{"file path .md", "/docs/readme.md"},
		{"deep path", "/a/b/c/d"},
		{"quote in name", `/bad"quote`},
	}
	for _, tt := range invalid {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if isValidMangleNameConstant(tt.input) {
				t.Errorf("expected %q to be invalid", tt.input)
			}
		})
	}
}

// =============================================================================
// hasFileExtension Tests
// =============================================================================

func TestHasFileExtension_WhenCommonExtensions_ShouldReturnTrue(t *testing.T) {
	t.Parallel()
	extensions := []string{
		"file.go", "file.md", "file.py", "file.js", "file.ts",
		"file.yaml", "file.json", "file.txt", "file.mg",
		"FILE.GO", "File.Md", // Case-insensitive
	}
	for _, ext := range extensions {
		t.Run(ext, func(t *testing.T) {
			t.Parallel()
			if !hasFileExtension(ext) {
				t.Errorf("expected %q to have file extension", ext)
			}
		})
	}
}

func TestHasFileExtension_WhenNoExtension_ShouldReturnFalse(t *testing.T) {
	t.Parallel()
	noExts := []string{"", "noext", "/coder", "/true", "Makefile"}
	for _, s := range noExts {
		t.Run(s, func(t *testing.T) {
			t.Parallel()
			if hasFileExtension(s) {
				t.Errorf("expected %q to NOT have file extension", s)
			}
		})
	}
}

// =============================================================================
// SessionContext Tests
// =============================================================================

func TestWithSessionContext_WhenSet_ShouldBeRetrievable(t *testing.T) {
	t.Parallel()
	sCtx := &SessionContext{
		CompressedHistory: "test history",
		DreamMode:         true,
	}
	ctx := WithSessionContext(context.Background(), sCtx)
	got := GetSessionContext(ctx)
	if got == nil {
		t.Fatal("expected session context, got nil")
	}
	if got.CompressedHistory != "test history" {
		t.Errorf("expected 'test history', got %q", got.CompressedHistory)
	}
	if !got.DreamMode {
		t.Error("expected DreamMode to be true")
	}
}

func TestGetSessionContext_WhenNotSet_ShouldReturnNil(t *testing.T) {
	t.Parallel()
	got := GetSessionContext(context.Background())
	if got != nil {
		t.Errorf("expected nil, got %v", got)
	}
}

func TestWithSessionContext_WhenNilSessionContext_ShouldReturnNilOnGet(t *testing.T) {
	t.Parallel()
	ctx := WithSessionContext(context.Background(), nil)
	got := GetSessionContext(ctx)
	if got != nil {
		t.Errorf("expected nil for nil session context, got %v", got)
	}
}

// =============================================================================
// KernelFact.ToFact Tests
// =============================================================================

func TestKernelFactToFact_WhenMultipleArgs_ShouldPreserveAll(t *testing.T) {
	t.Parallel()
	kf := KernelFact{
		Predicate: "multi",
		Args:      []any{"a", int64(1), true},
	}
	fact := kf.ToFact()
	if fact.Predicate != "multi" {
		t.Errorf("expected predicate 'multi', got %q", fact.Predicate)
	}
	if len(fact.Args) != 3 {
		t.Fatalf("expected 3 args, got %d", len(fact.Args))
	}
	if fact.Args[0] != "a" {
		t.Errorf("arg[0] = %v, want 'a'", fact.Args[0])
	}
	if fact.Args[1] != int64(1) {
		t.Errorf("arg[1] = %v, want 1", fact.Args[1])
	}
	if fact.Args[2] != true {
		t.Errorf("arg[2] = %v, want true", fact.Args[2])
	}
}

func TestKernelFactToFact_WhenNilArgs_ShouldReturnNilArgs(t *testing.T) {
	t.Parallel()
	kf := KernelFact{Predicate: "empty"}
	fact := kf.ToFact()
	if fact.Args != nil {
		t.Errorf("expected nil args, got %v", fact.Args)
	}
}

func TestKernelFactToFact_WhenEmptyArgs_ShouldReturnEmptySlice(t *testing.T) {
	t.Parallel()
	kf := KernelFact{Predicate: "empty", Args: []any{}}
	fact := kf.ToFact()
	if len(fact.Args) != 0 {
		t.Errorf("expected 0 args, got %d", len(fact.Args))
	}
}

// =============================================================================
// ArgName Tests
// =============================================================================

func TestArgName_WhenValidIndex_ShouldExtractName(t *testing.T) {
	t.Parallel()
	f := Fact{Predicate: "test", Args: []any{MangleAtom("/coder"), "plain"}}
	if got := ArgName(f, 0); got != "/coder" {
		t.Errorf("ArgName(f, 0) = %q, want '/coder'", got)
	}
	if got := ArgName(f, 1); got != "plain" {
		t.Errorf("ArgName(f, 1) = %q, want 'plain'", got)
	}
}

func TestArgName_WhenOutOfBounds_ShouldReturnEmpty(t *testing.T) {
	t.Parallel()
	f := Fact{Predicate: "test", Args: []any{"a"}}
	if got := ArgName(f, -1); got != "" {
		t.Errorf("ArgName(f, -1) = %q, want empty", got)
	}
	if got := ArgName(f, 5); got != "" {
		t.Errorf("ArgName(f, 5) = %q, want empty", got)
	}
}

// =============================================================================
// ArgFloat64 Tests
// =============================================================================

func TestArgFloat64_WhenValidIndex_ShouldExtract(t *testing.T) {
	t.Parallel()
	f := Fact{Predicate: "test", Args: []any{float64(3.14)}}
	v, ok := ArgFloat64(f, 0)
	if !ok || v != 3.14 {
		t.Errorf("ArgFloat64(f, 0) = (%f, %v), want (3.14, true)", v, ok)
	}
}

func TestArgFloat64_WhenOutOfBounds_ShouldReturnFalse(t *testing.T) {
	t.Parallel()
	f := Fact{Predicate: "test", Args: []any{float64(1.0)}}
	_, ok := ArgFloat64(f, 5)
	if ok {
		t.Error("expected false for out of bounds")
	}
	_, ok = ArgFloat64(f, -1)
	if ok {
		t.Error("expected false for negative index")
	}
}
