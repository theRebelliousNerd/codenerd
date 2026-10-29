package core

// RegisterAllValidators registers all standard validators with the registry.
// Call this during VirtualStore initialization.
//
// It composes the four grouped registrars rather than repeating their contents.
// It used to list all thirteen validators inline while the groups listed the
// same thirteen again, and only this function had a caller
// (virtual_store.go). Two copies of a safety list, one of them live: a
// validator added to RegisterFileValidators would have been silently absent in
// production, and post-action validators are precisely what catch a write that
// did not do what it claimed. One list, in one place.
func RegisterAllValidators(r *ValidatorRegistry) {
	RegisterFileValidators(r)
	RegisterSyntaxValidators(r)
	RegisterExecutionValidators(r)
	RegisterCodeDOMValidators(r)
}

// RegisterFileValidators registers only file-related validators.
func RegisterFileValidators(r *ValidatorRegistry) {
	// Priority 5-10.
	r.Register(NewDirectoryValidator())
	r.Register(NewFileWriteValidator())
	r.Register(NewFileEditValidator())
	r.Register(NewFileDeleteValidator())
	// Priority 15 — surgical diff-based verification.
	r.Register(NewEnhancedEditValidator())
	// Priority 100 — final redundant check, zero false positives.
	r.Register(NewParanoidFileValidator())
}

// RegisterSyntaxValidators registers only syntax validators.
func RegisterSyntaxValidators(r *ValidatorRegistry) {
	// Priority 20.
	r.Register(NewSyntaxValidator())
	r.Register(NewMangleSyntaxValidator())
}

// RegisterExecutionValidators registers only execution validators.
func RegisterExecutionValidators(r *ValidatorRegistry) {
	// Priority 8-10.
	r.Register(NewBuildValidator())
	r.Register(NewTestValidator())
	r.Register(NewExecutionValidator())
}

// RegisterCodeDOMValidators registers only CodeDOM validators.
func RegisterCodeDOMValidators(r *ValidatorRegistry) {
	// Priority 15-25.
	r.Register(NewLineEditValidator())
	r.Register(NewCodeDOMValidator())
}
