package observation

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"codenerd/internal/retain"
)

const goSource = `package widget

import (
	"fmt"
	"codenerd/internal/retain"
)

// Encode is named in this comment.
func Encode(v int) int {
	return v + 1
}

func caller() int {
	total := Encode(1)
	total += Encode(2)
	return total
}
`

// goSearch is what a search for "Encode" over goSource observes: the comment on
// line 8, the declaration on line 9, and the two call sites on lines 14 and 15.
func goSearch() Search {
	return Search{
		Query: "Encode",
		Match: []Match{
			{File: "widget.go", Line: 8, Text: "// Encode is named in this comment."},
			{File: "widget.go", Line: 9, Text: "func Encode(v int) int {"},
			{File: "widget.go", Line: 14, Text: "total := Encode(1)"},
			{File: "widget.go", Line: 15, Text: "total += Encode(2)"},
		},
	}
}

func fixedReader(files map[string]string) SourceReader {
	return func(file string) ([]byte, error) {
		content, ok := files[file]
		if !ok {
			return nil, fmt.Errorf("no such file: %s", file)
		}
		return []byte(content), nil
	}
}

func symbolByRef(t *testing.T, r CodeSearchResult, ref string) Symbol {
	t.Helper()
	for _, s := range r.Symbols {
		if s.Ref == ref {
			return s
		}
	}
	t.Fatalf("projection has no symbol %q; it has %v", ref, refsOf(r))
	return Symbol{}
}

func refsOf(r CodeSearchResult) []string {
	out := make([]string, 0, len(r.Symbols))
	for _, s := range r.Symbols {
		out = append(out, s.Ref)
	}
	return out
}

func TestProject_WhenMatchesLandInsideFunctions_ShouldNameTheSymbolsNotTheLines(t *testing.T) {
	t.Parallel()

	r := Project(goSearch(), fixedReader(map[string]string{"widget.go": goSource}), Limits{})

	if r.Matches != 4 || r.Files != 1 {
		t.Fatalf("projection must account for every observed match: got %d match(es) in %d file(s), want 4 in 1", r.Matches, r.Files)
	}

	decl := symbolByRef(t, r, "widget.go:Encode")
	if !decl.Declared {
		t.Errorf("a hit on a declaration line must mark the symbol as declared here, or the search cannot answer 'where is this defined'")
	}
	if decl.Kind != "function" {
		t.Errorf("symbol kind = %q, want function; the kind is what lets the reader tell a definition from a field", decl.Kind)
	}

	caller := symbolByRef(t, r, "widget.go:caller")
	if caller.Hits != 2 {
		t.Errorf("caller hits = %d, want 2: both call sites are inside caller, and collapsing them loses how heavily it depends on Encode", caller.Hits)
	}

	// The comment on line 8 sits outside every element, so it must be reported
	// as an unattributed file hit rather than silently attributed to the
	// function declared on the next line.
	if len(r.Unscoped) != 1 || r.Unscoped[0].File != "widget.go" || r.Unscoped[0].Hits != 1 {
		t.Errorf("hits outside every symbol = %v, want widget.go x1", r.Unscoped)
	}
}

func TestProject_WhenExactlyOneDeclarationIsFound_ShouldPointEdgesAtThatSymbol(t *testing.T) {
	t.Parallel()

	r := Project(goSearch(), fixedReader(map[string]string{"widget.go": goSource}), Limits{})

	var found bool
	for _, e := range r.Edges {
		if e.Kind != EdgeReferences {
			continue
		}
		found = true
		if e.From != "widget.go:caller" || e.To != "widget.go:Encode" {
			t.Errorf("reference edge = %s -> %s, want widget.go:caller -> widget.go:Encode", e.From, e.To)
		}
		if e.Count != 2 {
			t.Errorf("reference edge count = %d, want 2", e.Count)
		}
	}
	if !found {
		t.Fatalf("a search that found a declaration and two call sites must yield a reference edge; edges were %v", r.Edges)
	}

	// The declaration line is where the symbol is, not a use of it, so the
	// declaring symbol contributes no edge when that line was its only hit.
	for _, e := range r.Edges {
		if e.From == "widget.go:Encode" {
			t.Errorf("edge %v treats a declaration line as a use of the thing being declared", e)
		}
	}
}

func TestProject_WhenNoSingleDeclarationIsFound_ShouldPointEdgesAtTheQueryTerm(t *testing.T) {
	t.Parallel()

	// Two files each declaring Encode: naming one of them as the target of
	// every reference would be a guess, and a guess in a dependency edge is
	// worse than the honest answer that only the term is established.
	src := "package a\n\nfunc Encode() {}\n\nfunc use() {\n\tEncode()\n}\n"
	s := Search{
		Query: "Encode",
		Match: []Match{
			{File: "a.go", Line: 3, Text: "func Encode() {}"},
			{File: "b.go", Line: 3, Text: "func Encode() {}"},
			{File: "a.go", Line: 6, Text: "Encode()"},
		},
	}

	r := Project(s, fixedReader(map[string]string{"a.go": src, "b.go": src}), Limits{})

	for _, e := range r.Edges {
		if e.Kind == EdgeReferences && e.To != "Encode" {
			t.Errorf("with two declarations the reference target = %q, want the bare query term %q", e.To, "Encode")
		}
	}
}

func TestProject_WhenMatchIsAnImportLine_ShouldReportADependencyNotAReference(t *testing.T) {
	t.Parallel()

	// The imported path contains the searched term, which is exactly why an
	// import line gets misread as a use of it. Inside a Go `import (` block the
	// line is a bare quoted path with nothing on it that says "import", so the
	// file has to be scanned rather than the line inspected.
	s := Search{
		Query: "retain",
		Match: []Match{
			{File: "widget.go", Line: 5, Text: `"codenerd/internal/retain"`},
		},
	}

	r := Project(s, fixedReader(map[string]string{"widget.go": goSource}), Limits{})

	if len(r.Edges) != 1 {
		t.Fatalf("edges = %v, want exactly one import edge", r.Edges)
	}
	e := r.Edges[0]
	if e.Kind != EdgeImports || e.From != "widget.go" || e.To != "codenerd/internal/retain" {
		t.Errorf("edge = %+v, want widget.go -> codenerd/internal/retain imports", e)
	}
	if len(r.Unscoped) != 0 {
		t.Errorf("an import line is structure, not an unattributed hit; unscoped = %v", r.Unscoped)
	}
}

func TestProject_WhenHitIsInsideAMethodOfAClass_ShouldAttributeToTheInnermostSymbol(t *testing.T) {
	t.Parallel()

	py := "class Widget:\n    def encode(self):\n        return encode_all()\n\n    def other(self):\n        pass\n"
	s := Search{
		Query: "encode",
		Match: []Match{{File: "w.py", Line: 3, Text: "return encode_all()"}},
	}

	r := Project(s, fixedReader(map[string]string{"w.py": py}), Limits{})

	if len(r.Symbols) != 1 {
		t.Fatalf("symbols = %v, want exactly the innermost one", refsOf(r))
	}
	if r.Symbols[0].Ref != "w.py:encode" {
		t.Errorf("symbol = %q, want w.py:encode; attributing a hit to the enclosing class throws away the resolution this codec exists to provide", r.Symbols[0].Ref)
	}
}

func TestProject_WhenTheFileCannotBeRead_ShouldStillAccountForItsMatches(t *testing.T) {
	t.Parallel()

	s := Search{Query: "x", Match: []Match{{File: "gone.go", Line: 1, Text: "x"}}}
	r := Project(s, fixedReader(nil), Limits{})

	if r.Matches != 1 {
		t.Fatalf("match count = %d, want 1", r.Matches)
	}
	if len(r.Unscoped) != 1 || r.Unscoped[0].Hits != 1 {
		t.Errorf("a match in an unreadable file must be reported as unattributed, not dropped; unscoped = %v", r.Unscoped)
	}
}

func TestProject_WhenFileExceedsTheReadCeiling_ShouldNotProjectIt(t *testing.T) {
	t.Parallel()

	// A generated bundle is a legitimate search hit and an illegitimate read:
	// projecting it buys nothing and costs the whole file.
	huge := "package a\nfunc Encode() {}\n" + strings.Repeat("// filler\n", (maxProjectedFileBytes/10)+1)
	s := Search{Query: "Encode", Match: []Match{{File: "big.go", Line: 2, Text: "func Encode() {}"}}}

	r := Project(s, fixedReader(map[string]string{"big.go": huge}), Limits{})

	if len(r.Symbols) != 0 {
		t.Errorf("symbols = %v, want none: a file over the read ceiling is not projected", refsOf(r))
	}
	if len(r.Unscoped) != 1 {
		t.Errorf("unscoped = %v, want the oversized file's hits counted there", r.Unscoped)
	}
}

func TestProject_WhenSymbolsExceedTheLimit_ShouldCapAndSayHowManyWereDropped(t *testing.T) {
	t.Parallel()

	var src strings.Builder
	src.WriteString("package a\n")
	s := Search{Query: "fn"}
	for i := 0; i < 10; i++ {
		fmt.Fprintf(&src, "func fn%d() {\n\tfn()\n}\n", i)
		// Line of the body call inside the i-th function.
		s.Match = append(s.Match, Match{File: "a.go", Line: 3*i + 3, Text: "fn()"})
	}

	r := Project(s, fixedReader(map[string]string{"a.go": src.String()}), Limits{MaxSymbols: 3, MaxEdges: 2})

	if len(r.Symbols) != 3 || r.SymbolsOmitted != 7 {
		t.Errorf("symbols = %d (omitted %d), want 3 shown and 7 reported as dropped: a cap that hides its own omissions makes the projection look complete when it is not",
			len(r.Symbols), r.SymbolsOmitted)
	}
	if len(r.Edges) != 2 || r.EdgesOmitted != 8 {
		t.Errorf("edges = %d (omitted %d), want 2 shown and 8 reported as dropped", len(r.Edges), r.EdgesOmitted)
	}
}

func TestProject_ShouldProduceTheSameResultForTheSameObservation(t *testing.T) {
	t.Parallel()

	// codedom walks its pattern table with a map range, so element order is not
	// stable across runs. A projection that varied would break every caller
	// that compares or caches results, and would make the omitted tail of a
	// capped projection a different set each turn.
	read := fixedReader(map[string]string{"widget.go": goSource})
	first := Project(goSearch(), read, Limits{})
	for i := 0; i < 25; i++ {
		if got := Project(goSearch(), read, Limits{}); !reflect.DeepEqual(got, first) {
			t.Fatalf("projection %d differs from the first for identical input:\n%+v\nvs\n%+v", i, got, first)
		}
	}
}

func TestEncode_WhenThereAreNoMatches_ShouldNotPublishAHandle(t *testing.T) {
	t.Parallel()

	c := NewCodeSearch(retain.DefaultConfig())
	r := c.Encode(Search{Query: "nothing"}, fixedReader(nil), Limits{})

	if r.Handle != "" {
		t.Errorf("handle = %q, want empty: an expansion of nothing wastes a turn discovering that", r.Handle)
	}
}

func TestEncode_WhenTheSameObservationRepeats_ShouldReuseOneHandle(t *testing.T) {
	t.Parallel()

	c := NewCodeSearch(retain.DefaultConfig())
	read := fixedReader(map[string]string{"widget.go": goSource})

	first := c.Encode(goSearch(), read, Limits{})
	second := c.Encode(goSearch(), read, Limits{})

	if first.Handle == "" || first.Handle != second.Handle {
		t.Errorf("handles = %q and %q, want one content-addressed handle: a handle quoted back from an earlier turn has to still resolve while the same result is live",
			first.Handle, second.Handle)
	}
}

// TestHydrate_WhenSourceChangedAfterEncoding_ShouldReturnWhatWasObserved is the
// property the whole codec rests on. Re-running the search to expand it would
// answer from a world that has moved: an agent that reasoned about line 9 and
// then expanded to find something else there is holding a contradiction it
// cannot diagnose.
func TestHydrate_WhenSourceChangedAfterEncoding_ShouldReturnWhatWasObserved(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "widget.go")
	if err := os.WriteFile(path, []byte(goSource), 0o600); err != nil {
		t.Fatalf("seed source: %v", err)
	}

	c := NewCodeSearch(retain.DefaultConfig())
	encoded := c.Encode(goSearch(), func(string) ([]byte, error) { return os.ReadFile(path) }, Limits{})
	if encoded.Handle == "" {
		t.Fatal("encoding four matches must publish a handle")
	}

	// The file is rewritten so that a re-run would find nothing at all, and
	// then removed so that a re-run would fail outright.
	if err := os.WriteFile(path, []byte("package widget\n\nfunc Decode() {}\n"), 0o600); err != nil {
		t.Fatalf("rewrite source: %v", err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatalf("remove source: %v", err)
	}

	got, err := c.Hydrate(encoded.Handle, Window{})
	if err != nil {
		t.Fatalf("hydration must survive the source changing under it: %v", err)
	}
	want := goSearch().Match
	if !reflect.DeepEqual(got.Match, want) {
		t.Fatalf("hydration returned %+v, want the observed matches %+v", got.Match, want)
	}
	if got.Query != "Encode" {
		t.Errorf("hydrated query = %q, want the query the observation was made with", got.Query)
	}
}

func TestHydrate_ShouldNotTouchTheFilesystem(t *testing.T) {
	t.Parallel()

	reads := 0
	read := func(file string) ([]byte, error) {
		reads++
		return []byte(goSource), nil
	}

	c := NewCodeSearch(retain.DefaultConfig())
	encoded := c.Encode(goSearch(), read, Limits{})
	afterEncode := reads

	if _, err := c.Hydrate(encoded.Handle, Window{}); err != nil {
		t.Fatalf("hydrate: %v", err)
	}
	if reads != afterEncode {
		t.Errorf("hydration performed %d source read(s); it must answer from the retained bytes alone", reads-afterEncode)
	}
}

// TestCodeSearch_ShouldHoldNothingItCouldReRunWith pins the structural half of
// that guarantee. The comment on CodeSearch says hydration cannot re-run
// because the type has nothing to re-run with; a later edit that adds a source
// reader, a workspace root or a search function would quietly make the comment
// false while every other test still passed.
func TestCodeSearch_ShouldHoldNothingItCouldReRunWith(t *testing.T) {
	t.Parallel()

	typ := reflect.TypeOf(CodeSearch{})
	if typ.NumField() != 1 {
		t.Fatalf("CodeSearch has %d fields; it must hold only its retention, or hydration can grow a way to consult the live world", typ.NumField())
	}
	if got := typ.Field(0).Type; got != reflect.TypeOf((*retain.Store)(nil)) {
		t.Fatalf("CodeSearch's only field is %s, want *retain.Store", got)
	}
}

func TestHydrate_WhenWindowed_ShouldPageWithoutLosingAMatch(t *testing.T) {
	t.Parallel()

	c := NewCodeSearch(retain.DefaultConfig())
	encoded := c.Encode(goSearch(), fixedReader(map[string]string{"widget.go": goSource}), Limits{})

	var seen []Match
	offset := 0
	for i := 0; i < 10; i++ {
		page, err := c.Hydrate(encoded.Handle, Window{Offset: offset, Limit: 3})
		if err != nil {
			t.Fatalf("hydrate page at offset %d: %v", offset, err)
		}
		if page.Total != 4 {
			t.Fatalf("page total = %d, want the 4 retained matches", page.Total)
		}
		seen = append(seen, page.Match...)
		if page.NextOffset == 0 {
			break
		}
		offset = page.NextOffset
	}

	if !reflect.DeepEqual(seen, goSearch().Match) {
		t.Errorf("walking the window returned %+v, want every retained match exactly once", seen)
	}
}

func TestHydrate_WhenWindowNamesAFile_ShouldReturnOnlyThatFilesMatches(t *testing.T) {
	t.Parallel()

	s := Search{
		Query: "x",
		Match: []Match{
			{File: "a.go", Line: 1, Text: "x"},
			{File: "b.go", Line: 2, Text: "x"},
			{File: "a.go", Line: 3, Text: "x"},
		},
	}
	c := NewCodeSearch(retain.DefaultConfig())
	encoded := c.Encode(s, fixedReader(nil), Limits{})

	got, err := c.Hydrate(encoded.Handle, Window{File: "a.go"})
	if err != nil {
		t.Fatalf("hydrate: %v", err)
	}
	if got.Total != 2 {
		t.Fatalf("filtered total = %d, want the 2 matches in a.go", got.Total)
	}
	for _, m := range got.Match {
		if m.File != "a.go" {
			t.Errorf("file filter returned a match in %s", m.File)
		}
	}
}

func TestHydrate_WhenLimitIsAbsurd_ShouldStillBound(t *testing.T) {
	t.Parallel()

	s := Search{Query: "x"}
	for i := 1; i <= maxHydrateWindow+50; i++ {
		s.Match = append(s.Match, Match{File: "a.go", Line: i, Text: "x"})
	}
	c := NewCodeSearch(retain.DefaultConfig())
	encoded := c.Encode(s, fixedReader(nil), Limits{})

	got, err := c.Hydrate(encoded.Handle, Window{Limit: 100000})
	if err != nil {
		t.Fatalf("hydrate: %v", err)
	}
	if len(got.Match) != maxHydrateWindow {
		t.Errorf("expansion returned %d matches, want the hard cap of %d: the reason the lines were withheld is that there were too many of them",
			len(got.Match), maxHydrateWindow)
	}
	if got.NextOffset != maxHydrateWindow {
		t.Errorf("next offset = %d, want %d so the walk can continue", got.NextOffset, maxHydrateWindow)
	}
}

func TestHydrate_WhenHandleIsUnknownOrExpired_ShouldSaySoDistinctly(t *testing.T) {
	t.Parallel()

	c := NewCodeSearch(retain.Config{TTL: time.Minute})

	if _, err := c.Hydrate("obs:cs:deadbeef", Window{}); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown handle error = %v, want ErrNotFound: the caller's answer is to run the search again, not to retry the expansion", err)
	}
	if _, err := c.Hydrate("   ", Window{}); !errors.Is(err, ErrNotFound) {
		t.Errorf("empty handle error = %v, want ErrNotFound", err)
	}

	encoded := c.Encode(goSearch(), fixedReader(map[string]string{"widget.go": goSource}), Limits{})
	if _, err := c.Hydrate(encoded.Handle, Window{}); err != nil {
		t.Fatalf("fresh handle must resolve: %v", err)
	}
}

func TestResultText_ShouldNameTheHandleAndTheVerbThatRedeemsIt(t *testing.T) {
	t.Parallel()

	c := NewCodeSearch(retain.DefaultConfig())
	r := c.Encode(goSearch(), fixedReader(map[string]string{"widget.go": goSource}), Limits{})
	text := r.Text("search_expand")

	for _, want := range []string{"widget.go:Encode", "declared-here", "widget.go:caller", r.Handle, "search_expand"} {
		if !strings.Contains(text, want) {
			t.Errorf("rendered projection is missing %q; a handle nobody is told how to redeem is a handle nobody redeems.\n%s", want, text)
		}
	}
	// The point of the codec is that the matching lines do not enter context.
	if strings.Contains(text, "total := Encode(1)") {
		t.Errorf("rendered projection quoted a matching line back:\n%s", text)
	}
}

// TestResultText_ShouldNameEachSymbolOnce protects the cost of the projection,
// which is the only reason it exists.
//
// A reference edge's source is always a symbol already listed, and every
// reference edge from one search shares a target by construction. Rendering
// edges with both endpoints therefore printed each referencing symbol twice and
// repeated the target on every row — which made the projection LARGER than the
// wall of matching lines it replaced on exactly the searches it should help
// most: many symbols, one hit each. Measured on this repo before the fix, such
// a search cost 3420 bytes against grep's 2019.
func TestResultText_ShouldNameEachSymbolOnce(t *testing.T) {
	t.Parallel()

	src := "package a\n\nfunc Encode() {}\n\nfunc one() {\n\tEncode()\n}\n\nfunc two() {\n\tEncode()\n}\n"
	s := Search{
		Query: "Encode",
		Match: []Match{
			{File: "a.go", Line: 3, Text: "func Encode() {}"},
			{File: "a.go", Line: 6, Text: "Encode()"},
			{File: "a.go", Line: 10, Text: "Encode()"},
		},
	}
	r := Project(s, fixedReader(map[string]string{"a.go": src}), Limits{})
	text := r.Text("")

	for _, ref := range []string{"a.go:one", "a.go:two"} {
		if got := strings.Count(text, ref); got != 1 {
			t.Errorf("%s appears %d times, want once: a referencing symbol listed twice is the projection paying for the same fact twice.\n%s", ref, got, text)
		}
	}
	// The declaration is named twice and only twice: as its own row, and as the
	// target the other rows point at.
	if got := strings.Count(text, "a.go:Encode"); got != 2 {
		t.Errorf("the declaration is named %d times, want its own row plus one target line:\n%s", got, text)
	}
	if !strings.Contains(text, "references point to a.go:Encode") {
		t.Errorf("the rendering never says what the listed symbols reference:\n%s", text)
	}
}

func TestResultText_WhenASymbolBothDeclaresAndUses_ShouldSplitTheCount(t *testing.T) {
	t.Parallel()

	// Recursion: the hit count alone would read as all declaration, and the
	// reader would conclude nothing calls it.
	src := "package a\n\nfunc Encode(n int) int {\n\treturn Encode(n - 1)\n}\n"
	s := Search{
		Query: "Encode",
		Match: []Match{
			{File: "a.go", Line: 3, Text: "func Encode(n int) int {"},
			{File: "a.go", Line: 4, Text: "return Encode(n - 1)"},
		},
	}
	r := Project(s, fixedReader(map[string]string{"a.go": src}), Limits{})

	if !strings.Contains(r.Text(""), "declared-here (1 ref)") {
		t.Errorf("a symbol that declares and uses the searched thing must report both:\n%s", r.Text(""))
	}
}

func TestHydratedText_WhenSearchWasCapped_ShouldNotLetTheReaderConcludeThatIsAll(t *testing.T) {
	t.Parallel()

	s := goSearch()
	s.Truncated = true
	c := NewCodeSearch(retain.DefaultConfig())
	encoded := c.Encode(s, fixedReader(map[string]string{"widget.go": goSource}), Limits{})

	got, err := c.Hydrate(encoded.Handle, Window{})
	if err != nil {
		t.Fatalf("hydrate: %v", err)
	}
	text := got.Text()
	if !strings.Contains(text, "result cap") {
		t.Errorf("reaching the end of a capped search's matches must not read as 'that is all of them':\n%s", text)
	}
	if !strings.Contains(text, "widget.go:14: total := Encode(1)") {
		t.Errorf("hydration must print the observed lines:\n%s", text)
	}
}

func TestShared_ShouldBeOneStoreForEveryProducer(t *testing.T) {
	t.Parallel()

	// A handle minted by one code-search path is redeemed by a different call
	// site entirely. Separate stores would make every published handle
	// unredeemable, and the agent would spend a turn finding that out.
	if Shared() != Shared() {
		t.Fatal("Shared returned two codecs; a handle minted through one would not resolve through the other")
	}

	encoded := Shared().Encode(goSearch(), fixedReader(map[string]string{"widget.go": goSource}), Limits{})
	if _, err := Shared().Hydrate(encoded.Handle, Window{}); err != nil {
		t.Fatalf("a handle minted through Shared must resolve through Shared: %v", err)
	}
}
