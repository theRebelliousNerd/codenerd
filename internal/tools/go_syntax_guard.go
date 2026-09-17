package tools

import (
	"fmt"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
)

// RejectUnparseableGo refuses a file write whose resulting .go content does not
// parse. A syntactically invalid Go file can never be a correct write: accepting
// one strands a broken tree the model only discovers minutes later at
// verification (observed live: a truncated test file survived write_file and died
// in go test). The failure lands in-loop, where the model sees it at once.
// Non-Go paths are untouched.
func RejectUnparseableGo(path string, content []byte) error {
	if !strings.EqualFold(filepath.Ext(path), ".go") {
		return nil
	}
	if _, err := parser.ParseFile(token.NewFileSet(), path, content, parser.AllErrors); err != nil {
		first, _, _ := strings.Cut(err.Error(), "\n")
		return fmt.Errorf("refusing to write %s: Go syntax invalid: %s", path, first)
	}
	return nil
}

// RejectGoSyntaxRegression refuses a write that turns Go source that parses
// into source that does not. A file that already fails to parse stays editable
// so a broken file can be repaired incrementally; non-Go paths and parseable
// results return nil.
func RejectGoSyntaxRegression(path string, before, after []byte) error {
	if err := RejectUnparseableGo(path, after); err != nil {
		if RejectUnparseableGo(path, before) != nil {
			return nil
		}
		return err
	}
	return nil
}
