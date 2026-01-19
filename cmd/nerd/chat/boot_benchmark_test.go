package chat

import (
	"codenerd/internal/config"
	"testing"
)

func BenchmarkSystemBoot(b *testing.B) {
	// Setup a temporary workspace
	ws := b.TempDir()

	// Create minimal config
	cfg := config.DefaultUserConfig()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Run boot command
		cmd := performSystemBoot(cfg, nil, ws)
		msg := cmd() // blocking execution

		// Verify result
		if res, ok := msg.(bootCompleteMsg); !ok {
			b.Errorf("Boot returned unexpected message type: %T", msg)
		} else if res.err != nil {
			b.Errorf("Boot failed: %v", res.err)
		}
	}
}
