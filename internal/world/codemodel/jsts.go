package codemodel

import (
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
)

func (s *scrape) walkJS(n *sitter.Node) {
	if !nodeOK(n) {
		return
	}
	switch n.Type() {
	case "program":
		for i := 0; i < int(n.NamedChildCount()); i++ {
			s.walkJS(n.NamedChild(i))
		}
	case "import_statement":
		s.jsImport(n)
	case "export_statement":
		s.jsExport(n)
	case "lexical_declaration", "variable_declaration":
		s.jsLexical(n, nil)
	case "function_declaration", "generator_function_declaration":
		s.jsFunc(n, s.declSpan(n), false)
	case "class_declaration":
		s.jsClass(n, s.declSpan(n))
	case "interface_declaration":
		s.jsInterface(n, s.declSpan(n))
	case "type_alias_declaration":
		s.jsTypeAlias(n, s.declSpan(n))
	case "enum_declaration":
		s.jsEnum(n, s.declSpan(n))
	case "internal_module":
		s.jsNamespace(n, s.declSpan(n))
	case "function_expression", "generator_function":
		s.jsFuncExpr(n)
	case "arrow_function":
		s.jsArrow(n, nil, "")
	case "class_body":
		s.inClass++
		for i := 0; i < int(n.NamedChildCount()); i++ {
			s.walkJS(n.NamedChild(i))
		}
		s.inClass--
	case "method_definition":
		s.jsMethod(n)
	case "public_field_definition":
		s.jsField(n)
	case "statement_block":
		s.push(scopeBlock)
		for i := 0; i < int(n.NamedChildCount()); i++ {
			s.walkJS(n.NamedChild(i))
		}
		s.pop()
	case "for_statement", "for_in_statement":
		s.push(scopeBlock)
		for i := 0; i < int(n.NamedChildCount()); i++ {
			s.walkJS(n.NamedChild(i))
		}
		s.pop()
	case "catch_clause":
		s.jsCatch(n)
	case "call_expression":
		s.jsCall(n)
	case "new_expression":
		s.jsCall(n)
	case "member_expression":
		s.jsMember(n, false, false)
	case "jsx_element":
		for i := 0; i < int(n.NamedChildCount()); i++ {
			s.walkJS(n.NamedChild(i))
		}
	case "jsx_opening_element", "jsx_self_closing_element":
		s.jsJSX(n, true)
	case "jsx_closing_element":
		s.jsJSX(n, false)
	case "jsx_attribute":
		s.walkJS(field(n, "value"))
	case "jsx_expression":
		for i := 0; i < int(n.NamedChildCount()); i++ {
			s.walkJS(n.NamedChild(i))
		}
	case "pair":
		s.walkJS(field(n, "value"))
	case "shorthand_property_identifier", "shorthand_property_identifier_pattern":
		s.addLoad(n, false, false, false)
	case "identifier", "type_identifier":
		s.addLoad(n, false, false, false)
	case "nested_type_identifier":
		s.jsNestedType(n)
	case "property_identifier", "private_property_identifier", "predefined_type", "string", "string_fragment", "number", "comment", "regex":
		return
	case "template_string":
		for i := 0; i < int(n.NamedChildCount()); i++ {
			c := n.NamedChild(i)
			if nodeOK(c) && c.Type() == "template_substitution" {
				s.walkJS(c)
			}
		}
	default:
		for i := 0; i < int(n.NamedChildCount()); i++ {
			s.walkJS(n.NamedChild(i))
		}
	}
}

func (s *scrape) declSpan(n *sitter.Node) *sitter.Node {
	p := n.Parent()
	if nodeOK(p) && p.Type() == "export_statement" {
		d := field(p, "declaration")
		if d == nil || sameSpan(d, n) {
			return p
		}
	}
	return n
}

func sameSpan(a, b *sitter.Node) bool {
	return nodeOK(a) && nodeOK(b) && a.StartByte() == b.StartByte() && a.EndByte() == b.EndByte()
}

func (s *scrape) jsImport(n *sitter.Node) {
	src := unquoteJS(field(n, "source"), s.src)
	file := resolveTSImport(s.abs, s.root, src)
	resolved := ""
	if file != "" {
		resolved = displayPath(s.root, file)
	}
	imp := Import{
		Spec: src, Path: src, Resolved: resolved,
		Start: int(n.StartByte()), End: int(n.EndByte()),
		Line: s.lineOf(int(n.StartByte())),
	}
	if resolved != "" {
		imp.Path = resolved
	}
	clause := field(n, "import")
	if !nodeOK(clause) {
		for i := 0; i < int(n.NamedChildCount()); i++ {
			c := n.NamedChild(i)
			if nodeOK(c) && c.Type() == "import_clause" {
				clause = c
				break
			}
		}
	}
	if nodeOK(clause) {
		s.jsImportClause(clause, resolved, &imp)
	}
	if imp.Name != "" || len(imp.Imported) > 0 || src != "" {
		s.imports = append(s.imports, imp)
	}
}

func (s *scrape) jsImportClause(n *sitter.Node, resolved string, imp *Import) {
	for i := 0; i < int(n.NamedChildCount()); i++ {
		c := n.NamedChild(i)
		if !nodeOK(c) {
			continue
		}
		switch c.Type() {
		case "identifier":
			s.bindJSImport(c, "default", true, resolved, imp)
		case "named_imports":
			for j := 0; j < int(c.NamedChildCount()); j++ {
				sp := c.NamedChild(j)
				if !nodeOK(sp) || sp.Type() != "import_specifier" {
					continue
				}
				s.jsImportSpecifier(sp, resolved, imp)
			}
		case "namespace_import":
			id := c.NamedChild(0)
			if field(c, "name") != nil {
				// no name field in this grammar; the identifier is the child
			}
			if !nodeOK(id) {
				for k := 0; k < int(c.ChildCount()); k++ {
					ch := c.Child(k)
					if nodeOK(ch) && ch.Type() == "identifier" {
						id = ch
						break
					}
				}
			}
			if nodeOK(id) && id.Type() == "identifier" {
				s.bindJSImport(id, "*", false, resolved, imp)
			}
		}
	}
}

func (s *scrape) jsImportSpecifier(n *sitter.Node, resolved string, imp *Import) {
	name := field(n, "name")
	alias := field(n, "alias")
	if !nodeOK(name) {
		return
	}
	remote := s.text(name)
	if nodeOK(alias) {
		s.bindJSImport(alias, remote, true, resolved, imp)
		s.aliasUses = append(s.aliasUses, aliasUse{
			name: remote, start: int(name.StartByte()), end: int(name.EndByte()),
			line: s.lineOf(int(name.StartByte())), column: s.colOf(int(name.StartByte())),
			file: resolved,
		})
		return
	}
	s.bindJSImport(name, remote, false, resolved, imp)
}

func (s *scrape) bindJSImport(n *sitter.Node, remote string, alias bool, resolved string, imp *Import) {
	local := s.text(n)
	if local == "" {
		return
	}
	idx := s.addBind(local, bindImport, int(n.StartByte()), int(n.EndByte()), s.module, -1)
	if idx >= 0 {
		s.binds[idx].importFile = resolved
		s.binds[idx].importName = remote
		s.binds[idx].alias = alias
	}
	imp.Imported = append(imp.Imported, ImportedName{Remote: remote, Local: local, Aliased: alias && remote != local && remote != "default" && remote != "*"})
	if imp.Name == "" {
		imp.Name = local
	}
}

func (s *scrape) jsExport(n *sitter.Node) {
	if src := field(n, "source"); nodeOK(src) {
		s.jsReexport(n, src)
		return
	}
	isDefault := s.keyword(n, "default")
	if d := field(n, "declaration"); nodeOK(d) {
		s.exporting = true
		s.exportDefault = isDefault
		s.walkJS(d)
		s.exporting = false
		s.exportDefault = false
		return
	}
	if v := field(n, "value"); nodeOK(v) {
		s.jsDefaultValue(n, v)
		return
	}
	s.jsExportClause(n, "")
}

func (s *scrape) jsReexport(n *sitter.Node, src *sitter.Node) {
	spec := unquoteJS(src, s.src)
	file := resolveTSImport(s.abs, s.root, spec)
	resolved := ""
	if file != "" {
		resolved = displayPath(s.root, file)
	}
	imp := Import{
		Spec: spec, Path: spec, Resolved: resolved,
		Start: int(n.StartByte()), End: int(n.EndByte()),
		Line: s.lineOf(int(n.StartByte())),
	}
	if resolved != "" {
		imp.Path = resolved
	}
	for i := 0; i < int(n.NamedChildCount()); i++ {
		c := n.NamedChild(i)
		if !nodeOK(c) {
			continue
		}
		if c.Type() == "export_clause" {
			for j := 0; j < int(c.NamedChildCount()); j++ {
				sp := c.NamedChild(j)
				if !nodeOK(sp) || sp.Type() != "export_specifier" {
					continue
				}
				name := field(sp, "name")
				alias := field(sp, "alias")
				if !nodeOK(name) {
					continue
				}
				remote := s.text(name)
				imp.Imported = append(imp.Imported, ImportedName{Remote: remote, Local: s.text(alias), Aliased: nodeOK(alias)})
				s.aliasUses = append(s.aliasUses, aliasUse{
					name: remote, start: int(name.StartByte()), end: int(name.EndByte()),
					line: s.lineOf(int(name.StartByte())), column: s.colOf(int(name.StartByte())),
					file: resolved,
				})
				if nodeOK(alias) && s.nsRecv == "" && !s.inFunction() {
					// `export { useAuth as ua } from "./hooks"` binds ua locally.
					s.addBind(s.text(alias), bindLocal, int(alias.StartByte()), int(alias.EndByte()), s.cur, -1)
				}
			}
		}
	}
	s.imports = append(s.imports, imp)
}

func (s *scrape) jsExportClause(n *sitter.Node, from string) {
	_ = from
	for i := 0; i < int(n.NamedChildCount()); i++ {
		c := n.NamedChild(i)
		if !nodeOK(c) || c.Type() != "export_clause" {
			continue
		}
		for j := 0; j < int(c.NamedChildCount()); j++ {
			sp := c.NamedChild(j)
			if !nodeOK(sp) || sp.Type() != "export_specifier" {
				continue
			}
			name := field(sp, "name")
			if nodeOK(name) {
				// A local re-export is a use of the local binding.
				s.addLoad(name, false, false, false)
			}
		}
	}
}

func (s *scrape) jsDefaultValue(exportNode, v *sitter.Node) {
	switch v.Type() {
	case "function_expression", "generator_function", "arrow_function":
		s.exporting = true
		s.exportDefault = true
		if v.Type() == "arrow_function" {
			s.jsArrow(v, exportNode, "default")
		} else {
			s.jsFunc(v, exportNode, false)
		}
		s.exporting = false
		s.exportDefault = false
	default:
		// `export default Identifier` is a use, not a declaration.
		s.walkJS(v)
	}
}

func (s *scrape) jsLexical(n *sitter.Node, spanOverride *sitter.Node) {
	kw := s.declKeyword(n)
	var decls []*sitter.Node
	for i := 0; i < int(n.NamedChildCount()); i++ {
		c := n.NamedChild(i)
		if nodeOK(c) && c.Type() == "variable_declarator" {
			decls = append(decls, c)
		}
	}
	exportNode := (*sitter.Node)(nil)
	if p := n.Parent(); nodeOK(p) && p.Type() == "export_statement" {
		exportNode = p
	}
	if spanOverride != nil {
		exportNode = spanOverride
	}
	for i, d := range decls {
		span := d
		if len(decls) == 1 && exportNode != nil {
			span = exportNode
		}
		_ = i
		s.jsDeclarator(d, kw, span, exportNode != nil)
	}
}

func (s *scrape) declKeyword(n *sitter.Node) string {
	for i := 0; i < int(n.ChildCount()); i++ {
		c := n.Child(i)
		if !nodeOK(c) || c.IsNamed() {
			continue
		}
		switch s.text(c) {
		case "var", "let", "const":
			return s.text(c)
		}
	}
	if n.Type() == "variable_declaration" {
		return "var"
	}
	return "const"
}

func (s *scrape) jsBindScope(keyword string) *scope {
	if keyword == "var" {
		for p := s.cur; p != nil; p = p.parent {
			switch p.kind {
			case scopeFunction, scopeModule, scopeNamespace:
				return p
			}
		}
	}
	return s.cur
}

func (s *scrape) jsDeclarator(d *sitter.Node, keyword string, span *sitter.Node, exported bool) {
	nameNode := field(d, "name")
	value := field(d, "value")
	if !nodeOK(nameNode) {
		return
	}
	if spec, ok := s.requireSpec(value); ok {
		s.jsRequire(nameNode, spec, d)
		return
	}
	if nameNode.Type() == "object_pattern" || nameNode.Type() == "array_pattern" {
		s.jsPattern(nameNode, s.jsBindScope(keyword))
		s.walkJS(value)
		return
	}
	if !nodeOK(nameNode) || (nameNode.Type() != "identifier" && nameNode.Type() != "type_identifier") {
		s.walkJS(value)
		return
	}
	name := s.text(nameNode)
	sc := s.jsBindScope(keyword)
	if s.isFuncValue(value) {
		innerName := s.text(field(value, "name"))
		if value.Type() == "arrow_function" || innerName == "" {
			s.jsArrow(value, span, name)
		} else {
			s.jsFunc(value, span, false)
		}
		// A nested arrow is not an element, but its declarator still binds
		// (`const useAuth = () => 1` inside a function shadows the import).
		if !bound(sc, name) {
			kind := bindLocal
			elem := -1
			if e := len(s.elements) - 1; e >= 0 && s.elements[e].Name == name && s.elements[e].NameStart == int(nameNode.StartByte()) {
				kind = bindDecl
				elem = e
			}
			s.addBind(name, kind, int(nameNode.StartByte()), int(nameNode.EndByte()), sc, elem)
		}
		return
	}
	elem := -1
	if s.emitHere() && s.inClass == 0 {
		kind := KindVar
		if keyword == "const" || kindOfAssign(name) == KindConst {
			kind = KindConst
		}
		start, doc := s.leadingDoc(span)
		e := Element{
			Name: name, Kind: kind, Start: start, End: int(span.EndByte()), Doc: doc,
			NameStart: int(nameNode.StartByte()), NameEnd: int(nameNode.EndByte()),
		}
		if exported {
			if s.exportDefault {
				e.defaultExport = true
			} else {
				e.namedExport = true
			}
		}
		s.place(&e, d)
		elem = len(s.elements)
		s.elements = append(s.elements, e)
	}
	kind := bindLocal
	if elem >= 0 {
		kind = bindDecl
	}
	s.addBind(name, kind, int(nameNode.StartByte()), int(nameNode.EndByte()), sc, elem)
	s.walkJS(value)
}

func (s *scrape) isFuncValue(n *sitter.Node) bool {
	if !nodeOK(n) {
		return false
	}
	switch n.Type() {
	case "arrow_function", "function_expression", "generator_function", "function":
		return true
	}
	return false
}

func (s *scrape) requireSpec(n *sitter.Node) (string, bool) {
	if !nodeOK(n) || n.Type() != "call_expression" {
		return "", false
	}
	fn := field(n, "function")
	if !nodeOK(fn) || fn.Type() != "identifier" || s.text(fn) != "require" {
		return "", false
	}
	args := field(n, "arguments")
	if !nodeOK(args) {
		return "", false
	}
	for i := 0; i < int(args.NamedChildCount()); i++ {
		c := args.NamedChild(i)
		if nodeOK(c) && (c.Type() == "string" || c.Type() == "template_string") {
			return unquoteJS(c, s.src), true
		}
	}
	return "", false
}

func (s *scrape) jsRequire(nameNode *sitter.Node, spec string, stmt *sitter.Node) {
	file := resolveTSImport(s.abs, s.root, spec)
	resolved := ""
	if file != "" {
		resolved = displayPath(s.root, file)
	}
	imp := Import{
		Spec: spec, Path: spec, Resolved: resolved,
		Start: int(stmt.StartByte()), End: int(stmt.EndByte()),
		Line: s.lineOf(int(stmt.StartByte())),
	}
	if resolved != "" {
		imp.Path = resolved
	}
	switch nameNode.Type() {
	case "identifier":
		s.bindJSImport(nameNode, "default", true, resolved, &imp)
	case "object_pattern":
		for i := 0; i < int(nameNode.NamedChildCount()); i++ {
			c := nameNode.NamedChild(i)
			if !nodeOK(c) {
				continue
			}
			switch c.Type() {
			case "shorthand_property_identifier_pattern", "identifier":
				s.bindJSImport(c, s.text(c), false, resolved, &imp)
			case "pair":
				key := field(c, "key")
				val := field(c, "value")
				if nodeOK(key) && nodeOK(val) {
					s.bindJSImport(val, s.text(key), true, resolved, &imp)
					s.aliasUses = append(s.aliasUses, aliasUse{
						name: s.text(key), start: int(key.StartByte()), end: int(key.EndByte()),
						line: s.lineOf(int(key.StartByte())), column: s.colOf(int(key.StartByte())),
						file: resolved,
					})
				}
			}
		}
	}
	s.imports = append(s.imports, imp)
}

func (s *scrape) jsPattern(n *sitter.Node, sc *scope) {
	if !nodeOK(n) {
		return
	}
	switch n.Type() {
	case "identifier":
		s.addBind(s.text(n), bindLocal, int(n.StartByte()), int(n.EndByte()), sc, -1)
	case "object_pattern", "array_pattern":
		for i := 0; i < int(n.NamedChildCount()); i++ {
			c := n.NamedChild(i)
			if !nodeOK(c) {
				continue
			}
			switch c.Type() {
			case "shorthand_property_identifier_pattern", "identifier":
				s.addBind(s.text(c), bindLocal, int(c.StartByte()), int(c.EndByte()), sc, -1)
			case "pair", "object_assignment_pattern", "assignment_pattern":
				if v := field(c, "value"); nodeOK(v) && v.Type() == "identifier" {
					s.addBind(s.text(v), bindLocal, int(v.StartByte()), int(v.EndByte()), sc, -1)
				} else if left := field(c, "left"); nodeOK(left) {
					s.jsPattern(left, sc)
				} else {
					s.jsPattern(c, sc)
				}
			case "rest_pattern":
				s.jsPattern(c.NamedChild(0), sc)
			default:
				s.jsPattern(c, sc)
			}
		}
	default:
		if n.NamedChildCount() == 1 {
			s.jsPattern(n.NamedChild(0), sc)
		}
	}
}

func (s *scrape) jsFunc(n, span *sitter.Node, method bool) {
	nameNode := field(n, "name")
	name := s.text(nameNode)
	if name == "" && s.exportDefault {
		name = "default"
	}
	recv := ""
	if method {
		recv = s.classRecv
	} else if s.nsRecv != "" && s.inClass == 0 && !s.inFunction() {
		recv = s.nsRecv
	}
	start, doc := s.leadingDoc(span)
	body := field(n, "body")
	elem := -1
	emit := name != "" && !s.inFunction() && (s.inClass == 0 || method)
	if method {
		emit = name != "" && s.inClass > 0 && !s.inFunction()
	}
	if emit {
		kind := KindFunction
		if method {
			kind = KindMethod
		}
		e := Element{
			Name: name, Receiver: recv, Kind: kind,
			Start: start, End: int(span.EndByte()), Doc: doc,
		}
		if nodeOK(nameNode) {
			e.NameStart, e.NameEnd = int(nameNode.StartByte()), int(nameNode.EndByte())
		}
		if containsJSX(body) {
			e.Role = "jsx"
		}
		s.markExport(&e)
		if method {
			e.Exported = !s.memberPrivate(n, name)
			e.namedExport, e.defaultExport = false, false
		}
		s.place(&e, n)
		elem = len(s.elements)
		s.elements = append(s.elements, e)
	}
	if name != "" && name != "default" || (name == "default" && elem >= 0 && nodeOK(nameNode)) {
		if name != "" && !(name == "default" && !nodeOK(nameNode)) {
			kind := bindLocal
			sc := s.cur
			switch {
			case method && elem >= 0:
				kind = bindMethod
			case elem >= 0:
				kind = bindDecl
			}
			startB, endB := 0, 0
			if nodeOK(nameNode) {
				startB, endB = int(nameNode.StartByte()), int(nameNode.EndByte())
			}
			idx := s.addBind(name, kind, startB, endB, sc, elem)
			if method && recv != "" {
				s.members[recv+"\x00"+name] = idx
			}
			if !method && recv != "" && elem >= 0 {
				s.members[recv+"\x00"+name] = idx
			}
		}
	}
	fnKind := scopeFunction
	s.push(fnKind)
	s.jsParams(field(n, "parameters"))
	if rt := field(n, "return_type"); nodeOK(rt) {
		s.walkJS(rt)
	}
	s.walkJS(body)
	s.pop()
}

func (s *scrape) jsFuncExpr(n *sitter.Node) {
	// A function expression used as a value. If it is named, the name is
	// local to the function. Not an element: the declarator walker emits
	// those.
	s.push(scopeFunction)
	if name := field(n, "name"); nodeOK(name) && name.Type() == "identifier" {
		s.addBind(s.text(name), bindLocal, int(name.StartByte()), int(name.EndByte()), s.cur, -1)
	}
	s.jsParams(field(n, "parameters"))
	s.walkJS(field(n, "body"))
	s.pop()
}

func (s *scrape) jsArrow(n, span *sitter.Node, name string) {
	if span == nil {
		span = n
	}
	body := field(n, "body")
	elem := -1
	if name != "" && !s.inFunction() && s.inClass == 0 {
		start, doc := s.leadingDoc(span)
		e := Element{
			Name: name, Kind: KindFunction,
			Start: start, End: int(span.EndByte()), Doc: doc,
		}
		// The name lives on the declarator, which is outside `n` but inside
		// span. Find it as an identifier whose text is the name.
		if id := identNamed(span, name, s.src); id != nil {
			e.NameStart, e.NameEnd = int(id.StartByte()), int(id.EndByte())
		}
		if containsJSX(body) || containsJSX(n) {
			e.Role = "jsx"
		}
		s.markExport(&e)
		s.place(&e, n)
		if e.Signature == "" {
			e.Signature = firstLine(s.src, e.Start, e.End)
		}
		elem = len(s.elements)
		s.elements = append(s.elements, e)
		if e.NameStart > 0 {
			idx := s.addBind(name, bindDecl, e.NameStart, e.NameEnd, s.jsBindScope("const"), elem)
			_ = idx
		}
	}
	s.push(scopeArrow)
	if p := field(n, "parameters"); nodeOK(p) {
		s.jsParams(p)
	} else if p := field(n, "parameter"); nodeOK(p) {
		s.jsPattern(p, s.cur)
	}
	s.walkJS(body)
	s.pop()
}

func (s *scrape) markExport(e *Element) {
	if !s.exporting {
		return
	}
	if s.exportDefault {
		e.defaultExport = true
	} else if s.nsRecv == "" {
		e.namedExport = true
	}
}

func identNamed(n *sitter.Node, name, src string) *sitter.Node {
	if !nodeOK(n) {
		return nil
	}
	if n.Type() == "identifier" && n.Content([]byte(src)) == name {
		return n
	}
	for i := 0; i < int(n.NamedChildCount()); i++ {
		if id := identNamed(n.NamedChild(i), name, src); id != nil {
			return id
		}
	}
	return nil
}

func (s *scrape) jsClass(n, span *sitter.Node) {
	nameNode := field(n, "name")
	name := s.text(nameNode)
	if name == "" && s.exportDefault {
		name = "default"
	}
	prefix := s.classRecv
	if prefix == "" {
		prefix = s.nsRecv
	}
	recv := ""
	if prefix != "" && s.inClass > 0 {
		recv = prefix
	} else if s.nsRecv != "" && s.inClass == 0 {
		recv = s.nsRecv
	}
	start, doc := s.leadingDoc(span)
	elem := -1
	if name != "" && !s.inFunction() {
		e := Element{
			Name: name, Receiver: recv, Kind: KindClass,
			Start: start, End: int(span.EndByte()), Doc: doc,
		}
		if nodeOK(nameNode) {
			e.NameStart, e.NameEnd = int(nameNode.StartByte()), int(nameNode.EndByte())
		}
		s.markExport(&e)
		if recv != "" && s.inClass > 0 {
			e.Exported = !s.memberPrivate(n, name)
			e.namedExport, e.defaultExport = false, false
		}
		s.place(&e, n)
		elem = len(s.elements)
		s.elements = append(s.elements, e)
	}
	if name != "" && nodeOK(nameNode) {
		kind := bindLocal
		if elem >= 0 {
			kind = bindDecl
		}
		idx := s.addBind(name, kind, int(nameNode.StartByte()), int(nameNode.EndByte()), s.cur, elem)
		full := name
		if recv != "" {
			full = recv + "." + name
			s.members[recv+"\x00"+name] = idx
		}
		_ = full
	}
	prev := s.classRecv
	full := name
	if recv != "" && name != "" {
		full = recv + "." + name
	} else if prev != "" && name != "" {
		full = prev + "." + name
	}
	if name != "" {
		s.classRecv = full
	}
	s.walkJS(field(n, "body"))
	s.classRecv = prev
}

func (s *scrape) jsMethod(n *sitter.Node) {
	if s.inClass == 0 || s.inFunction() {
		s.push(scopeFunction)
		s.jsParams(field(n, "parameters"))
		s.walkJS(field(n, "body"))
		s.pop()
		return
	}
	s.jsFunc(n, n, true)
}

func (s *scrape) jsField(n *sitter.Node) {
	if s.inClass == 0 {
		s.walkJS(field(n, "value"))
		return
	}
	nameNode := field(n, "name")
	value := field(n, "value")
	name := s.text(nameNode)
	if s.isFuncValue(value) && name != "" && !s.inFunction() {
		// A class field holding a function is addressed as a method.
		start, doc := s.leadingDoc(n)
		e := Element{
			Name: name, Receiver: s.classRecv, Kind: KindMethod,
			Start: start, End: int(n.EndByte()), Doc: doc,
			Exported: !s.memberPrivate(n, name),
		}
		if nodeOK(nameNode) {
			e.NameStart, e.NameEnd = int(nameNode.StartByte()), int(nameNode.EndByte())
		}
		if containsJSX(value) {
			e.Role = "jsx"
		}
		s.place(&e, n)
		elem := len(s.elements)
		s.elements = append(s.elements, e)
		idx := s.addBind(name, bindMethod, e.NameStart, e.NameEnd, s.cur, elem)
		if s.classRecv != "" {
			s.members[s.classRecv+"\x00"+name] = idx
		}
		s.push(scopeArrow)
		s.walkJS(value)
		s.pop()
		return
	}
	s.walkJS(value)
}

func (s *scrape) memberPrivate(n *sitter.Node, name string) bool {
	if strings.HasPrefix(name, "#") {
		return true
	}
	for i := 0; i < int(n.ChildCount()); i++ {
		c := n.Child(i)
		if !nodeOK(c) {
			continue
		}
		if c.Type() == "accessibility_modifier" {
			switch s.text(c) {
			case "private", "protected":
				return true
			}
		}
		if c.Type() == "private_property_identifier" {
			return true
		}
	}
	return false
}

func (s *scrape) jsInterface(n, span *sitter.Node) {
	nameNode := field(n, "name")
	name := s.text(nameNode)
	if name == "" || s.inFunction() {
		s.walkJS(field(n, "body"))
		return
	}
	start, doc := s.leadingDoc(span)
	e := Element{
		Name: name, Kind: KindInterface,
		Start: start, End: int(span.EndByte()), Doc: doc,
	}
	if nodeOK(nameNode) {
		e.NameStart, e.NameEnd = int(nameNode.StartByte()), int(nameNode.EndByte())
		s.addBind(name, bindDecl, e.NameStart, e.NameEnd, s.cur, len(s.elements))
	}
	s.markExport(&e)
	s.place(&e, n)
	s.elements = append(s.elements, e)
	s.walkTypeBody(field(n, "body"))
}

func (s *scrape) jsTypeAlias(n, span *sitter.Node) {
	nameNode := field(n, "name")
	name := s.text(nameNode)
	if name == "" || s.inFunction() {
		s.walkJS(field(n, "value"))
		return
	}
	start, doc := s.leadingDoc(span)
	e := Element{
		Name: name, Kind: KindType,
		Start: start, End: int(span.EndByte()), Doc: doc,
	}
	if nodeOK(nameNode) {
		e.NameStart, e.NameEnd = int(nameNode.StartByte()), int(nameNode.EndByte())
		s.addBind(name, bindDecl, e.NameStart, e.NameEnd, s.cur, len(s.elements))
	}
	s.markExport(&e)
	s.place(&e, n)
	s.elements = append(s.elements, e)
	s.walkJS(field(n, "value"))
}

func (s *scrape) jsEnum(n, span *sitter.Node) {
	nameNode := field(n, "name")
	name := s.text(nameNode)
	if name == "" || s.inFunction() {
		return
	}
	start, doc := s.leadingDoc(span)
	e := Element{
		Name: name, Kind: KindEnum,
		Start: start, End: int(span.EndByte()), Doc: doc,
	}
	if nodeOK(nameNode) {
		e.NameStart, e.NameEnd = int(nameNode.StartByte()), int(nameNode.EndByte())
		s.addBind(name, bindDecl, e.NameStart, e.NameEnd, s.cur, len(s.elements))
	}
	s.markExport(&e)
	s.place(&e, n)
	s.elements = append(s.elements, e)
	body := field(n, "body")
	if !nodeOK(body) {
		for i := 0; i < int(n.NamedChildCount()); i++ {
			c := n.NamedChild(i)
			if nodeOK(c) && c.Type() == "enum_body" {
				body = c
				break
			}
		}
	}
	if !nodeOK(body) {
		return
	}
	for i := 0; i < int(body.NamedChildCount()); i++ {
		c := body.NamedChild(i)
		if !nodeOK(c) {
			continue
		}
		if v := field(c, "value"); nodeOK(v) {
			s.walkJS(v)
		}
	}
}

func (s *scrape) jsNamespace(n, span *sitter.Node) {
	nameNode := field(n, "name")
	name := s.text(nameNode)
	if name == "" {
		s.walkJS(field(n, "body"))
		return
	}
	start, doc := s.leadingDoc(span)
	elem := -1
	if !s.inFunction() {
		recv := s.nsRecv
		e := Element{
			Name: name, Receiver: recv, Kind: KindNamespace,
			Start: start, End: int(span.EndByte()), Doc: doc,
		}
		if nodeOK(nameNode) {
			e.NameStart, e.NameEnd = int(nameNode.StartByte()), int(nameNode.EndByte())
		}
		s.markExport(&e)
		s.place(&e, n)
		elem = len(s.elements)
		s.elements = append(s.elements, e)
		idx := s.addBind(name, bindDecl, e.NameStart, e.NameEnd, s.cur, elem)
		if recv != "" {
			s.members[recv+"\x00"+name] = idx
		}
	}
	prev := s.nsRecv
	if prev == "" {
		s.nsRecv = name
	} else {
		s.nsRecv = prev + "." + name
	}
	s.push(scopeNamespace)
	s.walkJS(field(n, "body"))
	s.pop()
	s.nsRecv = prev
}

func (s *scrape) walkTypeBody(n *sitter.Node) {
	if !nodeOK(n) {
		return
	}
	for i := 0; i < int(n.NamedChildCount()); i++ {
		c := n.NamedChild(i)
		if !nodeOK(c) {
			continue
		}
		if t := field(c, "type"); nodeOK(t) {
			s.walkJS(t)
			continue
		}
		// method_signature / property_signature: walk type children only.
		for j := 0; j < int(c.NamedChildCount()); j++ {
			ch := c.NamedChild(j)
			if !nodeOK(ch) {
				continue
			}
			switch ch.Type() {
			case "property_identifier", "identifier":
				continue
			default:
				s.walkJS(ch)
			}
		}
	}
}

func (s *scrape) jsParams(n *sitter.Node) {
	if !nodeOK(n) {
		return
	}
	if n.Type() == "identifier" {
		s.addBind(s.text(n), bindParam, int(n.StartByte()), int(n.EndByte()), s.cur, -1)
		return
	}
	for i := 0; i < int(n.NamedChildCount()); i++ {
		c := n.NamedChild(i)
		if !nodeOK(c) {
			continue
		}
		s.jsParam(c)
	}
}

func (s *scrape) jsParam(n *sitter.Node) {
	if n.Type() == "identifier" {
		s.addBind(s.text(n), bindParam, int(n.StartByte()), int(n.EndByte()), s.cur, -1)
		return
	}
	pat := field(n, "pattern")
	if !nodeOK(pat) {
		pat = field(n, "name")
	}
	if !nodeOK(pat) {
		for i := 0; i < int(n.NamedChildCount()); i++ {
			c := n.NamedChild(i)
			if !nodeOK(c) {
				continue
			}
			if c.Type() == "identifier" || c.Type() == "object_pattern" || c.Type() == "array_pattern" || c.Type() == "rest_pattern" {
				pat = c
				break
			}
		}
	}
	if nodeOK(pat) {
		if pat.Type() == "identifier" {
			s.addBind(s.text(pat), bindParam, int(pat.StartByte()), int(pat.EndByte()), s.cur, -1)
		} else {
			s.jsPattern(pat, s.cur)
		}
	}
	if t := field(n, "type"); nodeOK(t) {
		s.walkJS(t)
	}
}

func (s *scrape) jsCatch(n *sitter.Node) {
	s.push(scopeCatch)
	if p := field(n, "parameter"); nodeOK(p) {
		s.jsPattern(p, s.cur)
	}
	s.walkJS(field(n, "body"))
	s.pop()
}

func (s *scrape) jsCall(n *sitter.Node) {
	fn := field(n, "function")
	if !nodeOK(fn) {
		fn = field(n, "constructor")
	}
	s.jsCallee(fn, true, false)
	s.walkJS(field(n, "arguments"))
	if t := field(n, "type_arguments"); nodeOK(t) {
		s.walkJS(t)
	}
}

func (s *scrape) jsCallee(n *sitter.Node, call, jsx bool) {
	if !nodeOK(n) {
		return
	}
	switch n.Type() {
	case "identifier":
		if jsx && isIntrinsicTag(s.text(n)) {
			return
		}
		s.addLoad(n, call, jsx, false)
	case "member_expression":
		s.jsMember(n, call, jsx)
	case "non_null_expression":
		if c := n.NamedChild(0); nodeOK(c) {
			s.jsCallee(c, call, jsx)
		}
	default:
		s.walkJS(n)
	}
}

func (s *scrape) jsMember(n *sitter.Node, call, jsx bool) {
	obj := field(n, "object")
	prop := field(n, "property")
	if nodeOK(obj) && (obj.Type() == "identifier" || obj.Type() == "type_identifier") {
		s.addLoad(obj, false, false, false)
		s.addProp(prop, obj, call, jsx)
		return
	}
	s.walkJS(obj)
	if nodeOK(prop) {
		s.idents[s.text(prop)]++
		if call {
			s.loads = append(s.loads, nameLoad{
				name: s.text(prop), start: int(prop.StartByte()), end: int(prop.EndByte()),
				line: s.lineOf(int(prop.StartByte())), column: s.colOf(int(prop.StartByte())),
				sc: s.cur, call: true, jsx: jsx, prop: true, qual: s.text(obj),
			})
		}
	}
}

func (s *scrape) jsJSX(n *sitter.Node, call bool) {
	name := field(n, "name")
	if !nodeOK(name) {
		return
	}
	switch name.Type() {
	case "identifier":
		if isIntrinsicTag(s.text(name)) {
			s.walkJSXAttrs(n)
			return
		}
		if call {
			s.addLoad(name, true, true, false)
		} else {
			s.addLoad(name, false, true, true)
		}
	case "member_expression":
		if call {
			s.jsMember(name, true, true)
		} else {
			s.jsMember(name, false, true)
			// closing member tag: mark the property load as closeTag.
			if len(s.loads) > 0 {
				last := &s.loads[len(s.loads)-1]
				last.closeTag = true
				last.call = false
			}
		}
	case "nested_identifier":
		s.jsNestedType(name)
	default:
		s.walkJS(name)
	}
	s.walkJSXAttrs(n)
}

func (s *scrape) walkJSXAttrs(n *sitter.Node) {
	for i := 0; i < int(n.NamedChildCount()); i++ {
		c := n.NamedChild(i)
		if nodeOK(c) && c.Type() == "jsx_attribute" {
			s.walkJS(field(c, "value"))
		}
	}
}

func (s *scrape) jsNestedType(n *sitter.Node) {
	mod := field(n, "module")
	name := field(n, "name")
	if nodeOK(mod) {
		s.addLoad(mod, false, false, false)
	}
	if nodeOK(name) {
		s.addProp(name, mod, false, false)
	}
}

func unquoteJS(n *sitter.Node, src string) string {
	if !nodeOK(n) {
		return ""
	}
	var b strings.Builder
	var found bool
	var walk func(*sitter.Node)
	walk = func(n *sitter.Node) {
		if !nodeOK(n) {
			return
		}
		if n.Type() == "string_fragment" {
			b.WriteString(n.Content([]byte(src)))
			found = true
			return
		}
		for i := 0; i < int(n.NamedChildCount()); i++ {
			walk(n.NamedChild(i))
		}
	}
	walk(n)
	if found {
		return b.String()
	}
	t := strings.TrimSpace(n.Content([]byte(src)))
	if len(t) >= 2 {
		q := t[0]
		if (q == '"' || q == '\'' || q == '`') && t[len(t)-1] == q {
			return t[1 : len(t)-1]
		}
	}
	return t
}
