// Package codemodel is the one element model of parsed source that every
// model-facing CodeDOM surface reads: the structure index, the element tools
// (get_element, get_elements and the element edit verbs), the read_file and
// search_code codecs, and the holographic outline.
//
// Why it is one package. Until 2026-09-22 the model-facing element list came
// from four Go regexes with no body, no doc comment and no const or var, while
// a real go/ast model with bodies fed only the dormant Path-B handlers and the
// kernel's code_element facts. The model was told where a declaration was and
// then had to read the file as text to see it (8 get_elements -> read_file
// pairs on the same file in one run). One model, reached by everything, is what
// lets the structure be the view of code rather than a hint about where to read.
//
// It is a leaf: the standard library and internal/mangle's parser lock, nothing
// from the world or the tools, so internal/world and internal/tools/codedom can
// both import it without the cycle that kept them apart.
package codemodel

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

// Kind is what an element is.
type Kind string

const (
	// KindHeader is a Go file's package clause, file doc, build constraints
	// and imports: everything above the first declaration. It is addressable so
	// file-level regions are edited through the same verbs as declarations.
	KindHeader    Kind = "header"
	KindFunction  Kind = "function"
	KindMethod    Kind = "method"
	KindStruct    Kind = "struct"
	KindInterface Kind = "interface"
	KindType      Kind = "type"
	KindConst     Kind = "const"
	KindVar       Kind = "var"

	KindDecl  Kind = "decl"
	KindRule  Kind = "rule"
	KindFact  Kind = "fact"
	KindQuery Kind = "query"

	// KindSyntaxError is a region of a file that does not parse. A broken
	// file keeps its good declarations addressable and exposes the broken
	// region as an element, so it can be repaired structurally instead of
	// vanishing from every tool that could repair it.
	KindSyntaxError Kind = "syntax_error"
)

// Language names.
const (
	LangGo     = "go"
	LangMangle = "mangle"
)

// HeaderKey and EndKey are the pseudo-element addresses of a file's top and
// bottom. EndKey is an insertion anchor only; it names no text.
const (
	HeaderKey = "header"
	EndKey    = "end"
)

// Element is one addressable unit of a parsed file.
type Element struct {
	// Key is the element's address within its file: "Refresh",
	// "StructureIndex.Refresh", "header", "init#2", "rule:pred/2@3fa2c1".
	Key      string
	Name     string   // the declared identifier (predicate for Mangle)
	Receiver string   // method receiver base type
	Names    []string // every name a multi-name spec declares
	Kind     Kind

	// Start and End are byte offsets [Start, End) into File.Source: the doc
	// comment through the end of the declaration, trailing line comment
	// included. Replacing this span replaces the doc comment too.
	Start, End         int
	StartLine, EndLine int
	// DeclLine is the line of the keyword or name, after the doc comment.
	DeclLine int

	Signature string
	Doc       string // first line of the doc comment
	Exported  bool

	// Grouped marks a spec inside a parenthesized const/var/type group. The
	// group, not the spec, is the unit gofmt aligns.
	Grouped bool
	// UnitStart and UnitEnd span the top-level declaration the element
	// belongs to: the element itself, or its whole group. It is the unit an
	// edit reformats, so an edit never reformats a declaration it did not
	// touch.
	UnitStart, UnitEnd int

	// Revision is the hash of the element's own bytes: an edit elsewhere in
	// the file leaves it unchanged, so it can serve as an edit precondition
	// without going stale on every write to the file.
	Revision string

	// Err is the parser's message for a syntax_error element.
	Err string
}

// Import is one import spec of a Go file.
type Import struct {
	Name       string // explicit local name, "" when none
	Path       string
	Start, End int
	Line       int
}

// LocalName is the identifier the file refers to the import by. It is a
// guess from the path when the spec names none: the package clause of an
// external module is not visible here.
func (imp Import) LocalName() string {
	if imp.Name != "" {
		return imp.Name
	}
	return GuessPackageName(imp.Path)
}

// File is a parsed source file.
type File struct {
	Path     string
	Language string
	Package  string
	// Source is the file's text with line endings normalised to LF. Every
	// offset in the model is into this string.
	Source   string
	Elements []Element
	Imports  []Import
	// BuildConstraint is the //go:build expression, "" when none.
	BuildConstraint string
	// Err is the first parse error; Errors are all of them. A file with
	// errors still lists its well-formed declarations and one syntax_error
	// element per broken region.
	Err    error
	Errors []SyntaxError
	// Parsed reports whether the language parser accepted the whole file.
	Parsed bool

	lineStarts []int
}

// SyntaxError is one positioned parse error.
type SyntaxError struct {
	Line, Column int
	Msg          string
}

// LanguageOf returns the language CodeDOM models path in, or "" for a file it
// has no element model for.
func LanguageOf(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".go":
		return LangGo
	case ".mg", ".dl", ".mangle":
		return LangMangle
	}
	return ""
}

// SkipDir reports the directories the workspace walk never enters: vendored
// and generated trees, test fixtures, and hidden or underscore directories.
func SkipDir(name string) bool {
	switch name {
	case "vendor", "node_modules", "testdata":
		return true
	}
	return strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_")
}

// Normalize returns src with CRLF and lone CR line endings rewritten to LF.
// The model works on LF text; the edit verbs restore the file's own ending on
// write.
func Normalize(src string) string {
	if !strings.Contains(src, "\r") {
		return src
	}
	src = strings.ReplaceAll(src, "\r\n", "\n")
	return strings.ReplaceAll(src, "\r", "\n")
}

// Parse builds the model of path from src. ok is false for a language
// CodeDOM has no model for.
func Parse(path, src string) (f *File, ok bool) {
	switch LanguageOf(path) {
	case LangGo:
		return ParseGo(path, src), true
	case LangMangle:
		return ParseMangle(path, src), true
	}
	return nil, false
}

// Revision hashes an element's text. The text is LF-normalised first so a
// file's line-ending convention does not change its elements' identities.
func Revision(text string) string {
	sum := sha256.Sum256([]byte(Normalize(text)))
	return hex.EncodeToString(sum[:])[:12]
}

// Text returns an element's own bytes.
func (f *File) Text(e *Element) string {
	if e == nil || e.Start < 0 || e.End > len(f.Source) || e.Start > e.End {
		return ""
	}
	return f.Source[e.Start:e.End]
}

// LineText returns whole lines lo..hi (1-based, inclusive) of the source.
func (f *File) LineText(lo, hi int) string {
	f.ensureLines()
	if lo < 1 {
		lo = 1
	}
	if hi > len(f.lineStarts) {
		hi = len(f.lineStarts)
	}
	if lo > hi {
		return ""
	}
	start := f.lineStarts[lo-1]
	end := len(f.Source)
	if hi < len(f.lineStarts) {
		end = f.lineStarts[hi] - 1
	}
	return strings.TrimSuffix(f.Source[start:end], "\n")
}

// NumberedLines renders lines lo..hi with right-aligned line numbers.
func (f *File) NumberedLines(lo, hi int) string {
	text := f.LineText(lo, hi)
	if text == "" && lo > hi {
		return ""
	}
	width := len(fmt.Sprint(hi))
	var sb strings.Builder
	for i, line := range strings.Split(text, "\n") {
		fmt.Fprintf(&sb, "%*d  %s\n", width, lo+i, line)
	}
	return sb.String()
}

// LineCount is the number of lines in the file.
func (f *File) LineCount() int {
	f.ensureLines()
	return len(f.lineStarts)
}

// LineOf returns the 1-based line holding byte offset off.
func (f *File) LineOf(off int) int {
	f.ensureLines()
	return sort.Search(len(f.lineStarts), func(i int) bool { return f.lineStarts[i] > off })
}

// LineStart returns the offset of the first byte of line n (1-based).
func (f *File) LineStart(n int) int {
	f.ensureLines()
	if n < 1 {
		return 0
	}
	if n > len(f.lineStarts) {
		return len(f.Source)
	}
	return f.lineStarts[n-1]
}

func (f *File) ensureLines() {
	if f.lineStarts != nil {
		return
	}
	f.lineStarts = []int{0}
	for i := 0; i < len(f.Source); i++ {
		if f.Source[i] == '\n' && i+1 < len(f.Source) {
			f.lineStarts = append(f.lineStarts, i+1)
		}
	}
}

// Element returns the element with exactly this key, or nil.
func (f *File) Element(key string) *Element {
	for i := range f.Elements {
		if f.Elements[i].Key == key {
			return &f.Elements[i]
		}
	}
	return nil
}

// Header returns the file's header element, or nil (Mangle files have none).
func (f *File) Header() *Element {
	return f.Element(HeaderKey)
}

// Lookup returns the elements a query names within this file. A query is the
// element's key ("StructureIndex.Refresh", "init#2", "rule:pred/2@3fa2c1"),
// its bare name ("Refresh"), a receiver spelled as Go writes it
// ("(*StructureIndex).Refresh"), any name of a multi-name spec, or for Mangle
// "pred/2" or "pred". An exact key match wins outright, so every key the
// model is shown fetches exactly one element.
func (f *File) Lookup(query string) []*Element {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil
	}
	if e := f.Element(query); e != nil {
		return []*Element{e}
	}
	recv, name, qualified := splitReceiverQuery(query)
	var out []*Element
	for i := range f.Elements {
		e := &f.Elements[i]
		switch {
		case qualified && e.Receiver != "" && e.Receiver == recv && e.Name == name:
			out = append(out, e)
		case !qualified && e.Name == query:
			out = append(out, e)
		case !qualified && e.Kind != KindMethod && containsString(e.Names, query):
			out = append(out, e)
		case f.Language == LangMangle && (e.Name+"/"+manglArityOf(e.Key) == query):
			out = append(out, e)
		}
	}
	// A bare name that is both a function and a method name prefers the
	// function, so a function stays fetchable when a method shares its name.
	if !qualified && len(out) > 1 {
		var plain []*Element
		for _, e := range out {
			if e.Receiver == "" {
				plain = append(plain, e)
			}
		}
		if len(plain) == 1 {
			return plain
		}
	}
	return out
}

// splitReceiverQuery reads "T.M", "*T.M", "(*T).M" and "pkg.T.M" as a
// receiver-qualified method name.
func splitReceiverQuery(q string) (recv, name string, ok bool) {
	dot := strings.LastIndex(q, ".")
	if dot <= 0 || dot == len(q)-1 {
		return "", q, false
	}
	name = q[dot+1:]
	recv = q[:dot]
	if i := strings.LastIndex(recv, "."); i >= 0 && !strings.Contains(recv[i:], ")") {
		recv = recv[i+1:]
	}
	recv = strings.Trim(recv, "()*")
	if i := strings.Index(recv, "["); i >= 0 {
		recv = recv[:i]
	}
	if recv == "" {
		return "", q, false
	}
	return recv, name, true
}

func manglArityOf(key string) string {
	// key: kind:pred/arity[@hash][#n]
	_, rest, ok := strings.Cut(key, "/")
	if !ok {
		return ""
	}
	if i := strings.IndexAny(rest, "@#"); i >= 0 {
		rest = rest[:i]
	}
	return rest
}

func containsString(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// GuessPackageName is the package name an import path conventionally
// declares: its last element, without a major-version suffix or a "go-"
// prefix. It is a guess; callers that need certainty must not act on a
// mismatch.
func GuessPackageName(path string) string {
	elems := strings.Split(path, "/")
	name := elems[len(elems)-1]
	if len(elems) > 1 && isMajorVersion(name) {
		name = elems[len(elems)-2]
	}
	if i := strings.Index(name, ".v"); i > 0 && isMajorVersion(name[i+1:]) {
		name = name[:i]
	}
	name = strings.TrimPrefix(name, "go-")
	name = strings.TrimSuffix(name, "-go")
	name = strings.ReplaceAll(name, "-", "")
	name = strings.ReplaceAll(name, ".", "")
	return name
}

func isMajorVersion(s string) bool {
	if len(s) < 2 || s[0] != 'v' {
		return false
	}
	for _, c := range s[1:] {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// assignKeys gives every element a key unique within the file: its base key,
// or base#N for the Nth duplicate (several init functions, blank vars).
func assignKeys(elements []Element, base func(*Element) string) {
	seen := make(map[string]int, len(elements))
	for i := range elements {
		k := base(&elements[i])
		seen[k]++
		if n := seen[k]; n > 1 {
			k = fmt.Sprintf("%s#%d", k, n)
		}
		elements[i].Key = k
	}
}
