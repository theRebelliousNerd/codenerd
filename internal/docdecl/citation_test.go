package docdecl

import (
	"strings"
	"testing"
)

func TestAcceptsPath(t *testing.T) {
	samples := []struct {
		line string
		want bool
	}{
		{"internal/pkg/a.go", true},
		{"See internal/pkg/a.go:1", true},
		{"`internal/pkg/a.go`", true},
		{"./internal/pkg/a.go", true},
		{"/internal/pkg/a.go", true},
		{" (./internal/pkg/a.go)", true},
		{"foo/internal/nope.go", false},
		{"../internal/nope.go", false},
		{"https://example.com/internal/nope.go", false},
		{"xinternal/pkg/a.go", false},
		{"cmd/tool/main.go", true},
	}
	for _, s := range samples {
		i := strings.Index(s.line, "internal/")
		if i < 0 {
			i = strings.Index(s.line, "cmd/")
		}
		if i < 0 {
			t.Fatal(s.line)
		}
		if got := AcceptsPath(s.line, i); got != s.want {
			t.Errorf("%q accepts=%v want %v", s.line, got, s.want)
		}
	}
}

func TestParseSuffix(t *testing.T) {
	lines, hash, n := ParseSuffix(":1-4, 10")
	if n != len(":1-4, 10") || hash != "" || len(lines) != 2 || lines[0] != [2]int{1, 4} || lines[1] != [2]int{10, 10} {
		t.Fatalf("list: lines=%v hash=%q n=%d", lines, hash, n)
	}
	lines, hash, n = ParseSuffix(": the")
	if n != 0 || hash != "" || lines != nil {
		t.Fatalf("prose colon consumed: lines=%v hash=%q n=%d", lines, hash, n)
	}
	lines, hash, n = ParseSuffix("#Good:2")
	if hash != "Good" || n != len("#Good:2") || len(lines) != 1 || lines[0] != [2]int{2, 2} {
		t.Fatalf("hash then line: lines=%v hash=%q n=%d", lines, hash, n)
	}
	lines, hash, n = ParseSuffix(":12#TaxonomyEngine.ClassifyInput")
	if hash != "TaxonomyEngine.ClassifyInput" || len(lines) != 1 || lines[0] != [2]int{12, 12} {
		t.Fatalf("line then hash: lines=%v hash=%q n=%d", lines, hash, n)
	}
}

func TestGapBinds(t *testing.T) {
	if !GapBinds(" (`") {
		t.Fatal("parenthetical citation")
	}
	if !GapBinds(" defines ") {
		t.Fatal("defines")
	}
	if GapBinds(" is documented in the ") {
		t.Fatal("prose bound as a citation")
	}
	if GapBinds(" (`emitter.go:10`) see ") {
		t.Fatal("abbreviated filename should consume the symbol")
	}
	if !GapBinds(" / `OtherFile` | ") {
		t.Fatal("two symbols before one path")
	}
	if CodeShaped("vision") || CodeShaped("mu") {
		t.Fatal("lowercase prose is not code-shaped; mu binds only across a parenthetical gap")
	}
	if !CodeShaped("NotDefined") || !CodeShaped("keep_me") || !CodeShaped("pkg.Good") {
		t.Fatal("code-shaped names")
	}
}

func TestNormalizeSymbol(t *testing.T) {
	cases := []struct {
		in, want string
		ok       bool
	}{
		{"var uiDiffEngine", "uiDiffEngine", true},
		{"pkg.Good()", "pkg.Good", true},
		{"context_must_retain(Predicate)", "context_must_retain", true},
		{"(*Engine).Clear()", "(*Engine).Clear", true},
		{"keep_me/1", "keep_me/1", true},
		{"diff.NewEngine", "diff.NewEngine", true},
		{"pkg.Go", "pkg.Go", true},
		{"sync.RWMutex", "sync.RWMutex", true},
		{"internal/pkg/a.go", "", false},
		{"not a symbol", "", false},
		{"(*Engine)", "", false},
		{"nerd.md", "", false},
		{"usage.json", "", false},
		{"feedback.go", "", false},
		{"types.go", "", false},
		{"nerd.exe", "", false},
		{"config.yaml", "", false},
	}
	for _, tc := range cases {
		got, ok := NormalizeSymbol(tc.in)
		if ok != tc.ok || got != tc.want {
			t.Errorf("normalize %q = %q %v, want %q %v", tc.in, got, ok, tc.want, tc.ok)
		}
	}
}
