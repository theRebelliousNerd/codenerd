package features

// Test conveniences over the active configuration.

// Active returns the currently-installed FeaturesConfig or nil if
// none has been set. Reads are wait-free; callers must not mutate the
// returned pointer.
func Active() *FeaturesConfig { return active.Load() }
