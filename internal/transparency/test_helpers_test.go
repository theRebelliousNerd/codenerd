package transparency

// Test hooks over the process-wide transparency state.

// ProcessManager returns the process-wide manager, or nil.
func ProcessManager() *TransparencyManager {
	return processManager.Load()
}

// SetProcessBus overrides the process-wide Glass Box bus. Pass nil to clear.
// Returns the previous value so a test can restore it.
func SetProcessBus(bus *GlassBoxEventBus) *GlassBoxEventBus {
	return processBus.Swap(bus)
}
