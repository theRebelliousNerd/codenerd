package feedback

import (
	"testing"
)

func BenchmarkExtractPredicateFromError(b *testing.B) {
	errMsgs := []string{
		"undeclared predicate foo()",
		"expected base term got xyz(",
		"no viable alternative at input 'foo'",
		"some unrelated error message without predicate",
		"error involving multiple_words_pred(",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ExtractPredicateFromError(errMsgs[i%len(errMsgs)])
	}
}

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
