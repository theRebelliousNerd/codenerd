package world

import (
	"context"
	"fmt"
	"os"
	"path"
	"strings"
	"sync"

	"codenerd/internal/core"
	"codenerd/internal/logging"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/javascript"
	"github.com/smacker/go-tree-sitter/python"
	"github.com/smacker/go-tree-sitter/rust"
	"github.com/smacker/go-tree-sitter/typescript/typescript"
)

// Deep (holographic) mapping for the non-Go languages.
//
// MapFile used to return (nil, nil) for anything but .go, so the whole deep
// layer — code_defines / code_calls, and with them the call-graph section of
// holographic context, impact priorities and the "who calls this" review
// evidence — simply did not exist for Python, TypeScript, JavaScript or Rust,
// even though the fast scanner already emitted symbol_graph for all of them and
// the data-flow extractor already supported all of them.
//
// The fast scanner's symbol_graph cannot substitute: it carries no line range,
// so nothing can slice a function body out of a file, and it records no call
// edges at all.

// deepMappableExt reports whether the Cartographer can deep-map a file, i.e.
// whether MapFile will produce code_defines/code_calls for it.
func deepMappableExt(ext string) bool {
	switch strings.ToLower(ext) {
	case ".go", ".py", ".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs", ".rs":
		return true
	}
	return false
}

// tsParserSet lazily holds one tree-sitter parser per language. Parsers are not
// safe for concurrent use, so a Cartographer owns its own set; deep scans
// already construct one Cartographer per file.
type tsParserSet struct {
	mu      sync.Mutex
	parsers map[string]*sitter.Parser
}

func (s *tsParserSet) get(lang string) *sitter.Parser {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.parsers == nil {
		s.parsers = make(map[string]*sitter.Parser, 4)
	}
	if p, ok := s.parsers[lang]; ok {
		return p
	}
	p := sitter.NewParser()
	switch lang {
	case "python":
		p.SetLanguage(python.GetLanguage())
	case "javascript":
		p.SetLanguage(javascript.GetLanguage())
	case "typescript":
		p.SetLanguage(typescript.GetLanguage())
	case "rust":
		p.SetLanguage(rust.GetLanguage())
	default:
		p.Close()
		return nil
	}
	s.parsers[lang] = p
	return p
}

func (s *tsParserSet) close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, p := range s.parsers {
		p.Close()
	}
	s.parsers = nil
}

// mapNonGoFile emits code_defines/code_calls for a supported non-Go language.
// fsPath is read from disk; factPath is the identity written into the facts.
func (c *Cartographer) mapNonGoFile(fsPath, factPath, lang string) ([]core.Fact, error) {
	content, err := os.ReadFile(fsPath)
	if err != nil {
		return nil, err
	}
	parser := c.parsers.get(lang)
	if parser == nil {
		return nil, nil
	}
	tree, err := parser.ParseCtx(context.Background(), nil, content)
	if err != nil {
		logging.Get(logging.CategoryWorld).Error("Cartographer: %s parse failed: %s - %v", lang, fsPath, err)
		return nil, err
	}
	defer tree.Close()

	m := &deepMapper{
		lang:    lang,
		file:    factPath,
		module:  moduleIDForPath(factPath),
		content: content,
	}
	m.walk(tree.RootNode(), "")

	facts := m.facts
	// Data flow is an enhancement: a failure there must not lose the symbols.
	if c.dataFlowExtractor != nil {
		if dataFlow, dfErr := c.dataFlowExtractor.ExtractDataFlowForLanguage(fsPath, lang); dfErr == nil {
			facts = append(facts, dataFlow...)
		} else {
			logging.WorldDebug("Cartographer: data flow extraction failed for %s: %v (continuing with symbol facts only)", fsPath, dfErr)
		}
	}
	logging.WorldDebug("Cartographer: deep-mapped %s file %s - %d facts", lang, factPath, len(facts))
	return facts, nil
}

// moduleIDForPath is the qualifier used to build symbol IDs, mirroring the Go
// mapper's use of the package name: "widget.render" for widget.py. Without a
// qualifier, `render` in two files would collide into one call-graph node.
func moduleIDForPath(p string) string {
	base := path.Base(cleanSlash(p))
	if i := strings.Index(base, "."); i > 0 {
		base = base[:i]
	}
	return base
}

// deepMapper walks one file's AST accumulating definition and call facts.
type deepMapper struct {
	lang    string
	file    string
	module  string
	content []byte
	facts   []core.Fact
}

func (m *deepMapper) text(n *sitter.Node) string {
	if n == nil {
		return ""
	}
	return n.Content(m.content)
}

func (m *deepMapper) define(name, kind string, n *sitter.Node) string {
	if name == "" {
		return ""
	}
	id := m.module + "." + name
	m.facts = append(m.facts, core.Fact{
		Predicate: "code_defines",
		Args: []any{
			m.file,
			core.MangleAtom(id),
			core.MangleAtom(kind),
			int64(n.StartPoint().Row) + 1,
			int64(n.EndPoint().Row) + 1,
		},
	})
	return id
}

func (m *deepMapper) call(caller, callee string) {
	if caller == "" || callee == "" {
		return
	}
	m.facts = append(m.facts, core.Fact{
		Predicate: "code_calls",
		Args:      []any{core.MangleAtom(caller), core.MangleAtom(callee)},
	})
}

// walk descends the tree carrying the enclosing definition, so a call is
// attributed to the function that contains it rather than to whatever
// definition the walker happened to see last (the Go mapper's ast.Inspect
// closure has that flaw: a call in a var initializer after a func body is
// credited to that func).
func (m *deepMapper) walk(n *sitter.Node, enclosing string) {
	if n == nil {
		return
	}
	current := enclosing

	switch m.lang {
	case "python":
		switch n.Type() {
		case "function_definition":
			current = m.define(m.text(n.ChildByFieldName("name")), "/function", n)
		case "class_definition":
			current = m.define(m.text(n.ChildByFieldName("name")), "/class", n)
		case "call":
			m.call(enclosing, m.calleeName(n.ChildByFieldName("function")))
		}
	case "javascript", "typescript":
		switch n.Type() {
		case "function_declaration", "generator_function_declaration", "function_expression":
			current = m.define(m.text(n.ChildByFieldName("name")), "/function", n)
		case "method_definition":
			current = m.define(m.text(n.ChildByFieldName("name")), "/method", n)
		case "class_declaration":
			current = m.define(m.text(n.ChildByFieldName("name")), "/class", n)
		case "interface_declaration":
			current = m.define(m.text(n.ChildByFieldName("name")), "/interface", n)
		case "type_alias_declaration":
			m.define(m.text(n.ChildByFieldName("name")), "/type", n)
		case "variable_declarator":
			// const handler = (req) => {...} is the dominant way modern JS/TS
			// declares functions; without this the call graph is mostly empty.
			if v := n.ChildByFieldName("value"); v != nil {
				switch v.Type() {
				case "arrow_function", "function_expression", "function":
					current = m.define(m.text(n.ChildByFieldName("name")), "/function", n)
				}
			}
		case "call_expression":
			m.call(enclosing, m.calleeName(n.ChildByFieldName("function")))
		}
	case "rust":
		switch n.Type() {
		case "function_item":
			current = m.define(m.text(n.ChildByFieldName("name")), "/function", n)
		case "struct_item":
			m.define(m.text(n.ChildByFieldName("name")), "/struct", n)
		case "enum_item":
			m.define(m.text(n.ChildByFieldName("name")), "/enum", n)
		case "trait_item":
			m.define(m.text(n.ChildByFieldName("name")), "/interface", n)
		case "call_expression":
			m.call(enclosing, m.calleeName(n.ChildByFieldName("function")))
		case "macro_invocation":
			if macro := m.text(n.ChildByFieldName("macro")); macro != "" {
				m.call(enclosing, macro+"!")
			}
		}
	}

	for i := 0; i < int(n.ChildCount()); i++ {
		m.walk(n.Child(i), current)
	}
}

// calleeName renders a call target. Qualified targets (obj.method, mod::func)
// keep their qualifier; a bare name is qualified with the current module so it
// can match the code_defines ID emitted for a definition in the same file.
func (m *deepMapper) calleeName(fn *sitter.Node) string {
	if fn == nil {
		return ""
	}
	switch fn.Type() {
	case "identifier", "field_identifier", "property_identifier":
		return m.module + "." + m.text(fn)
	case "attribute", "member_expression":
		obj := fn.ChildByFieldName("object")
		prop := fn.ChildByFieldName("attribute")
		if prop == nil {
			prop = fn.ChildByFieldName("property")
		}
		if obj != nil && prop != nil {
			return fmt.Sprintf("%s.%s", lastSegment(m.text(obj)), m.text(prop))
		}
	case "scoped_identifier":
		p := m.text(fn.ChildByFieldName("path"))
		name := m.text(fn.ChildByFieldName("name"))
		if p != "" && name != "" {
			return fmt.Sprintf("%s.%s", lastSegment(p), name)
		}
		return name
	case "field_expression":
		val := m.text(fn.ChildByFieldName("value"))
		field := m.text(fn.ChildByFieldName("field"))
		if field != "" {
			return fmt.Sprintf("%s.%s", lastSegment(val), field)
		}
	}
	// Computed or otherwise unnameable target (obj[key](), (await f)()):
	// no stable node name exists, so no edge is emitted rather than a
	// fabricated one.
	return ""
}

// lastSegment reduces a receiver expression to its final name so `a.b.c.run`
// and `self.c.run` both attribute to `c.run`.
func lastSegment(s string) string {
	s = strings.TrimSpace(s)
	for _, sep := range []string{"::", "."} {
		if i := strings.LastIndex(s, sep); i >= 0 {
			s = s[i+len(sep):]
		}
	}
	if i := strings.LastIndexAny(s, "()[]{} \t"); i >= 0 {
		s = s[i+1:]
	}
	return s
}
