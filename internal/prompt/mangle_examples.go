package prompt

import (
	"regexp"
	"strings"

	"codenerd/internal/mangle"
)

// The Mangle the prompt corpus teaches codeNERD's own agents, measured: every
// fenced ```mangle block loaded through a fresh engine, as `nerd check-mangle`
// loads a file. A block that fails to load is counted unless it is presented
// as the wrong way to write something, or is a query or a fragment rather than
// a program. The count is the ladder's error-free gate G8; its ratchet is
// TestAtomCorpus_MangleExamplesTheEngineRejectsDoNotGrow
// (cmd/tools/validate_prompt_atoms), and `nerd gates measure` reports each
// failing block as a finding.

// mangleBlock is a fenced Mangle example inside an atom's content.
var mangleBlock = regexp.MustCompile("(?s)```mangle\n(.*?)```")

// wrongExampleMarker recognises an example the atom presents as the wrong way
// to write something: its failure to load is the point.
var wrongExampleMarker = regexp.MustCompile(`(?i)(wrong|bad|incorrect|don't|do not|avoid|anti-?pattern|❌|error:|fails|invalid|broken|never write|mistake|hallucinat|unsafe|not valid)`)

// MangleExample is one failing, unmarked example: the atom it is in, its
// ordinal among that atom's Mangle blocks (from 1), and the engine's error.
type MangleExample struct {
	AtomID string
	Block  int
	Error  string
}

// MangleExampleCount is the corpus's Mangle examples measured.
type MangleExampleCount struct {
	Total, Loading int
	// Failing are the unmarked examples that do not load.
	Failing []MangleExample
}

// CountMangleExamples measures the Mangle blocks in atoms.
func CountMangleExamples(atoms []*PromptAtom) (MangleExampleCount, error) {
	var c MangleExampleCount
	for _, atom := range atoms {
		if atom == nil {
			continue
		}
		content := atom.Content
		for i, m := range mangleBlock.FindAllStringSubmatchIndex(content, -1) {
			c.Total++
			body := content[m[2]:m[3]]
			engine, err := mangle.NewEngine(mangle.DefaultConfig(), nil)
			if err != nil {
				return c, err
			}
			loadErr := engine.LoadSchemaString(body)
			if loadErr == nil {
				c.Loading++
				continue
			}
			lead := content[max(0, m[0]-200):m[0]]
			if wrongExampleMarker.MatchString(lead) || wrongExampleMarker.MatchString(body) {
				continue
			}
			if strings.Contains(body, "?") || !strings.Contains(body, ".") {
				continue // a query or a fragment, not a program
			}
			c.Failing = append(c.Failing, MangleExample{AtomID: atom.ID, Block: i + 1, Error: loadErr.Error()})
		}
	}
	return c, nil
}
