package autopoiesis

import (
	"testing"
)

func BenchmarkExtractThoughtSteps(b *testing.B) {
	response := `
	Here are the steps I followed:
	1. First I did this.
	2. Then I did that.
	- I also did this other thing.
	* And finally this.
	`
	for i := 0; i < b.N; i++ {
		extractThoughtSteps(response)
	}
}

func BenchmarkExtractAssumptions(b *testing.B) {
	response := `
	I assume that the user wants to see the output.
	I expect that this will work.
	I presume that the file exists.
	`
	for i := 0; i < b.N; i++ {
		extractAssumptions(response)
	}
}

func BenchmarkExtractAlternatives(b *testing.B) {
	response := `
	Instead of doing X, I decided to do Y.
	Rather than using A, I used B.
	Could also use C but chose D.
	`
	for i := 0; i < b.N; i++ {
		extractAlternatives(response)
	}
}

func BenchmarkExtractDecisions(b *testing.B) {
	response := `
	I decided to use this because it's better.
	I chose that as it is faster.
	Using X because Y.
	`
	for i := 0; i < b.N; i++ {
		extractDecisions(response)
	}
}
