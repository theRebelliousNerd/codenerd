package prompt

import (
	"regexp"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"

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

// mangleExampleJob is one fenced block and where it sits in its atom.
type mangleExampleJob struct {
	atom    *PromptAtom
	ordinal int
	span    []int
}

// mangleExampleVerdict is one block's measurement: the engine's load result and
// whether a failure counts.
type mangleExampleVerdict struct {
	err     error // the engine could not be built
	loadErr error // the block did not load
	counted bool  // the failure is an unmarked program, not a query or wrong-way example
}

func measureMangleExample(job mangleExampleJob) mangleExampleVerdict {
	content := job.atom.Content
	body := content[job.span[2]:job.span[3]]
	engine, err := mangle.NewEngine(mangle.DefaultConfig(), nil)
	if err != nil {
		return mangleExampleVerdict{err: err}
	}
	loadErr := engine.LoadSchemaString(body)
	if loadErr == nil {
		return mangleExampleVerdict{}
	}
	lead := content[max(0, job.span[0]-200):job.span[0]]
	if wrongExampleMarker.MatchString(lead) || wrongExampleMarker.MatchString(body) {
		return mangleExampleVerdict{loadErr: loadErr}
	}
	if strings.Contains(body, "?") || !strings.Contains(body, ".") {
		return mangleExampleVerdict{loadErr: loadErr} // a query or a fragment, not a program
	}
	return mangleExampleVerdict{loadErr: loadErr, counted: true}
}

// CountMangleExamples measures the Mangle blocks in atoms. Each block loads
// through its own engine, so the blocks are measured concurrently (one worker
// per CPU); the count and the order of Failing are those of a sequential walk.
func CountMangleExamples(atoms []*PromptAtom) (MangleExampleCount, error) {
	var jobs []mangleExampleJob
	for _, atom := range atoms {
		if atom == nil {
			continue
		}
		for i, m := range mangleBlock.FindAllStringSubmatchIndex(atom.Content, -1) {
			jobs = append(jobs, mangleExampleJob{atom: atom, ordinal: i + 1, span: m})
		}
	}
	verdicts := make([]mangleExampleVerdict, len(jobs))
	var wg sync.WaitGroup
	var next atomic.Int64
	for w := 0; w < min(runtime.GOMAXPROCS(0), len(jobs)); w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				i := int(next.Add(1)) - 1
				if i >= len(jobs) {
					return
				}
				verdicts[i] = measureMangleExample(jobs[i])
			}
		}()
	}
	wg.Wait()

	var c MangleExampleCount
	for i, v := range verdicts {
		c.Total++
		if v.err != nil {
			return c, v.err
		}
		switch {
		case v.loadErr == nil:
			c.Loading++
		case v.counted:
			c.Failing = append(c.Failing, MangleExample{AtomID: jobs[i].atom.ID, Block: jobs[i].ordinal, Error: v.loadErr.Error()})
		}
	}
	return c, nil
}
