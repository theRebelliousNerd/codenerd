package codedom

import (
	"strings"
	"testing"
)

// An edit_element whose old is not in the element says why, so the next call
// is the one that works: old running into the next declaration, old being
// another declaration's text, or old agreeing with the element up to the line
// where it differs. The element's current text is still listed.
func TestEditElement_AMissSaysWhyOldIsNotThere(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, ref, old string
		want           []string
	}{
		{
			name: "runs into the next declaration",
			ref:  "T.M",
			old:  "\treturn t.a\n}\n\n// N returns a as text.",
			want: []string{"runs past this element", "edit_file's, or one edit per element"},
		},
		{
			name: "another declaration's text",
			ref:  "T.M",
			old:  "fmt.Sprint",
			want: []string{"outside this element", "another declaration's text"},
		},
		{
			name: "one line differs",
			ref:  "T.M",
			old:  "func (t *T) M() int {\n\treturn t.b\n}",
			want: []string{"line 9 differs", `the file has "return t.a", old has "return t.b"`},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			reg, _, rel := editFixture(t, "demo.go", twoMethods)
			_, _, err := execTool(reg, "edit_element", map[string]any{"path": rel, "ref": tc.ref, "old": tc.old, "new": "x"})
			if err == nil {
				t.Fatal("the edit applied")
			}
			msg := err.Error()
			for _, want := range append(tc.want, "Its text is:") {
				if !strings.Contains(msg, want) {
					t.Errorf("error does not say %q:\n%s", want, msg)
				}
			}
		})
	}
}
