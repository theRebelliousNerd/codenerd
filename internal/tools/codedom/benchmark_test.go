package codedom

import (
	"os"
	"path/filepath"
	"testing"
)

func BenchmarkExtractCodeElements(b *testing.B) {
	// Setup a temporary file with some content
	tmpDir := b.TempDir()
	goFile := filepath.Join(tmpDir, "bench.go")
	content := `package bench

func Func1() {}
type Struct1 struct {}
func (s *Struct1) Method1() {}
type Interface1 interface {}
`
	if err := os.WriteFile(goFile, []byte(content), 0644); err != nil {
		b.Fatalf("failed to write bench file: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := extractCodeElements(goFile)
		if err != nil {
			b.Fatalf("extractCodeElements failed: %v", err)
		}
	}
}
