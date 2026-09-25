package context_harness

import (
	"io"
	"testing"
)

func TestFileLogger_Writers(t *testing.T) {
	fl, err := NewFileLogger(t.TempDir(), io.Discard)
	if err != nil {
		t.Fatalf("NewFileLogger: %v", err)
	}
	defer fl.Close()

	// Every category writer must be non-nil and accept a write.
	writers := map[string]io.Writer{
		"jit":         fl.GetJITWriter(),
		"activation":  fl.GetActivationWriter(),
		"compression": fl.GetCompressionWriter(),
		"piggyback":   fl.GetPiggybackWriter(),
		"summary":     fl.GetSummaryWriter(),
		"feedback":    fl.GetFeedbackWriter(),
	}
	for name, w := range writers {
		if w == nil {
			t.Errorf("%s writer is nil", name)
			continue
		}
		if _, err := w.Write([]byte("log line\n")); err != nil {
			t.Errorf("%s writer write failed: %v", name, err)
		}
	}
}
