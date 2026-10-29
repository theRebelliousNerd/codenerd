package gates

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/lintreport"
)

func writeModuleFile(t *testing.T, root, name, content string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// Real gates on a temp module: G1 build, G2 vet, G3 test and G10 gofmt are
// measured clean, then a planted vet finding (a Printf verb that does not
// match its argument) and a planted gofmt finding make the second snapshot
// worse on those gates (and on G3: go test runs vet first), which the kernel
// refuses to keep; the
// reverse comparison is better and keepable.
func TestErrorFree_MeasuresRealGatesAndRefusesARegression(t *testing.T) {
	root := t.TempDir()
	writeModuleFile(t, root, "go.mod", "module gateprobe\n\ngo 1.22\n")
	writeModuleFile(t, root, "lib/lib.go", "package lib\n\n// Two is 2.\nfunc Two() int { return 2 }\n")
	writeModuleFile(t, root, "lib/lib_test.go", "package lib\n\nimport \"testing\"\n\nfunc TestTwo(t *testing.T) {\n\tif Two() != 2 {\n\t\tt.Fatal(\"two\")\n\t}\n}\n")
	// A nested checkout under a dot directory is not this module.
	writeModuleFile(t, root, ".claude/worktrees/x/bad.go", "package x\nfunc  F(){}\n")
	opts := MeasureOptions{Only: []string{"build", "vet", "test", "gofmt"}}
	ctx := context.Background()

	clean := MeasureErrorFree(ctx, root, opts)
	for _, g := range clean.Gates {
		if g.Status != MeasurePass || g.Count != 0 {
			t.Fatalf("clean module: %s %s = %s with %d finding(s) %v (%s %s)", g.Label, g.Gate, g.Status, g.Count, g.Findings, g.Reason, g.Output)
		}
	}
	if len(clean.Gates) != 4 {
		t.Fatalf("measured %d gates, want 4", len(clean.Gates))
	}

	writeModuleFile(t, root, "lib/say.go", "package lib\n\nimport \"fmt\"\n\n// Say prints n.\nfunc Say(n string) { fmt.Printf(\"%d\\n\", n) }\n")
	writeModuleFile(t, root, "lib/ugly.go", "package lib\n\n// Ugly is unformatted.\nfunc  Ugly()  {}\n")
	dirty := MeasureErrorFree(ctx, root, opts)
	vet, _ := dirty.Gate("vet")
	if vet.Status != MeasureFail || vet.Count != 1 || vet.Findings[0] != (lintreport.Finding{File: "lib/say.go", Line: 6, Column: vet.Findings[0].Column, Check: "printf", Message: vet.Findings[0].Message}) {
		t.Fatalf("vet = %+v, want the planted printf finding in lib/say.go:6", vet)
	}
	gofmt, _ := dirty.Gate("G10")
	if gofmt.Status != MeasureFail || gofmt.Count != 1 || gofmt.Findings[0].File != "lib/ugly.go" {
		t.Fatalf("gofmt = %+v, want lib/ugly.go only", gofmt)
	}

	path, err := dirty.Save(root)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadSnapshot(path)
	if err != nil || loaded.ID != dirty.ID || len(loaded.Gates) != 4 {
		t.Fatalf("LoadSnapshot = %+v, %v", loaded, err)
	}

	k, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("the shipped corpus must load: %v", err)
	}
	worse, err := Compare(k, clean, loaded)
	if err != nil {
		t.Fatal(err)
	}
	if worse.Keepable {
		t.Fatalf("a snapshot with a new vet and a new gofmt finding was keepable: %+v", worse)
	}
	// go test runs vet's printf check before the tests, so the planted
	// finding fails G3 as well.
	if got := strings.Join(worse.Regressed(), ","); got != "G2 vet,G3 test,G10 gofmt" {
		t.Fatalf("regressed = %s, want G2 vet,G3 test,G10 gofmt", got)
	}
	for _, g := range worse.Gates {
		if g.Gate == "vet" && (len(g.New) != 1 || !strings.HasPrefix(g.New[0], "lib/say.go | printf | ")) {
			t.Fatalf("vet's new findings = %v", g.New)
		}
		if g.Gate == "build" && g.Change != "same" {
			t.Fatalf("%s changed %s, want same", g.Gate, g.Change)
		}
	}

	better, err := Compare(k, loaded, clean)
	if err != nil {
		t.Fatal(err)
	}
	if !better.Keepable {
		t.Fatalf("fixing both findings was not keepable: %+v", better)
	}
	for _, g := range better.Gates {
		if (g.Gate == "vet" || g.Gate == "gofmt") && (g.Change != "better" || len(g.Gone) != 1) {
			t.Fatalf("%s = %+v, want better with one finding gone", g.Gate, g)
		}
	}
}

// The policy's other ways to get worse, and not: a failing gate whose count
// grows, a measured gate that can no longer run, and a count that changes
// shape without growing.
func TestCompare_CountsAndAvailability(t *testing.T) {
	k, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("the shipped corpus must load: %v", err)
	}
	f := func(n int) []lintreport.Finding {
		out := make([]lintreport.Finding, n)
		for i := range out {
			out[i] = lintreport.Finding{File: "a.go", Check: "U1000", Message: strings.Repeat("x", i+1)}
		}
		return out
	}
	snap := func(id string, ms ...Measurement) Snapshot {
		for i := range ms {
			ms[i].Count = len(ms[i].Findings)
		}
		return Snapshot{ID: id, Gates: ms}
	}
	before := snap("s1",
		Measurement{Gate: "staticcheck", Label: "G4", Status: MeasureFail, Findings: f(2)},
		Measurement{Gate: "golangci_lint", Label: "G5", Status: MeasurePass},
		Measurement{Gate: "govulncheck", Label: "G6", Status: MeasureUnavailable},
		Measurement{Gate: "vet", Label: "G2", Status: MeasureFail, Findings: f(2)},
	)
	after := snap("s2",
		Measurement{Gate: "staticcheck", Label: "G4", Status: MeasureFail, Findings: f(3)},
		Measurement{Gate: "golangci_lint", Label: "G5", Status: MeasureUnavailable},
		Measurement{Gate: "govulncheck", Label: "G6", Status: MeasureUnavailable},
		Measurement{Gate: "vet", Label: "G2", Status: MeasureFail, Findings: []lintreport.Finding{{File: "b.go", Check: "printf", Message: "m"}, {File: "a.go", Check: "U1000", Message: "x"}}},
	)
	c, err := Compare(k, before, after)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"staticcheck": "worse", "golangci_lint": "worse", "govulncheck": "same", "vet": "same"}
	for _, g := range c.Gates {
		if g.Change != want[g.Gate] {
			t.Errorf("%s = %s, want %s", g.Gate, g.Change, want[g.Gate])
		}
		if g.Gate == "vet" && (len(g.New) != 1 || len(g.Gone) != 1) {
			t.Errorf("vet new %v gone %v, want one each", g.New, g.Gone)
		}
	}
	if c.Keepable {
		t.Error("a pair with a regressed gate was keepable")
	}
}

// G6 reads govulncheck's message stream: a finding is a reachable advisory
// (its innermost frame names a function), one per advisory.
func TestReadGovulncheck(t *testing.T) {
	stream := `{"config":{"protocol_version":"v1.0.0"}}
{"finding":{"osv":"GO-2026-0001","trace":[{"module":"golang.org/x/text","version":"v0.37.0"}]}}
{"finding":{"osv":"GO-2026-0002","trace":[{"module":"golang.org/x/text","version":"v0.37.0","package":"golang.org/x/text/language","function":"Parse"},{"module":"gateprobe","package":"gateprobe/lib","function":"Use","position":{"filename":"/ws/lib/use.go","line":7,"column":3}}]}}
{"finding":{"osv":"GO-2026-0002","trace":[{"module":"golang.org/x/text","version":"v0.37.0","package":"golang.org/x/text/language","function":"Parse"}]}}`
	got, _, err := readGovulncheck(stream, "", filepath.FromSlash("/ws"))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Check != "GO-2026-0002" || got[0].File != "lib/use.go" || got[0].Line != 7 {
		t.Fatalf("readGovulncheck = %+v, want one reachable advisory at lib/use.go:7", got)
	}
}

// G3 reads go test -json: failing tests are findings; a package that failed
// with none, one finding.
func TestReadGoTest(t *testing.T) {
	stream := `{"Action":"run","Package":"m/a","Test":"TestOK"}
{"Action":"pass","Package":"m/a","Test":"TestOK"}
{"Action":"output","Package":"m/a","Test":"TestBad","Output":"    a_test.go:9: boom\n"}
{"Action":"fail","Package":"m/a","Test":"TestBad"}
{"Action":"fail","Package":"m/a"}
{"Action":"output","Package":"m/b","Output":"panic: init\n"}
{"Action":"fail","Package":"m/b"}`
	got, rest, err := readGoTest(stream, "", "/ws")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != (lintreport.Finding{File: "m/a", Check: "test", Message: "TestBad"}) || got[1].File != "m/b" || got[1].Check != "package" {
		t.Fatalf("readGoTest = %+v", got)
	}
	if !strings.Contains(rest, "boom") || !strings.Contains(rest, "panic: init") {
		t.Fatalf("rest does not carry the failing output: %q", rest)
	}
}
