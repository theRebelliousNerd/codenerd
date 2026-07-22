package world

import (
	"context"
	"strings"
	"testing"

	"github.com/smacker/go-tree-sitter/golang"
)

func BenchmarkExtractGoSymbols(b *testing.B) {
	parser := NewTreeSitterParser()
	defer parser.Close()

	code := `
package main

type MyStruct struct {
	Field1 string
	Field2 int
}

func (m *MyStruct) Method1(a int) error {
	return nil
}

func (m *MyStruct) Method2(a int, b string) (string, error) {
	return "", nil
}

func Function1(a int) error {
	return nil
}

func Function2(a int, b string) (string, error) {
	return "", nil
}
`
	// repeat the code to make it larger
	var sb strings.Builder
	for i := 0; i < 1000; i++ {
		sb.WriteString(code)
	}
	content := []byte(sb.String())

	parser.goParser.SetLanguage(golang.GetLanguage())
	tree, err := parser.goParser.ParseCtx(context.Background(), nil, content)
	if err != nil {
		b.Fatal(err)
	}
	defer tree.Close()

	root := tree.RootNode()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = parser.extractGoSymbols(root, "test.go", string(content))
	}
}
