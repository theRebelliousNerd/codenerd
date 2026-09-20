package world

import (
	"path/filepath"

	"codenerd/internal/core"
	"codenerd/internal/types"
)

// CodeElementFacts parses one source file into the CodeDOM fact layer:
// code_element and the element_signature / element_visibility / element_parent
// / code_interactable that hang off it.
//
// The ownership matrix in world_predicates.go gives those predicates to "the
// CodeDOM scope (session)" -- what is being looked at, not what is on disk --
// and the only thing that ever populated them was VirtualStore.handleOpenFile
// behind a kernel-derived next_action(/open_file). A headless run never
// dispatches that, so the layer was empty by construction: ladder run R1-16
// (2026-09-19) asked code_element 1,175 times and got no row, while
// code_defines answered 950 of 1,216 from the same kernel.
//
// This is the same projection FileScope.ScopeFacts emits, without the session
// scope around it, so a run can put the file it is working on into the layer
// directly.
type CodeElementFacts struct {
	parser *CodeElementParser
	root   string
}

// NewCodeElementFacts builds a parser rooted at the workspace, so refs and
// paths are the workspace-canonical ones the rest of the world model uses.
func NewCodeElementFacts(root string) *CodeElementFacts {
	return &CodeElementFacts{parser: NewCodeElementParserWithRoot(root), root: root}
}

// FileFacts returns path's CodeDOM facts with the file argument relabelled to
// the workspace-canonical identity every other world predicate is keyed by --
// the parser reports the absolute path it read, and a fact keyed by an
// absolute path answers no query the working context asks.
//
// A file the parser cannot read or finds no elements in yields no facts and no
// error: the scope layer is an addition to what a turn can reason over, never
// a condition on the turn proceeding.
func (c *CodeElementFacts) FileFacts(path string) ([]types.Fact, error) {
	if c == nil || c.parser == nil {
		return nil, nil
	}
	abs := path
	if !filepath.IsAbs(abs) {
		abs = filepath.Join(c.root, filepath.FromSlash(path))
	}
	elements, err := c.parser.ParseFile(abs)
	if err != nil || len(elements) == 0 {
		return nil, err
	}
	canonical := types.CanonicalPath(c.root, abs)
	var facts []core.Fact
	for i := range elements {
		facts = append(facts, types.RelabelPathArgs(elements[i].ToFacts(), elements[i].File, canonical)...)
	}
	return facts, nil
}
