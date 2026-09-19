package main

import (
	"regexp"
	"strings"
	"testing"

	"codenerd/internal/mangle"
	"codenerd/internal/prompt"
)

// mangleBlock is a fenced Mangle example inside an atom's content.
var mangleBlock = regexp.MustCompile("(?s)```mangle\n(.*?)```")

// wrongExampleMarker recognises an example the atom presents as the wrong way
// to write something: its failure to load is the point.
var wrongExampleMarker = regexp.MustCompile(`(?i)(wrong|bad|incorrect|don't|do not|avoid|anti-?pattern|❌|error:|fails|invalid|broken|never write|mistake|hallucinat|unsafe|not valid)`)

// maxUnmarkedFailingExamples is a ratchet on the Mangle the prompt corpus
// teaches codeNERD's own agents. Measured 2026-09-18 on the pinned engine
// (every block loaded through a fresh engine, as `nerd check-mangle` does):
// 2,144 blocks, 284 loading, 888 failing without being marked as a wrong-way
// example. The corpus taught the Google 0.4.0 typed-variable declaration form
// (Decl p(X.Type<int>)) 2,295 times and the reducers capitalized (fn:Sum,
// fn:Count, ...) about 600 times; the pinned engine parses neither. After the
// rewrite: 2,102 blocks, 570 loading, 654 failing unmarked -- most of them
// examples whose facts or declarations live elsewhere in the atom, a query
// written as a clause, or a top-level `=`. The number may only go down: lower
// it in the change that fixes more of them.
const maxUnmarkedFailingExamples = 654

func TestAtomCorpus_MangleExamplesTheEngineRejectsDoNotGrow(t *testing.T) {
	corpus, err := prompt.LoadEmbeddedCorpus()
	if err != nil {
		t.Fatalf("LoadEmbeddedCorpus: %v", err)
	}
	failing, total, loading := 0, 0, 0
	for _, atom := range corpus.All() {
		content := atom.Content
		for _, m := range mangleBlock.FindAllStringSubmatchIndex(content, -1) {
			total++
			body := content[m[2]:m[3]]
			engine, err := mangle.NewEngine(mangle.DefaultConfig(), nil)
			if err != nil {
				t.Fatalf("NewEngine: %v", err)
			}
			if engine.LoadSchemaString(body) == nil {
				loading++
				continue
			}
			lead := content[max(0, m[0]-200):m[0]]
			if wrongExampleMarker.MatchString(lead) || wrongExampleMarker.MatchString(body) {
				continue
			}
			if strings.Contains(body, "?") || !strings.Contains(body, ".") {
				continue // a query or a fragment, not a program
			}
			failing++
		}
	}
	t.Logf("mangle examples: %d blocks, %d load on the pinned engine, %d fail unmarked (ceiling %d)", total, loading, failing, maxUnmarkedFailingExamples)
	if failing > maxUnmarkedFailingExamples {
		t.Errorf("%d Mangle examples in the prompt corpus fail to load on the pinned engine without being marked as wrong-way examples; the ceiling is %d. Fix the example (run it through `nerd check-mangle`) or mark it as the wrong way.", failing, maxUnmarkedFailingExamples)
	}
}
