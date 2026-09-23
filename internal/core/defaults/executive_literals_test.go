package defaults

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// =============================================================================
// EXECUTIVE LITERALS
// =============================================================================
// The other half of the executive drift (executive_conclusions_test.go): Go
// deciding with its own number or its own string match. In the executive
// packages, count
//
//   - literal: an integer or float literal greater than 1 in a const
//     declaration, an if/for/switch condition, or the value of a composite
//     field whose name says it bounds something (Limit, Timeout, Max, Retries,
//     Attempts, Threshold, Budget) -- a knob config.json cannot reach;
//   - textmatch: a strings.Contains/HasPrefix/HasSuffix/EqualFold/ContainsAny
//     whose haystack is a .Description, an .Error(), or a model's .Response or
//     .Text (lower-cased or trimmed or not) -- a classifier over natural
//     language or error text where a typed measurement belongs.
//
// Per package, in a baseline that may only shrink. A new knob arrives through
// config (config_param); a new classifier through a typed fact.
//
// Regenerate: CODENERD_UPDATE_LITERALS=1 go test ./internal/core/defaults/ -run TestExecutiveLiteralBudget

const literalsBaselinePath = "testdata/executive_literals.txt"

var executiveDirs = []string{"internal/campaign", "internal/session", "cmd/nerd/chat", "internal/verification"}

var boundingField = regexp.MustCompile(`Limit|Timeout|Max|Retries|Attempts|Threshold|Budget`)

// countBigLiterals counts numeric literals greater than 1 under n. A literal
// handed to a format or logging call is text, not a knob.
func countBigLiterals(n ast.Node) int {
	count := 0
	ast.Inspect(n, func(m ast.Node) bool {
		if call, ok := m.(*ast.CallExpr); ok {
			if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
				if x, ok := sel.X.(*ast.Ident); ok && (x.Name == "fmt" || x.Name == "logging" || x.Name == "log") {
					return false
				}
			}
		}
		lit, ok := m.(*ast.BasicLit)
		if !ok || (lit.Kind != token.INT && lit.Kind != token.FLOAT) {
			return true
		}
		if v, err := strconv.ParseFloat(strings.ReplaceAll(lit.Value, "_", ""), 64); err == nil && v > 1 {
			count++
		}
		return true
	})
	return count
}

// isTextHaystack reports whether expr is natural-language or error text: the
// field or call itself, or a local of the enclosing function assigned from one
// (lower := strings.ToLower(task.Description), the usual shape).
func isTextHaystack(expr ast.Expr, textLocals map[string]bool) bool {
	switch e := expr.(type) {
	case *ast.Ident:
		return textLocals[e.Name]
	case *ast.SelectorExpr:
		switch e.Sel.Name {
		case "Description", "Response", "Text":
			return true
		}
	case *ast.CallExpr:
		if sel, ok := e.Fun.(*ast.SelectorExpr); ok {
			if sel.Sel.Name == "Error" && len(e.Args) == 0 {
				return true
			}
			if (sel.Sel.Name == "ToLower" || sel.Sel.Name == "TrimSpace") && len(e.Args) == 1 {
				return isTextHaystack(e.Args[0], textLocals)
			}
		}
	}
	return false
}

// textLocalsOf returns the locals fn assigns from natural-language or error
// text, to a fixpoint (a local assigned from such a local is one too).
func textLocalsOf(fn *ast.FuncDecl) map[string]bool {
	locals := make(map[string]bool)
	if fn.Body == nil {
		return locals
	}
	for changed := true; changed; {
		changed = false
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			as, ok := n.(*ast.AssignStmt)
			if !ok || len(as.Lhs) != len(as.Rhs) {
				return true
			}
			for i, lhs := range as.Lhs {
				id, ok := lhs.(*ast.Ident)
				if !ok || locals[id.Name] {
					continue
				}
				if isTextHaystack(as.Rhs[i], locals) {
					locals[id.Name] = true
					changed = true
				}
			}
			return true
		})
	}
	return locals
}

// executiveLiteralCounts returns "literal <dir>" and "textmatch <dir>" counts.
func executiveLiteralCounts(t *testing.T) map[string]int {
	t.Helper()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	root := repoRootFrom(t, cwd)
	counts := make(map[string]int)
	fset := token.NewFileSet()
	for _, dir := range executiveDirs {
		entries, err := os.ReadDir(filepath.Join(root, dir))
		if err != nil {
			t.Fatalf("read %s: %v", dir, err)
		}
		literal, textmatch := "literal "+dir, "textmatch "+dir
		counts[literal], counts[textmatch] = 0, 0
		for _, entry := range entries {
			path := filepath.Join(root, dir, entry.Name())
			if entry.IsDir() || !isProductionGo(path) {
				continue
			}
			f, perr := parser.ParseFile(fset, path, nil, 0)
			if perr != nil {
				t.Fatalf("parse %s: %v", path, perr)
			}
			for _, decl := range f.Decls {
				locals := map[string]bool{}
				if fn, ok := decl.(*ast.FuncDecl); ok {
					locals = textLocalsOf(fn)
				}
				countDecl(decl, locals, counts, literal, textmatch)
			}
		}
	}
	return counts
}

// countDecl adds one declaration's knobs and text matches to counts.
func countDecl(decl ast.Decl, locals map[string]bool, counts map[string]int, literal, textmatch string) {
	ast.Inspect(decl, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.GenDecl:
			if x.Tok == token.CONST {
				counts[literal] += countBigLiterals(x)
				return false
			}
		case *ast.IfStmt:
			if x.Cond != nil {
				counts[literal] += countBigLiterals(x.Cond)
			}
		case *ast.ForStmt:
			if x.Cond != nil {
				counts[literal] += countBigLiterals(x.Cond)
			}
		case *ast.SwitchStmt:
			if x.Tag != nil {
				counts[literal] += countBigLiterals(x.Tag)
			}
		case *ast.KeyValueExpr:
			if key, ok := x.Key.(*ast.Ident); ok && boundingField.MatchString(key.Name) {
				counts[literal] += countBigLiterals(x.Value)
			}
		case *ast.CallExpr:
			sel, ok := x.Fun.(*ast.SelectorExpr)
			if !ok || len(x.Args) == 0 {
				return true
			}
			if pkg, ok := sel.X.(*ast.Ident); !ok || pkg.Name != "strings" {
				return true
			}
			switch sel.Sel.Name {
			case "Contains", "HasPrefix", "HasSuffix", "EqualFold", "ContainsAny":
				if isTextHaystack(x.Args[0], locals) {
					counts[textmatch]++
				}
			}
		}
		return true
	})
}

// TestExecutiveLiteralBudget fails when an executive package gains a numeric
// knob or a text classifier, and when one loses some without the baseline
// being refreshed.
func TestExecutiveLiteralBudget(t *testing.T) {
	current := executiveLiteralCounts(t)
	keys := make([]string, 0, len(current))
	for k := range current {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	if os.Getenv("CODENERD_UPDATE_LITERALS") == "1" {
		var lines []string
		for _, k := range keys {
			lines = append(lines, k+" "+strconv.Itoa(current[k]))
		}
		header := strings.Join([]string{
			"# Executive literals -- per package, counts that may only shrink.",
			"#",
			"# Regenerate: CODENERD_UPDATE_LITERALS=1 go test ./internal/core/defaults/ -run TestExecutiveLiteralBudget",
			"# See executive_literals_test.go. \"literal\": a numeric literal > 1 in a const,",
			"# an if/for/switch condition, or a bounding composite field. \"textmatch\": a",
			"# strings match over .Description, .Error(), .Response or .Text.",
			"", "",
		}, "\n")
		if err := os.WriteFile(literalsBaselinePath, []byte(header+strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
			t.Fatalf("write %s: %v", literalsBaselinePath, err)
		}
		t.Logf("rewrote %s with %d count(s)", literalsBaselinePath, len(lines))
		return
	}

	data, err := os.ReadFile(literalsBaselinePath)
	if err != nil {
		t.Fatalf("read %s: %v\nRegenerate with: CODENERD_UPDATE_LITERALS=1 go test ./internal/core/defaults/ -run TestExecutiveLiteralBudget", literalsBaselinePath, err)
	}
	baseline := make(map[string]int)
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		i := strings.LastIndex(line, " ")
		n, err := strconv.Atoi(line[i+1:])
		if err != nil {
			t.Fatalf("baseline line %q: %v", line, err)
		}
		baseline[line[:i]] = n
	}
	for _, k := range keys {
		want, known := baseline[k]
		switch {
		case !known || current[k] > want:
			t.Errorf("%s: %d, baseline %d. A new knob arrives through config (config_param), a new classifier through a typed fact -- not a Go literal or a string match.", k, current[k], want)
		case current[k] < want:
			t.Errorf("%s: %d, baseline %d -- good; refresh the baseline so the count stays a measurement: CODENERD_UPDATE_LITERALS=1 go test ./internal/core/defaults/ -run TestExecutiveLiteralBudget", k, current[k], want)
		}
	}
}
