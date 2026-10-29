package mangle

import "testing"

func BenchmarkFixUnquotedStrings(b *testing.B) {
	input := "foo(bar)"
	b.ResetTimer()
	for b.Loop() {
		fixUnquotedStrings(input)
	}
}

func BenchmarkFixUnquotedStrings_Complex(b *testing.B) {
	input := "p(foo, bar, \"baz\", /qux, 123)"
	b.ResetTimer()
	for b.Loop() {
		fixUnquotedStrings(input)
	}
}

func BenchmarkIsNumeric(b *testing.B) {
	input := "123.456"
	b.ResetTimer()
	for b.Loop() {
		isNumeric(input)
	}
}

func BenchmarkIsNumeric_NonNumeric(b *testing.B) {
	input := "non-numeric-string"
	b.ResetTimer()
	for b.Loop() {
		isNumeric(input)
	}
}
