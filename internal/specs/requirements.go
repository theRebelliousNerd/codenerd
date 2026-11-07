package specs

import (
	"fmt"
	"strings"

	"codenerd/internal/mangle"

	"codeberg.org/TauCeti/mangle-go/ast"
)

// SpecDirs are the workspace directories whose *.mg files are machine-checkable
// specs, relative to the workspace root.
var SpecDirs = []string{"Docs/specs", ".nerd/specs"}

// Requirement is one stated obligation: an id and what it says.
type Requirement struct {
	ID   string // "/dred_retract_rederives"
	Text string
	File string
}

// Requires demands one Property of the element CodeRef.
type Requires struct {
	ID       string
	CodeRef  string
	Property Property
	File     string
}

// RequiresCalls demands that the element CodeRef call the element Other.
type RequiresCalls struct {
	ID      string
	CodeRef string
	Other   string
	File    string
}

// SpecFile is one parsed spec file.
type SpecFile struct {
	Path         string
	Requirements []Requirement
	Requires     []Requires
	Calls        []RequiresCalls
}

// ParseSpecFile reads a spec file: ground facts of spec_requirement/2,
// requires/3 and requires_calls/3, and nothing else. A spec file states
// obligations; it cannot declare a predicate or derive anything, so it can
// never change what the kernel decides outside spec checking. A file with any
// refused statement is refused whole, and every refusal is named: half a
// contract would check some obligations and silently drop the rest.
func ParseSpecFile(path, src string) (*SpecFile, error) {
	unit, err := mangle.ParseUnit(strings.NewReader(src))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	var problems []string
	refuse := func(format string, args ...any) { problems = append(problems, fmt.Sprintf(format, args...)) }
	for _, d := range unit.Decls {
		// The parser reports the implicit Package() of every unit; only a
		// predicate the file declares itself is refused.
		if sym := d.DeclaredAtom.Predicate.Symbol; sym != "Package" && sym != "Use" {
			refuse("a spec file declares no predicates (it declares %s)", sym)
		}
	}
	f := &SpecFile{Path: path}
	described := map[string]bool{}
	for _, c := range unit.Clauses {
		if len(c.Premises) > 0 || c.Transform != nil {
			refuse("%s is a rule: a spec file holds facts only", c.Head.Predicate.Symbol)
			continue
		}
		atom := c.Head
		switch atom.Predicate.Symbol {
		case "spec_requirement":
			id, text, ok := twoArgs(atom)
			if !ok {
				refuse("spec_requirement takes (Id /name, Text string): %s", atom)
				continue
			}
			described[id] = true
			f.Requirements = append(f.Requirements, Requirement{ID: id, Text: text, File: path})
		case "requires":
			if len(atom.Args) != 3 {
				refuse("requires takes (Id /name, CodeRef string, Property /name): %s", atom)
				continue
			}
			id, ok1 := nameArg(atom.Args[0])
			ref, ok2 := stringArg(atom.Args[1])
			prop, ok3 := nameArg(atom.Args[2])
			if !ok1 || !ok2 || !ok3 {
				refuse("requires takes (Id /name, CodeRef string, Property /name): %s", atom)
				continue
			}
			if !knownProperty(prop) {
				refuse("requires %s: property %s is not one of %s (calls is requires_calls)", id, prop, propertyList())
				continue
			}
			f.Requires = append(f.Requires, Requires{ID: id, CodeRef: ref, Property: Property(strings.TrimPrefix(prop, "/")), File: path})
		case "requires_calls":
			if len(atom.Args) != 3 {
				refuse("requires_calls takes (Id /name, CodeRef string, Other string): %s", atom)
				continue
			}
			id, ok1 := nameArg(atom.Args[0])
			ref, ok2 := stringArg(atom.Args[1])
			other, ok3 := stringArg(atom.Args[2])
			if !ok1 || !ok2 || !ok3 {
				refuse("requires_calls takes (Id /name, CodeRef string, Other string): %s", atom)
				continue
			}
			f.Calls = append(f.Calls, RequiresCalls{ID: id, CodeRef: ref, Other: other, File: path})
		default:
			refuse("%s is not a spec predicate (spec_requirement, requires, requires_calls)", atom.Predicate.Symbol)
		}
	}
	for _, r := range f.Requires {
		if !described[r.ID] {
			refuse("requires names %s, which has no spec_requirement in this file", r.ID)
		}
	}
	for _, r := range f.Calls {
		if !described[r.ID] {
			refuse("requires_calls names %s, which has no spec_requirement in this file", r.ID)
		}
	}
	if len(problems) > 0 {
		return nil, fmt.Errorf("%s is refused whole: %s", path, strings.Join(problems, "; "))
	}
	return f, nil
}

func twoArgs(a ast.Atom) (id, text string, ok bool) {
	if len(a.Args) != 2 {
		return "", "", false
	}
	id, ok1 := nameArg(a.Args[0])
	text, ok2 := stringArg(a.Args[1])
	return id, text, ok1 && ok2
}

func nameArg(t ast.BaseTerm) (string, bool) {
	c, ok := t.(ast.Constant)
	if !ok || c.Type != ast.NameType {
		return "", false
	}
	return c.Symbol, true
}

func stringArg(t ast.BaseTerm) (string, bool) {
	c, ok := t.(ast.Constant)
	if !ok || c.Type != ast.StringType {
		return "", false
	}
	return c.Symbol, true
}

func knownProperty(name string) bool {
	for _, p := range Properties {
		if "/"+string(p) == name {
			return true
		}
	}
	return false
}

func propertyList() string {
	names := make([]string, len(Properties))
	for i, p := range Properties {
		names[i] = "/" + string(p)
	}
	return strings.Join(names, ", ")
}
