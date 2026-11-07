package session

import (
	"slices"
	"strings"
	"testing"
)

func TestDiffGoFile(t *testing.T) {
	t.Parallel()
	const base = `package p

import "strings"

// Doc comment.
type Box[T any] struct{ v T }

func (b *Box[T]) Get() T { return b.v }

func (b Box[T]) Len() int { return len(strings.TrimSpace("x")) }

func Leaf() int { return 1 }

func Other() int { return 2 }
`
	cases := []struct {
		name    string
		after   string
		want    []string
		fallbak string
	}{
		{name: "identical", after: base},
		{name: "comments and formatting are not changes", after: strings.Replace(strings.Replace(base, "// Doc comment.", "// Another doc comment.\n// With two lines.", 1), "func Leaf() int { return 1 }", "func Leaf() int {\n\t// explained\n\treturn 1\n}", 1)},
		{name: "plain function body", after: strings.Replace(base, "return 1 }", "return 3 }", 1), want: []string{"Leaf"}},
		{name: "method body of a generic type", after: strings.Replace(base, "return b.v }", "var zero T; _ = zero; return b.v }", 1), want: []string{"Box.Get"}},
		{name: "two bodies", after: strings.Replace(strings.Replace(base, "return 1 }", "return 5 }", 1), "return 2 }", "return 6 }", 1), want: []string{"Leaf", "Other"}},
		{name: "added plain function", after: base + "\nfunc Added() int { return Leaf() }\n"},
		{name: "added method", after: base + "\nfunc (b Box[T]) Extra() {}\n", fallbak: "method Box.Extra was added"},
		{name: "removed function", after: strings.Replace(base, "func Other() int { return 2 }\n", "", 1), fallbak: "Other was removed"},
		{name: "signature", after: strings.Replace(base, "func Leaf() int", "func Leaf() int64", 1), fallbak: "signature of Leaf"},
		{name: "receiver kind", after: strings.Replace(base, "func (b Box[T]) Len()", "func (b *Box[T]) Len()", 1), fallbak: "signature of Box.Len"},
		{name: "struct field", after: strings.Replace(base, "struct{ v T }", "struct {\n\tv T\n\tw int\n}", 1), fallbak: "package-level"},
		{name: "import", after: strings.Replace(base, `import "strings"`, "import (\n\t\"strings\"\n\t\"fmt\"\n)", 1) + "\nvar _ = fmt.Sprint\n", fallbak: "imports changed"},
		{name: "directive", after: strings.Replace(base, "func Leaf()", "//go:noinline\nfunc Leaf()", 1), fallbak: "directive"},
		{name: "does not parse", after: "package p\n\nfunc (", fallbak: "does not parse"},
		{name: "cgo", after: strings.Replace(base, `import "strings"`, "import \"C\"\nimport \"strings\"", 1), fallbak: "cgo"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, why := diffGoFile("p.go", []byte(base), true, []byte(tc.after))
			if tc.fallbak != "" {
				if !strings.Contains(why, tc.fallbak) {
					t.Fatalf("why = %q, want it to say %q (changed %v)", why, tc.fallbak, got)
				}
				return
			}
			if why != "" {
				t.Fatalf("fell back: %s", why)
			}
			var names []string
			for _, k := range got {
				names = append(names, k.String())
			}
			if !slices.Equal(names, tc.want) {
				t.Fatalf("changed = %v, want %v", names, tc.want)
			}
		})
	}
}

func TestDiffGoFile_ALinknameIsNeverBodyOnly(t *testing.T) {
	t.Parallel()
	src := "package p\n\nimport _ \"unsafe\"\n\n//go:linkname f runtime.f\nfunc f() int { return 1 }\n"
	_, why := diffGoFile("p.go", []byte(src), true, []byte(strings.Replace(src, "return 1", "return 2", 1)))
	if !strings.Contains(why, "linkname") {
		t.Fatalf("why = %q, want go:linkname to force the fallback", why)
	}
}
