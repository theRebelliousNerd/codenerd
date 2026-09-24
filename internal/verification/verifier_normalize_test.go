package verification

import (
	"testing"

	"codenerd/internal/config"
)

func TestSetTaskExecutor(t *testing.T) {
	v := NewTaskVerifier(nil, nil, nil, config.JITConfig{})
	// Setting a nil executor must not panic and should store the value.
	v.SetTaskExecutor(nil)
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.taskExecutor != nil {
		t.Error("expected taskExecutor to remain nil after SetTaskExecutor(nil)")
	}
}
