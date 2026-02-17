package campaign

import (
	"testing"
)

func BenchmarkExtractPathFromDescription(b *testing.B) {
	descriptions := []string{
		"Create internal/domain/foo.go",
		"file: path/to/file.go",
		"cmd/nerd/main.go",
		"Just a description without path",
		"internal/pkg/utils.go needs update",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, desc := range descriptions {
			extractPathFromDescription(desc)
		}
	}
}
