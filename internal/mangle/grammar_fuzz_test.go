package mangle_test

import (
	"testing"
	"github.com/google/mangle/parse"
)

func FuzzParseAtom(f *testing.F) {
	// Add seed corpus
	f.Add("foo(1)")
	f.Add("bar(\"baz\")")
	f.Add("pred(/atom)")
	f.Add("p(X, Y)")

	f.Fuzz(func(t *testing.T, data string) {
		// Just verify it doesn't panic
		_, _ = parse.Atom(data)
	})
}
