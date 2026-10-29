package codemodel

import (
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
)

func (s *scrape) walkPy(n *sitter.Node) {
	if !nodeOK(n) {
		return
	}
	switch n.Type() {
	case "module", "block":
		for i := 0; i < int(n.NamedChildCount()); i++ {
			s.walkPy(n.NamedChild(i))
		}
	case "decorated_definition":
		s.pyDecorated(n)
	case "class_definition":
		s.pyClass(n, n)
	case "function_definition":
		s.pyFunc(n, n)
	case "import_from_statement", "import_statement", "future_import_statement":
		s.pyImport(n)
	case "expression_statement":
		s.pyExprStmt(n)
	case "assignment", "augmented_assignment":
		s.pyAssign(n, n)
	case "global_statement":
		s.pyScopeDecl(n, true)
	case "nonlocal_statement":
		s.pyScopeDecl(n, false)
	case "for_statement":
		s.pyFor(n)
	case "with_statement":
		s.pyWith(n)
	case "except_clause":
		s.pyExcept(n)
	case "lambda":
		s.pyLambda(n)
	case "list_comprehension", "set_comprehension", "dictionary_comprehension", "generator_expression":
		s.pyComp(n)
	case "for_in_clause":
		s.pyForIn(n, s.cur)
	case "named_expression":
		if name := field(n, "name"); nodeOK(name) {
			s.bindTarget(name, s.assignScope())
		}
		s.walkPy(field(n, "value"))
	case "call":
		s.pyCall(n)
	case "attribute":
		s.pyAttr(n, false)
	case "keyword_argument":
		s.walkPy(field(n, "value"))
	case "identifier":
		s.addLoad(n, false, false, false)
	case "string", "integer", "float", "true", "false", "none", "comment", "ellipsis", "concatenated_string":
		return
	default:
		for i := 0; i < int(n.NamedChildCount()); i++ {
			s.walkPy(n.NamedChild(i))
		}
	}
}

func (s *scrape) pyDecorated(n *sitter.Node) {
	def := field(n, "definition")
	if !nodeOK(def) {
		for i := 0; i < int(n.NamedChildCount()); i++ {
			c := n.NamedChild(i)
			if nodeOK(c) && (c.Type() == "function_definition" || c.Type() == "class_definition") {
				def = c
				break
			}
		}
	}
	switch {
	case !nodeOK(def):
		s.walkPy(n)
	case def.Type() == "class_definition":
		s.pyClass(def, n)
	case def.Type() == "function_definition":
		s.pyFunc(def, n)
	default:
		s.walkPy(def)
	}
}

func (s *scrape) pyClass(n, spanNode *sitter.Node) {
	nameNode := field(n, "name")
	name := s.text(nameNode)
	recv, _ := s.qualify(name)
	start, doc := s.leadingDoc(spanNode)
	end := int(spanNode.EndByte())
	body := field(n, "body")
	if ds := pyDocstring(body, s.src); ds != "" {
		doc = firstLineText(ds)
	}
	elem := -1
	if s.emitHere() && name != "" {
		e := Element{
			Name: name, Receiver: recv, Kind: KindClass,
			Start: start, End: end, Doc: doc,
		}
		if nodeOK(nameNode) {
			e.NameStart, e.NameEnd = int(nameNode.StartByte()), int(nameNode.EndByte())
		}
		s.place(&e, n)
		elem = len(s.elements)
		s.elements = append(s.elements, e)
	}
	if nodeOK(nameNode) {
		kind := bindLocal
		if elem >= 0 {
			kind = bindDecl
		}
		idx := s.addBind(name, kind, int(nameNode.StartByte()), int(nameNode.EndByte()), s.cur, elem)
		if elem >= 0 && s.classRecv != "" {
			s.members[s.classRecv+"\x00"+name] = idx
		}
		if elem >= 0 && s.classRecv == "" && s.nsRecv != "" {
			s.members[s.nsRecv+"\x00"+name] = idx
		}
	}
	prev := s.classRecv
	if name != "" {
		if prev == "" {
			s.classRecv = name
		} else {
			s.classRecv = prev + "." + name
		}
	}
	s.push(scopeClass)
	s.walkPy(body)
	// superclasses are loads, not a scope.
	for _, sup := range fieldsByName(n, "superclasses") {
		s.walkPy(sup)
	}
	if arg := n.ChildByFieldName("superclasses"); !nodeOK(arg) {
		// argument_list of bases sits as a named child that is not the body.
		for i := 0; i < int(n.NamedChildCount()); i++ {
			c := n.NamedChild(i)
			if !nodeOK(c) || c == body || c == nameNode {
				continue
			}
			if c.Type() == "argument_list" {
				s.walkPy(c)
			}
		}
	}
	s.pop()
	s.classRecv = prev
}

func (s *scrape) pyFunc(n, spanNode *sitter.Node) {
	nameNode := field(n, "name")
	name := s.text(nameNode)
	isMethod := s.cur != nil && s.cur.kind == scopeClass
	recv := ""
	if isMethod {
		recv = s.classRecv
	}
	start, doc := s.leadingDoc(spanNode)
	if ds := pyDocstring(field(n, "body"), s.src); ds != "" {
		doc = firstLineText(ds)
	}
	elem := -1
	if s.emitHere() && name != "" {
		kind := KindFunction
		if isMethod {
			kind = KindMethod
		}
		e := Element{
			Name: name, Receiver: recv, Kind: kind,
			Start: start, End: int(spanNode.EndByte()), Doc: doc,
		}
		if nodeOK(nameNode) {
			e.NameStart, e.NameEnd = int(nameNode.StartByte()), int(nameNode.EndByte())
		}
		s.place(&e, n)
		elem = len(s.elements)
		s.elements = append(s.elements, e)
	}
	if nodeOK(nameNode) {
		kind := bindLocal
		switch {
		case isMethod && elem >= 0:
			kind = bindMethod
		case elem >= 0:
			kind = bindDecl
		}
		sc := s.cur
		idx := s.addBind(name, kind, int(nameNode.StartByte()), int(nameNode.EndByte()), sc, elem)
		if isMethod && recv != "" {
			s.members[recv+"\x00"+name] = idx
		}
	}
	s.push(scopeFunction)
	s.pyParams(field(n, "parameters"))
	if rt := field(n, "return_type"); nodeOK(rt) {
		s.walkPy(rt)
	}
	s.walkPy(field(n, "body"))
	s.pop()
}

func (s *scrape) pyParams(n *sitter.Node) {
	if !nodeOK(n) {
		return
	}
	for i := 0; i < int(n.NamedChildCount()); i++ {
		c := n.NamedChild(i)
		if !nodeOK(c) {
			continue
		}
		switch c.Type() {
		case "identifier":
			s.addBind(s.text(c), bindParam, int(c.StartByte()), int(c.EndByte()), s.cur, -1)
		case "typed_parameter", "typed_default_parameter", "default_parameter":
			s.pyParam(c)
		case "list_splat_pattern", "dictionary_splat_pattern":
			s.pySplat(c)
		default:
			s.pyParam(c)
		}
	}
}

func (s *scrape) pyParam(n *sitter.Node) {
	var name *sitter.Node
	for i := 0; i < int(n.NamedChildCount()); i++ {
		c := n.NamedChild(i)
		if !nodeOK(c) {
			continue
		}
		if c.Type() == "identifier" && name == nil {
			name = c
			continue
		}
		s.walkPy(c)
	}
	if name != nil {
		s.addBind(s.text(name), bindParam, int(name.StartByte()), int(name.EndByte()), s.cur, -1)
	}
}

func (s *scrape) pySplat(n *sitter.Node) {
	for i := 0; i < int(n.NamedChildCount()); i++ {
		c := n.NamedChild(i)
		if nodeOK(c) && c.Type() == "identifier" {
			s.addBind(s.text(c), bindParam, int(c.StartByte()), int(c.EndByte()), s.cur, -1)
			return
		}
	}
}

func (s *scrape) pyImport(n *sitter.Node) {
	future := n.Type() == "future_import_statement"
	level := 0
	mod := ""
	if mn := field(n, "module_name"); nodeOK(mn) {
		level, mod = s.pyModule(mn)
	} else {
		for i := 0; i < int(n.NamedChildCount()); i++ {
			c := n.NamedChild(i)
			if nodeOK(c) && (c.Type() == "relative_import" || c.Type() == "dotted_name") {
				level, mod = s.pyModule(c)
				break
			}
		}
	}
	var bounds []pyBound
	if n.Type() == "import_statement" || future {
		for _, name := range fieldsByName(n, "name") {
			bounds = append(bounds, s.pyImportName(name, true)...)
		}
		if len(bounds) == 0 {
			for i := 0; i < int(n.NamedChildCount()); i++ {
				c := n.NamedChild(i)
				if !nodeOK(c) {
					continue
				}
				if c.Type() == "dotted_name" || c.Type() == "aliased_import" {
					bounds = append(bounds, s.pyImportName(c, true)...)
				}
			}
		}
	} else {
		for _, name := range fieldsByName(n, "name") {
			bounds = append(bounds, s.pyImportName(name, false)...)
		}
	}
	if future {
		mod = "__future__"
		level = 0
	}
	if n.Type() == "import_statement" && len(bounds) > 0 && bounds[0].module != "" {
		mod = bounds[0].module
	}
	spec := mod
	if level > 0 {
		spec = strings.Repeat(".", level) + mod
	}

	// `from . import a, b` resolves each name to its own submodule.
	perName := level > 0 && mod == "" && n.Type() == "import_from_statement"
	if !perName {
		file := ""
		if !future {
			file = resolvePythonModule(s.abs, s.root, mod, level)
		}
		s.addPyImport(n, spec, level, file, bounds)
		return
	}
	for _, b := range bounds {
		file := resolvePythonModule(s.abs, s.root, b.remote, level)
		s.addPyImport(n, spec, level, file, []pyBound{b})
	}
}

type pyBound struct {
	remote, local string
	module        string
	aliased       bool
	localNode     *sitter.Node
	remoteNode    *sitter.Node
}

func (s *scrape) pyModule(n *sitter.Node) (level int, mod string) {
	if !nodeOK(n) {
		return 0, ""
	}
	switch n.Type() {
	case "relative_import":
		for i := 0; i < int(n.NamedChildCount()); i++ {
			c := n.NamedChild(i)
			if !nodeOK(c) {
				continue
			}
			if c.Type() == "import_prefix" || c.Type() == "relative_import_prefix" {
				level = strings.Count(s.text(c), ".")
			}
			if c.Type() == "dotted_name" {
				mod = s.text(c)
			}
		}
		if level == 0 {
			// import_prefix may be unnamed.
			for i := 0; i < int(n.ChildCount()); i++ {
				c := n.Child(i)
				if nodeOK(c) && !c.IsNamed() {
					if dots := strings.Count(s.text(c), "."); dots > level {
						level = dots
					}
				}
			}
		}
		return level, mod
	case "dotted_name":
		return 0, s.text(n)
	default:
		return 0, s.text(n)
	}
}

func (s *scrape) pyImportName(n *sitter.Node, moduleImport bool) []pyBound {
	if !nodeOK(n) {
		return nil
	}
	switch n.Type() {
	case "aliased_import":
		remote := field(n, "name")
		alias := field(n, "alias")
		mod := s.text(remote)
		local := s.text(alias)
		b := pyBound{remote: lastDot(mod), local: local, module: mod, aliased: true, localNode: alias, remoteNode: remote}
		if moduleImport {
			b.remote = mod
			if local == "" {
				b.local = firstDot(mod)
				b.localNode = remote
				b.aliased = false
			}
		}
		return []pyBound{b}
	case "dotted_name":
		mod := s.text(n)
		if moduleImport {
			return []pyBound{{remote: mod, local: firstDot(mod), module: mod, localNode: n}}
		}
		return []pyBound{{remote: lastDot(mod), local: lastDot(mod), module: mod, localNode: n}}
	case "identifier":
		t := s.text(n)
		return []pyBound{{remote: t, local: t, localNode: n}}
	default:
		return nil
	}
}

func (s *scrape) addPyImport(n *sitter.Node, spec string, level int, file string, bounds []pyBound) {
	if len(bounds) == 0 {
		return
	}
	imp := Import{
		Spec: spec, Level: level,
		Start: int(n.StartByte()), End: int(n.EndByte()),
		Line: s.lineOf(int(n.StartByte())),
		Name: bounds[0].local,
	}
	if file != "" {
		imp.Resolved = displayPath(s.root, file)
		imp.Path = imp.Resolved
	} else {
		imp.Path = spec
	}
	for _, b := range bounds {
		imp.Imported = append(imp.Imported, ImportedName{Remote: b.remote, Local: b.local, Aliased: b.aliased})
		if !nodeOK(b.localNode) {
			continue
		}
		idx := s.addBind(b.local, bindImport, int(b.localNode.StartByte()), int(b.localNode.EndByte()), s.cur, -1)
		if idx >= 0 {
			s.binds[idx].importFile = imp.Resolved
			s.binds[idx].importName = b.remote
			s.binds[idx].alias = b.aliased
			if b.module != "" && !b.aliased && strings.Contains(b.module, ".") {
				// `import os.path` binds the first component, not the leaf.
				s.binds[idx].importName = b.module
			}
		}
		if b.aliased && nodeOK(b.remoteNode) {
			// The remote spelling is a use of the export, not of the local alias.
			rn := b.remoteNode
			if rn.Type() != "identifier" {
				if id := firstIdent(rn); id != nil {
					rn = id
				}
			}
			s.aliasUses = append(s.aliasUses, aliasUse{
				name: lastDot(b.remote), start: int(rn.StartByte()), end: int(rn.EndByte()),
				line: s.lineOf(int(rn.StartByte())), column: s.colOf(int(rn.StartByte())),
				file: imp.Resolved,
			})
		}
	}
	s.imports = append(s.imports, imp)
}

func (s *scrape) pyExprStmt(n *sitter.Node) {
	if n.NamedChildCount() == 0 {
		return
	}
	inner := n.NamedChild(0)
	if !nodeOK(inner) {
		return
	}
	switch inner.Type() {
	case "assignment", "augmented_assignment":
		s.pyAssign(inner, n)
	case "string", "concatenated_string":
		return
	default:
		s.walkPy(inner)
	}
}

func (s *scrape) pyAssign(n, spanNode *sitter.Node) {
	left := field(n, "left")
	right := field(n, "right")
	if !nodeOK(left) {
		for i := 0; i < int(n.NamedChildCount()); i++ {
			c := n.NamedChild(i)
			if nodeOK(c) {
				left = c
				break
			}
		}
	}
	names := pyAssignNames(left, s.src)
	sc := s.assignScope()
	if n.Type() != "augmented_assignment" && sc.kind == scopeModule && len(names) > 0 {
		start, doc := s.leadingDoc(spanNode)
		e := Element{
			Name: names[0], Names: names, Kind: kindOfAssign(names[0]),
			Start: start, End: int(spanNode.EndByte()), Doc: doc,
		}
		if nodeOK(left) && left.Type() == "identifier" {
			e.NameStart, e.NameEnd = int(left.StartByte()), int(left.EndByte())
		} else if len(names) > 0 {
			e.NameStart, e.NameEnd = int(spanNode.StartByte()), int(spanNode.StartByte())
			if id := firstIdent(left); id != nil {
				e.NameStart, e.NameEnd = int(id.StartByte()), int(id.EndByte())
			}
		}
		s.place(&e, n)
		elem := len(s.elements)
		s.elements = append(s.elements, e)
		if names[0] == "__all__" {
			if vals := pyStringList(right, s.src); vals != nil {
				s.hasAll = true
				s.all = map[string]bool{}
				for _, v := range vals {
					s.all[v] = true
				}
			}
		}
		if id := firstIdent(left); id != nil && (sc.binds == nil || !bound(sc, names[0])) {
			s.addBind(names[0], bindDecl, int(id.StartByte()), int(id.EndByte()), sc, elem)
		}
		// Other names in a tuple unpack are bindings too, but one element
		// carries them (Names), matching a Go multi-name spec. The first
		// identifier is already bound, so bindTarget does not count it twice.
		s.bindTarget(left, sc)
		s.walkPy(right)
		return
	}
	s.bindTarget(left, sc)
	s.walkPy(right)
}

func kindOfAssign(name string) Kind {
	if name != "" && name == strings.ToUpper(name) && strings.ContainsFunc(name, func(r rune) bool { return r >= 'A' && r <= 'Z' }) {
		return KindConst
	}
	return KindVar
}

func pyAssignNames(n *sitter.Node, src string) []string {
	var names []string
	var walk func(*sitter.Node)
	walk = func(n *sitter.Node) {
		if !nodeOK(n) {
			return
		}
		switch n.Type() {
		case "identifier":
			names = append(names, n.Content([]byte(src)))
		case "pattern_list", "tuple", "list", "parenthesized_expression", "expression_list":
			for i := 0; i < int(n.NamedChildCount()); i++ {
				walk(n.NamedChild(i))
			}
		}
	}
	walk(n)
	return names
}

func firstIdent(n *sitter.Node) *sitter.Node {
	if !nodeOK(n) {
		return nil
	}
	if n.Type() == "identifier" {
		return n
	}
	for i := 0; i < int(n.NamedChildCount()); i++ {
		if id := firstIdent(n.NamedChild(i)); id != nil {
			return id
		}
	}
	return nil
}

func (s *scrape) bindTarget(n *sitter.Node, sc *scope) {
	if !nodeOK(n) {
		return
	}
	switch n.Type() {
	case "identifier":
		s.bindAssign(n, sc)
	case "pattern_list", "tuple", "list", "parenthesized_expression", "expression_list":
		for i := 0; i < int(n.NamedChildCount()); i++ {
			s.bindTarget(n.NamedChild(i), sc)
		}
	case "attribute", "subscript":
		s.walkPy(n)
	case "list_splat_pattern", "dictionary_splat_pattern":
		for i := 0; i < int(n.NamedChildCount()); i++ {
			s.bindTarget(n.NamedChild(i), sc)
		}
	default:
		if n.NamedChildCount() == 1 {
			s.bindTarget(n.NamedChild(0), sc)
			return
		}
		s.walkPy(n)
	}
}

func (s *scrape) pyFor(n *sitter.Node) {
	left := field(n, "left")
	right := field(n, "right")
	s.bindTarget(left, s.assignScope())
	s.walkPy(right)
	for i := 0; i < int(n.NamedChildCount()); i++ {
		c := n.NamedChild(i)
		if c == left || c == right {
			continue
		}
		s.walkPy(c)
	}
}

func (s *scrape) pyWith(n *sitter.Node) {
	for i := 0; i < int(n.NamedChildCount()); i++ {
		c := n.NamedChild(i)
		if !nodeOK(c) {
			continue
		}
		if c.Type() == "with_item" || c.Type() == "with_clause" {
			s.pyWithItem(c)
			continue
		}
		s.walkPy(c)
	}
}

func (s *scrape) pyWithItem(n *sitter.Node) {
	if alias := field(n, "alias"); nodeOK(alias) {
		s.bindTarget(alias, s.assignScope())
	}
	if v := field(n, "value"); nodeOK(v) {
		if v.Type() == "as_pattern" {
			s.pyAsPattern(v)
			return
		}
		s.walkPy(v)
	}
	for i := 0; i < int(n.NamedChildCount()); i++ {
		c := n.NamedChild(i)
		if nodeOK(c) && c.Type() == "as_pattern" {
			s.pyAsPattern(c)
		}
	}
}

func (s *scrape) pyAsPattern(n *sitter.Node) {
	alias := field(n, "alias")
	if !nodeOK(alias) {
		for i := 0; i < int(n.NamedChildCount()); i++ {
			c := n.NamedChild(i)
			if nodeOK(c) && (c.Type() == "as_pattern_target" || c.Type() == "identifier") && i > 0 {
				alias = c
				break
			}
		}
	}
	if nodeOK(alias) {
		if alias.Type() == "as_pattern_target" {
			s.bindTarget(alias, s.assignScope())
		} else {
			s.bindAssign(alias, s.assignScope())
		}
	}
	for i := 0; i < int(n.NamedChildCount()); i++ {
		c := n.NamedChild(i)
		if c == alias || (nodeOK(c) && c.Type() == "as_pattern_target") {
			continue
		}
		s.walkPy(c)
	}
}

func (s *scrape) pyExcept(n *sitter.Node) {
	s.pyAsPattern(n)
}

func (s *scrape) pyLambda(n *sitter.Node) {
	s.push(scopeLambda)
	s.pyParams(field(n, "parameters"))
	s.walkPy(field(n, "body"))
	s.pop()
}

// pyComp gives the comprehension its own scope so the loop variable does
// not leak into the enclosing function and shadow the module name.
func (s *scrape) pyComp(n *sitter.Node) {
	s.push(scopeComp)
	for i := 0; i < int(n.NamedChildCount()); i++ {
		c := n.NamedChild(i)
		if !nodeOK(c) {
			continue
		}
		if c.Type() == "for_in_clause" {
			s.pyForIn(c, s.cur.parent)
			continue
		}
		s.walkPy(c)
	}
	s.pop()
}

func (s *scrape) pyForIn(n *sitter.Node, iterScope *scope) {
	left := field(n, "left")
	right := field(n, "right")
	s.bindTarget(left, s.cur)
	saved := s.cur
	if iterScope != nil {
		s.cur = iterScope
	}
	s.walkPy(right)
	s.cur = saved
	for i := 0; i < int(n.NamedChildCount()); i++ {
		c := n.NamedChild(i)
		if c == left || c == right {
			continue
		}
		if nodeOK(c) && c.Type() == "if_clause" {
			s.walkPy(c)
		}
	}
}

func (s *scrape) pyCall(n *sitter.Node) {
	fn := field(n, "function")
	switch {
	case !nodeOK(fn):
	case fn.Type() == "identifier":
		s.addLoad(fn, true, false, false)
	case fn.Type() == "attribute":
		s.pyAttr(fn, true)
	default:
		s.walkPy(fn)
	}
	s.walkPy(field(n, "arguments"))
}

func (s *scrape) pyAttr(n *sitter.Node, call bool) {
	obj := field(n, "object")
	attr := field(n, "attribute")
	if !nodeOK(attr) {
		s.walkPy(obj)
		return
	}
	if nodeOK(obj) && obj.Type() == "identifier" {
		s.addLoad(obj, false, false, false)
		s.addProp(attr, obj, call, false)
		return
	}
	s.walkPy(obj)
	s.idents[s.text(attr)]++
	if call {
		s.loads = append(s.loads, nameLoad{
			name: s.text(attr), start: int(attr.StartByte()), end: int(attr.EndByte()),
			line: s.lineOf(int(attr.StartByte())), column: s.colOf(int(attr.StartByte())),
			sc: s.cur, call: true, prop: true, qual: s.text(obj),
		})
	}
}

func (s *scrape) pyScopeDecl(n *sitter.Node, global bool) {
	sc := s.assignScope()
	if sc == nil {
		return
	}
	for i := 0; i < int(n.NamedChildCount()); i++ {
		c := n.NamedChild(i)
		if !nodeOK(c) || c.Type() != "identifier" {
			continue
		}
		name := s.text(c)
		if global {
			if sc.globals == nil {
				sc.globals = map[string]bool{}
			}
			sc.globals[name] = true
		} else {
			if sc.nonlocals == nil {
				sc.nonlocals = map[string]bool{}
			}
			sc.nonlocals[name] = true
		}
		s.idents[name]++
	}
}

func pyDocstring(body *sitter.Node, src string) string {
	if !nodeOK(body) {
		return ""
	}
	var first *sitter.Node
	if body.Type() == "block" {
		first = body.NamedChild(0)
	} else {
		first = body
	}
	if !nodeOK(first) {
		return ""
	}
	if first.Type() == "expression_statement" && first.NamedChildCount() > 0 {
		first = first.NamedChild(0)
	}
	if !nodeOK(first) {
		return ""
	}
	if first.Type() == "string" || first.Type() == "concatenated_string" {
		return pyString(first, src)
	}
	return ""
}

func pyString(n *sitter.Node, src string) string {
	if !nodeOK(n) {
		return ""
	}
	var b strings.Builder
	var walk func(*sitter.Node)
	walk = func(n *sitter.Node) {
		if !nodeOK(n) {
			return
		}
		if n.Type() == "string_content" {
			b.WriteString(n.Content([]byte(src)))
			return
		}
		if n.NamedChildCount() == 0 && n.Type() != "string_start" && n.Type() != "string_end" {
			return
		}
		for i := 0; i < int(n.NamedChildCount()); i++ {
			walk(n.NamedChild(i))
		}
	}
	walk(n)
	if b.Len() > 0 {
		return b.String()
	}
	t := n.Content([]byte(src))
	t = strings.Trim(t, `"'`)
	return t
}

func pyStringList(n *sitter.Node, src string) []string {
	if !nodeOK(n) {
		return nil
	}
	if n.Type() == "parenthesized_expression" && n.NamedChildCount() == 1 {
		return pyStringList(n.NamedChild(0), src)
	}
	if n.Type() != "list" {
		return nil
	}
	var out []string
	for i := 0; i < int(n.NamedChildCount()); i++ {
		c := n.NamedChild(i)
		if !nodeOK(c) || (c.Type() != "string" && c.Type() != "concatenated_string") {
			return nil
		}
		out = append(out, pyString(c, src))
	}
	return out
}

func firstLineText(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	return strings.TrimSpace(s)
}

func bound(sc *scope, name string) bool {
	return sc != nil && sc.binds != nil && func() bool { _, ok := sc.binds[name]; return ok }()
}

func firstDot(s string) string {
	if i := strings.IndexByte(s, '.'); i >= 0 {
		return s[:i]
	}
	return s
}

func lastDot(s string) string {
	if i := strings.LastIndexByte(s, '.'); i >= 0 {
		return s[i+1:]
	}
	return s
}
