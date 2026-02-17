package feedback

import (
	"testing"
)

func BenchmarkQuickFix(b *testing.B) {
	pv := NewPreValidator()
	code := `
next_action(/run) :- test_state("failing").
blocked(X) :- \+ permitted(X).
total = fn:Sum(amount).
|> fn:group_by(category).
state(X, "active").
state(Y, "pending").
`
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pv.QuickFix(code)
	}
}
