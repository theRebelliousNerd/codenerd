package campaign

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/gates"
)

func r7bNodes() []SubsystemNode {
	return []SubsystemNode{
		// Empty Paths is the whole tree, and must not inherit a finding
		// that named a package directory or nothing at all.
		{ID: RecurseWiringNodeID, Title: "Cross-subsystem wiring", CrossCutting: true},
		{ID: ".", Paths: []string{"."}},
		{ID: "compileerr", Paths: []string{"compileerr"}},
		{ID: "testbuild", Paths: []string{"testbuild"}},
		{ID: "testsfail", Paths: []string{"testsfail"}},
		{ID: "samea", Paths: []string{"samea"}},
		{ID: "sameb", Paths: []string{"sameb"}},
		{ID: "initpanic", Paths: []string{"initpanic"}},
		{ID: "a", Paths: []string{"a"}},
		{ID: "pkg", Paths: []string{"pkg"}},
		{ID: "sub", Paths: []string{"pkg/sub"}},
	}
}

func TestAttributeNode_DirectoryNotWiring(t *testing.T) {
	nodes := r7bNodes()
	cases := []struct {
		target string
		want   string
	}{
		{"compileerr/broken.go", "compileerr"},
		{"compileerr", "compileerr"},
		{"testbuild/bad_test.go", "testbuild"},
		{"testsfail/ok.go", "testsfail"},
		{".::TestRoot", "."},
		{"samea::TestSame", "samea"},
		{"sameb::TestSame", "sameb"},
		{"testsfail::TestFails", "testsfail"},
		{"testsfail::TestPanics", "testsfail"},
		{"initpanic", "initpanic"},
		{"main.go", "."},
		{"a", "a"},
		{"a/x.go", "a"},
		{"pkg/b.go", "pkg"},
		{"pkg/sub/a.go", "sub"},
		{gates.UnattributedTarget, RecurseUnattributedNodeID},
		{gates.UnattributedTarget + "::parse::rejects_bad", RecurseUnattributedNodeID},
		{"example.com/r7b/testsfail::TestFails", RecurseUnattributedNodeID},
		{"example.com/r7b/compileerr", RecurseUnattributedNodeID},
		{"", RecurseUnattributedNodeID},
	}
	for _, tc := range cases {
		got := attributeNode(tc.target, nodes)
		if got != tc.want {
			t.Errorf("attributeNode(%q) = %q, want %q", tc.target, got, tc.want)
		}
		if got == RecurseWiringNodeID {
			t.Errorf("attributeNode(%q) guessed wiring", tc.target)
		}
	}
}

func TestAttributeNode_CapturedGoTestReachesItsPackage(t *testing.T) {
	root := writeModule(t, "example.com/r7b")
	out := gatesTestdata(t, "go_test.txt")
	fs := gates.Findings(root, gates.Result{
		Gate: gates.Gate{ID: "go:test", Kind: gates.Test}, ExitCode: 1, Output: out,
	})
	want := map[string]string{
		"compileerr/broken.go":  "compileerr",
		"testbuild/bad_test.go": "testbuild",
		".::TestRoot":           ".",
		"samea::TestSame":       "samea",
		"sameb::TestSame":       "sameb",
		"testsfail::TestFails":  "testsfail",
		"testsfail::TestPanics": "testsfail",
	}
	if len(fs) != len(want) {
		t.Fatalf("findings = %d, want %d: %v", len(fs), len(want), findingTargets(fs))
	}
	for _, f := range fs {
		got := attributeNode(f.Target, r7bNodes())
		if got != want[f.Target] {
			t.Errorf("%s → %s, want %s", f.Target, got, want[f.Target])
		}
	}
}

// The captured line "FAIL\texample.com/r7b/compileerr [build failed]" is the
// package, with no file. Under its own module that is the compileerr node.
// Under a different module the import path is kept and reported, not given
// to wiring or to a directory that happens to share a suffix.
func TestUnattributedFinding_IsReportedNotAssignedToWiring(t *testing.T) {
	line := gatesLine(t, "go_test.txt", "example.com/r7b/compileerr [build failed]")
	own := gates.Findings(writeModule(t, "example.com/r7b"), gates.Result{
		Gate: gates.Gate{ID: "go:test", Kind: gates.Test}, ExitCode: 1, Output: line + "\n",
	})
	if len(own) != 1 || own[0].Target != "compileerr" {
		t.Fatalf("own module: %+v", own)
	}
	own[0].Node = attributeNode(own[0].Target, r7bNodes())
	if own[0].Node != "compileerr" {
		t.Fatalf("node = %s", own[0].Node)
	}
	set := gates.Set{Gates: []gates.Gate{{ID: "go:test", Kind: gates.Test, Scope: gates.ScopeAll, Language: "go"}}}
	run := &recurseRun{state: map[string]gateRun{gateKey("go:test", ""): {findings: own}}}
	if got := run.open(SubsystemNode{ID: "compileerr", Paths: []string{"compileerr"}, Languages: []string{"go"}}, nil, set); len(got) != 1 || got[0].Target != "compileerr" {
		t.Fatalf("compileerr open = %+v", got)
	}
	if got := run.open(SubsystemNode{ID: RecurseWiringNodeID, CrossCutting: true}, nil, set); len(got) != 0 {
		t.Fatalf("wiring was given the package failure: %+v", got)
	}

	other := gates.Findings(writeModule(t, "example.com/other"), gates.Result{
		Gate: gates.Gate{ID: "go:test", Kind: gates.Test}, ExitCode: 1, Output: line + "\n",
	})
	if len(other) != 1 || other[0].Target != "example.com/r7b/compileerr" {
		t.Fatalf("other module: %+v", other)
	}
	other[0].Node = attributeNode(other[0].Target, r7bNodes())
	if other[0].Node != RecurseUnattributedNodeID {
		t.Fatalf("unresolved node = %s, want unattributed", other[0].Node)
	}
	var buf bytes.Buffer
	r := &recurseRun{
		out:   &buf,
		state: map[string]gateRun{gateKey("go:test", ""): {findings: other}},
	}
	r.reportUnattributed()
	wantLine := "go:test example.com/r7b/compileerr: " + other[0].Message
	if !strings.Contains(buf.String(), "recurse: unattributed finding "+wantLine) {
		t.Fatalf("log = %q", buf.String())
	}
	sum := r.result.Summary()
	if !strings.Contains(sum, "unattributed findings: 1") || !strings.Contains(sum, wantLine) {
		t.Fatalf("summary:\n%s", sum)
	}
	r.out = nil
	r.reportUnattributed()
	if strings.Count(buf.String(), "unattributed finding") != 1 {
		t.Fatalf("logged twice: %q", buf.String())
	}
	if got := r.open(SubsystemNode{ID: RecurseWiringNodeID, CrossCutting: true}, nil, set); len(got) != 0 {
		t.Fatalf("wiring open = %+v", got)
	}
	if got := r.open(SubsystemNode{ID: "compileerr", Paths: []string{"compileerr"}, Languages: []string{"go"}}, nil, set); len(got) != 0 {
		t.Fatalf("compileerr was guessed: %+v", got)
	}

	r.state = map[string]gateRun{}
	r.reportUnattributed()
	if len(r.result.Unattributed) != 0 || strings.Contains(r.result.Summary(), "unattributed") {
		t.Fatalf("stale summary: %q\n%+v", r.result.Summary(), r.result.Unattributed)
	}
	r.out = nil
	r.reportUnattributed()

	base := (&RecurseCycleResult{Passes: 2, Cycles: 3, Kept: 1, Improved: 1, Reverted: 4, Refused: 5, Unverified: 6}).Summary()
	if strings.Contains(base, "unattributed") || !strings.HasPrefix(base, "Recurse: 2 passes, 3 attempts: 1 kept (1 improvements), 4 reverted, 5 refused, 6 unverified.") {
		t.Fatalf("baseline summary changed: %q", base)
	}
}

func writeModule(t *testing.T, module string) string {
	t.Helper()
	root := t.TempDir()
	body := "module " + module + "\n\ngo 1.21\n"
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func gatesTestdata(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "gates", "testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func gatesLine(t *testing.T, file, want string) string {
	t.Helper()
	text := strings.ReplaceAll(gatesTestdata(t, file), "\r\n", "\n")
	for _, line := range strings.Split(text, "\n") {
		if strings.Contains(line, want) {
			return line
		}
	}
	t.Fatalf("%s has no line containing %q", file, want)
	return ""
}

func findingTargets(fs []gates.Finding) []string {
	out := make([]string, len(fs))
	for i, f := range fs {
		out[i] = f.Target
	}
	return out
}
