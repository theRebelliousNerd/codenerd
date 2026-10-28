package logging

// Test seams: conveniences only tests call. They live here, not in
// production files, so the dead-code budget measures production code.

// IsDebugSelf returns whether debug-self mode is enabled in the effective config.
func IsDebugSelf() bool {
	configMu.RLock()
	defer configMu.RUnlock()
	return config.DebugSelf
}
