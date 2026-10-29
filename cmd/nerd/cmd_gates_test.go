package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/gates"
)

// `nerd gates measure` twice around a planted vet finding, then `nerd gates
// diff`: the second snapshot is worse on G2, the diff names the new finding
// and exits non-zero.
func TestGatesCommand_MeasureTwiceAndDiffRefusesARegression(t *testing.T) {
	root := t.TempDir()
	write := func(name, content string) {
		full := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("go.mod", "module gatescmd\n\ngo 1.22\n")
	write("lib/lib.go", "package lib\n\n// Two is 2.\nfunc Two() int { return 2 }\n")

	oldWorkspace, oldOnly := workspace, gatesOnly
	workspace, gatesOnly = root, []string{"vet", "gofmt", "policy_corpus"}
	t.Cleanup(func() { workspace, gatesOnly = oldWorkspace, oldOnly })

	measure := func() string {
		var out bytes.Buffer
		gatesMeasureCmd.SetOut(&out)
		gatesMeasureCmd.SetContext(t.Context())
		if err := gatesMeasureCmd.RunE(gatesMeasureCmd, nil); err != nil {
			t.Fatalf("gates measure: %v", err)
		}
		return out.String()
	}
	first := measure()
	if !strings.Contains(first, "G2   vet            pass") || !strings.Contains(first, "G7   policy_corpus  unavailable") {
		t.Fatalf("first measurement:\n%s", first)
	}
	write("lib/say.go", "package lib\n\nimport \"fmt\"\n\n// Say prints n.\nfunc Say(n string) { fmt.Printf(\"%d\\n\", n) }\n")
	measure()

	paths, err := gates.SnapshotPaths(root)
	if err != nil || len(paths) != 2 {
		t.Fatalf("snapshots = %v, %v; want two under .nerd/gates", paths, err)
	}
	var out bytes.Buffer
	gatesDiffCmd.SetOut(&out)
	gatesFindings = true
	t.Cleanup(func() { gatesFindings = false })
	err = gatesDiffCmd.RunE(gatesDiffCmd, nil)
	if err == nil || !strings.Contains(err.Error(), "G2 vet") {
		t.Fatalf("gates diff = %v, want a refusal naming G2 vet:\n%s", err, out.String())
	}
	if !strings.Contains(out.String(), "NOT keepable") || !strings.Contains(out.String(), "+ lib/say.go | printf | ") {
		t.Fatalf("diff output:\n%s", out.String())
	}

	// The reverse pair is keepable.
	out.Reset()
	if err := gatesDiffCmd.RunE(gatesDiffCmd, []string{paths[1], paths[0]}); err != nil {
		t.Fatalf("the reverse diff: %v\n%s", err, out.String())
	}
}

// On codeNERD's own tree, G7 checks the policy corpus and G8 counts the atom
// corpus's failing Mangle examples from the files on disk -- the count the
// atom-corpus ratchet test holds.
func TestGatesCommand_CodeNERDTreeGates(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	g8 := atomCorpusGate{}.Measure(t.Context(), root)
	if g8.Status == gates.MeasureUnavailable || g8.Status == gates.MeasureUnverified {
		t.Fatalf("G8 on the tree = %s: %s", g8.Status, g8.Reason)
	}
	if len(g8.Findings) == 0 || len(g8.Findings) > 338 || !strings.Contains(g8.Findings[0].File, "internal/prompt/atoms/") {
		t.Fatalf("G8 found %d failing example(s), first %+v; want 1..338 in the atom files", len(g8.Findings), g8.Findings)
	}
	g7 := policyCorpusGate{}.Measure(t.Context(), root)
	if g7.Status != gates.MeasurePass && g7.Status != gates.MeasureFail {
		t.Fatalf("G7 on the tree = %s: %s", g7.Status, g7.Reason)
	}
}
