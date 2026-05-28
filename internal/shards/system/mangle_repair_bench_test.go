package system

import (
	"testing"
)

func BenchmarkCheckSafety(b *testing.B) {
	shard := &MangleRepairShard{}
	rule := "p(X) :- q(X, Y), not r(Z), not s(X)."
	b.ResetTimer()
	for b.Loop() {
		shard.checkSafety(rule)
	}
}
