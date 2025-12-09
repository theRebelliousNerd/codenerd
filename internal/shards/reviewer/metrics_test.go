package reviewer

import (
	"testing"
)

// =============================================================================
// LANGUAGE DETECTION TESTS
// =============================================================================

func TestDetectLanguage(t *testing.T) {
	tests := []struct {
		path     string
		expected Language
	}{
		// Go
		{"main.go", LangGo},
		{"internal/core/kernel.go", LangGo},

		// Python
		{"script.py", LangPython},
		{"gui.pyw", LangPython},

		// JavaScript/TypeScript
		{"app.js", LangJavaScript},
		{"component.jsx", LangJavaScript},
		{"app.mjs", LangJavaScript},
		{"lib.cjs", LangJavaScript},
		{"app.ts", LangTypeScript},
		{"component.tsx", LangTypeScript},

		// Java
		{"Main.java", LangJava},

		// C#
		{"Program.cs", LangCSharp},

		// Rust
		{"main.rs", LangRust},

		// C/C++
		{"main.c", LangC},
		{"header.h", LangC},
		{"main.cpp", LangCPP},
		{"main.cc", LangCPP},
		{"header.hpp", LangCPP},

		// Ruby
		{"app.rb", LangRuby},

		// Swift
		{"ViewController.swift", LangSwift},

		// Kotlin
		{"Main.kt", LangKotlin},
		{"build.gradle.kts", LangKotlin},

		// PHP
		{"index.php", LangPHP},

		// Unknown
		{"data.json", LangUnknown},
		{"style.css", LangUnknown},
		{"README.md", LangUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := detectLanguage(tt.path)
			if got != tt.expected {
				t.Errorf("detectLanguage(%q) = %d, want %d", tt.path, got, tt.expected)
			}
		})
	}
}

// =============================================================================
// COMMENT AND STRING STRIPPING TESTS
// =============================================================================

func TestStripCommentsAndStrings(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		lang     Language
		expected string
	}{
		// Single-line comments
		{
			name:     "go single line comment",
			line:     `if x > 0 { // check positive`,
			lang:     LangGo,
			expected: `if x > 0 { `,
		},
		{
			name:     "python hash comment",
			line:     `if x > 0:  # check positive`,
			lang:     LangPython,
			expected: `if x > 0:  `,
		},

		// Inline block comments
		{
			name:     "inline block comment",
			line:     `if /* condition */ x > 0 {`,
			lang:     LangGo,
			expected: `if   x > 0 {`,
		},

		// String literals
		{
			name:     "double quote string with keywords",
			line:     `log("if this and that")`,
			lang:     LangGo,
			expected: `log("")`,
		},
		{
			name:     "single quote string",
			line:     `log('for each item')`,
			lang:     LangJavaScript,
			expected: `log('')`,
		},
		{
			name:     "backtick template literal",
			line:     "fmt.Sprintf(`if %s while %s`)",
			lang:     LangGo,
			expected: "fmt.Sprintf(``)",
		},

		// Escaped quotes in strings
		{
			name:     "escaped quotes",
			line:     `log("He said \"if\" then")`,
			lang:     LangGo,
			expected: `log("")`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripCommentsAndStrings(tt.line, tt.lang)
			if got != tt.expected {
				t.Errorf("stripCommentsAndStrings(%q, %d) = %q, want %q",
					tt.line, tt.lang, got, tt.expected)
			}
		})
	}
}

// =============================================================================
// CYCLOMATIC COMPLEXITY DECISION POINT TESTS
// =============================================================================

func TestCountDecisionPoints_BasicConditionals(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		lang     Language
		expected int
	}{
		// Simple if
		{"simple if go", "if x > 0 {", LangGo, 1},
		{"simple if python", "if x > 0:", LangPython, 1},
		{"simple if js", "if (x > 0) {", LangJavaScript, 1},

		// else if / elif - should count as 1, NOT 2
		{"else if go", "} else if x > 0 {", LangGo, 1},
		{"else if js", "} else if (x > 0) {", LangJavaScript, 1},
		{"elif python", "elif x > 0:", LangPython, 1},

		// Multiple conditions on same line (rare but possible)
		{"two ifs", "if a { } if b { }", LangGo, 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := countDecisionPoints(tt.line, tt.lang)
			if got != tt.expected {
				t.Errorf("countDecisionPoints(%q, %d) = %d, want %d",
					tt.line, tt.lang, got, tt.expected)
			}
		})
	}
}

func TestCountDecisionPoints_Loops(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		lang     Language
		expected int
	}{
		// for loops
		{"for go", "for i := 0; i < n; i++ {", LangGo, 1},
		{"for python", "for item in items:", LangPython, 1},
		{"for js", "for (let i = 0; i < n; i++) {", LangJavaScript, 1},

		// while loops
		{"while python", "while x > 0:", LangPython, 1},
		{"while js", "while (x > 0) {", LangJavaScript, 1},

		// Rust loop
		{"loop rust", "loop {", LangRust, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := countDecisionPoints(tt.line, tt.lang)
			if got != tt.expected {
				t.Errorf("countDecisionPoints(%q, %d) = %d, want %d",
					tt.line, tt.lang, got, tt.expected)
			}
		})
	}
}

func TestCountDecisionPoints_LogicalOperators(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		lang     Language
		expected int
	}{
		// && and ||
		{"and operator", "if a && b {", LangGo, 2},        // if + &&
		{"or operator", "if a || b {", LangGo, 2},         // if + ||
		{"both operators", "if a && b || c {", LangGo, 3}, // if + && + ||

		// Python and/or
		{"python and", "if a and b:", LangPython, 2},       // if + and
		{"python or", "if a or b:", LangPython, 2},         // if + or
		{"python both", "if a and b or c:", LangPython, 3}, // if + and + or

		// Operators without if (in assignment, for example)
		{"standalone and", "result = a && b", LangGo, 1}, // just &&
		{"standalone or", "result = a || b", LangGo, 1},  // just ||
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := countDecisionPoints(tt.line, tt.lang)
			if got != tt.expected {
				t.Errorf("countDecisionPoints(%q, %d) = %d, want %d",
					tt.line, tt.lang, got, tt.expected)
			}
		})
	}
}

func TestCountDecisionPoints_SwitchCase(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		lang     Language
		expected int
	}{
		// Go switch/case
		{"case go", "case 1:", LangGo, 1},
		{"case with value", "case \"foo\":", LangGo, 1},

		// Go select
		{"select go", "select {", LangGo, 1},

		// JavaScript/TypeScript case
		{"case js", "case 'foo':", LangJavaScript, 1},

		// Rust match
		{"match rust", "match x {", LangRust, 1},

		// Python match/case
		{"case python", "case 1:", LangPython, 1},
		{"match python", "match x:", LangPython, 1},

		// Kotlin when
		{"when kotlin", "when (x) {", LangKotlin, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := countDecisionPoints(tt.line, tt.lang)
			if got != tt.expected {
				t.Errorf("countDecisionPoints(%q, %d) = %d, want %d",
					tt.line, tt.lang, got, tt.expected)
			}
		})
	}
}

func TestCountDecisionPoints_ExceptionHandling(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		lang     Language
		expected int
	}{
		// catch
		{"catch js", "} catch (err) {", LangJavaScript, 1},
		{"catch java", "} catch (Exception e) {", LangJava, 1},

		// Python except
		{"except python", "except ValueError:", LangPython, 1},
		{"except bare", "except:", LangPython, 1},

		// Ruby rescue
		{"rescue ruby", "rescue StandardError => e", LangRuby, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := countDecisionPoints(tt.line, tt.lang)
			if got != tt.expected {
				t.Errorf("countDecisionPoints(%q, %d) = %d, want %d",
					tt.line, tt.lang, got, tt.expected)
			}
		})
	}
}

func TestCountDecisionPoints_Ternary(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		lang     Language
		expected int
	}{
		// Standard ternary
		{"ternary js", "let x = a > 0 ? 1 : 0", LangJavaScript, 1},
		{"ternary java", "int x = a > 0 ? 1 : 0;", LangJava, 1},

		// Go and Rust don't have ternary - should NOT count ?
		{"no ternary go", "func foo() error { return err }", LangGo, 0},
		{"rust error prop", "let x = foo()?;", LangRust, 0},

		// Python ternary uses 'if' which is already counted
		{"ternary python", "x = 1 if a > 0 else 0", LangPython, 1}, // just the if

		// Optional chaining should NOT be counted
		{"optional chain js", "let x = obj?.foo?.bar", LangJavaScript, 0},
		{"optional chain ts", "const x = obj?.method()", LangTypeScript, 0},

		// Null coalescing - counted for languages that have it
		{"null coalesce js", "let x = a ?? b", LangJavaScript, 1},
		{"null coalesce ts", "const x = a ?? b", LangTypeScript, 1},
		{"null coalesce cs", "var x = a ?? b;", LangCSharp, 1},

		// Mixed: ternary + null coalescing
		{"mixed js", "let x = a ? b : c ?? d", LangJavaScript, 2}, // ? + ??
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := countDecisionPoints(tt.line, tt.lang)
			if got != tt.expected {
				t.Errorf("countDecisionPoints(%q, %d) = %d, want %d",
					tt.line, tt.lang, got, tt.expected)
			}
		})
	}
}

func TestCountDecisionPoints_RustSpecific(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		expected int
	}{
		// if let
		{"if let", "if let Some(x) = opt {", 1},
		{"if let chain", "if let Some(x) = opt && x > 0 {", 2}, // if let + &&

		// while let
		{"while let", "while let Some(x) = iter.next() {", 1},

		// loop (infinite)
		{"loop", "loop {", 1},

		// match
		{"match", "match x {", 1},

		// ? is error propagation, not ternary
		{"error prop", "let x = foo()?;", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := countDecisionPoints(tt.line, LangRust)
			if got != tt.expected {
				t.Errorf("countDecisionPoints(%q, LangRust) = %d, want %d",
					tt.line, got, tt.expected)
			}
		})
	}
}

func TestCountDecisionPoints_SwiftSpecific(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		expected int
	}{
		// guard
		{"guard", "guard let x = optional else { return }", 1},
		{"guard with condition", "guard x > 0 else { return }", 1},

		// if let
		{"if let", "if let x = optional {", 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := countDecisionPoints(tt.line, LangSwift)
			if got != tt.expected {
				t.Errorf("countDecisionPoints(%q, LangSwift) = %d, want %d",
					tt.line, got, tt.expected)
			}
		})
	}
}

func TestCountDecisionPoints_CommentsAndStrings(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		lang     Language
		expected int
	}{
		// Keywords in comments should NOT be counted
		{
			name:     "if in comment",
			line:     "x = 1 // if this fails, try again",
			lang:     LangGo,
			expected: 0,
		},
		{
			name:     "keywords in python comment",
			line:     "x = 1  # for each if case while loop",
			lang:     LangPython,
			expected: 0,
		},

		// Keywords in strings should NOT be counted
		{
			name:     "if in string",
			line:     `log("if condition failed")`,
			lang:     LangGo,
			expected: 0,
		},
		{
			name:     "multiple keywords in string",
			line:     `log("for each if case while")`,
			lang:     LangJavaScript,
			expected: 0,
		},

		// Real code + comment with keywords
		{
			name:     "real if with comment containing if",
			line:     "if x > 0 { // check if positive",
			lang:     LangGo,
			expected: 1, // only the real if
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := countDecisionPoints(tt.line, tt.lang)
			if got != tt.expected {
				t.Errorf("countDecisionPoints(%q, %d) = %d, want %d",
					tt.line, tt.lang, got, tt.expected)
			}
		})
	}
}

func TestCountDecisionPoints_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		lang     Language
		expected int
	}{
		// Empty line
		{"empty", "", LangGo, 0},

		// Line with only whitespace
		{"whitespace", "   \t  ", LangGo, 0},

		// Keyword as substring of identifier - should NOT count
		{
			name:     "notify contains if", // n-o-t-i-f-y, but "if" without word boundary
			line:     "notify(users)",
			lang:     LangGo,
			expected: 0,
		},
		{
			name:     "platform contains for", // p-l-a-t-f-o-r-m
			line:     "platform = linux",
			lang:     LangGo,
			expected: 0,
		},
		{
			name:     "caseInsensitive contains case",
			line:     "caseInsensitive = true",
			lang:     LangGo,
			expected: 0, // using word boundary, "case" inside identifier shouldn't match
		},

		// Multiple decision points on complex line
		{
			name:     "complex condition",
			line:     "if a > 0 && b < 10 || c == 5 {",
			lang:     LangGo,
			expected: 3, // if + && + ||
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := countDecisionPoints(tt.line, tt.lang)
			if got != tt.expected {
				t.Errorf("countDecisionPoints(%q, %d) = %d, want %d",
					tt.line, tt.lang, got, tt.expected)
			}
		})
	}
}

// =============================================================================
// FUNCTION DECLARATION DETECTION TESTS
// =============================================================================

func TestIsFunctionDeclaration(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		lang     Language
		expected bool
	}{
		// Go
		{"go func", "func main() {", LangGo, true},
		{"go method", "func (s *Server) Start() error {", LangGo, true},
		{"go anon func with space", "x := func () {}", LangGo, true}, // anonymous func with space
		{"go anon func no space", "x := func() {}", LangGo, false},   // no space after func - limitation of text-based detection

		// Python
		{"python def", "def main():", LangPython, true},
		{"python async def", "async def fetch():", LangPython, true},
		{"python not def", "x = undefined", LangPython, false},

		// Rust
		{"rust fn", "fn main() {", LangRust, true},
		{"rust async fn", "async fn fetch() {", LangRust, true},
		{"rust pub fn", "pub fn new() -> Self {", LangRust, true},

		// JavaScript
		{"js function", "function main() {", LangJavaScript, true},
		{"js async function", "async function fetch() {", LangJavaScript, true},
		{"js arrow function", "const main = () => {", LangJavaScript, true},
		{"js arrow not detected", "const main = x => x + 1", LangJavaScript, false}, // no => {

		// Kotlin
		{"kotlin fun", "fun main() {", LangKotlin, true},
		{"kotlin suspend fun", "suspend fun fetch() {", LangKotlin, true}, // contains "fun " - correctly detected

		// Not function declarations
		{"go comment", "// func is not real", LangGo, true}, // limitation of text-based detection
		{"go string", `log("func call")`, LangGo, true},     // limitation
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isFunctionDeclaration(tt.line, tt.lang)
			if got != tt.expected {
				t.Errorf("isFunctionDeclaration(%q, %d) = %v, want %v",
					tt.line, tt.lang, got, tt.expected)
			}
		})
	}
}

// =============================================================================
// REGRESSION TEST: ELSE-IF DOUBLE COUNTING BUG
// =============================================================================

func TestElseIfNotDoubleCounted(t *testing.T) {
	// This was the original bug: "else if" was matching both "else if " and "if "
	tests := []struct {
		name     string
		line     string
		lang     Language
		expected int
	}{
		{
			name:     "else if should be 1",
			line:     "} else if x > 0 {",
			lang:     LangGo,
			expected: 1, // NOT 2
		},
		{
			name:     "elif should be 1",
			line:     "elif x > 0:",
			lang:     LangPython,
			expected: 1, // NOT 2
		},
		{
			name:     "js else if should be 1",
			line:     "} else if (x > 0) {",
			lang:     LangJavaScript,
			expected: 1, // NOT 2
		},
		{
			name:     "chained else if",
			line:     "} else if (a) { } else if (b) {",
			lang:     LangJavaScript,
			expected: 2, // two else-ifs
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := countDecisionPoints(tt.line, tt.lang)
			if got != tt.expected {
				t.Errorf("REGRESSION: countDecisionPoints(%q) = %d, want %d (else-if double counting bug)",
					tt.line, got, tt.expected)
			}
		})
	}
}
