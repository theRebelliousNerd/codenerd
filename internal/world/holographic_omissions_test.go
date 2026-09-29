package world

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"unicode/utf8"

	"codenerd/internal/core"
	"codenerd/internal/tools/catalog"
	"codenerd/internal/tools/codedom"
)

// The tools a holographic omission may name. Checked against catalog.Names,
// which is the production register sequence, so a renamed tool cannot stay
// in the prompt after it leaves the catalog.
var omissionTools = []string{
	"package_outline",
	"get_elements",
	"get_element",
	"callers_of",
	"importers_of",
}

func TestOmissionToolsAreInTheCatalog(t *testing.T) {
	names, err := catalog.Names()
	if err != nil {
		t.Fatalf("catalog.Names: %v", err)
	}
	have := make(map[string]struct{}, len(names))
	for _, name := range names {
		have[name] = struct{}{}
	}
	for _, tool := range omissionTools {
		if _, ok := have[tool]; !ok {
			t.Errorf("%s is not in catalog.Names()", tool)
		}
	}
}

// TestPromptSection_OmissionsStateTheTrueRemainder is the contract for every
// bound that used to shrink a pool before its "and N more" line: a package
// past the file cap, a sibling past the byte cap, a call graph past the edge
// cap, and a doc comment past the old 100-character cut. Each count in the
// section is the true one, and each omission names a tool that reads the rest.
func TestPromptSection_OmissionsStateTheTrueRemainder(t *testing.T) {
	dir := t.TempDir()
	docBody := strings.Repeat("d", 180)
	longParam := strings.Repeat("a", 120)
	target := filepath.Join(dir, "aaa_target.go")
	src := fmt.Sprintf("package p\n\n// %s\nfunc Documented() {}\n\nfunc LongSig(%s int) {}\n", docBody, longParam)
	if err := os.WriteFile(target, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	// 100 siblings. With the target and the oversized file that is 102 Go
	// files. Name order parses the first 100, so aaa_target.go is parsed and
	// the last extra plus the oversized file are not.
	const extraFiles = 100
	for i := 0; i < extraFiles; i++ {
		body := fmt.Sprintf("package p\n\nfunc Extra%d() {}\n\ntype Extra%dType struct{}\n", i, i)
		if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("extra_%03d.go", i)), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	hugeName := "zz_generated_monster.go"
	hugePath := filepath.Join(dir, hugeName)
	f, err := os.Create(hugePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(maxSiblingFileBytes + 1); err != nil {
		f.Close()
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	hugeInfo, err := os.Stat(hugePath)
	if err != nil {
		t.Fatal(err)
	}

	const bareEdges = 120
	calls := make([]core.Fact, 0, bareEdges*2)
	for i := 0; i < bareEdges; i++ {
		caller := fmt.Sprintf("caller_%03d", i)
		calls = append(calls, core.Fact{Predicate: "code_calls", Args: []any{caller, "p.Documented"}})
		// A duplicate CodeDOM-ref row. It must not change either count.
		if i%4 == 0 {
			calls = append(calls, core.Fact{Predicate: "code_calls", Args: []any{"fn:" + caller, "fn:p.Documented"}})
		}
	}
	q := &stubQuerier{facts: map[string][]core.Fact{
		"code_defines": {{
			Predicate: "code_defines",
			Args:      []any{target, "p.Documented", "/function", int64(3), int64(4)},
		}},
		"code_calls": calls,
	}}

	h := NewHolographicProvider(q, dir)
	hc, err := h.GetContext(target)
	if err != nil {
		t.Fatalf("GetContext: %v", err)
	}

	wantUnparsed := expectUnparsedGoFiles(t, dir)
	if wantUnparsed < 2 {
		t.Fatalf("fixture did not leave files unparsed (got %d)", wantUnparsed)
	}
	if hc.FilesUnparsed != wantUnparsed {
		t.Fatalf("FilesUnparsed = %d, want %d (the files that did not parse)", hc.FilesUnparsed, wantUnparsed)
	}
	if len(hc.SkippedSiblings) != 1 || filepath.Base(hc.SkippedSiblings[0].File) != hugeName || hc.SkippedSiblings[0].Size != hugeInfo.Size() {
		t.Fatalf("SkippedSiblings = %+v, want %s at %d bytes", hc.SkippedSiblings, hugeName, hugeInfo.Size())
	}
	if hc.CallGraphEdges != bareEdges {
		t.Fatalf("CallGraphEdges = %d, want %d (fn: duplicates must not count)", hc.CallGraphEdges, bareEdges)
	}
	if hc.CallerCount != bareEdges {
		t.Fatalf("CallerCount = %d, want %d distinct callers", hc.CallerCount, bareEdges)
	}
	if len(hc.CallGraph) != maxCallGraphEdges {
		t.Fatalf("stored CallGraph edges = %d, want the storage cap %d", len(hc.CallGraph), maxCallGraphEdges)
	}

	var gotDoc string
	for _, sig := range hc.PackageSignatures {
		if sig.Name == "Documented" && sig.File == filepath.Base(target) {
			gotDoc = sig.DocComment
		}
	}
	if gotDoc != docBody {
		t.Fatalf("Documented doc comment = %q (len %d), want the whole first line (len %d)", gotDoc, len(gotDoc), len(docBody))
	}

	section := h.PromptSection(context.Background(), target)
	again := h.PromptSection(context.Background(), target)
	if section != again {
		t.Fatal("a cache hit rendered a different section than the parse that filled it")
	}

	unparsedPhrase := fmt.Sprintf("%d of the package's Go files were not parsed", wantUnparsed)
	if wantUnparsed == 1 {
		unparsedPhrase = "1 of the package's Go files was not parsed"
	}
	if !strings.Contains(section, unparsedPhrase) {
		t.Errorf("section does not state the unparsed-file count %q:\n%s", unparsedPhrase, section)
	}
	hugePhrase := fmt.Sprintf("`%s` (%d bytes) exceeds the %d-byte sibling parse bound", hugeName, hugeInfo.Size(), maxSiblingFileBytes)
	if !strings.Contains(section, hugePhrase) {
		t.Errorf("section does not name the oversized sibling %q:\n%s", hugePhrase, section)
	}
	for _, tool := range []string{"package_outline", "get_elements"} {
		if !strings.Contains(section, "`"+tool+"`") {
			t.Errorf("unparsed-file note does not name %s", tool)
		}
	}

	exported := 0
	for _, sig := range hc.PackageSignatures {
		if sig.Exported {
			exported++
		}
	}
	if exported <= maxSigs {
		t.Fatalf("fixture exported %d signatures, want more than %d so the remainder line exists", exported, maxSigs)
	}
	sigPhrase := fmt.Sprintf("and %d more exported among the parsed files of package `p`", exported-maxSigs)
	if !strings.Contains(section, sigPhrase) {
		t.Errorf("signature remainder is not the parsed-pool count %q:\n%s", sigPhrase, section)
	}
	if strings.Contains(section, "more exported in package `") {
		t.Errorf("signature remainder claims the whole package while %d files were not parsed:\n%s", hc.FilesUnparsed, section)
	}
	if len(hc.PackageTypes) <= maxTypes {
		t.Fatalf("fixture has %d types, want more than %d", len(hc.PackageTypes), maxTypes)
	}
	typePhrase := fmt.Sprintf("and %d more among the parsed files of package `p`", len(hc.PackageTypes)-maxTypes)
	if !strings.Contains(section, typePhrase) {
		t.Errorf("type remainder is not the parsed-pool count %q:\n%s", typePhrase, section)
	}
	if strings.Contains(section, "more in package `") {
		t.Errorf("type remainder claims the whole package while files were not parsed:\n%s", section)
	}

	// The undercount the storage cap used to render: unique callers in the
	// stored prefix, minus the names shown. The section must say the true
	// remainder instead.
	storedCallers := map[string]struct{}{}
	for _, e := range hc.CallGraph {
		storedCallers[e.Caller] = struct{}{}
	}
	undercount := len(storedCallers) - maxCallers
	trueOmitted := hc.CallerCount - maxCallers
	if undercount == trueOmitted {
		t.Fatalf("fixture does not distinguish the stored-prefix remainder (%d) from the true one", undercount)
	}
	callerPhrase := fmt.Sprintf("and %d more callers (%d matching call-graph edges, %d not stored here)",
		trueOmitted, hc.CallGraphEdges, hc.CallGraphEdges-len(hc.CallGraph))
	if !strings.Contains(section, callerPhrase) {
		t.Errorf("caller remainder is not the true count %q:\n%s", callerPhrase, section)
	}
	wrong := fmt.Sprintf("and %d more callers", undercount)
	if strings.Contains(section, wrong) {
		t.Errorf("section states the stored-prefix undercount %q:\n%s", wrong, section)
	}
	if strings.Contains(section, "`fn:") {
		t.Errorf("a duplicate fn: row was rendered as a caller:\n%s", section)
	}
	if !strings.Contains(section, "`callers_of`") {
		t.Errorf("caller remainder does not name callers_of:\n%s", section)
	}
	if strings.Contains(section, docBody[:100]+"...") {
		t.Errorf("doc comment was cut at 100 characters in the section")
	}

	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	elements := codedom.ElementsFromSource(target, string(data))
	var longCut bool
	for _, el := range elements {
		label := strings.TrimSpace(el.Signature)
		if label == "" {
			label = el.Name
		}
		n := utf8.RuneCountInString(label)
		if n <= maxOutlineSignature {
			continue
		}
		longCut = true
		rest := n - maxOutlineSignature
		if !strings.Contains(section, fmt.Sprintf("%d more characters", rest)) || !strings.Contains(section, "`get_element`") {
			t.Errorf("outline cut of %q does not state the %d characters it left off", label, rest)
		}
	}
	if !longCut {
		t.Fatal("fixture produced no outline signature over maxOutlineSignature")
	}
}

// expectUnparsedGoFiles applies the parse bounds the same way parsePackage
// does: name order, the first maxPackageFilesToParse files are attempted, a
// file over maxSiblingFileBytes is not parsed, and every other Go file is.
// The fixture's small files all parse, so a file inside the window that is
// not oversized counts as parsed.
func expectUnparsedGoFiles(t *testing.T, dir string) int {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasSuffix(name, ".go") && !strings.HasSuffix(name, "_test.go") {
			names = append(names, name)
		}
	}
	// Same order parsePackage uses. Full paths in one directory sort as base names.
	sort.Strings(names)
	attempted := len(names)
	if attempted > maxPackageFilesToParse {
		attempted = maxPackageFilesToParse
	}
	parsed := 0
	for i, name := range names {
		if i >= attempted {
			break
		}
		info, err := os.Stat(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		if info.Size() > maxSiblingFileBytes {
			continue
		}
		parsed++
	}
	return len(names) - parsed
}

// TestPromptSection_PrioritizedCallerRemainderNamesTheTool pins the impact
// branch, which is a different pool from the call graph. The list is not
// sliced before the count, and the line names callers_of.
func TestPromptSection_PrioritizedCallerRemainderNamesTheTool(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target.go")
	if err := os.WriteFile(target, []byte("package p\n\nfunc Target() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	const n = maxCallers + 2
	facts := make([]core.Fact, 0, n)
	for i := 0; i < n; i++ {
		facts = append(facts, core.Fact{
			Predicate: "context_priority_file",
			Args:      []any{fmt.Sprintf("f%d.go", i), fmt.Sprintf("Fn%02d", i), int64(3)},
		})
	}
	h := NewHolographicProvider(&stubQuerier{facts: map[string][]core.Fact{
		"context_priority_file": facts,
	}}, dir)
	section := h.PromptSection(context.Background(), target)
	want := fmt.Sprintf("and %d more callers; `callers_of` lists every call site", n-maxCallers)
	if !strings.Contains(section, want) {
		t.Fatalf("prioritized remainder = missing %q:\n%s", want, section)
	}
	if strings.Contains(section, "not stored here") {
		t.Fatalf("prioritized remainder was mixed with the call-graph storage cap:\n%s", section)
	}
}
