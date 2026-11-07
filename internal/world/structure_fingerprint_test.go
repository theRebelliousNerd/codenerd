package world

import (
	"context"
	"strings"
	"testing"
	"time"
)

// The structure index fingerprints each element by its own text, so the spec
// pairing can tell which element moved: a reformat moves nothing, an edit to
// one function moves that function only, and a file-level citation (the
// header) follows the whole file.
func TestSpecSnapshotFingerprintsAreElementScoped(t *testing.T) {
	stamp := time.Now().Add(-time.Hour)
	const src = "package p\n\n// A adds one.\nfunc A(x int) int { return x + 1 }\n\n// B doubles.\nfunc B(x int) int { return x * 2 }\n"
	root := specWorkspace(t, map[string]string{
		"go.mod": "module example.com/ws\n\ngo 1.26\n",
		"p/p.go": src,
	}, stamp, stamp)
	snapshot := func() map[string]string {
		t.Helper()
		elems, _, _, _, err := NewStructureIndex(root).SpecSnapshot(context.Background(), nil)
		if err != nil {
			t.Fatal(err)
		}
		out := map[string]string{}
		for _, e := range elems {
			if e.Fingerprint == "" {
				t.Errorf("%s has no fingerprint", e.Ref)
			}
			out[e.Ref] = e.Fingerprint
		}
		return out
	}
	edit := func(from, to string) {
		t.Helper()
		writeSpecFile(t, root, "p/p.go", strings.Replace(src, from, to, 1), stamp, stamp)
	}
	base := snapshot()
	a, b, file := "p.A", "p.B", "p/p.go:header"
	if len(base) == 0 || base[a] == "" || base[b] == "" || base[a] == base[b] {
		t.Fatalf("A and B must each have their own fingerprint: %v", base)
	}

	edit("func A(x int) int { return x + 1 }", "func A(x int) int {\n\treturn x+1\n}")
	reformatted := snapshot()
	if reformatted[a] != base[a] || reformatted[b] != base[b] {
		t.Errorf("a reformat is not a change: %v -> %v", base, reformatted)
	}
	if reformatted[file] != base[file] {
		t.Errorf("a reformat does not move the file's fingerprint either")
	}

	edit("return x * 2", "return x * 3")
	edited := snapshot()
	if edited[b] == base[b] {
		t.Errorf("B's body changed but its fingerprint did not")
	}
	if edited[a] != base[a] {
		t.Errorf("an edit to B moved A's fingerprint")
	}
	if edited[file] == base[file] {
		t.Errorf("a file-level citation follows the whole file, which changed")
	}
}
