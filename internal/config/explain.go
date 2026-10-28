package config

import "fmt"

// ExplainConfig is the `explain` section of .nerd/config.json: the bounds on
// a "why was this derived?" answer (core.RealKernel.ExplainEnvelope, /explain).
// Recording a fixpoint is capped by the recorder's per-pass reset; these cap
// what is EXTRACTED from it, so an answer about a fact with a wide derivation
// cannot dump the whole proof graph into a turn.
type ExplainConfig struct {
	// MaxProofs is how many alternative proofs of the goal are built.
	MaxProofs int `json:"max_proofs,omitempty"`
	// MaxDepth is how deep a proof is expanded before a sub-proof is marked
	// partial (depth limit).
	MaxDepth int `json:"max_depth,omitempty"`
	// MaxNodes caps the distinct proof nodes an answer carries.
	MaxNodes int `json:"max_nodes,omitempty"`
	// MaxEdges caps the premise edges an answer carries.
	MaxEdges int `json:"max_edges,omitempty"`
	// MaxBytes caps the rendered size of the proof facts an answer carries.
	MaxBytes int `json:"max_bytes,omitempty"`
}

// DefaultExplainConfig is the explain section with every field written down.
// 3 proofs at depth 32 is what /explain asked for before the section existed.
func DefaultExplainConfig() ExplainConfig {
	return ExplainConfig{MaxProofs: 3, MaxDepth: 32, MaxNodes: 400, MaxEdges: 800, MaxBytes: 64 << 10}
}

// GetExplainConfig returns the explain section with every absent field
// defaulted. A nil receiver is the defaults.
func (c *UserConfig) GetExplainConfig() ExplainConfig {
	if c == nil || c.Explain == nil {
		return DefaultExplainConfig()
	}
	return c.Explain.WithDefaults()
}

// WithDefaults fills every absent field from DefaultExplainConfig.
func (c ExplainConfig) WithDefaults() ExplainConfig {
	d := DefaultExplainConfig()
	if c.MaxProofs == 0 {
		c.MaxProofs = d.MaxProofs
	}
	if c.MaxDepth == 0 {
		c.MaxDepth = d.MaxDepth
	}
	if c.MaxNodes == 0 {
		c.MaxNodes = d.MaxNodes
	}
	if c.MaxEdges == 0 {
		c.MaxEdges = d.MaxEdges
	}
	if c.MaxBytes == 0 {
		c.MaxBytes = d.MaxBytes
	}
	return c
}

// Check reports the contradictions in an explain section, addressed under
// prefix ("explain").
func (c ExplainConfig) Check(prefix string) []Problem {
	c = c.WithDefaults()
	var out []Problem
	for _, f := range []struct {
		key string
		v   int
	}{
		{"max_proofs", c.MaxProofs},
		{"max_depth", c.MaxDepth},
		{"max_nodes", c.MaxNodes},
		{"max_edges", c.MaxEdges},
		{"max_bytes", c.MaxBytes},
	} {
		if f.v < 1 {
			out = append(out, Problem{
				Severity: SeverityError,
				Path:     prefix + "." + f.key,
				Message:  fmt.Sprintf("%d is not a positive bound", f.v),
				Fix:      "a positive integer, or remove the key for the default",
			})
		}
	}
	if c.MaxNodes > 0 && c.MaxProofs > c.MaxNodes {
		out = append(out, Problem{
			Severity: SeverityError,
			Path:     prefix + ".max_proofs",
			Message:  fmt.Sprintf("%d proofs cannot fit in max_nodes %d", c.MaxProofs, c.MaxNodes),
			Fix:      "raise max_nodes or lower max_proofs",
		})
	}
	return out
}
