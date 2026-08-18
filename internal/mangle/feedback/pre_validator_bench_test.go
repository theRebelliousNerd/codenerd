package feedback

import (
	"testing"
)

func BenchmarkNewPreValidator(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = NewPreValidator()
	}
}

func BenchmarkPreValidator_Validate(b *testing.B) {
	pv := NewPreValidator()
	code := `
		status(X, "active")
		\+ permitted(X)
		Total = sum(Amount)
		|> fn:group_by(X)
	`
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = pv.Validate(code)
	}
}
