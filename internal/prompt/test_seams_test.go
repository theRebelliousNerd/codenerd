package prompt

import "context"

// Test seams: conveniences only tests call. They live here, not in
// production files, so the dead-code budget measures production code.

// collectKnowledgeAtoms queries project knowledge plus the selected expert's
// registered knowledge DB and converts hits to ephemeral PromptAtoms.
// This is the Semantic Knowledge Bridge with a bounded expert extension:
// the project LocalStore is searched semantically (with a lexical fallback
// when no embedding engine is configured) and the single selected expert DB
// is searched lexically through the shared handle. Sibling experts are never
// consulted. Missing knowledge reads as missing (nil); nothing is fabricated.
func (c *JITPromptCompiler) collectKnowledgeAtoms(ctx context.Context, cc *CompilationContext) []*PromptAtom {
	config, _, _ := c.configSnapshot()
	return c.collectKnowledgeAtomsFrom(ctx, cc, config, c.snapshotSources(cc), nil)
}

// collectLearningAtoms queries the LearningStore for relevant past learnings.
func (c *JITPromptCompiler) collectLearningAtoms(ctx context.Context, cc *CompilationContext) []*PromptAtom {
	config, _, _ := c.configSnapshot()
	return c.collectLearningAtomsFrom(ctx, cc, config, c.snapshotSources(cc), nil)
}

func (c *JITPromptCompiler) collectDynamicKernelSnapshot(kernel KernelQuerier, cc *CompilationContext) (atoms []*PromptAtom, fingerprint string, err error) {
	return c.collectDynamicKernelSnapshotWithConfig(kernel, cc, c.GetConfig())
}

func (c *JITPromptCompiler) collectKernelInjectedAtoms(cc *CompilationContext) ([]*PromptAtom, error) {
	return c.collectKernelInjectedAtomsFromConfig(c.kernel, cc, c.GetConfig())
}

func (c *JITPromptCompiler) collectKernelInjectedAtomsFrom(kernel KernelQuerier, cc *CompilationContext) ([]*PromptAtom, error) {
	return c.collectKernelInjectedAtomsFromConfig(kernel, cc, c.GetConfig())
}
