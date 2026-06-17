package prompt_evolution

import "testing"

func BenchmarkNewProblemClassifier(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = NewProblemClassifier()
	}
}

func BenchmarkClassify(b *testing.B) {
	classifier := NewProblemClassifier()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		classifier.Classify("I want to add a new feature to integrate with the API")
	}
}
