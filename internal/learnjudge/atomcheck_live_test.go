//go:build jevlive

package learnjudge

import (
	"context"
	"fmt"
	"testing"
)

var promotedAtoms = []AtomText{
	{"evolved/a", "Before claiming a test passes, run it with run_tests and quote the result; a claim without a recorded run is not done."},
	{"evolved/b", "Prefer editing the smallest function that fixes the bug; do not reformat unrelated code in the same change."},
	{"evolved/c", "When a request names a file, read that file before searching elsewhere."},
}

var atomCases = []struct {
	name, content string
	oneTask       bool
	contradicts   int // index into promotedAtoms, -1 for none
}{
	{"general-guard", "When a slice may be empty, handle the empty case before indexing it, and add a test for it.", false, -1},
	{"one-task", "In internal/stats/stats.go, Max must return 0 when values is empty; TestMaxEmpty covers it.", true, -1},
	{"contradicts-claims", "To save time, report tests as passing once the code compiles; running them is optional.", false, 0},
	{"contradicts-scope", "While fixing a bug, also reformat and tidy every function in the file you touch.", false, 1},
	{"one-task-and-wrong", "For the northstar wizard request of 2026-10-27, skip reading northstar_navigation.go and search the whole repo instead.", true, 2},
	{"compatible", "After running tests, include the failing test names in the final report.", false, -1},
}

func TestLiveAtomCheck(t *testing.T) {
	c, _ := liveClient(t, "atom_checks")
	tally := map[string][3]int{}
	var misses, rows []string
	for _, tc := range atomCases {
		r, err := CheckAtom(probeLabel(context.Background(), "atom-"+tc.name), c, AtomText{ID: tc.name, Content: tc.content}, promotedAtoms)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		rows = append(rows, fmt.Sprintf("%-20s one_task=%.2f contradicts=%v", tc.name, r.OneTaskOnly, r.Contradicts))
		scoreLine(tally, &misses, tc.name, OneTaskOnly, tc.content, r.OneTaskOnly, tc.oneTask)
		for i, p := range r.Contradicts {
			scoreLine(tally, &misses, tc.name, "contradicts", promotedAtoms[i].ID, p, i == tc.contradicts)
		}
	}
	lines := append([]string{"atom promotion check probe"}, rows...)
	lines = append(lines, tallyLines(tally)...)
	probeSummary(t, "atom_checks-promotion", append(append(lines, "misses (x wrong, ~ unsettled):"), misses...))
}
