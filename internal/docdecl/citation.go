package docdecl

import (
	"math"
	"regexp"
	"strings"
)

// The citation lexer: how a line of documentation names code. Two graders
// share it so they cannot disagree about what a citation is: the citation
// auditor (cmd/tools/audit_doc_citations) checks each citation against the
// tree, and the spec pairing layer (internal/specs) turns each into a link
// from a document section to the code element it names.
//
// A citation is a workspace path (`internal/pkg/a.go`, optionally followed by
// `:12-20` lines or `#Symbol`), a backticked symbol, or a backticked symbol
// bound to a path by adjacency. The lexer only reads text; it never touches
// the filesystem, so what a path or symbol resolves to is the caller's rule.

// AdjacencyWindow is how close a backticked symbol must sit to a path before
// it is a citation of that path. It is the grammar of "next to" -- a
// parenthetical `Sym` (`path`) or a short `path defines Sym` -- not a cap on
// output or runtime. A sentence between them is prose.
const AdjacencyWindow = 240

// PathHit is one path citation in a line.
type PathHit struct {
	// Start and End span the whole citation, suffix included.
	Start, End int
	// Rel is the workspace-relative path alone.
	Rel string
	// Raw is the citation as written, suffix included.
	Raw string
	// Lines are the `:12-20` spans that followed the path.
	Lines [][2]int
	// Hash is the `#Symbol` selector.
	Hash string
}

// SymHit is one backticked symbol in a line.
type SymHit struct {
	Start, End int
	Sym        string
}

// Lexer finds the paths a line cites. Its path grammar is rooted: a citation
// starts at one of the roots, which keeps `https://host/internal/x.go` and
// `foo/internal/x.go` from reading as workspace paths.
type Lexer struct {
	pathRE *regexp.Regexp
}

// NewLexer builds a lexer for paths that begin with one of roots (top-level
// directories) and end in one of exts (without the dot).
func NewLexer(roots, exts []string) *Lexer {
	quote := func(in []string) string {
		out := make([]string, len(in))
		for i, s := range in {
			out[i] = regexp.QuoteMeta(s)
		}
		return strings.Join(out, "|")
	}
	return &Lexer{pathRE: regexp.MustCompile(`(?:` + quote(roots) + `)/[A-Za-z0-9_./-]+\.(?:` + quote(exts) + `)`)}
}

var (
	tickRE       = regexp.MustCompile("`([^`\n]+)`")
	tickSpan     = regexp.MustCompile("`[^`]*`")
	symbolRE     = regexp.MustCompile(`^(\(\*?[A-Za-z_][A-Za-z0-9_]*\)\.)?[A-Za-z_][A-Za-z0-9_]*(?:\.[A-Za-z_][A-Za-z0-9_]*)*(?:/[0-9]+)?$`)
	predicateRE  = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*/[0-9]+$`)
	abbrevFileRE = regexp.MustCompile(`(?:^|[^A-Za-z0-9_/])[A-Za-z0-9_.-]+\.(?:go|mg)\b`)
	gapWordRE    = regexp.MustCompile(`[A-Za-z]+`)
)

// connectorWords are the words that may sit between a path and the symbol it
// cites (`defines`, `at`, `see`, ...) without the backtick becoming a
// sentence. A word that is not in this set -- `documented`, `vision` as prose
// around a path -- means the backtick is not a citation of that path.
var connectorWords = newConnectorWords()

func newConnectorWords() map[string]bool {
	m := map[string]bool{}
	for _, w := range strings.Fields("defines defined define declares declared declare declaration at in of the a an and or via on called see from is by its this that to for with") {
		m[w] = true
	}
	return m
}

// Paths returns the path citations in line, in order.
func (l *Lexer) Paths(line string) []PathHit {
	var out []PathHit
	for _, ix := range l.pathRE.FindAllStringIndex(line, -1) {
		if !AcceptsPath(line, ix[0]) || !validRel(line[ix[0]:ix[1]]) {
			continue
		}
		rel := line[ix[0]:ix[1]]
		lines, hash, n := ParseSuffix(line[ix[1]:])
		end := ix[1] + n
		out = append(out, PathHit{
			Start: ix[0], End: end, Rel: rel, Raw: line[ix[0]:end],
			Lines: lines, Hash: hash,
		})
	}
	return out
}

// validRel rejects `..` and empty segments. Joining `internal/../../outside.go`
// would read outside the repository; that string is not a repo-relative citation.
func validRel(rel string) bool {
	if strings.Contains(rel, "\\") || strings.Contains(rel, "//") {
		return false
	}
	for _, seg := range strings.Split(rel, "/") {
		if seg == "" || seg == "." || seg == ".." {
			return false
		}
	}
	return true
}

// AcceptsPath reports whether a regex hit is a path rooted at a workspace
// root. `./internal/...` and a leading slash are rooted. `foo/internal/...`
// and `https://host/internal/...` are the tail of a longer path, which does
// not exist at the repo root.
func AcceptsPath(line string, start int) bool {
	if start == 0 {
		return true
	}
	prev := line[start-1]
	if prev != '/' {
		return !isIdentByte(prev)
	}
	if start == 1 {
		return true
	}
	if line[start-2] != '.' {
		return !isIdentByte(line[start-2]) && line[start-2] != '.' && line[start-2] != '/'
	}
	if start == 2 {
		return true
	}
	b := line[start-3]
	return !isIdentByte(b) && b != '.' && b != '/'
}

// Symbols returns the backticked symbols in line, in order.
func Symbols(line string) []SymHit {
	var out []SymHit
	for _, m := range tickRE.FindAllStringSubmatchIndex(line, -1) {
		sym, ok := NormalizeSymbol(line[m[2]:m[3]])
		if !ok {
			continue
		}
		out = append(out, SymHit{Start: m[0], End: m[1], Sym: sym})
	}
	return out
}

// NormalizeSymbol reads the text of a backtick as a symbol: the declaration
// keyword and the call parentheses are dropped, and anything that is not an
// identifier path (a filename, a sentence) is refused.
func NormalizeSymbol(inner string) (string, bool) {
	s := strings.TrimSpace(inner)
	if s == "" {
		return "", false
	}
	for _, p := range []string{"var ", "func ", "type ", "const "} {
		if strings.HasPrefix(s, p) {
			s = strings.TrimSpace(strings.TrimPrefix(s, p))
			break
		}
	}
	if strings.HasSuffix(s, ")") {
		if i := strings.LastIndex(s, "("); i > 0 {
			s = strings.TrimSpace(s[:i])
		}
	}
	if s == "" || !symbolRE.MatchString(s) || FileShapedSymbol(s) {
		return "", false
	}
	return s, true
}

// fileExt is the lowercase tail of a backticked filename. Measured against
// Docs/architecture: `nerd.md`, `feedback.go`, `usage.json`, `types.go`,
// `config.yaml`, `nerd.exe` were graded as pkg.Func because the symbol
// grammar allows a dot. An exported selector is not all-lowercase
// (`sync.RWMutex`, `pkg.Go`), so this set does not swallow declarations.
var fileExt = map[string]bool{
	"md": true, "json": true, "go": true, "mg": true, "txt": true,
	"yml": true, "yaml": true, "toml": true, "xml": true, "html": true,
	"css": true, "png": true, "jpg": true, "jpeg": true, "svg": true,
	"gif": true, "sql": true, "sh": true, "ps1": true, "mod": true,
	"sum": true, "exe": true, "lock": true, "csv": true, "tsv": true,
	"proto": true,
}

// FileShapedSymbol reports a backtick that is a filename, not a declaration.
func FileShapedSymbol(sym string) bool {
	i := strings.LastIndex(sym, ".")
	if i <= 0 || i == len(sym)-1 {
		return false
	}
	ext := sym[i+1:]
	if ext != strings.ToLower(ext) {
		return false
	}
	return fileExt[ext]
}

// PredicateShaped reports a symbol written as a Mangle predicate with its
// arity, `impact_caller/2`.
func PredicateShaped(sym string) bool { return predicateRE.MatchString(sym) }

// Bind attaches each symbol to the nearest path it can be a citation of, by
// index into paths. Symbols that bind to no path are returned apart: they are
// global citations, resolved by name alone.
func Bind(line string, paths []PathHit, syms []SymHit) (map[int][]SymHit, []SymHit) {
	bound := map[int][]SymHit{}
	var unbound []SymHit
	for _, sym := range syms {
		best := -1
		bestDist := int(^uint(0) >> 1)
		bestAfter := false
		for i, p := range paths {
			var between string
			dist := 0
			after := false
			switch {
			case sym.End <= p.Start:
				between = line[sym.End:p.Start]
				dist = p.Start - sym.End
				after = true
			case p.End <= sym.Start:
				between = line[p.End:sym.Start]
				dist = sym.Start - p.End
			default:
				continue
			}
			if !GapBinds(between) {
				continue
			}
			if gapHasWord(between) && !CodeShaped(sym.Sym) {
				continue
			}
			if dist < bestDist || (dist == bestDist && after && !bestAfter) {
				best = i
				bestDist = dist
				bestAfter = after
			}
		}
		if best < 0 {
			unbound = append(unbound, sym)
			continue
		}
		bound[best] = append(bound[best], sym)
	}
	return bound, unbound
}

// GapBinds reports whether the text between a symbol and a path is the
// grammar of a citation (a parenthesis, `defines`) rather than a sentence.
func GapBinds(between string) bool {
	if len(between) > AdjacencyWindow {
		return false
	}
	if abbrevFileRE.MatchString(between) {
		return false
	}
	words := gapWords(between)
	if len(words) > 4 {
		return false
	}
	for _, w := range words {
		if !connectorWords[strings.ToLower(w)] {
			return false
		}
	}
	return true
}

func gapHasWord(between string) bool {
	return len(gapWords(between)) > 0
}

func gapWords(between string) []string {
	prose := tickSpan.ReplaceAllString(between, " ")
	return gapWordRE.FindAllString(prose, -1)
}

// CodeShaped is the bar for a symbol separated from its path by words
// (`path defines Sym`). An all-lowercase backtick in that spot is prose
// (`the vision`). A parenthetical citation has no words between the ticks
// and the path, so a short name like `mu` still counts there.
func CodeShaped(sym string) bool {
	if strings.ContainsAny(sym, "._/") || strings.Contains(sym, "(*") {
		return true
	}
	for i := 0; i < len(sym); i++ {
		if sym[i] >= 'A' && sym[i] <= 'Z' {
			return true
		}
	}
	return false
}

// ParseSuffix reads what follows a path: `#Symbol`, `:12-20, 30` lines, or
// both in either order. n is how many bytes of s it consumed.
func ParseSuffix(s string) (lines [][2]int, hash string, n int) {
	i := 0
	if strings.HasPrefix(s, "#") {
		if sel, k := readSelector(s[1:]); k > 0 {
			hash = sel
			i = 1 + k
		}
	}
	if i < len(s) && s[i] == ':' {
		if ls, k, ok := readLineList(s[i+1:]); ok {
			lines = ls
			i += 1 + k
		}
	}
	if hash == "" && i < len(s) && s[i] == '#' {
		if sel, k := readSelector(s[i+1:]); k > 0 {
			hash = sel
			i += 1 + k
		}
	}
	return lines, hash, i
}

func readSelector(s string) (string, int) {
	if s == "" || !isIdentStart(s[0]) {
		return "", 0
	}
	i := 1
	for i < len(s) && isIdentCont(s[i]) {
		i++
	}
	for i < len(s) && s[i] == '.' {
		j := i + 1
		if j >= len(s) || !isIdentStart(s[j]) {
			break
		}
		j++
		for j < len(s) && isIdentCont(s[j]) {
			j++
		}
		i = j
	}
	return s[:i], i
}

func readLineList(s string) ([][2]int, int, bool) {
	if s == "" || !isDigit(s[0]) {
		return nil, 0, false
	}
	var spans [][2]int
	i := 0
	for {
		start, k := readInt(s[i:])
		if k == 0 {
			break
		}
		i += k
		end := start
		if i < len(s) && s[i] == '-' && i+1 < len(s) && isDigit(s[i+1]) {
			n2, k2 := readInt(s[i+1:])
			i += 1 + k2
			end = n2
		}
		spans = append(spans, [2]int{start, end})
		j := i
		for j < len(s) && (s[j] == ' ' || s[j] == '\t') {
			j++
		}
		if j >= len(s) || s[j] != ',' {
			break
		}
		j++
		for j < len(s) && (s[j] == ' ' || s[j] == '\t') {
			j++
		}
		if j >= len(s) || !isDigit(s[j]) {
			break
		}
		i = j
	}
	if len(spans) == 0 {
		return nil, 0, false
	}
	return spans, i, true
}

func readInt(s string) (int, int) {
	if s == "" || !isDigit(s[0]) {
		return 0, 0
	}
	i := 0
	n := 0
	for i < len(s) && isDigit(s[i]) {
		d := int(s[i] - '0')
		if n > (math.MaxInt-d)/10 {
			for i < len(s) && isDigit(s[i]) {
				i++
			}
			return math.MaxInt, i
		}
		n = n*10 + d
		i++
	}
	return n, i
}

func isDigit(b byte) bool { return b >= '0' && b <= '9' }

func isIdentByte(b byte) bool { return isIdentCont(b) }
