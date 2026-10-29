package retrieval

import (
	"testing"
)

func BenchmarkExtractKeywords(b *testing.B) {
	issueText := `
	We have an issue in internal/retrieval/sparse.go where the function ExtractKeywords is too slow.
	The regexp.MustCompile is called inside the function which parses the string for ClassName and other methodCall().
	Please fix the ValueError by using global variables.
	`
	b.ResetTimer()
	for b.Loop() {
		ExtractKeywords(issueText)
	}
}
