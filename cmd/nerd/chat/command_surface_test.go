package chat

import (
	"go/ast"
	"go/parser"
	"go/token"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// dispatchedCommands returns every slash command handleCommand dispatches,
// read from the source so the test cannot drift from the switch it guards.
func dispatchedCommands(t *testing.T) map[string]bool {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "commands.go", nil, 0)
	if err != nil {
		t.Fatalf("parse commands.go: %v", err)
	}
	out := map[string]bool{}
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "handleCommand" || fn.Body == nil {
			continue
		}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			cc, ok := n.(*ast.CaseClause)
			if !ok {
				return true
			}
			for _, e := range cc.List {
				lit, ok := e.(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					continue
				}
				s, err := strconv.Unquote(lit.Value)
				if err == nil && strings.HasPrefix(s, "/") {
					out[s] = true
				}
			}
			return true
		})
	}
	if len(out) == 0 {
		t.Fatal("found no dispatched commands in handleCommand; the parser has drifted from the source")
	}
	return out
}

// TestCommandSurface_RegistryAndDispatcherAgree pins, in both directions, the contract the
// 2026-09-18 audit found broken in both directions: /shards, /autopoiesis and
// /facts were tested and neither registered nor dispatched, and /explain and
// /explain-off were dispatched but not registered, so /help and the switch
// disagreed about what a command was. Aliases count as registered when the
// registry declares them.
func TestCommandSurface_RegistryAndDispatcherAgree(t *testing.T) {
	dispatched := dispatchedCommands(t)
	registered := map[string]bool{}
	for _, c := range CommandRegistry {
		registered[c.Name] = true
		for _, a := range c.Aliases {
			registered[a] = true
		}
	}
	var missingDispatch, missingRegistry []string
	for name := range registered {
		if !dispatched[name] {
			missingDispatch = append(missingDispatch, name)
		}
	}
	for name := range dispatched {
		if !registered[name] {
			missingRegistry = append(missingRegistry, name)
		}
	}
	sort.Strings(missingDispatch)
	sort.Strings(missingRegistry)
	if len(missingDispatch) > 0 {
		t.Errorf("registered in CommandRegistry but handleCommand has no case for: %v", missingDispatch)
	}
	if len(missingRegistry) > 0 {
		t.Errorf("dispatched by handleCommand but absent from CommandRegistry (invisible to /help): %v", missingRegistry)
	}
}
