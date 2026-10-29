package mangle

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
)

func TestCodeUsesSerializedMangleParser(t *testing.T) {
	_, testFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate parse lock test")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(testFile), "..", ".."))
	lockFile := filepath.Join(filepath.Dir(testFile), "parse_lock.go")
	fset := token.NewFileSet()

	err := filepath.WalkDir(repoRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if path == repoRoot {
				return nil
			}
			if entry.Name() == "vendor" || entry.Name() == "testdata" || entry.Name() == "node_modules" || strings.HasPrefix(entry.Name(), ".") {
				return filepath.SkipDir
			}
			if _, err := os.Stat(filepath.Join(path, "go.mod")); err == nil {
				return filepath.SkipDir
			} else if !os.IsNotExist(err) {
				return err
			}
			return nil
		}
		if filepath.Ext(path) != ".go" || path == lockFile {
			return nil
		}

		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return err
		}
		parseAliases := make(map[string]struct{})
		for _, imported := range file.Imports {
			importPath, err := strconv.Unquote(imported.Path.Value)
			if err != nil {
				return err
			}
			if importPath != "codeberg.org/TauCeti/mangle-go/parse" {
				continue
			}
			alias := "parse"
			if imported.Name != nil {
				alias = imported.Name.Name
			}
			if alias == "." || alias == "_" {
				position := fset.Position(imported.Pos())
				t.Errorf("%s imports the unsafe parser as %q; use internal/mangle ParseUnit or ParseAtom", position, alias)
				continue
			}
			parseAliases[alias] = struct{}{}
		}

		ast.Inspect(file, func(node ast.Node) bool {
			selector, ok := node.(*ast.SelectorExpr)
			if !ok || selector.Sel.Name == "SourceUnit" {
				return true
			}
			qualifier, ok := selector.X.(*ast.Ident)
			if !ok {
				return true
			}
			if _, direct := parseAliases[qualifier.Name]; direct {
				position := fset.Position(selector.Pos())
				t.Errorf("%s calls the unsafe parser directly; use internal/mangle ParseUnit or ParseAtom", position)
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatalf("scan production parser calls: %v", err)
	}
}

func TestParseUnit_Concurrent(t *testing.T) {
	const numGoroutines = 100
	var wg sync.WaitGroup
	startCh := make(chan struct{})

	// Valid mangle unit string
	unitStr := `
		Decl test_pred(Int).
		test_pred(1).
	`

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-startCh // Wait for the green light
			reader := strings.NewReader(unitStr)
			_, err := ParseUnit(reader)
			if err != nil {
				t.Errorf("ParseUnit failed: %v", err)
			}
		}()
	}

	close(startCh) // Unleash all goroutines
	wg.Wait()
}

func TestParseAtom_Concurrent(t *testing.T) {
	const numGoroutines = 100
	var wg sync.WaitGroup
	startCh := make(chan struct{})

	// ParseAtom expects arguments to have types or be variables/strings
	// Use an atom that is valid for ParseAtom
	atomStr := "test_pred(1)"

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-startCh // Wait for the green light
			_, err := ParseAtom(atomStr)
			if err != nil {
				t.Errorf("ParseAtom failed: %v", err)
			}
		}()
	}

	close(startCh) // Unleash all goroutines
	wg.Wait()
}

func TestParseMixed_Concurrent(t *testing.T) {
	const numGoroutines = 100
	var wg sync.WaitGroup
	startCh := make(chan struct{})

	for i := 0; i < numGoroutines; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			<-startCh
			source := `
				Decl test_pred(Int).
				test_pred(1).
			`
			_, err := ParseUnit(strings.NewReader(source))
			if err != nil {
				t.Errorf("ParseUnit failed: %v", err)
			}
		}()

		go func() {
			defer wg.Done()
			<-startCh
			source := "test_pred(1)"
			_, err := ParseAtom(source)
			if err != nil {
				t.Errorf("ParseAtom failed: %v", err)
			}
		}()
	}

	close(startCh)
	wg.Wait()
}

func TestParseUnit_Error(t *testing.T) {
	// Test parsing invalid syntax
	invalidStr := "p(X) :- "
	_, err := ParseUnit(strings.NewReader(invalidStr))
	if err == nil {
		t.Errorf("Expected ParseUnit to return an error for invalid syntax")
	}
}

func TestParseAtom_Success(t *testing.T) {
	atom, err := ParseAtom("test_pred(1)")
	if err != nil {
		t.Fatalf("ParseAtom failed: %v", err)
	}
	if atom.Predicate.Symbol != "test_pred" {
		t.Errorf("Expected predicate 'test_pred', got '%v'", atom.Predicate.Symbol)
	}
}

func TestParseAtom_Error(t *testing.T) {
	_, err := ParseAtom("invalid syntax (((")
	if err == nil {
		t.Fatalf("ParseAtom should fail on invalid syntax")
	}
}
