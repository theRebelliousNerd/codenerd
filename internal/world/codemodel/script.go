package codemodel

import (
	"context"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/javascript"
	"github.com/smacker/go-tree-sitter/python"
	tsx "github.com/smacker/go-tree-sitter/typescript/tsx"
	typescript "github.com/smacker/go-tree-sitter/typescript/typescript"
)

// Script languages share one binding model. Python's locality is
// function-wide and its class body is not a scope a method can see, so a
// use is not resolved where it is encountered: the walk records every bind
// and every load, and resolution runs once the walk is finished.

type scopeKind int

const (
	scopeModule scopeKind = iota
	scopeClass
	scopeFunction
	scopeArrow
	scopeLambda
	scopeComp
	scopeBlock
	scopeNamespace
	scopeCatch
)

type scope struct {
	kind      scopeKind
	parent    *scope
	binds     map[string]int
	globals   map[string]bool
	nonlocals map[string]bool
}

type bindKind int

const (
	bindDecl bindKind = iota
	bindMethod
	bindImport
	bindLocal
	bindParam
	bindRemoved
)

// nameBind is one declared name. id is assigned after imports resolve,
// because a named import shares its id with the export it names.
type nameBind struct {
	name       string
	id         string
	kind       bindKind
	start, end int
	importFile string
	importName string // remote name, "*" for a namespace import, "default" for a default import
	alias      bool
	elem       int // index into elements, or -1
}

// refUse is one identifier a script file refers to a binding by.
type refUse struct {
	Name         string
	Start, End   int
	Line, Column int
	Bind         string
	Call         bool
	JSX          bool
	Qual         string
}

type nameLoad struct {
	name         string
	start, end   int
	line, column int
	sc           *scope
	call         bool
	jsx          bool
	closeTag     bool // JSX closing tag: a rename site, not a second call
	prop         bool
	qual         string
	objStart     int
	objEnd       int
}

type scrape struct {
	src  string
	b    []byte
	path string // path stored on the file
	abs  string // filesystem path the source lives at
	root string // workspace root; resolved imports are relative to it
	lang string

	module *scope
	cur    *scope
	scopes []*scope

	binds    []nameBind
	loads    []nameLoad
	elements []Element
	imports  []Import
	idents   map[string]int
	members  map[string]int // receiver+"\x00"+name -> bind index

	classRecv string
	nsRecv    string
	inClass   int

	exporting     bool
	exportDefault bool

	hasAll bool
	all    map[string]bool

	errSpans [][2]int
	errTop   []bool
	errors   []SyntaxError

	pending   []pendingCall
	useBuf    []refUse
	aliasUses []aliasUse
}

// aliasUse is the remote name of an aliased import (`import { useAuth as ua }`,
// `from m import use_auth as ua`). It is a use of the export, not of the local.
type aliasUse struct {
	name         string
	start, end   int
	line, column int
	file         string
}

// ParseScript parses a Python, TypeScript, TSX or JavaScript file. Imports
// are resolved as if the source lived at path.
func ParseScript(path, src string) *File {
	return parseScriptAt(path, path, "", src)
}

func parseScriptAt(path, absPath, root, src string) *File {
	src = Normalize(src)
	lang := LanguageOf(path)
	f := &File{
		Path:     path,
		Language: lang,
		Package:  scriptModuleName(path),
		Source:   src,
		Idents:   map[string]int{},
	}
	parser := sitter.NewParser()
	defer parser.Close()
	parser.SetLanguage(scriptGrammar(path))
	tree, err := parser.ParseCtx(context.Background(), nil, []byte(src))
	if err != nil || tree == nil {
		f.Parsed = false
		f.Err = err
		if err == nil {
			f.Err = fmt.Errorf("parser returned no tree")
		}
		f.Errors = []SyntaxError{{Line: 1, Column: 1, Msg: f.Err.Error()}}
		return f
	}
	defer tree.Close()
	rootNode := tree.RootNode()
	sc := newScrape(path, absPath, root, src, lang)
	switch lang {
	case LangPython:
		sc.walkPy(rootNode)
		sc.fixPythonScopes()
	default:
		sc.walkJS(rootNode)
	}
	sc.collectErrors(rootNode)
	return sc.finish(f, rootNode)
}

func newScrape(path, abs, root, src, lang string) *scrape {
	mod := &scope{kind: scopeModule, binds: map[string]int{}}
	return &scrape{
		src: src, b: []byte(src), path: path, abs: abs, root: root, lang: lang,
		module: mod, cur: mod, scopes: []*scope{mod},
		idents: map[string]int{}, members: map[string]int{},
	}
}

func scriptGrammar(path string) *sitter.Language {
	switch strings.ToLower(filepathExt(path)) {
	case ".py", ".pyi":
		return python.GetLanguage()
	case ".tsx", ".jsx":
		return tsx.GetLanguage()
	case ".js", ".mjs", ".cjs":
		return javascript.GetLanguage()
	default:
		return typescript.GetLanguage()
	}
}

func filepathExt(path string) string {
	i := strings.LastIndexAny(path, `/\`)
	base := path
	if i >= 0 {
		base = path[i+1:]
	}
	if d := strings.LastIndex(base, "."); d >= 0 {
		return base[d:]
	}
	return ""
}

func scriptModuleName(path string) string {
	base := path
	if i := strings.LastIndexAny(base, `/\`); i >= 0 {
		base = base[i+1:]
	}
	if d := strings.LastIndex(base, "."); d > 0 {
		base = base[:d]
	}
	return base
}

func (s *scrape) push(kind scopeKind) *scope {
	sc := &scope{kind: kind, parent: s.cur, binds: map[string]int{}}
	s.scopes = append(s.scopes, sc)
	s.cur = sc
	return sc
}

func (s *scrape) pop() {
	if s.cur != nil && s.cur.parent != nil {
		s.cur = s.cur.parent
	}
}

func (s *scrape) text(n *sitter.Node) string {
	if n == nil {
		return ""
	}
	return n.Content(s.b)
}

func (s *scrape) lineOf(off int) int {
	if off < 0 {
		off = 0
	}
	if off > len(s.src) {
		off = len(s.src)
	}
	return strings.Count(s.src[:off], "\n") + 1
}

func (s *scrape) colOf(off int) int {
	if off < 0 {
		off = 0
	}
	if off > len(s.src) {
		off = len(s.src)
	}
	if i := strings.LastIndex(s.src[:off], "\n"); i >= 0 {
		return off - i
	}
	return off + 1
}

func nodeOK(n *sitter.Node) bool { return n != nil && !n.IsNull() }

func field(n *sitter.Node, name string) *sitter.Node {
	if !nodeOK(n) {
		return nil
	}
	c := n.ChildByFieldName(name)
	if !nodeOK(c) {
		return nil
	}
	return c
}

func fieldsByName(n *sitter.Node, name string) []*sitter.Node {
	if !nodeOK(n) {
		return nil
	}
	var out []*sitter.Node
	for i := 0; i < int(n.ChildCount()); i++ {
		if n.FieldNameForChild(i) != name {
			continue
		}
		if c := n.Child(i); nodeOK(c) {
			out = append(out, c)
		}
	}
	return out
}

func (s *scrape) keyword(n *sitter.Node, kw string) bool {
	if !nodeOK(n) {
		return false
	}
	for i := 0; i < int(n.ChildCount()); i++ {
		c := n.Child(i)
		if nodeOK(c) && !c.IsNamed() && s.text(c) == kw {
			return true
		}
	}
	return false
}

// leadingDoc pulls a contiguous run of comment siblings onto the element.
// A blank line breaks the run, so a comment that belongs to the previous
// declaration stays with it.
func (s *scrape) leadingDoc(n *sitter.Node) (start int, doc string) {
	start = int(n.StartByte())
	var nearest *sitter.Node
	cur := n
	for {
		prev := cur.PrevNamedSibling()
		if !nodeOK(prev) || prev.Type() != "comment" {
			break
		}
		between := ""
		if int(prev.EndByte()) <= int(cur.StartByte()) && int(cur.StartByte()) <= len(s.src) {
			between = s.src[prev.EndByte():cur.StartByte()]
		}
		if strings.Count(between, "\n") > 1 {
			break
		}
		if nearest == nil {
			nearest = prev
		}
		start = int(prev.StartByte())
		cur = prev
	}
	if nearest != nil {
		doc = commentDoc(s.text(nearest))
	}
	return start, doc
}

func commentDoc(text string) string {
	t := strings.TrimSpace(text)
	switch {
	case strings.HasPrefix(t, "/**"):
		t = strings.TrimSuffix(strings.TrimPrefix(t, "/**"), "*/")
	case strings.HasPrefix(t, "/*"):
		t = strings.TrimSuffix(strings.TrimPrefix(t, "/*"), "*/")
	default:
		t = strings.TrimPrefix(t, "#")
		t = strings.TrimPrefix(t, "//")
	}
	line := strings.TrimSpace(strings.Split(t, "\n")[0])
	line = strings.TrimPrefix(strings.TrimSpace(line), "*")
	return strings.TrimSpace(line)
}

func firstLine(src string, start, end int) string {
	if start < 0 {
		start = 0
	}
	if end > len(src) {
		end = len(src)
	}
	if start >= end {
		return ""
	}
	slice := src[start:end]
	if i := strings.IndexByte(slice, '\n'); i >= 0 {
		slice = slice[:i]
	}
	return strings.TrimSpace(slice)
}

func (s *scrape) addBind(name string, kind bindKind, start, end int, sc *scope, elem int) int {
	if name == "" || sc == nil {
		return -1
	}
	idx := len(s.binds)
	s.binds = append(s.binds, nameBind{
		name: name, kind: kind, start: start, end: end, elem: elem,
	})
	if kind != bindMethod {
		if sc.binds == nil {
			sc.binds = map[string]int{}
		}
		sc.binds[name] = idx
	}
	s.idents[name]++
	return idx
}

func (s *scrape) addLoad(n *sitter.Node, call, jsx, close bool) {
	if !nodeOK(n) {
		return
	}
	name := s.text(n)
	if name == "" {
		return
	}
	s.loads = append(s.loads, nameLoad{
		name: name, start: int(n.StartByte()), end: int(n.EndByte()),
		line: s.lineOf(int(n.StartByte())), column: s.colOf(int(n.StartByte())),
		sc: s.cur, call: call, jsx: jsx, closeTag: close,
	})
	s.idents[name]++
}

func (s *scrape) addProp(prop, obj *sitter.Node, call, jsx bool) {
	if !nodeOK(prop) {
		return
	}
	name := s.text(prop)
	if name == "" {
		return
	}
	l := nameLoad{
		name: name, start: int(prop.StartByte()), end: int(prop.EndByte()),
		line: s.lineOf(int(prop.StartByte())), column: s.colOf(int(prop.StartByte())),
		sc: s.cur, call: call, jsx: jsx, prop: true,
	}
	if nodeOK(obj) {
		l.qual = s.text(obj)
		l.objStart = int(obj.StartByte())
		l.objEnd = int(obj.EndByte())
	}
	s.loads = append(s.loads, l)
	s.idents[name]++
}

func (s *scrape) bindAssign(n *sitter.Node, sc *scope) {
	if !nodeOK(n) || sc == nil {
		return
	}
	name := s.text(n)
	if name == "" {
		return
	}
	if sc.binds != nil {
		if idx, ok := sc.binds[name]; ok {
			// The declaration's own identifier is not a use of itself.
			if s.binds[idx].start == int(n.StartByte()) {
				return
			}
			s.addLoad(n, false, false, false)
			return
		}
	}
	s.addBind(name, bindLocal, int(n.StartByte()), int(n.EndByte()), sc, -1)
}

func (s *scrape) assignScope() *scope {
	for p := s.cur; p != nil; p = p.parent {
		switch p.kind {
		case scopeFunction, scopeArrow, scopeLambda, scopeComp:
			return p
		}
	}
	return s.cur
}

func (s *scrape) inFunction() bool {
	for p := s.cur; p != nil; p = p.parent {
		switch p.kind {
		case scopeFunction, scopeArrow, scopeLambda:
			return true
		}
	}
	return false
}

func (s *scrape) emitHere() bool {
	if s.cur == nil {
		return false
	}
	switch s.cur.kind {
	case scopeModule, scopeClass, scopeNamespace:
		return !s.inFunction()
	}
	return false
}

func isHookName(name string) bool {
	return len(name) > 3 && strings.HasPrefix(name, "use") && name[3] >= 'A' && name[3] <= 'Z'
}

func isComponentName(name string) bool {
	r, _ := utf8.DecodeRuneInString(name)
	return r != utf8.RuneError && unicode.IsUpper(r)
}

func isIntrinsicTag(name string) bool {
	if name == "" {
		return true
	}
	r, _ := utf8.DecodeRuneInString(name)
	return unicode.IsLower(r)
}

func containsJSX(n *sitter.Node) bool {
	if !nodeOK(n) {
		return false
	}
	switch n.Type() {
	case "jsx_element", "jsx_self_closing_element", "jsx_fragment":
		return true
	}
	for i := 0; i < int(n.ChildCount()); i++ {
		if containsJSX(n.Child(i)) {
			return true
		}
	}
	return false
}

func (s *scrape) place(e *Element, inner *sitter.Node) {
	if e.Start < 0 {
		e.Start = 0
	}
	if e.End > len(s.src) {
		e.End = len(s.src)
	}
	if e.End < e.Start {
		e.End = e.Start
	}
	e.StartLine = s.lineOf(e.Start)
	if e.End > e.Start {
		e.EndLine = s.lineOf(e.End - 1)
	} else {
		e.EndLine = e.StartLine
	}
	e.UnitStart, e.UnitEnd = e.Start, e.End
	if e.Start <= e.End && e.End <= len(s.src) {
		e.Revision = Revision(s.src[e.Start:e.End])
	}
	if inner != nil {
		e.Signature = firstLine(s.src, int(inner.StartByte()), int(inner.EndByte()))
		e.DeclLine = s.lineOf(int(inner.StartByte()))
	} else if e.NameStart > 0 {
		e.DeclLine = s.lineOf(e.NameStart)
	}
	if e.DeclLine == 0 {
		e.DeclLine = e.StartLine
	}
}

func (s *scrape) qualify(name string) (recv, full string) {
	switch {
	case s.classRecv != "":
		recv = s.classRecv
	case s.nsRecv != "":
		recv = s.nsRecv
	}
	full = name
	if recv != "" {
		full = recv + "." + name
	}
	return recv, full
}

// fixPythonScopes drops assignment binds that a global or nonlocal
// declaration said are not locals. The assignment is re-recorded as a load
// so the name still resolves, to the module or to the enclosing function.
func (s *scrape) fixPythonScopes() {
	for _, sc := range s.scopes {
		if sc.kind != scopeFunction && sc.kind != scopeLambda {
			continue
		}
		for name := range sc.globals {
			s.rebindAsLoad(sc, name)
		}
		for name := range sc.nonlocals {
			s.rebindAsLoad(sc, name)
		}
	}
}

func (s *scrape) rebindAsLoad(sc *scope, name string) {
	if sc.binds == nil {
		return
	}
	idx, ok := sc.binds[name]
	if !ok {
		return
	}
	b := s.binds[idx]
	delete(sc.binds, name)
	s.binds[idx].kind = bindRemoved
	s.loads = append(s.loads, nameLoad{
		name: name, start: b.start, end: b.end, sc: sc,
		line: s.lineOf(b.start), column: s.colOf(b.start),
	})
}

func (s *scrape) assignBindIDs() {
	for i := range s.binds {
		b := &s.binds[i]
		if b.kind == bindRemoved {
			continue
		}
		if b.kind == bindImport && !b.alias && b.importName != "" && b.importName != "*" && b.importName != "default" && b.importFile != "" {
			b.id = "i:" + b.importFile + "#" + b.importName
			continue
		}
		b.id = "d:" + s.path + "#" + fmt.Sprint(b.start)
	}
	for i := range s.binds {
		if s.binds[i].elem >= 0 && s.binds[i].elem < len(s.elements) {
			s.elements[s.binds[i].elem].bind = s.binds[i].id
		}
	}
}

func (s *scrape) resolve() {
	resolved := make([]int, len(s.loads))
	for i := range resolved {
		resolved[i] = -1
	}
	for i := range s.loads {
		if s.loads[i].prop {
			continue
		}
		resolved[i] = s.resolveLoad(s.loads[i])
	}
	byObj := map[int]int{}
	for i, l := range s.loads {
		if !l.prop && resolved[i] >= 0 {
			byObj[l.start] = resolved[i]
		}
	}
	for i := range s.loads {
		l := &s.loads[i]
		if !l.prop {
			continue
		}
		resolved[i] = s.resolveProp(*l, byObj)
	}
	for i, l := range s.loads {
		idx := resolved[i]
		if l.closeTag {
			if idx >= 0 {
				s.addUse(l, s.binds[idx].id)
			}
			continue
		}
		if l.call && !(l.jsx && isIntrinsicTag(l.name) && l.qual == "") {
			s.emitCall(l, idx)
		}
		if idx < 0 || s.binds[idx].kind == bindRemoved {
			continue
		}
		// A property resolved only as a cross-file method call has no bind
		// id of its own; resolveProp returns -1 in that case and emitCall
		// already recorded the target. A property that found a member bind
		// is a rename site.
		if l.prop && s.binds[idx].kind != bindMethod && s.binds[idx].kind != bindDecl && s.binds[idx].kind != bindImport {
			continue
		}
		s.addUse(l, s.binds[idx].id)
	}
}

// uses collected during resolve. Held on the scrape until finish.
func (s *scrape) emitCall(l nameLoad, idx int) {
	// filled in finish via s.calls
	s.pending = append(s.pending, pendingCall{load: l, bind: idx})
}

type pendingCall struct {
	load nameLoad
	bind int
}

func (s *scrape) addUse(l nameLoad, id string) {
	if id == "" {
		return
	}
	s.useBuf = append(s.useBuf, refUse{
		Name: l.name, Start: l.start, End: l.end, Line: l.line, Column: l.column,
		Bind: id, Call: l.call && !l.closeTag, JSX: l.jsx, Qual: l.qual,
	})
}

func (s *scrape) resolveLoad(l nameLoad) int {
	if s.lang == LangPython {
		return s.resolvePy(l)
	}
	return lookupScope(l.sc, l.name, false)
}

// resolvePy skips class scopes for a use that originates in a function: a
// method does not see names assigned in the class body. global jumps to the
// module; nonlocal jumps to the enclosing function, still skipping classes.
func (s *scrape) resolvePy(l nameLoad) int {
	for p := l.sc; p != nil; p = p.parent {
		if p.kind != scopeFunction && p.kind != scopeLambda {
			continue
		}
		if p.globals[l.name] {
			return lookupScope(s.module, l.name, false)
		}
		if p.nonlocals[l.name] {
			return lookupNonlocal(p.parent, l.name)
		}
	}
	return lookupScope(l.sc, l.name, originatedInFunction(l.sc))
}

func originatedInFunction(sc *scope) bool {
	for p := sc; p != nil; p = p.parent {
		switch p.kind {
		case scopeFunction, scopeArrow, scopeLambda:
			return true
		}
	}
	return false
}

func lookupScope(sc *scope, name string, skipClass bool) int {
	for p := sc; p != nil; p = p.parent {
		if skipClass && p.kind == scopeClass {
			continue
		}
		if p.binds != nil {
			if idx, ok := p.binds[name]; ok {
				return idx
			}
		}
	}
	return -1
}

func lookupNonlocal(sc *scope, name string) int {
	for p := sc; p != nil; p = p.parent {
		if p.kind == scopeClass {
			continue
		}
		if p.binds != nil {
			if idx, ok := p.binds[name]; ok && (p.kind == scopeFunction || p.kind == scopeLambda || p.kind == scopeModule) {
				return idx
			}
		}
	}
	return -1
}

// resolveProp links obj.member to a method or namespace member, or to an
// export reached through `import *`. A parameter named self does not link,
// so self.method stays a by-name call rather than a use of every method
// that happens to share the spelling.
func (s *scrape) resolveProp(l nameLoad, byObj map[int]int) int {
	idx, ok := byObj[l.objStart]
	if !ok || idx < 0 {
		return -1
	}
	b := s.binds[idx]
	switch b.kind {
	case bindImport:
		if b.importName == "*" && b.importFile != "" {
			id := "i:" + b.importFile + "#" + l.name
			// Synthesize a bind so the property shares the export's id.
			s.binds = append(s.binds, nameBind{
				name: l.name, id: id, kind: bindImport,
				start: l.start, end: l.end,
				importFile: b.importFile, importName: l.name,
			})
			return len(s.binds) - 1
		}
		return -1
	case bindDecl:
		if b.elem < 0 || b.elem >= len(s.elements) {
			return -1
		}
		e := &s.elements[b.elem]
		recv := e.Name
		if e.Receiver != "" {
			recv = e.Receiver + "." + e.Name
		}
		if mi, ok := s.members[recv+"\x00"+l.name]; ok {
			return mi
		}
	}
	return -1
}

func (s *scrape) finish(f *File, root *sitter.Node) *File {
	s.resolveImports()
	s.assignBindIDs()
	for _, a := range s.aliasUses {
		if a.file == "" || a.name == "" {
			continue
		}
		s.useBuf = append(s.useBuf, refUse{
			Name: a.name, Start: a.start, End: a.end, Line: a.line, Column: a.column,
			Bind: "i:" + a.file + "#" + a.name,
		})
	}
	s.resolve()
	s.applyRoles()
	s.applyExportFlags()

	var elems []Element
	for i := range s.elements {
		e := s.elements[i]
		if spanOverlaps(e.Start, e.End, s.errSpans) {
			continue
		}
		elems = append(elems, e)
	}
	for i, sp := range s.errSpans {
		if i >= len(s.errTop) || !s.errTop[i] {
			continue
		}
		e := Element{
			Kind: KindSyntaxError, Name: "syntax_error",
			Start: sp[0], End: sp[1], Err: "syntax error",
		}
		s.place(&e, nil)
		elems = append(elems, e)
	}
	if h := s.header(elems); h != nil {
		elems = append([]Element{*h}, elems...)
	}
	assignKeys(elems, goBaseKey)
	f.Elements = elems
	f.Imports = s.imports
	f.Idents = s.idents
	f.uses = s.useBuf
	f.binds = s.binds
	f.Calls = s.callsFromPending()
	f.Errors = s.errors
	if root != nil && root.HasError() {
		f.Parsed = false
	} else {
		f.Parsed = true
	}
	if len(f.Errors) > 0 {
		e := f.Errors[0]
		f.Err = fmt.Errorf("line %d:%d: %s", e.Line, e.Column, e.Msg)
		if f.Parsed {
			f.Parsed = false
		}
	}
	return f
}

// topLevelSpan is filled by collectErrors: errTop[i] says the span is a
// direct child of the root. Stored beside errSpans.
func (s *scrape) header(elems []Element) *Element {
	first := len(s.src)
	for _, e := range elems {
		if e.Kind == KindSyntaxError {
			continue
		}
		if e.Start < first {
			first = e.Start
		}
	}
	if first <= 0 {
		return nil
	}
	if first > len(s.src) {
		first = len(s.src)
	}
	end := len(strings.TrimRight(s.src[:first], " \t\n"))
	if end == 0 {
		return nil
	}
	e := Element{Kind: KindHeader, Name: HeaderKey, Start: 0, End: end}
	s.place(&e, nil)
	e.Signature = ""
	return &e
}

func (s *scrape) applyRoles() {
	if s.lang == LangPython {
		return
	}
	for i := range s.elements {
		e := &s.elements[i]
		if e.Kind != KindFunction || e.Receiver != "" {
			continue
		}
		hook := isHookName(e.Name)
		comp := (isComponentName(e.Name) || (e.defaultExport && e.Name == "default")) && e.Role == "jsx"
		if hook {
			e.Role = "hook"
		} else if comp {
			e.Role = "component"
		} else {
			e.Role = ""
		}
	}
}

func (s *scrape) applyExportFlags() {
	for i := range s.elements {
		e := &s.elements[i]
		switch s.lang {
		case LangPython:
			if e.Receiver != "" {
				e.Exported = !strings.HasPrefix(e.Name, "_")
				continue
			}
			if s.hasAll {
				e.Exported = s.all[e.Name]
				continue
			}
			e.Exported = e.Name != "" && !strings.HasPrefix(e.Name, "_")
		default:
			// Methods and nested classes carry the visibility the walker
			// decided (private, protected, a leading underscore or '#').
			// Top-level names are exported only when the file says export.
			if e.Kind == KindMethod || (e.Kind == KindClass && e.Receiver != "") {
				continue
			}
			e.Exported = e.namedExport || e.defaultExport
		}
	}
}

func (s *scrape) callsFromPending() []Call {
	var out []Call
	for _, p := range s.pending {
		l := p.load
		if l.jsx && l.qual == "" && isIntrinsicTag(l.name) {
			continue
		}
		c := Call{
			Name: l.name, Qualifier: l.qual, Line: l.line,
			Start: l.start, End: l.end, JSX: l.jsx,
		}
		if p.bind >= 0 && p.bind < len(s.binds) {
			s.fillCallTarget(&c, &s.binds[p.bind], l)
		} else if l.prop {
			s.fillUnboundProp(&c, l)
		}
		out = append(out, c)
	}
	return out
}

func (s *scrape) fillCallTarget(c *Call, b *nameBind, l nameLoad) {
	switch b.kind {
	case bindLocal, bindParam:
		c.Local = true
		c.Bound = true
	case bindImport:
		c.Bound = true
		c.TargetFile = b.importFile
		c.TargetName = b.importName
		// A bind synthesized on the property span is the export itself
		// (`hooks.useAuth` through `import *`). A bind that sits on the
		// object is a named import used as a receiver (`User.get`).
		if l.prop && b.start != l.start && b.importName != "" && b.importName != "*" && b.importName != "default" {
			c.TargetRecv = b.importName
			c.TargetName = l.name
			c.Name = l.name
		}
	case bindDecl, bindMethod:
		c.Bound = true
		c.TargetFile = s.path
		c.TargetName = b.name
		if b.elem >= 0 && b.elem < len(s.elements) {
			c.TargetRecv = s.elements[b.elem].Receiver
			c.TargetName = s.elements[b.elem].Name
		}
	}
}

// fillUnboundProp records a cross-file method call. The qualifier resolved
// to a named import during resolveProp, which returns -1 so the property is
// not a rename of a local, but the call still names the imported receiver.
func (s *scrape) fillUnboundProp(c *Call, l nameLoad) {
	// The object load was resolved; find it by span.
	for i := range s.loads {
		o := &s.loads[i]
		if o.prop || o.start != l.objStart {
			continue
		}
		idx := s.resolveLoad(*o)
		if idx < 0 || idx >= len(s.binds) {
			return
		}
		b := &s.binds[idx]
		if b.kind == bindImport && b.importFile != "" && b.importName != "" && b.importName != "*" && b.importName != "default" {
			c.Bound = true
			c.TargetFile = b.importFile
			c.TargetRecv = b.importName
			c.TargetName = l.name
		}
		return
	}
}

func (s *scrape) collectErrors(n *sitter.Node) {
	if !nodeOK(n) {
		return
	}
	if n.Type() == "ERROR" || n.IsError() {
		sp := [2]int{int(n.StartByte()), int(n.EndByte())}
		s.errSpans = append(s.errSpans, sp)
		s.errTop = append(s.errTop, parentIsRoot(n))
		s.errors = append(s.errors, SyntaxError{
			Line: s.lineOf(sp[0]), Column: s.colOf(sp[0]), Msg: "syntax error",
		})
		return
	}
	if !n.HasError() {
		return
	}
	for i := 0; i < int(n.ChildCount()); i++ {
		s.collectErrors(n.Child(i))
	}
}

func parentIsRoot(n *sitter.Node) bool {
	p := n.Parent()
	if !nodeOK(p) {
		return true
	}
	switch p.Type() {
	case "module", "program":
		return true
	}
	return false
}

func spanOverlaps(start, end int, spans [][2]int) bool {
	for _, sp := range spans {
		if start < sp[1] && sp[0] < end {
			return true
		}
	}
	return false
}

// affectedKeys are the elements a script edit is allowed to reshape: the
// ones whose spans overlap the change. A method replacement overlaps its
// class; a sibling method does not.
func affectedKeys(f *File, start, end int) []string {
	if f == nil {
		return nil
	}
	var keys []string
	for i := range f.Elements {
		e := &f.Elements[i]
		var hit bool
		if start == end {
			hit = start > e.Start && start < e.End
		} else {
			hit = start < e.End && end > e.Start
		}
		if hit {
			keys = append(keys, e.Key)
		}
	}
	return keys
}
