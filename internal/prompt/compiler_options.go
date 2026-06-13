package prompt

import (
	"database/sql"
)

// CompilerOption is a functional option for configuring the compiler.
type CompilerOption func(*JITPromptCompiler) error

// WithEmbeddedCorpus sets the embedded atom corpus.
func WithEmbeddedCorpus(corpus *EmbeddedCorpus) CompilerOption {
	return func(c *JITPromptCompiler) error {
		c.embeddedCorpus = corpus
		return nil
	}
}

// WithProjectDB sets the project-level atom database.
func WithProjectDB(db *sql.DB) CompilerOption {
	return func(c *JITPromptCompiler) error {
		c.projectDB = db
		return nil
	}
}

// WithKernel sets the Mangle kernel for rule-based selection.
func WithKernel(kernel KernelQuerier) CompilerOption {
	return func(c *JITPromptCompiler) error {
		c.kernel = kernel
		c.selector.SetKernel(kernel)
		return nil
	}
}

// WithVectorSearcher sets the vector searcher for semantic selection.
func WithVectorSearcher(vs VectorSearcher) CompilerOption {
	return func(c *JITPromptCompiler) error {
		c.vectorSearcher = vs
		c.selector.SetVectorSearcher(vs)
		return nil
	}
}

// WithConfig sets the compiler configuration.
func WithConfig(config CompilerConfig) CompilerOption {
	return func(c *JITPromptCompiler) error {
		c.config = config
		c.selector.SetVectorSearchTimeout(config.VectorSearchTimeout)
		return nil
	}
}

// WithDefaultTokenBudget sets the default token budget for prompt compilation.
// Use this to pass config.ContextWindow.MaxTokens from the application config.
func WithDefaultTokenBudget(budget int) CompilerOption {
	return func(c *JITPromptCompiler) error {
		if budget > 0 {
			c.config.DefaultTokenBudget = budget
		}
		return nil
	}
}

// WithConfigFactory sets the config factory for generating AgentConfigs.
func WithConfigFactory(factory *ConfigFactory) CompilerOption {
	return func(c *JITPromptCompiler) error {
		c.configFactory = factory
		return nil
	}
}

// Compile generates a system prompt for the given context.
