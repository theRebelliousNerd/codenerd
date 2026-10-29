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

// WithWorkspace binds the project root whose specialist registry
// (.nerd/agents.json) the compiler's prompts advertise. The boot owner that
// resolved the workspace passes it; the compiler never falls back to the
// process working directory, which an interactive session started from a
// subdirectory does not share with its project root.
func WithWorkspace(root string) CompilerOption {
	return func(c *JITPromptCompiler) error {
		c.workspace = root
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
		// The sibling knob, and it was wired nowhere. SetVectorWeight had no
		// production caller and CompilerConfig.VectorSearchWeight had no
		// reader: two halves of one missing wire, which is why the selector
		// and the config each carried their own 0.3 with the same
		// "70% logic, 30% vector" comment attached.
		//
		// The zero is guarded because the two setters do NOT agree about what
		// one means. SetVectorSearchTimeout reads zero as "unset" and
		// substitutes ten seconds; SetVectorWeight clamps to [0,1] and takes a
		// zero literally, as pure logic. So wiring this unguarded would make a
		// partially-filled CompilerConfig silently turn vector scoring off --
		// the exact silent-failure shape this is being fixed to remove. Pure
		// logic is expressed by installing no vector searcher at all
		// (selector.go skips the search when vectorSearcher is nil).
		if config.VectorSearchWeight > 0 {
			c.selector.SetVectorWeight(config.VectorSearchWeight)
		}
		return nil
	}
}
