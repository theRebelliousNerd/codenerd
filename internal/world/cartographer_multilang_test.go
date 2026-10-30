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
// TODO: Gap 01: Null/Undefined/Empty: Empty File Inputs: Add test for empty file parsing to verify MapFileAs handles 0-byte or whitespace-only inputs without crashing.
// TODO: Gap 02: Null/Undefined/Empty: Empty Language Strings: Add test passing empty string as language.
// TODO: Gap 03: Null/Undefined/Empty: Missing File Permissions: Add test simulating os.ErrPermission.
// TODO: Gap 04: Null/Undefined/Empty: Whitespace-Only Files: Add test for parsing a file with only whitespaces.
// TODO: Gap 05: Type Coercion: Malformed Syntax Injection: Add test with invalid syntax injected.
// TODO: Gap 06: Type Coercion: Invalid UTF-8 Boundaries: Add test parsing invalid UTF-8 sequences.
// TODO: Gap 07: Type Coercion: Subtle Syntax Errors: Add test with missing closing parenthesis.
// TODO: Gap 08: User Request Extremes: Deep Recursion (Stack Overflow): Add test with 10,000+ levels of deeply nested functions/calls to verify tree walking does not cause a stack overflow.
// TODO: Gap 09: User Request Extremes: Massive Horizontal Breadth: Add test mapping a file with 1,000,000 flat functions.
// TODO: Gap 10: State Conflicts: Concurrent MapFileAs Access: Add test for concurrent MapFileAs calls on the same Cartographer to verify it correctly prevents race conditions.
// TODO: Gap 11: State Conflicts: Time-of-Check to Time-of-Use (TOCTOU): Add test for a file that is deleted between the time it is queued for mapping and when it is read.
// TODO: Gap 12: Null/Undefined/Empty: Nil Child Node Handling: Add test injecting nil child node into AST traversal.
// TODO: Gap 13: Type Coercion: Symlink Loop Exhaustion: Add test pointing mapNonGoFile to a symlink loop.
// TODO: Gap 14: Extremes: Extreme Identifier Length: Add test with a 1MB long function name.
// TODO: Gap 15: Type Coercion: Invalid Mangle Atoms: Add test verifying emission of malformed Mangle facts doesn't happen.
// TODO: Gap 16: State Conflicts: Symbolic Path Namespace Collisions: Add test for FILE.PY and file.py module ID collisions.
// TODO: Gap 17: User Request Extremes: Context Cancellation Timeouts: Add test simulating context timeout during AST walk.
// TODO: Gap 18: Null/Undefined/Empty: Context Cancelled before Walk: Add test where context is cancelled prior to recursive walk.
// TODO: Gap 19: Type Coercion: Single-Line Minified Files: Add test with 50MB single-line minified file.
// TODO: Gap 20: User Request Extremes: Fact ID Wrapper Wraparound: Add test with a 5-billion-line file to check Row overflow.
// TODO: Gap 21: Type Coercion: Duplicate Method Names: Add test with duplicate method names in the same class.
// TODO: Gap 22: Null/Undefined/Empty: Missing Name Nodes in AST: Add test verifying m.text(nil) behavior on missing name fields.
// TODO: Gap 23: State Conflicts: CGO Segfault Isolation: Add test simulating CGO boundary crash.
// TODO: Gap 24: Type Coercion: Unrecognized File Extensions in Bypass: Add test explicitly mapping binary files like .zip.
// TODO: Gap 25: User Request Extremes: Massive Pathological Regexes: Add test with ReDoS vulnerable regex.
// TODO: Gap 26: Null/Undefined/Empty: Anonymous Functions Edge Cases: Add test for anonymous functions in array assignments.
// TODO: Gap 27: State Conflicts: The Ouroboros Feedback Loop Failure: Add test verifying silent failure detection for malformed LLM tools.
// TODO: Gap 28: Type Coercion: Mixed Language TSX/JSX Parsing: Add test evaluating TSX syntax parsed with basic TS parser.
// TODO: Gap 29: Type Coercion: Cross-Language FFI Symbol Matching: Add test for Python calling Rust via FFI.
// TODO: Gap 30: Null/Undefined/Empty: DataFlow Extraction Panics: Add test mocking a panic inside ExtractDataFlowForLanguage.
// TODO: Gap 31: Null/Undefined/Empty: Dangling Symlinks: Add test where fsPath is a dangling symlink.
// TODO: Gap 32: User Request Extremes: Read timeout on network drives: Add test mocking a very slow filesystem read.
// TODO: Gap 33: Null/Undefined/Empty: lastSegment with empty strings: Add test verifying lastSegment behaves correctly with string of delimiters.
// TODO: Gap 34: Null/Undefined/Empty: moduleIDForPath with no extension: Add test where file path has no extension.
// TODO: Gap 35: User Request Extremes: moduleIDForPath with multiple extensions: Add test where file is named component.spec.ts.
// TODO: Gap 36: Type Coercion: Escaped Strings in Macros: Add test with rust macro containing escaped strings.
// TODO: Gap 37: State Conflicts: Tree-sitter ABI Version Mismatch: Add test mocking tree-sitter version mismatch.
// TODO: Gap 38: Type Coercion: Deeply nested object destructuring: Add test with deep object destructuring.
// TODO: Gap 39: Type Coercion: Generator Functions and yields: Add test with yield* yielding to another generator.
// TODO: Gap 40: Type Coercion: Optional Chaining resolution: Add test with TS optional chaining obj?.run().
// TODO: Gap 41: State Conflicts: Unclosed Tree-sitter instances: Add test simulating panicked caller failing to close Cartographer.
// TODO: Gap 42: Type Coercion: Handling of Windows Carriage Returns: Add test verifying line numbers for all 3 newline encodings.
// TODO: Gap 43: User Request Extremes: Extreme Indentation: Add test with extreme indentation to verify memory safety.
// TODO: Gap 44: Null/Undefined/Empty: Empty Node Iteration: Add test verifying int(n.ChildCount()) doesn't underflow.
// TODO: Gap 45: State Conflicts: File Deletion During Walk: Add test where file is deleted after m.walk but before dataflow extraction.
// TODO: Gap 46: Type Coercion: Unknown Language Requests: Add test explicitly mapping a cpp file to verify allowlist.
// TODO: Gap 47: Type Coercion: Non-ASCII Identifiers: Add test mapping multi-byte unicode identifiers.
// TODO: Gap 48: User Request Extremes: Pathological Number of Facts: Add stress test mapping file generating 1 million facts.
// TODO: Gap 49: Null/Undefined/Empty: Handling of Nil Pointers in Call Graph: Add test mapping call with space-only strings.
// TODO: Gap 50: Null/Undefined/Empty: Directory paths provided instead of file paths: Add test explicitly passing a directory path.
// TODO: Gap 51: Type Coercion: Mixed CR/LF combinations mid-file: Add test verifying line number metrics with mixed newlines.
// TODO: Gap 52: Null/Undefined/Empty: Empty module name returned by moduleIDForPath: Add test mapping hidden files like .py.
// TODO: Gap 53: State Conflicts: Closing Cartographer multiple times: Add test verifying idempotency of the Close method.
// TODO: Gap 54: Extremes: Very deep folder structures: Add test for OS path limits with 2000 character paths.
// TODO: Gap 55: Type Coercion: Symlinks pointing to binary files: Add test preventing binary analysis of /bin/bash.
// TODO: Gap 56: State Conflicts: Parser reuse across goroutines: Add test verifying parser isolation in tsParserSet.
// TODO: Gap 57: Type Coercion: Invalid UTF-16 surrogate halves: Add test with isolated UTF-16 surrogate half.
// TODO: Gap 58: Extremes: Gigantic arrays: Add test for array evaluation breadth with 1 million items.
// TODO: Gap 59: Extremes: Gigantic object literals: Add test for dictionary evaluation with 1 million keys.
// TODO: Gap 60: Type Coercion: Conflicting file extensions: Add test verifying moduleIDForPath on script.go.py.
// TODO: Gap 61: Null/Undefined/Empty: Files with only comments: Add test asserting 0 facts emitted for comments-only file.
// TODO: Gap 62: Null/Undefined/Empty: Files with only docstrings: Add test asserting 0 facts emitted for docstring-only file.
// TODO: Gap 63: Type Coercion: Minified CSS in JS: Add test evaluating memory usage on JS file containing giant CSS string.
// TODO: Gap 64: Type Coercion: Base64 data in JS: Add test verifying parsing performance on 10MB base64 string.
// TODO: Gap 65: Extremes: Deeply nested template strings: Add test with JS template strings nested 50 levels deep.
// TODO: Gap 66: Extremes: Huge number of arguments: Add test handling a function definition with 10,000 arguments.
// TODO: Gap 67: Type Coercion: Typescript types with circular references: Add test verifying AST builder does not infinite loop.
// TODO: Gap 68: Null/Undefined/Empty: Unnamed classes in JS: Add test asserting skipped default exported classes.
// TODO: Gap 69: Null/Undefined/Empty: Unnamed functions in Python decorators: Add test asserting skipped anonymous functions in decorators.
// TODO: Gap 70: State Conflicts: CGO memory limits: Add test mapping 1000 files with a 10MB memory limit using GOMEMLIMIT.
// TODO: Gap 71: Null/Undefined/Empty: Empty object destructuring: Add test handling empty object destructuring syntax.
// TODO: Gap 72: Null/Undefined/Empty: Empty array destructuring: Add test handling empty array destructuring syntax.
// TODO: Gap 73: Null/Undefined/Empty: Empty TS Interfaces: Add test verifying if empty interface definition is emitted.
// TODO: Gap 74: Null/Undefined/Empty: Empty TS Types: Add test verifying if empty type alias definition is emitted.
// TODO: Gap 75: Null/Undefined/Empty: Empty Rust Structs: Add test verifying if empty struct definition is emitted.
// TODO: Gap 76: Null/Undefined/Empty: Empty Rust Enums: Add test verifying if empty enum definition is emitted.
// TODO: Gap 77: Null/Undefined/Empty: Empty Rust Traits: Add test verifying if empty trait definition is emitted.
// TODO: Gap 78: Null/Undefined/Empty: Empty Python Classes: Add test verifying if empty class definition is emitted.
// TODO: Gap 79: Null/Undefined/Empty: Empty Python Functions: Add test verifying if empty function definition is emitted.
// TODO: Gap 80: Null/Undefined/Empty: Missing Javascript Functions: Add test verifying if empty javascript function definition is emitted.
// TODO: Gap 81: State Conflicts: Invalid Tree-Sitter Node Operations: Add test simulating corrupt memory causing tree-sitter node type strings to be invalid.
// TODO: Gap 82: Type Coercion: Javascript Immediately Invoked Function Expressions (IIFE): Add test parsing Javascript IIFEs.
// TODO: Gap 83: Type Coercion: Javascript async arrow functions: Add test parsing async arrow functions correctly.
// TODO: Gap 84: Type Coercion: Python async functions: Add test parsing Python async functions correctly.
// TODO: Gap 85: Type Coercion: Rust async functions: Add test parsing Rust async functions correctly.
// TODO: Gap 86: Type Coercion: Rust macros with different brackets: Add test parsing different macro bracket styles correctly.
// TODO: Gap 87: Type Coercion: Python nested classes: Add test parsing Python nested classes correctly.
// TODO: Gap 88: Type Coercion: Python nested functions: Add test parsing Python nested functions correctly.
// TODO: Gap 89: Type Coercion: Javascript nested functions: Add test parsing JS nested functions correctly.
// TODO: Gap 90: Type Coercion: Javascript nested classes: Add test parsing JS nested classes correctly.
// TODO: Gap 91: Type Coercion: Typescript nested namespaces: Add test parsing TS nested namespaces correctly.
// TODO: Gap 92: Type Coercion: Python multiple inheritance: Add test checking if Python multiple inheritance crashes parser.
// TODO: Gap 93: Type Coercion: Rust lifetime parameters: Add test checking if Rust lifetime parameters crash parser.
// TODO: Gap 94: Type Coercion: Rust generic parameters: Add test checking if Rust generic parameters crash parser.
// TODO: Gap 95: Type Coercion: Typescript generic parameters: Add test checking if TS generic parameters crash parser.
// TODO: Gap 96: Type Coercion: Javascript classes with private fields: Add test checking if private fields crash JS parser.
// TODO: Gap 97: Type Coercion: Javascript classes with static fields: Add test checking if static fields crash JS parser.
// TODO: Gap 98: Type Coercion: Javascript classes with static methods: Add test checking if static methods crash JS parser.
// TODO: Gap 99: Type Coercion: Python static methods: Add test checking if static methods crash Python parser.
// TODO: Gap 100: Type Coercion: Python class methods: Add test checking if class methods crash Python parser.
