package ui

import (
	"testing"
)

func BenchmarkComputeKey_Int(b *testing.B) {
	for i := 0; i < b.N; i++ {
		ComputeKey(12345, 67890, 42)
	}
}

func BenchmarkComputeKey_String(b *testing.B) {
	for i := 0; i < b.N; i++ {
		ComputeKey("component", "status", "active")
	}
}

func BenchmarkComputeKey_Float(b *testing.B) {
	for i := 0; i < b.N; i++ {
		ComputeKey(0.123, 0.456, 0.789)
	}
}

func BenchmarkComputeKey_Mixed(b *testing.B) {
	for i := 0; i < b.N; i++ {
		ComputeKey("shard", 1, true, "running", 0.55)
	}
}
