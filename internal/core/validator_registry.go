package core

// RegisterAllValidators registers all standard validators with the registry.
// Call this during VirtualStore initialization.
func RegisterAllValidators(r *ValidatorRegistry) {
	// File validators (priority 5-10)
	r.Register(NewDirectoryValidator())
	r.Register(NewFileWriteValidator())
	r.Register(NewFileEditValidator())
	r.Register(NewFileDeleteValidator())

	// Syntax validators (priority 20)
	r.Register(NewSyntaxValidator())
	r.Register(NewMangleSyntaxValidator())

	// Execution validators (priority 8-10)
	r.Register(NewBuildValidator())
	r.Register(NewTestValidator())
	r.Register(NewExecutionValidator())

	// CodeDOM validators (priority 15-25)
	r.Register(NewLineEditValidator())
	r.Register(NewCodeDOMValidator())
}

// RegisterFileValidators registers only file-related validators.
func RegisterFileValidators(r *ValidatorRegistry) {
	r.Register(NewDirectoryValidator())
	r.Register(NewFileWriteValidator())
	r.Register(NewFileEditValidator())
	r.Register(NewFileDeleteValidator())
}

// RegisterSyntaxValidators registers only syntax validators.
func RegisterSyntaxValidators(r *ValidatorRegistry) {
	r.Register(NewSyntaxValidator())
	r.Register(NewMangleSyntaxValidator())
}

// RegisterExecutionValidators registers only execution validators.
func RegisterExecutionValidators(r *ValidatorRegistry) {
	r.Register(NewBuildValidator())
	r.Register(NewTestValidator())
	r.Register(NewExecutionValidator())
}

// RegisterCodeDOMValidators registers only CodeDOM validators.
func RegisterCodeDOMValidators(r *ValidatorRegistry) {
	r.Register(NewLineEditValidator())
	r.Register(NewCodeDOMValidator())
}
