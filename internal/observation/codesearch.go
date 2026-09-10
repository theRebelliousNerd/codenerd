package observation

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"

	"codenerd/internal/retain"
	"codenerd/internal/tools/codedom"
)

// Match is one hit exactly as the search reported it: the path it displayed,
// the line number it counted, and the text it would have printed.
type Match struct {
	File string `json:"file"`
	Line int    `json:"line"`
	Text string `json:"text"`
}

// Search is the raw observation a code search produced.
//
// This is the shape that gets retained, so it is deliberately the whole of what
// the search found rather than a summary of it. A handle that expands to
// something less than the tool saw is a handle that will be discovered to be
// lossy at the worst moment — when an agent redeems it because the projection
// was not enough.
type Search struct {
	Query string  `json:"query"`
	Match []Match `json:"matches"`
	// Truncated records that the search itself stopped at its result cap, so a
	// hydration that reaches the end of the retained matches can say whether
	// that is the end of the matches or the end of what was looked at.
	Truncated bool `json:"truncated,omitempty"`
}

// Symbol is a code element the search landed inside.
type Symbol struct {
	// Ref is "file:Name", the form used on both ends of an edge.
	Ref       string `json:"ref"`
	Name      string `json:"name"`
	Kind      string `json:"kind"`
	File      string `json:"file"`
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
	// Declared marks a hit that landed on the symbol's own declaration line —
	// this is where the searched thing is defined, not merely mentioned.
	Declared bool `json:"declared,omitempty"`
	// Hits counts matching lines inside the symbol, declaration included.
	Hits int `json:"hits"`
}

// Edge kinds. Only dependency-bearing relations are edges; where a symbol is
// declared is a property of the symbol, not a relation, and duplicating it here
// would charge for the same fact twice.
const (
	EdgeReferences = "references"
	EdgeImports    = "imports"
)

// Edge is a dependency the search revealed.
type Edge struct {
	From  string `json:"from"`
	To    string `json:"to"`
	Kind  string `json:"kind"`
	Count int    `json:"count"`
}

// FileHits counts matches in a file that fell outside every code element —
// package clauses, top-level comments, data files. They are reported per file
// rather than per line because that is all the structure there is: enumerating
// them would be the wall of lines this codec exists to avoid.
type FileHits struct {
	File string `json:"file"`
	Hits int    `json:"hits"`
}

// CodeSearchResult is the projection handed to the reasoning.
type CodeSearchResult struct {
	Query     string `json:"query"`
	Matches   int    `json:"matches"`
	Files     int    `json:"files"`
	Truncated bool   `json:"truncated,omitempty"`

	Symbols         []Symbol   `json:"symbols,omitempty"`
	SymbolsOmitted  int        `json:"symbols_omitted,omitempty"`
	Edges           []Edge     `json:"edges,omitempty"`
	EdgesOmitted    int        `json:"edges_omitted,omitempty"`
	Unscoped        []FileHits `json:"unscoped,omitempty"`
	UnscopedOmitted int        `json:"unscoped_omitted,omitempty"`

	// Handle redeems the retained raw observation. Empty means nothing was
	// retained, which is only the zero-match case.
	Handle string `json:"handle,omitempty"`
}

// SourceReader returns the bytes of a matched file.
//
// Projection takes this as an argument rather than holding one, which is what
// makes hydration structurally unable to re-read anything: the codec type has
// no reader field for Hydrate to reach.
type SourceReader func(file string) ([]byte, error)

// Limits bound the projection. The codec's whole purpose is a bounded cost per
// observation, so a search that matches five hundred symbols must still cost
// what forty cost; the counts of what was dropped are reported so the omission
// is visible rather than silent.
type Limits struct {
	MaxSymbols  int
	MaxEdges    int
	MaxUnscoped int
}

// DefaultLimits sizes a projection for a working turn.
func DefaultLimits() Limits {
	return Limits{MaxSymbols: 40, MaxEdges: 60, MaxUnscoped: 20}
}

func (l Limits) resolved() Limits {
	def := DefaultLimits()
	if l.MaxSymbols <= 0 {
		l.MaxSymbols = def.MaxSymbols
	}
	if l.MaxEdges <= 0 {
		l.MaxEdges = def.MaxEdges
	}
	if l.MaxUnscoped <= 0 {
		l.MaxUnscoped = def.MaxUnscoped
	}
	return l
}

// maxProjectedFileBytes bounds a single file read during projection.
//
// A search legitimately matches generated files — a 30 MB bundle, a vendored
// blob, a checked-in fixture. Extracting elements from one buys nothing (the
// regexes find nothing useful in minified output) and would turn a bounded
// projection into an unbounded read. Over the ceiling, the file's hits are
// reported as unscoped, which is the truth: nothing structural was learned.
const maxProjectedFileBytes = 1 << 20

// retentionKind labels retained payloads. It is part of the content address,
// so an identical byte sequence retained by a different codec gets a different
// id and the two can never be confused for one another.
const retentionKind = "code_search"

// handlePrefix marks an id as redeemable so a model can recognise one in a
// result without being told what it is, and so an id minted here is visibly not
// an MCP handle.
const handlePrefix = "obs:cs:"

// CodeSearch encodes code-search observations and retains the raw ones.
//
// It holds a retain.Store and nothing else, on purpose. Every field on this
// type is something Hydrate could reach, and the one guarantee hydration has to
// make is that it answers from the retained bytes rather than from the world as
// it stands now. A source reader, a workspace root or a search function stored
// here would each be a way for that guarantee to be quietly lost in a later
// edit; with none of them present, re-running is not a thing this type can do.
type CodeSearch struct {
	store *retain.Store
}

// NewCodeSearch creates a codec over its own retention.
func NewCodeSearch(cfg retain.Config) *CodeSearch {
	cfg.Prefix = handlePrefix
	return &CodeSearch{store: retain.New(cfg)}
}

var (
	sharedOnce sync.Once
	sharedCS   *CodeSearch
)

// Shared returns the process-wide code-search codec.
//
// One store, not one per producer. A handle is minted by whichever path ran the
// search and redeemed later by a different call site entirely; if those two
// held separate stores, every published handle would be unredeemable and the
// agent would spend a turn finding that out. This is the same arrangement the
// MCP control plane uses for the same reason.
func Shared() *CodeSearch {
	sharedOnce.Do(func() {
		sharedCS = NewCodeSearch(retain.DefaultConfig())
	})
	return sharedCS
}

// Encode projects an observation and retains the raw form under a handle.
//
// Retention failing is not an error the caller has to handle: the projection is
// still correct and still useful, it simply cannot be expanded. An empty Handle
// says so, and that is better than refusing to return the structure because the
// bytes did not fit.
func (c *CodeSearch) Encode(s Search, read SourceReader, limits Limits) CodeSearchResult {
	result := Project(s, read, limits)
	if c == nil || len(s.Match) == 0 {
		return result
	}
	payload, err := json.Marshal(s)
	if err != nil {
		// Marshalling a struct of strings and ints cannot fail today. If a
		// future field makes it possible, the projection must still be
		// returned: losing the whole observation because the expandable copy
		// could not be encoded is a strictly worse failure.
		return result
	}
	result.Handle = c.store.Mint(retentionKind, payload)
	return result
}

// Window bounds a hydration. Expanding is itself budgeted, because the reason
// the raw lines were withheld is that there were too many of them, and an
// expansion that returns all of them has moved the problem one turn later.
type Window struct {
	// File limits the window to one path, which is the axis that matters:
	// "show me the raw lines in the file I care about" is the question a
	// projection provokes.
	File   string
	Offset int
	Limit  int
}

// Hydrated is the raw observation, as observed, bounded to a window.
type Hydrated struct {
	Handle    string  `json:"handle"`
	Query     string  `json:"query"`
	Match     []Match `json:"matches"`
	Total     int     `json:"total"`
	Offset    int     `json:"offset"`
	Truncated bool    `json:"truncated,omitempty"`
	// NextOffset is zero once the window reached the end of the retained
	// matches; otherwise it is the offset that continues the walk.
	NextOffset int `json:"next_offset,omitempty"`
}

// ErrNotFound is returned for an unknown or expired handle. The caller's
// correct response differs from every other failure: run the search again, do
// not retry the expansion.
var ErrNotFound = retain.ErrNotFound

// maxHydrateWindow caps a single expansion regardless of what was asked for.
const maxHydrateWindow = 200

// defaultHydrateWindow is what an unbounded request gets.
const defaultHydrateWindow = 40

// Hydrate reopens a retained observation.
//
// It reads the retained bytes and consults nothing else. That is the point: a
// re-run would answer from a world that has moved since the reasoning was
// built, so an agent that cited a line and then expanded to find different
// content would be holding a contradiction with no way to diagnose it — and for
// a tool with any side effect at all, "show me the rest of what you already
// told me" would be a second side effect.
func (c *CodeSearch) Hydrate(handle string, w Window) (Hydrated, error) {
	if c == nil {
		return Hydrated{}, ErrNotFound
	}
	handle = strings.TrimSpace(handle)
	if handle == "" {
		return Hydrated{}, fmt.Errorf("%w: empty handle", ErrNotFound)
	}

	kind, payload, err := c.store.Get(handle)
	if err != nil {
		return Hydrated{}, err
	}
	if kind != retentionKind {
		return Hydrated{}, fmt.Errorf("%w: %s is not a code-search observation", ErrNotFound, handle)
	}

	var s Search
	if err := json.Unmarshal(payload, &s); err != nil {
		return Hydrated{}, fmt.Errorf("retained observation %s is unreadable: %w", handle, err)
	}

	selected := s.Match
	if file := strings.TrimSpace(w.File); file != "" {
		filtered := make([]Match, 0, len(s.Match))
		for _, m := range s.Match {
			if m.File == file {
				filtered = append(filtered, m)
			}
		}
		selected = filtered
	}

	limit := w.Limit
	if limit <= 0 {
		limit = defaultHydrateWindow
	}
	if limit > maxHydrateWindow {
		limit = maxHydrateWindow
	}
	offset := w.Offset
	if offset < 0 {
		offset = 0
	}
	if offset > len(selected) {
		offset = len(selected)
	}
	end := offset + limit
	if end > len(selected) {
		end = len(selected)
	}

	out := Hydrated{
		Handle:    handle,
		Query:     s.Query,
		Match:     append([]Match(nil), selected[offset:end]...),
		Total:     len(selected),
		Offset:    offset,
		Truncated: s.Truncated,
	}
	if end < len(selected) {
		out.NextOffset = end
	}
	return out, nil
}

// Project turns raw matches into symbols and dependency edges.
//
// It is exported separately from Encode so the projection can be exercised, and
// reasoned about, without retention in the picture: the two halves fail
// differently and a test that could not separate them would be testing both at
// once.
func Project(s Search, read SourceReader, limits Limits) CodeSearchResult {
	limits = limits.resolved()

	result := CodeSearchResult{
		Query:     s.Query,
		Matches:   len(s.Match),
		Truncated: s.Truncated,
	}

	// First appearance order, so the projection of a given observation is the
	// same every time it is computed. Map iteration here would make the
	// omitted tail of a capped result vary between two runs of the same search.
	var order []string
	byFile := make(map[string][]Match)
	for _, m := range s.Match {
		if _, seen := byFile[m.File]; !seen {
			order = append(order, m.File)
		}
		byFile[m.File] = append(byFile[m.File], m)
	}
	result.Files = len(order)

	symbols := make(map[string]*Symbol)
	var symbolOrder []string
	// declHits counts, per symbol, the matches that landed on its declaration
	// line. Reference edges exclude those: a declaration is where the thing is,
	// not a dependency on it.
	declHits := make(map[string]int)
	imports := make(map[string]*Edge)
	var importOrder []string
	unscoped := make(map[string]int)

	for _, file := range order {
		matches := byFile[file]

		var content string
		if read != nil {
			if data, err := read(file); err == nil && len(data) <= maxProjectedFileBytes {
				content = string(data)
			}
		}
		if content == "" {
			unscoped[file] += len(matches)
			continue
		}

		elements := sortedElements(codedom.ElementsFromSource(file, content))
		importAt := importTargets(file, content, matches)

		for _, m := range matches {
			if target, ok := importAt[m.Line]; ok {
				key := file + "\x00" + target
				if edge, seen := imports[key]; seen {
					edge.Count++
				} else {
					imports[key] = &Edge{From: file, To: target, Kind: EdgeImports, Count: 1}
					importOrder = append(importOrder, key)
				}
				continue
			}

			element, ok := innermost(elements, m.Line)
			if !ok {
				unscoped[file]++
				continue
			}

			ref := file + ":" + element.Name
			sym, seen := symbols[ref]
			if !seen {
				sym = &Symbol{
					Ref:       ref,
					Name:      element.Name,
					Kind:      element.Type,
					File:      file,
					StartLine: element.StartLine,
					EndLine:   element.EndLine,
				}
				symbols[ref] = sym
				symbolOrder = append(symbolOrder, ref)
			}
			sym.Hits++
			if m.Line == element.StartLine {
				sym.Declared = true
				declHits[ref]++
			}
		}
	}

	// The edge target: when the search found exactly one declaration, every
	// reference is a dependency on that specific symbol and the edge can say
	// so. With none, or with several, naming one would be a guess, so the edges
	// point at the query term — which is what was actually established.
	target := strings.TrimSpace(s.Query)
	declared := 0
	for _, ref := range symbolOrder {
		if symbols[ref].Declared {
			declared++
			if declared == 1 {
				target = ref
			}
		}
	}
	if declared != 1 {
		target = strings.TrimSpace(s.Query)
	}

	edges := make([]Edge, 0, len(symbolOrder)+len(importOrder))
	for _, ref := range symbolOrder {
		// Only the declaration line is excluded. A symbol whose remaining hits
		// point at itself is recursing, and that is a real dependency: a
		// recursive function reported as pure declaration reads as "nothing
		// calls this", which is the opposite of true.
		refs := symbols[ref].Hits - declHits[ref]
		if refs <= 0 {
			continue
		}
		edges = append(edges, Edge{From: ref, To: target, Kind: EdgeReferences, Count: refs})
	}
	for _, key := range importOrder {
		edges = append(edges, *imports[key])
	}

	result.Symbols, result.SymbolsOmitted = capSymbols(symbols, symbolOrder, limits.MaxSymbols)
	result.Edges, result.EdgesOmitted = capEdges(edges, limits.MaxEdges)
	result.Unscoped, result.UnscopedOmitted = capUnscoped(unscoped, order, limits.MaxUnscoped)
	return result
}

// sortedElements gives the extracted elements a stable order.
//
// codedom walks its pattern table with a map range, so two elements declared on
// the same line come back in a different order on different runs. Without this
// the projection of one unchanged observation would differ between runs, which
// breaks both test assertions and any caching keyed on the result.
func sortedElements(elements []codedom.CodeElement) []codedom.CodeElement {
	out := append([]codedom.CodeElement(nil), elements...)
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.StartLine != b.StartLine {
			return a.StartLine < b.StartLine
		}
		if a.EndLine != b.EndLine {
			return a.EndLine < b.EndLine
		}
		if a.Name != b.Name {
			return a.Name < b.Name
		}
		return a.Type < b.Type
	})
	return out
}

// innermost returns the tightest element containing a line.
//
// Tightest rather than first: a match inside a method of a class is inside both
// extents, and attributing it to the class would lose exactly the resolution
// this codec exists to provide. It also limits the damage from an unbalanced
// brace, which makes codedom report an extent running to end of file.
func innermost(elements []codedom.CodeElement, line int) (codedom.CodeElement, bool) {
	var best codedom.CodeElement
	found := false
	for _, e := range elements {
		if line < e.StartLine || line > e.EndLine {
			continue
		}
		if !found || (e.EndLine-e.StartLine) < (best.EndLine-best.StartLine) {
			best = e
			found = true
		}
	}
	return best, found
}

var (
	goImportRe   = regexp.MustCompile(`"([^"]+)"`)
	pyImportRe   = regexp.MustCompile(`^\s*(?:from|import)\s+([A-Za-z_][\w.]*)`)
	jsImportRe   = regexp.MustCompile(`(?:from|require\(|import)\s*['"]([^'"]+)['"]`)
	rustImportRe = regexp.MustCompile(`^\s*(?:pub\s+)?use\s+([A-Za-z_][\w:]*)`)
	javaImportRe = regexp.MustCompile(`^\s*import\s+(?:static\s+)?([A-Za-z_][\w.]*)`)
)

// goSingleImport is the one-line form; the block form is tracked by scanning.
const goSingleImport = "import "

// importTargets maps matched line numbers to the dependency that line names.
//
// An import line is the one match that is a dependency edge on its own, without
// any enclosing symbol — and it is also the match most likely to be misread as
// a reference, because the imported path usually contains the searched term. Go
// needs the file scanned rather than the line inspected: inside an `import (`
// block the line is a bare quoted path with nothing on it that says "import".
func importTargets(file, content string, matches []Match) map[int]string {
	wanted := make(map[int]struct{}, len(matches))
	for _, m := range matches {
		wanted[m.Line] = struct{}{}
	}

	out := make(map[int]string)
	lang := languageOf(file)
	if lang == "" {
		return out
	}

	inGoBlock := false
	for i, line := range strings.Split(content, "\n") {
		lineNo := i + 1
		trimmed := strings.TrimSpace(strings.TrimSuffix(line, "\r"))

		if lang == "go" {
			switch {
			case strings.HasPrefix(trimmed, "import ("):
				inGoBlock = true
				continue
			case inGoBlock && trimmed == ")":
				inGoBlock = false
				continue
			}
		}

		if _, ok := wanted[lineNo]; !ok {
			continue
		}

		var target string
		switch lang {
		case "go":
			if inGoBlock || strings.HasPrefix(trimmed, goSingleImport) {
				if m := goImportRe.FindStringSubmatch(trimmed); m != nil {
					target = m[1]
				}
			}
		case "python":
			if m := pyImportRe.FindStringSubmatch(trimmed); m != nil {
				target = m[1]
			}
		case "js":
			if strings.HasPrefix(trimmed, "import") || strings.Contains(trimmed, "require(") {
				if m := jsImportRe.FindStringSubmatch(trimmed); m != nil {
					target = m[1]
				}
			}
		case "rust":
			if m := rustImportRe.FindStringSubmatch(trimmed); m != nil {
				target = m[1]
			}
		case "jvm":
			if m := javaImportRe.FindStringSubmatch(trimmed); m != nil {
				target = m[1]
			}
		}
		if target != "" {
			out[lineNo] = target
		}
	}
	return out
}

func languageOf(file string) string {
	dot := strings.LastIndex(file, ".")
	if dot < 0 {
		return ""
	}
	switch strings.ToLower(file[dot+1:]) {
	case "go":
		return "go"
	case "py":
		return "python"
	case "js", "jsx", "ts", "tsx", "mjs", "cjs":
		return "js"
	case "rs":
		return "rust"
	case "java", "kt", "scala":
		return "jvm"
	default:
		return ""
	}
}

// capSymbols orders symbols by what a reader needs first — declarations, then
// the heaviest users — and reports how many were dropped.
func capSymbols(symbols map[string]*Symbol, order []string, limit int) ([]Symbol, int) {
	out := make([]Symbol, 0, len(order))
	for _, ref := range order {
		out = append(out, *symbols[ref])
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Declared != b.Declared {
			return a.Declared
		}
		if a.Hits != b.Hits {
			return a.Hits > b.Hits
		}
		return a.Ref < b.Ref
	})
	if len(out) <= limit {
		return out, 0
	}
	return out[:limit], len(out) - limit
}

func capEdges(edges []Edge, limit int) ([]Edge, int) {
	sort.SliceStable(edges, func(i, j int) bool {
		a, b := edges[i], edges[j]
		if a.Kind != b.Kind {
			// References before imports: a call graph answers the question that
			// prompted the search more often than an import list does.
			return a.Kind == EdgeReferences
		}
		// Target before weight, so edges sharing a target stay adjacent and can
		// be rendered under it once instead of repeating it on every row.
		if a.To != b.To {
			return a.To < b.To
		}
		if a.Count != b.Count {
			return a.Count > b.Count
		}
		return a.From < b.From
	})
	if len(edges) <= limit {
		return edges, 0
	}
	return edges[:limit], len(edges) - limit
}

func capUnscoped(hits map[string]int, order []string, limit int) ([]FileHits, int) {
	out := make([]FileHits, 0, len(hits))
	for _, file := range order {
		if n, ok := hits[file]; ok {
			out = append(out, FileHits{File: file, Hits: n})
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Hits != out[j].Hits {
			return out[i].Hits > out[j].Hits
		}
		return out[i].File < out[j].File
	})
	if len(out) <= limit {
		return out, 0
	}
	return out[:limit], len(out) - limit
}

// Text renders the projection for a model or an operator.
//
// Text rather than JSON, unlike the MCP control plane next door, and the reason
// is the one this codec exists for: this result is a table, and JSON pays for
// every column name once per row. Naming the columns once in a header is the
// difference between a projection that is cheaper than the wall of lines it
// replaced and one that is merely differently shaped.
func (r CodeSearchResult) Text(expandVerb string) string {
	var sb strings.Builder

	fmt.Fprintf(&sb, "code search %q: %d match(es) in %d file(s)", r.Query, r.Matches, r.Files)
	if r.Truncated {
		sb.WriteString(" (search stopped at its result cap)")
	}
	sb.WriteString("\n")

	if r.Matches == 0 {
		return sb.String()
	}

	// A reference edge's source is always a symbol that is already listed above,
	// and every reference edge from one search shares a target by construction.
	// Rendering each edge with both endpoints therefore printed every
	// referencing symbol twice and repeated the target on each row — which made
	// the projection LARGER than the wall of lines it replaced on exactly the
	// searches it should help most: many symbols, one hit each. Measured on
	// this repo before the fix, such a search cost 3420 bytes against grep's
	// 2019. The relation is stated once instead, and no edge is lost: a listed
	// symbol either declares the searched thing or references the target.
	refCount := make(map[string]int, len(r.Edges))
	refTarget := ""
	var imports []Edge
	for _, e := range r.Edges {
		if e.Kind == EdgeImports {
			imports = append(imports, e)
			continue
		}
		refCount[e.From] = e.Count
		if refTarget == "" {
			refTarget = e.To
		}
	}

	if len(r.Symbols) > 0 {
		sb.WriteString("symbols (kind ref lines hits):\n")
		for _, s := range r.Symbols {
			fmt.Fprintf(&sb, "  %s %s %d-%d x%d", s.Kind, s.Ref, s.StartLine, s.EndLine, s.Hits)
			if s.Declared {
				sb.WriteString(" declared-here")
				// A symbol that both declares and uses the searched thing —
				// recursion, or a type naming itself — would otherwise have its
				// hit count read as all declaration.
				if n := refCount[s.Ref]; n > 0 {
					fmt.Fprintf(&sb, " (%d ref)", n)
				}
			}
			sb.WriteString("\n")
		}
		if r.SymbolsOmitted > 0 {
			fmt.Fprintf(&sb, "  ... %d more symbol(s) not shown\n", r.SymbolsOmitted)
		}
	}

	if refTarget != "" {
		fmt.Fprintf(&sb, "references point to %s\n", refTarget)
	}

	if len(imports) > 0 {
		sb.WriteString("imports (file -> dependency):\n")
		for _, e := range imports {
			fmt.Fprintf(&sb, "  %s -> %s\n", e.From, e.To)
		}
	}

	if r.EdgesOmitted > 0 {
		// Unindented and named: with references folded into the symbol table
		// this line belongs to no list above it, and an indented note under the
		// imports would read as "more imports".
		fmt.Fprintf(&sb, "... %d more dependency edge(s) not shown\n", r.EdgesOmitted)
	}

	if len(r.Unscoped) > 0 {
		sb.WriteString("hits outside any symbol (file count):\n")
		for _, f := range r.Unscoped {
			fmt.Fprintf(&sb, "  %s x%d\n", f.File, f.Hits)
		}
		if r.UnscopedOmitted > 0 {
			fmt.Fprintf(&sb, "  ... %d more file(s) not shown\n", r.UnscopedOmitted)
		}
	}

	if r.Handle != "" {
		// The handle is useless unless the reader is told the verb that
		// redeems it, and saying the expansion never re-runs the search is what
		// makes it safe to rely on: the lines it returns are the lines this
		// projection was computed from.
		fmt.Fprintf(&sb, "raw matching lines retained as %s", r.Handle)
		if expandVerb != "" {
			fmt.Fprintf(&sb, " — %s handle=%s reads them without re-running the search", expandVerb, r.Handle)
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

// Text renders a hydration as the lines the search would have printed.
func (h Hydrated) Text() string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "raw matches for %q from %s: %d-%d of %d\n",
		h.Query, h.Handle, h.windowStart(), h.Offset+len(h.Match), h.Total)
	for _, m := range h.Match {
		fmt.Fprintf(&sb, "%s:%d: %s\n", m.File, m.Line, m.Text)
	}
	if h.NextOffset > 0 {
		fmt.Fprintf(&sb, "... %d more retained match(es); continue with offset=%d\n",
			h.Total-h.NextOffset, h.NextOffset)
	} else if h.Truncated {
		// End of the retained matches is not end of the matches when the search
		// itself stopped early, and an agent that concludes "that is all of
		// them" from a capped search has drawn a false negative.
		sb.WriteString("end of retained matches; the search itself stopped at its result cap, so more may exist\n")
	}
	return sb.String()
}

func (h Hydrated) windowStart() int {
	if len(h.Match) == 0 {
		return h.Offset
	}
	return h.Offset + 1
}
