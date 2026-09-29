package config

// JITConfig configures the JIT Prompt Compiler.
// The JIT compiler dynamically assembles system prompts from YAML atoms
// based on the current context (operational mode, shard type, language, etc.).
type JITConfig struct {
	// Enabled controls whether JIT compilation is used (default: true)
	// When false, falls back to static prompts
	Enabled bool `yaml:"enabled" json:"enabled"`

	// FallbackEnabled allows fallback to static prompts on JIT failure (default: true)
	FallbackEnabled bool `yaml:"fallback_enabled" json:"fallback_enabled"`

	// TokenBudget is the maximum tokens for compiled prompts (default: 200000)
	// Can be overridden via ContextWindow.MaxTokens in config.json
	TokenBudget int `yaml:"token_budget" json:"token_budget"`

	// ReservedTokens is tokens reserved for response generation (default: 8000)
	ReservedTokens int `yaml:"reserved_tokens" json:"reserved_tokens"`

	// ReservedTokensFallbackRatio is the ratio used to calculate reserved tokens when budget is exceeded (default: 10)
	ReservedTokensFallbackRatio int `yaml:"reserved_tokens_fallback_ratio" json:"reserved_tokens_fallback_ratio"`

	// DebugMode enables verbose JIT logging (default: false)
	DebugMode bool `yaml:"debug_mode" json:"debug_mode"`

	// TraceLLMIO logs full JIT prompts and LLM I/O when enabled (default: false)
	TraceLLMIO bool `yaml:"trace_llm_io" json:"trace_llm_io"`

	// SemanticTopK is the number of semantic search results to consider (default: 20)
	SemanticTopK int `yaml:"semantic_top_k" json:"semantic_top_k"`

	// KernelContextRows caps injectable_context rows merged into the single
	// kernel-context atom (default: 60). Past the cap the block keeps the
	// first rows and appends a "first N of M" marker, so the loss is visible.
	KernelContextRows int `yaml:"kernel_context_rows" json:"kernel_context_rows"`

	// KernelContextRowChars caps one injectable_context row (default: 1024).
	// A row is a single declarative sentence by contract.
	KernelContextRowChars int `yaml:"kernel_context_row_chars" json:"kernel_context_row_chars"`

	// KernelInjectedAtomChars is the ceiling on either merged kernel-injected
	// atom (default: 16384, ~4k tokens at 4 chars/token). Both atoms are
	// mandatory, so without this ceiling one merged block could outrank the
	// whole optional corpus or be rejected wholesale by budget fitting.
	KernelInjectedAtomChars int `yaml:"kernel_injected_atom_chars" json:"kernel_injected_atom_chars"`

	// SpecialistKnowledgeBlocks caps specialist_knowledge topics merged into
	// one atom (default: 12).
	SpecialistKnowledgeBlocks int `yaml:"specialist_knowledge_blocks" json:"specialist_knowledge_blocks"`

	// SpecialistTopicChars caps a specialist_knowledge topic heading (default: 200)
	SpecialistTopicChars int `yaml:"specialist_topic_chars" json:"specialist_topic_chars"`

	// SpecialistBlockChars caps one specialist_knowledge body so a single
	// verbose expert cannot crowd out its siblings inside the shared ceiling
	// (default: 4096).
	SpecialistBlockChars int `yaml:"specialist_block_chars" json:"specialist_block_chars"`

	// PredicateLimit caps predicates injected into a prompt by the JIT
	// predicate selector (default: 100).
	PredicateLimit int `yaml:"predicate_limit" json:"predicate_limit"`

	// PredicateVecLimit caps vector candidates merged into predicate
	// selection (default: 200).
	PredicateVecLimit int `yaml:"predicate_vec_limit" json:"predicate_vec_limit"`

	// FallbackIdentityMaxBytes bounds the fallback identity prompt built when
	// JIT compilation fails (default: 1048576). OOM protection: a cut keeps
	// the head on a rune boundary and appends a truncation marker, so the
	// model sees the cut instead of a silently short identity.
	FallbackIdentityMaxBytes int `yaml:"fallback_identity_max_bytes" json:"fallback_identity_max_bytes"`

	enabledSet         bool
	fallbackEnabledSet bool
}

// UnmarshalJSON tracks which boolean fields were explicitly set so defaults can apply.
func (c *JITConfig) UnmarshalJSON(data []byte) error {
	type alias JITConfig
	aux := struct {
		Enabled         *bool `json:"enabled"`
		FallbackEnabled *bool `json:"fallback_enabled"`
		*alias
	}{
		alias: (*alias)(c),
	}
	if err := decodeStrictJSON(data, &aux); err != nil {
		return err
	}
	if aux.Enabled != nil {
		c.Enabled = *aux.Enabled
		c.enabledSet = true
	}
	if aux.FallbackEnabled != nil {
		c.FallbackEnabled = *aux.FallbackEnabled
		c.fallbackEnabledSet = true
	}
	return nil
}

// DefaultJITConfig returns sensible defaults for JIT compilation.
// Note: TokenBudget should be overridden from config.ContextWindow.MaxTokens if available.
func DefaultJITConfig() JITConfig {
	return JITConfig{
		Enabled:                     true,
		FallbackEnabled:             true,
		TokenBudget:                 200000, // 200k tokens default
		ReservedTokens:              8000,
		ReservedTokensFallbackRatio: 10,
		DebugMode:                   false,
		TraceLLMIO:                  false,
		SemanticTopK:                20,
		KernelContextRows:           60,
		KernelContextRowChars:       1024,
		KernelInjectedAtomChars:     16 * 1024,
		SpecialistKnowledgeBlocks:   12,
		SpecialistTopicChars:        200,
		SpecialistBlockChars:        4 * 1024,
		PredicateLimit:              100,
		PredicateVecLimit:           200,
		FallbackIdentityMaxBytes:    1024 * 1024,
	}
}
