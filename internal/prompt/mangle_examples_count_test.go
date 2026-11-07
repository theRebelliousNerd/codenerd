package prompt

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

// sequentialMangleExampleCount is the reference walk CountMangleExamples must
// agree with, block for block, however many workers measure the blocks.
func sequentialMangleExampleCount(atoms []*PromptAtom) MangleExampleCount {
	var c MangleExampleCount
	for _, atom := range atoms {
		if atom == nil {
			continue
		}
		for i, m := range mangleBlock.FindAllStringSubmatchIndex(atom.Content, -1) {
			c.Total++
			v := measureMangleExample(mangleExampleJob{atom: atom, ordinal: i + 1, span: m})
			switch {
			case v.loadErr == nil:
				c.Loading++
			case v.counted:
				c.Failing = append(c.Failing, MangleExample{AtomID: atom.ID, Block: i + 1, Error: v.loadErr.Error()})
			}
		}
	}
	return c
}

// Concurrent measurement keeps the sequential walk's totals and keeps Failing
// in atom-then-block order, so a ratchet diff and a gate finding list stay
// stable run to run. The corpus mixes loading blocks, an unmarked program the
// engine rejects, a wrong-way example, a query, a fragment and a nil atom.
func TestCountMangleExamples_ConcurrentMeasureKeepsTheSequentialCountAndOrder(t *testing.T) {
	fence := func(body string) string { return "```mangle\n" + body + "\n```\n" }
	var atoms []*PromptAtom
	for i := 0; i < 40; i++ {
		var content strings.Builder
		content.WriteString("Teaching text.\n")
		content.WriteString(fence(fmt.Sprintf("Decl ok_%d(X).\nok_%d(1).", i, i)))
		content.WriteString(fence(fmt.Sprintf("load_me_%d(X :- .", i))) // unmarked program that fails to load
		content.WriteString("This is the wrong way to write it:\n")
		content.WriteString(fence(fmt.Sprintf("wrong_%d(X :- .", i))) // marked: its failure is the point
		content.WriteString(fence(fmt.Sprintf("?query_%d(X)", i)))    // a query, not a program
		atoms = append(atoms, &PromptAtom{ID: fmt.Sprintf("atom/%02d", i), Content: content.String()})
		if i%7 == 0 {
			atoms = append(atoms, nil)
		}
	}

	got, err := CountMangleExamples(atoms)
	if err != nil {
		t.Fatalf("CountMangleExamples: %v", err)
	}
	want := sequentialMangleExampleCount(atoms)
	if want.Total != 160 || len(want.Failing) != 40 {
		t.Fatalf("fixture drifted: reference counted %d blocks, %d failing", want.Total, len(want.Failing))
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("concurrent count differs from the sequential walk:\n got  %+v\n want %+v", got, want)
	}
	for i, f := range got.Failing {
		if f.AtomID != fmt.Sprintf("atom/%02d", i) || f.Block != 2 {
			t.Fatalf("Failing[%d] = %+v, want atom/%02d block 2 in corpus order", i, f, i)
		}
	}
}
