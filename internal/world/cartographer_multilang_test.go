package world

import (
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/core"
)

func mapFixture(t *testing.T, name, content string) []core.Fact {
	t.Helper()
	root := t.TempDir()
	full := filepath.Join(root, name)
	writeWorkspaceFile(t, root, name, content)

	c := NewCartographer()
	defer c.Close()
	facts, err := c.MapFileAs(full, name)
	if err != nil {
		t.Fatalf("MapFileAs(%s): %v", name, err)
	}
	return facts
}

func definitions(facts []core.Fact) map[string]string {
	out := map[string]string{}
	for _, f := range facts {
		if f.Predicate != "code_defines" || len(f.Args) < 5 {
			continue
		}
		id := f.Args[1].(string)
		out[id] = string(f.Args[2].(core.MangleAtom))
	}
	return out
}

func calls(facts []core.Fact) map[string][]string {
	out := map[string][]string{}
	for _, f := range facts {
		if f.Predicate != "code_calls" || len(f.Args) < 2 {
			continue
		}
		caller := f.Args[0].(string)
		callee := f.Args[1].(string)
		out[caller] = append(out[caller], callee)
	}
	return out
}

func hasCall(t *testing.T, facts []core.Fact, caller, callee string) {
	t.Helper()
	for _, c := range calls(facts)[caller] {
		if c == callee {
			return
		}
	}
	t.Errorf("missing code_calls(%s, %s); calls=%v", caller, callee, calls(facts))
}

// TestCartographer_WhenPythonFile_ShouldEmitDefinesAndCalls — MapFile used to
// return nothing but for .go, so the deep layer (call graph, line ranges,
// impact priorities) simply did not exist for any other language even though
// the fast scanner and the data-flow extractors both supported them.
func TestCartographer_WhenPythonFile_ShouldEmitDefinesAndCalls(t *testing.T) {
	facts := mapFixture(t, "svc.py", `
class Widget:
    def build(self):
        return helper()

def helper():
    return 1
`)
	defs := definitions(facts)
	if defs["svc.Widget"] != "/class" {
		t.Errorf("class definition missing or mistyped: %v", defs)
	}
	if defs["svc.build"] != "/function" {
		t.Errorf("method definition missing: %v", defs)
	}
	if defs["svc.helper"] != "/function" {
		t.Errorf("function definition missing: %v", defs)
	}
	hasCall(t, facts, "svc.build", "svc.helper")
}

// TestCartographer_WhenTypeScriptFile_ShouldAttributeCallsToEnclosingFunction —
// arrow-function consts are how modern TS declares most functions; missing them
// leaves the call graph empty.
func TestCartographer_WhenTypeScriptFile_ShouldEmitDefinesAndCalls(t *testing.T) {
	facts := mapFixture(t, "app.ts", `
export interface Config { name: string }

export class Service {
  start() { return boot(); }
}

export const handler = (req: string) => { return boot(); };

function boot() { return 1; }
`)
	defs := definitions(facts)
	for id, want := range map[string]string{
		"app.Config":  "/interface",
		"app.Service": "/class",
		"app.start":   "/method",
		"app.handler": "/function",
		"app.boot":    "/function",
	} {
		if defs[id] != want {
			t.Errorf("definition %s = %q, want %q (all: %v)", id, defs[id], want, defs)
		}
	}
	hasCall(t, facts, "app.start", "app.boot")
	hasCall(t, facts, "app.handler", "app.boot")
}

func TestCartographer_WhenRustFile_ShouldEmitDefinesAndCalls(t *testing.T) {
	facts := mapFixture(t, "lib.rs", `
pub struct Widget { pub id: u32 }

pub trait Render { fn render(&self); }

pub fn helper() -> u32 { 1 }

pub fn run() -> u32 { helper() }
`)
	defs := definitions(facts)
	for id, want := range map[string]string{
		"lib.Widget": "/struct",
		"lib.Render": "/interface",
		"lib.helper": "/function",
		"lib.run":    "/function",
	} {
		if defs[id] != want {
			t.Errorf("definition %s = %q, want %q (all: %v)", id, defs[id], want, defs)
		}
	}
	hasCall(t, facts, "lib.run", "lib.helper")
}

// TestCartographer_WhenCallOutsideAnyFunction_ShouldNotAttributeIt — a call in
// module scope belongs to no caller; inventing one poisons the call graph.
func TestCartographer_WhenCallOutsideAnyFunction_ShouldNotAttributeIt(t *testing.T) {
	facts := mapFixture(t, "top.py", `
def a():
    return 1

a()
`)
	for caller, callees := range calls(facts) {
		if caller == "" {
			t.Errorf("module-level call attributed to an empty caller: %v", callees)
		}
		if caller == "top.a" {
			t.Errorf("module-level call wrongly attributed to the preceding function: %v", callees)
		}
	}
}

// TestCartographer_WhenMappedAs_ShouldLabelFactsWithCanonicalPath — deep facts
// have to carry the same identity as file_topology, including the data-flow
// facts the extractor stamps with the path it was given to read.
func TestCartographer_WhenMappedAs_ShouldLabelFactsWithCanonicalPath(t *testing.T) {
	root := t.TempDir()
	writeWorkspaceFile(t, root, "internal/svc/run.go", "package svc\n\nfunc Run() error {\n\tvar e error\n\tif e != nil {\n\t\treturn e\n\t}\n\treturn nil\n}\n")

	c := NewCartographer()
	defer c.Close()
	facts, err := c.MapFileAs(filepath.Join(root, "internal", "svc", "run.go"), "internal/svc/run.go")
	if err != nil {
		t.Fatal(err)
	}
	if len(facts) == 0 {
		t.Fatal("no deep facts emitted")
	}
	for _, f := range facts {
		for _, a := range f.Args {
			if s, ok := a.(string); ok && filepath.IsAbs(s) {
				t.Errorf("fact %s carries the filesystem path %q instead of the canonical identity", f.Predicate, s)
			}
		}
	}
}

// TestCartographer_WhenNonGoFile_ShouldNotEmitRefSpelledCalls pins the Go-only
// scope of the dual fn:-prefixed code_calls rows: module-qualified IDs cannot
// reconstruct the py:/ts:/rs: refs (file path + parent qualification), so a
// dual row there would be a guess that joins nothing. See deepMapper.call.
func TestCartographer_WhenNonGoFile_ShouldNotEmitRefSpelledCalls(t *testing.T) {
	fixtures := map[string]string{
		"svc.py": "def f():\n    return g()\n\ndef g():\n    return 1\n",
		"app.ts": "function f() { return g(); }\nfunction g() { return 1; }\n",
		"lib.rs": "pub fn f() -> u32 { g() }\npub fn g() -> u32 { 1 }\n",
	}
	for name, src := range fixtures {
		t.Run(name, func(t *testing.T) {
			facts := mapFixture(t, name, src)
			seen := false
			for _, f := range facts {
				if f.Predicate != "code_calls" || len(f.Args) < 2 {
					continue
				}
				seen = true
				for _, a := range f.Args[:2] {
					if s, ok := a.(string); ok && strings.HasPrefix(s, "fn:") {
						t.Errorf("non-Go code_calls carries a Go ref spelling: %v", f.Args)
					}
				}
			}
			if !seen {
				t.Fatalf("no code_calls facts for %s; the pin is vacuous", name)
			}
		})
	}
}
