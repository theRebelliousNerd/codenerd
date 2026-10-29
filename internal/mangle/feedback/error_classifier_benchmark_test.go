package feedback

import (
	"testing"
)

func BenchmarkExtractLineCol(b *testing.B) {
	errMsgs := []string{
		"123:45 parse error",
		"Error: 12:34 something",
		"error at line 123",
		"Error Line 55",
		"just some generic error without line info",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		extractLineCol(errMsgs[i%len(errMsgs)])
	}
}
