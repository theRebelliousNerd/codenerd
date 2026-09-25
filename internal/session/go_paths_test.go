package session

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/tools"
)

// throughLink returns ws as it is reached through a symlink to it: how a
// macOS temp directory (/var -> /private/var) and any linked workspace are
// spelled.
func throughLink(t *testing.T, ws string) string {
	t.Helper()
	link := filepath.Join(t.TempDir(), "workspace-link")
	if err := os.Symlink(ws, link); err != nil {
		t.Skipf("this platform will not make a symlink here: %v", err)
	}
	return link
}

// An overlay key names a file only in the spelling go itself uses for it. A
// key spelled through the link would be ignored by go without a word.
func TestGoOverlayKey_EverySpellingOfTheWorkspaceNamesTheResolvedFile(t *testing.T) {
	real := t.TempDir()
	mustWrite(t, filepath.Join(real, "p", "a.go"), "package p\n")
	link := throughLink(t, real)
	root, err := tools.CanonicalWorkspaceRoot(real)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(root, "p", "a.go")

	for _, tc := range []struct{ workspace, path string }{
		{link, "p/a.go"},
		{link, filepath.Join(link, "p", "a.go")},
		{real, filepath.Join(link, "p", "a.go")},
		{link, filepath.Join(real, "p", "a.go")},
		// A file the turn created and then removed still has one name.
		{link, "p/gone.go"},
	} {
		got := goOverlayKey(tc.workspace, tc.path)
		w := want
		if strings.HasSuffix(tc.path, "gone.go") {
			w = filepath.Join(root, "p", "gone.go")
		}
		if got != w {
			t.Errorf("goOverlayKey(%q, %q) = %q, want %q", tc.workspace, tc.path, got, w)
		}
	}
	if got := goWorkspace(link); got != root {
		t.Errorf("goWorkspace(link) = %q, want the resolved root %q", got, root)
	}
}

// Every gate that compares a "before the turn" run with the run now builds a
// go -overlay keyed by paths under the workspace. With the workspace reached
// through a link those keys named files go never saw, the baseline run
// measured the tree as the turn left it, and whatever the turn broke read as
// already broken: vet, the importer gate and test attribution all passed.
func TestGates_AWorkspaceReachedThroughALinkKeepsItsVerdicts(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to the real go toolchain")
	}

	t.Run("vet charges the lock the turn added", func(t *testing.T) {
		ws := throughLink(t, vetWorkspace(t, map[string]string{"go.mod": "module vetprobe\n\ngo 1.25\n", "p/state.go": vetStateMutex, "p/use.go": vetUse}))
		v := verifyVet(context.Background(), ws, []string{"p/state.go"}, map[string]PreImage{"p/state.go": existed(vetStateBefore)})
		if v.Verdict() != VerifyFailed || !strings.Contains(v.Output, "passes lock by value") {
			t.Fatalf("verifyVet through a link = %+v, want failed naming use.go's copied lock", v)
		}
	})

	t.Run("an importer the turn broke fails the gate", func(t *testing.T) {
		ws := throughLink(t, importerModule(t, aAfter))
		result := mutationResult()
		result.WrittenPaths = []string{"a/a.go"}
		result.PreWriteContents = map[string]PreImage{"a/a.go": existed(aBefore)}
		v, _ := gateTests(context.Background(), ws, result, false)
		if v.Verdict() != VerifyFailed || !strings.Contains(v.Output, "TestLabelIsTheOldName") {
			t.Fatalf("gate through a link = %s (%s):\n%s\nwant failed on b's test", v.Verdict(), v.Reason, v.Output)
		}
	})

	t.Run("a failure a created file causes is the turn's", func(t *testing.T) {
		real := writeBaselineModule(t, map[string]string{
			"go.mod":  "module verifyprobe\n\ngo 1.21\n",
			"calc.go": "package verifyprobe\n\nvar Extra = \"\"\n",
			"calc_test.go": "package verifyprobe\n\nimport \"testing\"\n\n" +
				"func TestExtra(t *testing.T) { if Extra != \"\" { t.Fatalf(\"broken by new file: %q\", Extra) } }\n",
		})
		writeWorkspaceFile(t, real, "extra.go", "package verifyprobe\n\nfunc init() { Extra = \"broken\" }\n")
		ws := throughLink(t, real)
		head := verifyTests(context.Background(), ws, []string{"."})
		got := attributeTestFailures(context.Background(), ws, []string{"."}, []string{"extra.go"}, map[string]PreImage{"extra.go": {}}, head)
		if got.Outcome != VerifyFailed || len(got.PreExistingFailures) != 0 {
			t.Fatalf("attribution through a link = %v, pre-existing %v; want failed and charged to the turn", got.Outcome, got.PreExistingFailures)
		}
	})

	t.Run("the repository is asked with the change really taken out", func(t *testing.T) {
		real := t.TempDir()
		writeMod(t, real)
		mustWrite(t, filepath.Join(real, "subject", "subject.go"), "package subject\n\nfunc Emit() string { return \"kept\" }\n")
		mustWrite(t, filepath.Join(real, "guard", "guard_test.go"), "package guard\n\nimport (\n\t\"testing\"\n\n\t\"probe/subject\"\n)\n\n"+
			"func TestSubjectStillEmits(t *testing.T) {\n\tif subject.Emit() != \"kept\" {\n\t\tt.Fatal(\"subject changed\")\n\t}\n}\n")
		ws := throughLink(t, real)
		noticed, detail := pinnedByExistingTests(context.Background(), ws,
			pinUnit{path: "subject/subject.go", name: "Emit", content: "package subject\n\nfunc Emit() string { return \"lost\" }\n"})
		if !noticed || !strings.Contains(detail, "TestSubjectStillEmits") {
			t.Fatalf("pinnedByExistingTests through a link = %v (%s), want the guard's test to notice", noticed, detail)
		}
	})
}
