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

func BenchmarkCheckInfiniteLoopRisk(b *testing.B) {
	shard := &MangleRepairShard{}
	rule := "next_action(/do_something) :- system_startup(/start, _), some_other_pred(X)."
	b.ResetTimer()
	for b.Loop() {
		shard.checkInfiniteLoopRisk(rule)
	}
}
