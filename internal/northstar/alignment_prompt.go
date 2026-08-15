package northstar

import (
	"strings"
	"sync"
)

// =============================================================================
// ALIGNMENT PROMPT ATOMS
// =============================================================================
//
// buildAlignmentSystemPrompt used to inline its scaffolding as scattered
// sb.WriteString literals. That made the single prompt in codeNERD that decides
// whether work may proceed the one prompt nobody could inspect, diff, tune, or
// evolve, and it had already drifted from the northstar wizard atoms beside it.
//
// The scaffolding is now atoms, declared once in
// internal/prompt/atoms/northstar/guardian_alignment.yaml, which is what the
// JIT corpus ships and what a human edits. Project data (the vision itself)
// stays in Go because it comes from the knowledge store, not the corpus.
//
// The text below is the guardian's resolved copy of those atoms.
// TestAlignmentAtoms_ShouldMatchTheCorpusYAML parses the YAML and fails if the
// two ever diverge by a byte, so "atomized" is an enforced property rather than
// a claim. northstar deliberately does NOT import internal/prompt: it is a leaf
// package that the campaign gate and the CLI both depend on, and pulling the
// entire prompt/core/tools tree in here to read five strings would make the
// vision guardian un-buildable whenever any of those packages is mid-edit.
//
// A host that already has the corpus loaded can override the resolution with
// SetAlignmentAtomResolver, which is how a running session serves evolved atoms
// to the guardian without this package growing a dependency.

const (
	atomGuardianRole             = "northstar/guardian/role"
	atomGuardianModuleRefinement = "northstar/guardian/module_refinement"
	atomGuardianTask             = "northstar/guardian/task"
	atomGuardianOutputContract   = "northstar/guardian/output_contract"
	atomGuardianUserInstruction  = "northstar/guardian/user_instruction"
)

// alignmentAtomText mirrors guardian_alignment.yaml exactly (content blocks,
// trailing whitespace trimmed). Do not edit one without the other; the parity
// test enforces it.
var alignmentAtomText = map[string]string{
	atomGuardianRole: `You are the Northstar Alignment Guardian for a software project.

You judge one thing: whether the subject you are shown moves this project
toward its stated vision, away from it, or nowhere. You are not a code
reviewer, a style critic, or a planner. Correct code that serves the wrong
goal is misaligned; rough code that serves the right goal is aligned.`,

	atomGuardianModuleRefinement: `When a module northstar is shown alongside the project vision, the module
REFINES the project vision - it never replaces or overrides it. Work that
satisfies a module's stated purpose while contradicting the project mission
is misaligned. Judge against both, and let the stricter one decide.`,

	atomGuardianTask: `## Your Task
Evaluate whether the given subject/change aligns with this vision.

Weigh, in this order:
1. Does it serve the mission and the stated problem?
2. Does it serve a declared persona's actual need?
3. Does it advance a declared capability or requirement, or does it add
   scope that no capability or requirement asked for?
4. Does it respect every constraint, and does it avoid or mitigate the
   declared risks?

Score 0.0-1.0. Reserve "blocked" for work that actively contradicts the
mission or violates a stated constraint - not for work that is merely
unrelated to it.`,

	atomGuardianOutputContract: `Respond in this EXACT format:
SCORE: <0.0-1.0>
RESULT: <passed|warning|failed|blocked>
EXPLANATION: <one sentence explanation>
SUGGESTIONS: <comma-separated suggestions, or 'none'>`,

	atomGuardianUserInstruction: `Evaluate alignment with the project vision.`,
}

// AlignmentAtomResolver resolves an atom ID to its content. Returning false
// falls back to the built-in copy.
type AlignmentAtomResolver func(id string) (string, bool)

var (
	alignmentResolverMu sync.RWMutex
	alignmentResolver   AlignmentAtomResolver
)

// SetAlignmentAtomResolver installs a host-provided atom source (typically the
// live prompt corpus, so evolved atoms reach the guardian). Passing nil
// restores the built-in copies.
func SetAlignmentAtomResolver(r AlignmentAtomResolver) {
	alignmentResolverMu.Lock()
	defer alignmentResolverMu.Unlock()
	alignmentResolver = r
}

// AlignmentAtom returns the content of a guardian alignment atom.
func AlignmentAtom(id string) string {
	alignmentResolverMu.RLock()
	resolver := alignmentResolver
	alignmentResolverMu.RUnlock()

	if resolver != nil {
		if text, ok := resolver(id); ok && strings.TrimSpace(text) != "" {
			return strings.TrimSpace(text)
		}
	}
	return alignmentAtomText[id]
}

// AlignmentAtomIDs lists the atoms the Guardian composes its prompt from, in
// composition order.
func AlignmentAtomIDs() []string {
	return []string{
		atomGuardianRole,
		atomGuardianModuleRefinement,
		atomGuardianTask,
		atomGuardianOutputContract,
		atomGuardianUserInstruction,
	}
}
