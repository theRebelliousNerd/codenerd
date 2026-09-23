package verification

import "testing"

func TestSetTaskExecutor(t *testing.T) {
	v := NewTaskVerifier(nil, nil)
	// Setting a nil executor must not panic and should store the value.
	v.SetTaskExecutor(nil)
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.taskExecutor != nil {
		t.Error("expected taskExecutor to remain nil after SetTaskExecutor(nil)")
	}
}
