package core

import "testing"

func BenchmarkNewRealKernelWithWorkspace(b *testing.B) {
	dir := b.TempDir()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		k, err := NewRealKernelWithWorkspace(dir)
		if err != nil {
			b.Fatalf("boot failed: %v", err)
		}
		_ = k
	}
}
